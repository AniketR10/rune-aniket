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

package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/ide/ideshell"
	"unstable.build/go-tui/text/modeless"
)

// TestGoREPLEndToEndEval drives the Go REPL handler against the real go
// toolchain and a real gopls, asserting that each accepted line renders
// the expected output once accumulated state is folded in. Steps run in
// sequence on one shell so accumulation (imports, decls, vars) is
// exercised across lines.
func TestGoREPLEndToEndEval(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping toolchain+gopls e2e test in -short mode")
	}
	tests := []struct {
		name  string
		steps []evalStep
	}{
		{"scalar expression", []evalStep{
			{`1 + 1`, []string{"go> 1 + 1", "2"}},
		}},
		{"value call prints result", []evalStep{
			{`import "strings"`, []string{`go> import "strings"`}},
			{`strings.ToUpper("hi")`, []string{`go> strings.ToUpper("hi")`, `"HI"`}},
		}},
		{"multi-value call groups as tuple", []evalStep{
			{`import "fmt"`, []string{`go> import "fmt"`}},
			{`fmt.Println("hello")`, []string{`go> fmt.Println("hello")`, "hello", "(6, <nil>)"}},
		}},
		{"statement then expression accumulate", []evalStep{
			{`x := 21`, []string{"go> x := 21", "21"}},
			{`x * 2`, []string{"go> x * 2", "42"}},
		}},
		{"local package import and use", []evalStep{
			{`import "example.com/replmod/greeter"`,
				[]string{`go> import "example.com/replmod/greeter"`}},
			{`greeter.Greet("Rune")`,
				[]string{`go> greeter.Greet("Rune")`, `"Hello, Rune!"`}},
		}},
		{"unused import does not error", []evalStep{
			{`import "math"`, []string{`go> import "math"`}},
		}},
		{"error then recovery", []evalStep{
			{`undefinedSymbol`, nil}, // compiler error surfaces; tail unchecked
			{`7 * 6`, []string{"go> 7 * 6", "42"}},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rig := newREPLRig(t)
			for i, st := range tc.steps {
				rig.submitKeys(st.line)
				if st.wantTail == nil {
					continue
				}
				got := rig.lines()
				require.GreaterOrEqual(t, len(got), len(st.wantTail)+1,
					"step %d %q: too few rendered lines: %q", i, st.line, got)
				// The final rendered line is the fresh prompt; the tail
				// before it must match the expected echo+output.
				tail := got[len(got)-len(st.wantTail)-1 : len(got)-1]
				require.Equal(t, st.wantTail, tail,
					"step %d %q frame:\n%s", i, st.line, rig.frame())
			}
		})
	}
}

// TestGoREPLEndToEndCompletion drives tab completion through the handler
// against real gopls: member completion over a local package and import
// path completion. All cases run on one warmed-up shell.
func TestGoREPLEndToEndCompletion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping toolchain+gopls e2e test in -short mode")
	}
	rig := newREPLRig(t)
	rig.submitKeys(`import "example.com/replmod/greeter"`)
	rig.submitKeys(`greeter.Greet("Rune")`) // warm gopls package metadata

	t.Run("member completion opens overlay", func(t *testing.T) {
		rig.reset()
		cands := rig.completeOverlay("greeter.")
		require.Equal(t, []string{"Greet", "Prefix", "Shout"}, cands)
	})

	t.Run("single member auto-accepts inline", func(t *testing.T) {
		rig.reset()
		rig.typeText("greeter.S")
		rig.tab()
		require.Equal(t, "go> Shout", lastPrompt(rig))
	})

	t.Run("import path completion lists package", func(t *testing.T) {
		cands := rig.completePackages(`import "example.com/replmod/g`)
		require.Contains(t, cands, "example.com/replmod/greeter")
	})

	t.Run("builtin completion opens overlay", func(t *testing.T) {
		rig.reset()
		cands := rig.completeOverlay("/")
		require.Equal(t,
			[]string{"/type", "/print", "/write", "/clear", "/doc", "/help", "/quit"},
			cands)
	})

	t.Run("single builtin auto-accepts whole token", func(t *testing.T) {
		rig.reset()
		rig.typeText("/ty")
		rig.tab()
		require.Equal(t, "go> /type", lastPrompt(rig))
	})
}

// TestGoREPLEndToEndSignatureHelp drives signature help through the
// handler against real gopls: typing "(" after a callable shows the
// function signature as a transient hint above the prompt.
func TestGoREPLEndToEndSignatureHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping toolchain+gopls e2e test in -short mode")
	}
	rig := newREPLRig(t)
	rig.submitKeys(`import "fmt"`)
	rig.submitKeys(`fmt.Println("warm")`) // warm gopls package metadata

	rig.reset()
	rig.typeText("fmt.Println(")

	// gopls can return no signature on the first probe against a freshly
	// opened synthetic file; <tab> with the cursor just after "("
	// re-requests signature help, so retry until the hint appears.
	var frame string
	require.Eventually(t, func() bool {
		rig.tab()
		frame = rig.frame()
		return strings.Contains(frame, "Println(")
	}, 10*time.Second, 200*time.Millisecond,
		"signature hint should appear, frame:\n%s", rig.frame())
	require.Contains(t, frame, "Println(", "frame:\n%s", frame)
}

type evalStep struct {
	line     string
	wantTail []string
}

// --- e2e harness ------------------------------------------------------

const (
	replWidth  = 40
	replHeight = 10
)

// replRig drives a fully wired Go REPL handler: a real go-toolchain
// runner, a real gopls behind semanticapi.LSP, and the ideshell handler,
// rooted at a throwaway copy of testdata/replmod.
type replRig struct {
	t     *testing.T
	shell *ideshell.Handler
	drain *drainHandler
	sched *tickScheduler
	sess  *goSession
}

func newREPLRig(t *testing.T) *replRig {
	t.Helper()
	goplsBin := findGopls(t)
	dir := copyReplModule(t)

	files := []testFile{{
		name:    "greeter/greeter.go",
		content: mustRead(t, filepath.Join(dir, "greeter", "greeter.go")),
	}}
	env := initGoplsFromDir(t, goplsBin, dir, files)

	// A single real file scheme rooted at the workspace backs both the
	// command executor and the REPL's file system, so gopls, `go run`,
	// and the REPL all see the same on-disk directory.
	scheme := newTestSchemeRooted(dir)
	runner := &executorRunner{
		executor:   scheme,
		fs:         scheme,
		moduleDir:  dir,
		programDir: filepath.Join(dir, ".rune", "cache", "go-repl-e2e"),
	}
	session := newGoSession(runner, scheme, env.mgr)

	ti := &nopInterrupter{}
	sched := newTickScheduler(ti)
	shell, registry := ideshell.New(
		sched.schedule,
		ti,
		commandEditor{te: modeless.Editor()},
		ideshell.Config{
			DisableShellInterpreter: session,
			Prompt:                  "go> ",
		},
	)
	require.NoError(t, registry.RegisterREPLCommand(
		textapi.CommandManual{Name: "go", Summary: "Evaluate Go"}, session,
	))
	t.Cleanup(func() {
		_ = shell.Close()
		_ = os.RemoveAll(runner.programDir)
	})

	d := &drainHandler{Handler: shell, sched: sched}
	d.Resize(replWidth, replHeight)
	return &replRig{t: t, shell: shell, drain: d, sched: sched, sess: session}
}

// submitKeys types line followed by <enter> through the handler, then
// blocks until the async command finishes and applies its output. It
// drives real keystrokes, so no synthetic ^C abort appears in the
// rendered output the way it does with Submit.
func (r *replRig) submitKeys(line string) {
	r.t.Helper()
	r.typeText(line)
	r.drain.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	r.shell.Wait()
	r.sched.drain()
}

// typeText sends each rune of s as a literal key event, mapping space to
// KeySpace. Unlike typeKeys it does not interpret handlertest tokens, so
// arbitrary Go source (with spaces and punctuation) types verbatim.
func (r *replRig) typeText(s string) {
	r.t.Helper()
	for _, ch := range s {
		ev := term.Event{Ch: ch, Type: term.EventKey}
		if ch == ' ' {
			ev = term.Event{Key: term.KeySpace, Type: term.EventKey}
		}
		r.drain.Handle(ev)
	}
}

// reset dismisses any open overlay and clears the input line so the next
// case starts from a clean prompt. modeless binds neither <ctrl-u> nor
// <ctrl-c> to kill-line, so the line is emptied with backspaces.
func (r *replRig) reset() {
	r.t.Helper()
	r.drain.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	for range replWidth * 4 {
		r.drain.Handle(term.Event{Type: term.EventKey, Key: term.KeyBackspace})
	}
}

// lines returns the non-empty, right-trimmed content lines of the
// current frame, dropping the blank top padding so command output can be
// asserted deterministically.
func (r *replRig) lines() []string {
	r.t.Helper()
	var out []string
	for ln := range strings.SplitSeq(r.frame(), "\n") {
		if t := strings.TrimRight(ln, " "); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// frame renders the current handler state to a golden string.
func (r *replRig) frame() string {
	r.t.Helper()
	return handlertest.DrawHandler(r.drain, replWidth, replHeight)
}

// tab sends a <tab> to trigger completion at the cursor.
func (r *replRig) tab() {
	r.t.Helper()
	r.drain.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})
}

// completeOverlay types prefix, presses <tab>, and returns the candidate
// labels shown in the completion overlay. When gopls returns a single
// match the shell accepts it inline and no overlay opens, so the result
// is empty.
func (r *replRig) completeOverlay(prefix string) []string {
	r.t.Helper()
	r.typeText(prefix)
	r.tab()
	return overlayCandidates(r.frame())
}

// completePackages asks the session for import-path candidates,
// retrying because gopls returns an empty set on the first query against
// a freshly opened synthetic file until it has indexed the package.
func (r *replRig) completePackages(line string) []string {
	r.t.Helper()
	cmd, args := splitCompletionLine(line)
	var cands []string
	require.Eventually(r.t, func() bool {
		it, err := r.sess.Complete(context.Background(), cmd, args)
		require.NoError(r.t, err)
		cands, err = iterator.ToSlice(context.Background(), it)
		require.NoError(r.t, err)
		return len(cands) > 0
	}, 10*time.Second, 200*time.Millisecond)
	return cands
}

// overlayCandidates extracts the indented candidate rows the completion
// overlay renders (lines that start with whitespace and a word, sitting
// above the prompt row).
func overlayCandidates(frame string) []string {
	var out []string
	for ln := range strings.SplitSeq(frame, "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(ln, "go> ") {
			continue
		}
		if strings.HasPrefix(ln, "    ") {
			out = append(out, trimmed)
		}
	}
	return out
}

// lastPrompt returns the trimmed final prompt line of the current frame,
// with the cursor block stripped, so the accepted input can be asserted.
func lastPrompt(r *replRig) string {
	r.t.Helper()
	ls := r.lines()
	last := ls[len(ls)-1]
	last = strings.TrimRight(last, "▐")
	return strings.TrimRight(last, " ")
}

// splitCompletionLine splits a raw line into the cmd/args shape the shell
// passes to Complete: the first whitespace-delimited token is cmd, the
// rest are args.
func splitCompletionLine(line string) (string, []string) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], fields[1:]
}

// copyReplModule copies testdata/replmod into a throwaway directory so
// the REPL's synthesized program and gopls overlays never touch the
// checked-in fixture.
func copyReplModule(t *testing.T) string {
	t.Helper()
	dst := t.TempDir()
	src := "testdata/replmod"
	require.NoError(t, filepath.Walk(src,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			target := filepath.Join(dst, rel)
			if info.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, 0o644)
		}))
	return dst
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}
