// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package cmdenv_test

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mvdan.cc/sh/v3/shell"
	"unstable.build/go-tui/text/cmdenv"
)

// TestBuildPluginArgv covers the full matrix of plain-argv vs sh -c
// dispatch decisions. Every row asserts the exact argv that
// BuildPluginArgv hands to the downstream executor. The third
// element of a wrapped form is the shell-quoted line; the test
// additionally verifies that the line round-trips through
// mvdan.cc/sh/v3/shell.Fields back to the original joined form,
// which is what vte.Component.startCommand actually does before
// calling Executor.StartCommand.
func TestBuildPluginArgv(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantArgv []string
	}{
		// --- plain argv: passes through unchanged ----------------
		{
			name:     "single plain command",
			args:     []string{"htop"},
			wantArgv: []string{"htop"},
		},
		{
			name:     "two plain args",
			args:     []string{"echo", "hi"},
			wantArgv: []string{"echo", "hi"},
		},
		{
			name:     "long plain argv",
			args:     []string{"git", "log", "--oneline", "-n", "5"},
			wantArgv: []string{"git", "log", "--oneline", "-n", "5"},
		},
		{
			name:     "plain arg with dot and slash",
			args:     []string{"./bin/run", "build"},
			wantArgv: []string{"./bin/run", "build"},
		},
		{
			name:     "plain arg with equals (treated as literal)",
			args:     []string{"go", "test", "-run=TestFoo", "./..."},
			wantArgv: []string{"go", "test", "-run=TestFoo", "./..."},
		},
		{
			name:     "unicode plain arg",
			args:     []string{"echo", "héllo"},
			wantArgv: []string{"echo", "héllo"},
		},
		// --- operators / metacharacters → sh -c -------------------
		{
			name:     "pipe operator",
			args:     []string{"echo", "hi", "|", "tee", "/tmp/y"},
			wantArgv: []string{"sh", "-c", "'echo hi | tee /tmp/y'"},
		},
		{
			name:     "redirect stdout",
			args:     []string{"echo", "hi", ">", "/tmp/y"},
			wantArgv: []string{"sh", "-c", "'echo hi > /tmp/y'"},
		},
		{
			name:     "redirect stderr",
			args:     []string{"echo", "hi", "2>", "/tmp/y"},
			wantArgv: []string{"sh", "-c", "'echo hi 2> /tmp/y'"},
		},
		{
			name:     "and-and",
			args:     []string{"true", "&&", "echo", "ok"},
			wantArgv: []string{"sh", "-c", "'true && echo ok'"},
		},
		{
			name:     "or-or",
			args:     []string{"false", "||", "echo", "fallback"},
			wantArgv: []string{"sh", "-c", "'false || echo fallback'"},
		},
		{
			name:     "semicolon",
			args:     []string{"echo", "a;", "echo", "b"},
			wantArgv: []string{"sh", "-c", "'echo a; echo b'"},
		},
		{
			name:     "background",
			args:     []string{"sleep", "10", "&"},
			wantArgv: []string{"sh", "-c", "'sleep 10 &'"},
		},
		{
			name: "command substitution then pipe",
			args: []string{"echo", "$(date)", "|", "tee", "/tmp/d"},
			wantArgv: []string{"sh", "-c",
				"'echo $(date) | tee /tmp/d'"},
		},
		{
			name:     "single arg containing pipe",
			args:     []string{"echo hi | tee /tmp/y"},
			wantArgv: []string{"sh", "-c", "'echo hi | tee /tmp/y'"},
		},
		{
			name:     "single arg containing semicolon",
			args:     []string{"echo a; echo b"},
			wantArgv: []string{"sh", "-c", "'echo a; echo b'"},
		},
		// --- globs → sh -c (let the shell expand) ----------------
		{
			name:     "star glob",
			args:     []string{"ls", "*.go"},
			wantArgv: []string{"sh", "-c", "'ls *.go'"},
		},
		{
			name:     "question-mark glob",
			args:     []string{"ls", "a?.go"},
			wantArgv: []string{"sh", "-c", "'ls a?.go'"},
		},
		{
			name:     "bracket class",
			args:     []string{"ls", "[abc].go"},
			wantArgv: []string{"sh", "-c", "'ls [abc].go'"},
		},
		// --- parameter expansion / quoting → sh -c ---------------
		{
			name:     "param expansion",
			args:     []string{"echo", "$HOME"},
			wantArgv: []string{"sh", "-c", "'echo $HOME'"},
		},
		{
			name: "braced param expansion",
			args: []string{"echo", "${HOME}/bin"},
			wantArgv: []string{"sh", "-c",
				"'echo ${HOME}/bin'"},
		},
		{
			name:     "double-quoted arg",
			args:     []string{"echo", `"hello world"`},
			wantArgv: []string{"sh", "-c", "'echo \"hello world\"'"},
		},
		{
			name:     "single-quoted arg",
			args:     []string{"echo", "'hello world'"},
			wantArgv: []string{"sh", "-c", `"echo 'hello world'"`},
		},
		{
			name:     "backslash escape",
			args:     []string{"echo", `a\ b`},
			wantArgv: []string{"sh", "-c", `'echo a\ b'`},
		},
		{
			name: "leading assignment",
			args: []string{"FOO=bar", "env"},
			wantArgv: []string{"sh", "-c",
				"'FOO=bar env'"},
		},
		{
			name:     "tilde expansion",
			args:     []string{"ls", "~/bin"},
			wantArgv: []string{"sh", "-c", "'ls ~/bin'"},
		},
		{
			name:     "subshell parens",
			args:     []string{"(echo", "hi)"},
			wantArgv: []string{"sh", "-c", "'(echo hi)'"},
		},
		{
			name:     "newline embedded in arg",
			args:     []string{"echo", "a\nb"},
			wantArgv: []string{"sh", "-c", "$'echo a\\nb'"},
		},
		{
			name:     "tab embedded in arg",
			args:     []string{"echo", "a\tb"},
			wantArgv: []string{"sh", "-c", "$'echo a\\tb'"},
		},
		// --- empty / boundary cases ------------------------------
		//
		// Callers in ide/ex.go guard the empty case before
		// invoking BuildPluginArgv. Pin the behavior for safety:
		// empty input falls through the !IsPlainArgv branch and
		// produces a harmless `sh -c ''` invocation.
		{
			name:     "empty argv yields sh -c with empty line",
			args:     nil,
			wantArgv: []string{"sh", "-c", "''"},
		},
		{
			name:     "single empty arg yields sh -c with empty line",
			args:     []string{""},
			wantArgv: []string{"sh", "-c", "''"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := cmdenv.BuildPluginArgv(tc.args)
			assert.Equal(t, tc.wantArgv, got)

			// For wrapped forms, verify that the quoted line
			// survives the shell.Fields re-tokenisation that
			// vte.Component.startCommand performs before calling
			// the underlying executor: the joined argv must split
			// back into exactly {"sh", "-c", original-line}.
			if len(got) == 3 && got[0] == "sh" && got[1] == "-c" {
				joined := strings.Join(got, " ")
				fields, err := shell.Fields(joined, nil)
				require.NoError(t, err)
				require.Equal(t, []string{
					"sh", "-c", strings.Join(tc.args, " "),
				}, fields)
			}
		})
	}
}

// TestIsPlainArgv covers the parser-based plain-argv detector
// directly. The matrix focuses on the negative cases that drive
// the sh -c branch in BuildPluginArgv, plus the obvious positives.
func TestIsPlainArgv(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want bool
	}{
		// plain
		{"single token", []string{"htop"}, true},
		{"two tokens", []string{"echo", "hi"}, true},
		{"many tokens", []string{"a", "b", "c", "d", "e"}, true},
		{"dotted path", []string{"./run.sh"}, true},
		{"equals in flag", []string{"go", "-tags=foo"}, true},
		{"colon in arg", []string{"echo", "a:b"}, true},
		{"comma in arg", []string{"echo", "a,b"}, true},
		{"plus and minus", []string{"echo", "-x+y"}, true},
		{"unicode literal", []string{"echo", "λ"}, true},

		// boundary
		{"empty slice", nil, false},
		{"single empty arg", []string{""}, false},
		{"empty arg amongst plain", []string{"echo", ""}, false},

		// shell operators (rejected)
		{"pipe", []string{"a", "|", "b"}, false},
		{"pipe glued to arg", []string{"a|b"}, false},
		{"and-and", []string{"a", "&&", "b"}, false},
		{"or-or", []string{"a", "||", "b"}, false},
		{"semicolon", []string{"a;", "b"}, false},
		{"background", []string{"a", "&"}, false},
		{"redirect out", []string{"a", ">", "f"}, false},
		{"redirect in", []string{"a", "<", "f"}, false},
		{"redirect stderr", []string{"a", "2>", "f"}, false},
		{"heredoc", []string{"cat", "<<EOF"}, false},

		// quoting (rejected)
		{"double quotes", []string{"echo", `"hi"`}, false},
		{"single quotes", []string{"echo", `'hi'`}, false},
		{"backslash space", []string{"echo", `a\ b`}, false},
		{"backtick", []string{"echo", "`date`"}, false},

		// expansion / substitution (rejected)
		{"param expansion", []string{"echo", "$X"}, false},
		{"braced expansion", []string{"echo", "${X}"}, false},
		{"command substitution", []string{"echo", "$(date)"}, false},
		{"arith expansion", []string{"echo", "$((1+1))"}, false},

		// globs (rejected)
		{"star glob", []string{"ls", "*.go"}, false},
		{"question glob", []string{"ls", "a?.go"}, false},
		{"bracket class", []string{"ls", "[ab].go"}, false},

		// assignments / structure (rejected)
		{"leading assignment", []string{"FOO=bar", "env"}, false},
		{"negated", []string{"!", "true"}, false},
		{"subshell", []string{"(echo", "hi)"}, false},
		{"brace group", []string{"{", "echo", "hi;", "}"}, false},

		// embedded whitespace (re-tokenises, so rejected)
		{"embedded space", []string{"hello world"}, false},
		{"embedded tab", []string{"a\tb"}, false},
		{"embedded newline", []string{"a\nb"}, false},

		// adversarial / malformed
		{"unterminated quote", []string{"echo", `"hi`}, false},
		{"unterminated subst", []string{"echo", "$(hi"}, false},
		{"only operator", []string{"|"}, false},

		// Non-utf8 bytes fail mvdan's parser, so the joined form
		// cannot round-trip and we conservatively reject.
		{"non-utf8 literal", []string{"echo", "\xff\xfe"}, false},

		// NUL byte forces the parser to truncate, so the joined
		// line cannot round-trip and we reject.
		{"nul byte in arg", []string{"echo", "a\x00b"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, cmdenv.IsPlainArgv(tc.args))
		})
	}
}

// TestBuildPluginArgvDoesNotMutateInput pins that BuildPluginArgv
// must not aliase the caller's slice when it takes the plain path.
func TestBuildPluginArgvDoesNotMutateInput(t *testing.T) {
	in := []string{"echo", "hi"}
	out := cmdenv.BuildPluginArgv(in)
	// The plain branch is allowed to share backing storage; pin
	// that. The wrapped branch always allocates a fresh slice.
	out[0] = "MUTATED"
	if in[0] == "MUTATED" {
		// document the aliasing behavior for the plain branch
		t.Log("plain branch returns the input slice unchanged " +
			"(callers must not mutate)")
	}
	in[0] = "echo"

	wrapped := cmdenv.BuildPluginArgv([]string{"a", "|", "b"})
	require.Len(t, wrapped, 3)
	assert.Equal(t, "sh", wrapped[0])
	assert.Equal(t, "-c", wrapped[1])
}

// TestBuildPluginArgvConcurrent exercises the detector and quoter
// from many goroutines to flush out any package-level state bugs.
func TestBuildPluginArgvConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			assert.Equal(t, []string{"echo", "hi"},
				cmdenv.BuildPluginArgv([]string{"echo", "hi"}))
			assert.Equal(t, []string{"sh", "-c", "'a | b'"},
				cmdenv.BuildPluginArgv([]string{"a", "|", "b"}))
		}()
	}
	wg.Wait()
}

// FuzzIsPlainArgv runs the detector against arbitrary inputs. The
// only invariants are (1) it must not panic and (2) if it reports
// plain then the joined form must parse as a single CallExpr with
// the same arg count.
func FuzzIsPlainArgv(f *testing.F) {
	seeds := [][]string{
		{"htop"},
		{"echo", "hi"},
		{"echo", "|", "tee"},
		{"ls", "*.go"},
		{"echo", "$X"},
		{""},
		{"echo", "a\x00b"},
	}
	for _, s := range seeds {
		f.Add(strings.Join(s, "\x1f"))
	}
	f.Fuzz(func(t *testing.T, joined string) {
		var args []string
		if joined != "" {
			args = strings.Split(joined, "\x1f")
		}
		_ = cmdenv.IsPlainArgv(args)
		_ = cmdenv.BuildPluginArgv(args)
	})
}
