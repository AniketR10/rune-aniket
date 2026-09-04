// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package agentools

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mvdan.cc/sh/v3/syntax"

	"unstable.build/rune/cmd/rune-agent/agent"
	"unstable.build/rune/cmd/rune-agent/configedit"
)

func TestIsGrepInvocation(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   bool
	}{
		// --- direct grep family invocations -----------------------------
		{"plain grep", "grep foo .", true},
		{"plain rg", "rg foo", true},
		{"plain ag", "ag foo", true},
		{"plain ack", "ack foo", true},
		{"egrep", "egrep -r foo .", true},
		{"fgrep", "fgrep -l foo .", true},
		{"rgrep", "rgrep foo .", true},
		{"zgrep", "zgrep foo file.gz", true},
		{"zegrep", "zegrep -i pat file.gz", true},
		{"zfgrep", "zfgrep -l pat file.gz", true},
		{"bzgrep", "bzgrep foo file.bz2", true},
		{"xzgrep", "xzgrep foo file.xz", true},
		{"lzgrep", "lzgrep foo file.lz", true},
		{"pcregrep", "pcregrep -r foo .", true},
		{"pcre2grep", "pcre2grep -r foo .", true},
		{"ugrep", "ugrep foo .", true},
		{"ripgrep binary name", "ripgrep foo .", true},

		// --- awk / sed / perl are not grep guard targets ----------------
		{"plain awk", "awk '/foo/' file", false},
		{"gawk", "gawk '{print}' file", false},
		{"mawk", "mawk '/foo/' file", false},
		{"nawk", "nawk '/foo/' file", false},
		{"plain sed", `sed -n '/foo/p' file`, false},
		{"gsed", `gsed -n '/foo/p' file`, false},
		{"sed substitute", `sed -i 's/foo/bar/g' file`, false},
		{"sed inplace macOS", `sed -i '' 's/foo/bar/g' file`, false},
		{"sed inplace backup", `sed -i.bak 's/foo/bar/g' file`, false},
		{"gsed inplace", `gsed -i 's/x/y/g' file`, false},
		{"plain perl -ne", `perl -ne 'print if /foo/' file`, false},
		{"perl -pe", `perl -pe 's/foo/bar/' file`, false},
		{"perl inplace", `perl -i -pe 's/x/y/g' file`, false},
		{"awk print field", `awk '{print $1}' file`, false},
		{"awk aggregate", `awk 'BEGIN{s=0}{s+=$1}END{print s}' file`, false},

		// --- path / quoting variants -----------------------------------
		{"absolute path grep", "/usr/bin/grep foo .", true},
		{"opt homebrew grep", "/opt/homebrew/bin/grep foo .", true},
		{"relative path grep", "./grep foo", true},
		{"double-quoted grep program", `"grep" foo .`, true},
		{"single-quoted grep program", `'grep' foo .`, true},
		{"backslash-escaped grep", `\grep foo .`, true},
		{"grep with quoted args", `grep "hello world" file.go`, true},

		// --- compound statements ---------------------------------------
		{"cd then grep", "cd /tmp && grep foo .", true},
		{"cd then grep with quoted path", `cd "/tmp" && grep foo .`, true},
		{"cd; grep via two stmts", "cd /tmp; grep foo .", true},
		{"grep && echo", "grep foo && echo ok", true},
		{"ls && grep", "ls && grep foo", true},
		{"grep && grep", "grep foo && grep bar", true},
		{"grep || true", "grep foo || true", true},
		{"if grep -q", "if grep -q foo file; then echo yes; fi", true},
		{"while grep", "while grep -q foo file; do sleep 1; done", true},
		{"for then grep", `for f in *.go; do grep foo "$f"; done`, true},
		{"case then grep", `case "$x" in *) grep foo;; esac`, true},
		{"function body uses grep", `f() { grep foo; }; f`, true},
		{"brace group", `{ grep foo; }`, true},
		{"subshell group", `( grep foo )`, true},

		// --- pipes: first stage only -----------------------------------
		// We only guard the first (leftmost) stage of a pipeline.
		// grep/sed/awk/perl appearing as a downstream filter are
		// allowed because they are filtering stdin rather than
		// performing a filesystem search.
		{"grep first stage", "grep foo | head", true},
		{"grep then sort", "grep -rl foo . 2>/dev/null | sort -u", true},
		{"sed first stage", `sed -n '/foo/p' file | sort`, false},
		{"awk first stage", `awk '/foo/' file | sort`, false},
		{"perl first stage", `perl -ne 'print if /foo/' file | sort`, false},
		{"grep first in 3-stage", "grep foo file | cmd1 | cmd2", true},
		{"cat | grep (filter)", "cat file | grep foo", false},
		{"cat | sed (filter)", "cat file | sed -n '/foo/p'", false},
		{"cat | awk (filter)", "cat file | awk '/foo/'", false},
		{"cat | perl (filter)", `cat file | perl -ne 'print if /foo/'`, false},
		{"find | xargs grep (filter)", "find . | xargs grep foo", false},
		{"find | xargs sed (filter)", `find . | xargs sed -n '/foo/p'`, false},
		{"find | xargs awk (filter)", `find . | xargs awk '/foo/'`, false},
		{"git ls-files | xargs grep (filter)", "git ls-files | xargs grep foo", false},
		{"3-stage with grep at end (filter)", "cmd1 | cmd2 | grep foo", false},
		{"3-stage with grep middle (filter)", "cmd1 | grep foo | cmd2", false},

		// --- redirects -------------------------------------------------
		{"grep with stdout redirect", "grep foo > out", true},
		{"grep with append redirect", "grep foo >> out", true},
		{"grep with stderr redirect", "grep foo 2>/dev/null", true},
		{"grep with merged redirect", "grep foo 2>&1", true},
		{"grep with combined redirect", "grep foo &>/dev/null", true},
		{"grep with stdin redirect", "grep foo < in", true},
		{"grep with heredoc", "grep foo <<EOF\nhello\nEOF\n", true},
		{"grep with herestring", `grep foo <<< "hello"`, true},
		{"redirect source is grep", `cat < $(grep -l foo)`, true},

		// --- command substitution / backticks --------------------------
		{"grep inside cmd subst", "echo $(grep foo)", true},
		{"grep with cmd subst arg", "grep $(echo foo) .", true},
		{"legacy backtick grep", "echo `grep foo`", true},
		{"env assignment value uses grep", `VAR=$(grep foo) echo "$VAR"`, true},
		{"process substitution", "diff <(grep foo a) <(grep foo b)", true},

		// --- wrapper programs ------------------------------------------
		{"env assignment then grep", "FOO=bar grep foo", true},
		{"LC_ALL then grep", "LC_ALL=C grep foo", true},
		{"sudo grep", "sudo grep foo", true},
		{"doas grep", "doas grep foo", true},
		{"time grep", "time grep foo", true},
		{"nohup grep", "nohup grep foo &", true},
		{"nice grep", "nice grep foo", true},
		{"ionice grep", "ionice -c2 grep foo", true},
		{"watch grep", "watch grep foo", true},
		{"timeout grep", "timeout 5 grep foo", true},
		{"stdbuf grep", "stdbuf -oL grep foo", true},
		{"parallel grep", "parallel grep foo ::: *.go", true},
		{"xargs grep", "xargs grep foo", true},
		{"command grep", "command grep foo", true},
		{"exec grep", "exec grep foo", true},
		{"git grep", "git grep foo", true},
		{"hg grep", "hg grep foo", true},
		{"jj grep", "jj grep foo", true},
		{"sudo xargs grep", "sudo xargs grep foo", true},

		// --- find -exec ------------------------------------------------
		{"find -exec grep", `find . -name '*.go' -exec grep foo {} \;`, true},
		{"find -exec sed", `find . -name '*.go' -exec sed -n '/foo/p' {} \;`, false},

		// --- negatives: must NOT flag legitimate non-search shell work -
		{"empty script", "", false},
		{"only whitespace", "   \t\n", false},
		{"only cd", "cd /tmp", false},
		{"only sort", "sort file", false},
		// Regression: an `if` without an `else` branch used to panic
		// because syntax.IfClause.Else is a typed-nil *IfClause, which
		// dereferences when wrapped in a syntax.Command interface.
		{"if without else", "if true; then echo hi; fi", false},
		{"if/elif without else", "if true; then a; elif false; then b; fi", false},
		// Regression: function declarations with a body that is a
		// compound command must be inspected as a full Stmt so a nil
		// inner Cmd does not panic and so redirections on the body
		// are scanned.
		{"function body uses grep with redirect", `f() { grep foo; } > out; f`, true},
		{"non-grep pipeline", "cat foo | head", false},
		{"malformed shell", "grep 'unterminated", false},
		{"non-grep command", "ls -la", false},
		{"echo with grep literal arg", "echo grep foo", false},
		{"make build", "make build", false},
		{"go test", "go test ./...", false},
		{"npm install", "npm install", false},
		{"python -c", `python -c "print(1)"`, false},
		{"node -e", `node -e "console.log(1)"`, false},
		{"ruby -e", `ruby -e 'puts 1'`, false},
		{"tar extract", "tar -xzf file.tar.gz", false},
		{"curl", "curl http://example.com", false},
		{"docker ps", "docker ps", false},
		{"git commit", `git commit -m "msg"`, false},
		{"git log", "git log --oneline", false},
		{"git log --grep flag is not grep program", "git log --grep=foo", false},
		{"git status", "git status", false},
		{"cp", "cp src dst", false},
		{"mv", "mv old new", false},
		{"rm", "rm -rf node_modules", false},
		{"ln", "ln -s a b", false},
		{"chmod", "chmod +x file", false},
		{"head", "head -n 10 file", false},
		{"tail", "tail -n 10 file", false},
		{"cat", "cat file", false},
		{"sort uniq", "sort file | uniq", false},
		{"wc", "wc -l file", false},
		{"find without grep action", `find . -name '*.go' -mtime -1`, false},
		{"cut for csv", "cut -d, -f1 file.csv", false},
		{"tr", "tr a b < file", false},
		{"tee", "tee output.txt", false},
		{"printf", `printf "%s\n" foo`, false},
		{"echo", "echo hello", false},

		// --- known gaps: documented evasion routes we do NOT cover ----
		// We do not re-parse the body of `bash -c "..."` or `sh -c
		// "..."`. A model determined enough to wrap grep in a `-c`
		// string still gets through. Documenting the gap so the test
		// suite is honest.
		{"bash -c wraps grep (known gap)", `bash -c "grep foo ."`, false},
		{"sh -c wraps grep (known gap)", `sh -c "grep foo ."`, false},
		// We cannot statically resolve variable program names.
		{"variable program name (known gap)", "gr=grep; $gr foo", false},
		// eval body is a literal we do not re-parse.
		{"eval wraps grep (known gap)", `eval "grep foo"`, false},

		// --- additional parser-walker coverage -------------------------
		// coproc clauses (CoprocClause branch).
		{"coproc grep", "coproc grep foo file", true},
		{"coproc named with grep body", `coproc MYCO { grep foo file; }`, true},
		{"coproc non-grep", "coproc cat file", false},

		// Subshell / Block branches with no grep.
		{"subshell no grep", "( echo hi )", false},
		{"brace block no grep", "{ echo hi; }", false},

		// CaseClause with all-non-grep arms exercises the "return false"
		// tail of the for-range loop.
		{"case no grep in any arm", `case $x in a) echo hi;; b) echo bye;; esac`, false},
		{"case grep in non-first arm", `case $x in a) echo hi;; b) grep foo;; esac`, true},

		// ForClause with no grep in its body.
		{"for body no grep", `for f in a b; do echo "$f"; done`, false},

		// FuncDecl body redirections are scanned (Body Stmt redirs).
		{"function with no grep", `f() { echo hi; }; f`, false},
		{"function body redir uses grep", `f() { echo hi; } > $(grep -l foo); f`, true},

		// Constructs we currently treat as "not a grep invocation".
		// These reach the `default` branch of commandMentionsGrep.
		{"arithmetic command", "(( i++ ))", false},
		{"test clause", `[[ -f x ]]`, false},
		{"let clause", "let x=1", false},
		{"declare clause", "declare -a arr", false},

		// CallExpr with no Args (only assignments) — Assigns are
		// already exercised above; this also covers the empty-Args
		// short-circuit path.
		{"env assignment only, no command", "VAR=value", false},

		// Wrapper followed by a non-literal program word — exercises
		// the "seenWrapper && !ok ⇒ continue" branch.
		{"sudo then variable program", `sudo $CMD grep foo`, true},
		{"sudo only variable program (no grep)", `sudo $CMD other`, false},

		// Words built from concatenated literal+single+double quotes —
		// exercises the literalProgramName builder loop.
		{"concatenated literal program name", `gr"e"p foo`, true},
		{"single-quote concat program name", `gr'ep' foo`, true},

		// DblQuoted containing a command substitution exercises the
		// recursive partsMentionGrep DblQuoted branch.
		{"dblquoted contains cmd subst grep", `echo "x $(grep foo) y"`, true},
		{"dblquoted parameter expansion only", `echo "x ${VAR} y"`, false},

		// Word with a literal containing whitespace cannot be a real
		// program name. Build it via a here-doc-style redir target
		// where leading-whitespace tokens parse as one Word.
		{"empty double-quoted program", `"" foo`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGrepInvocation(tt.script)
			assert.Equal(t, tt.want, got, "isGrepInvocation(%q)", tt.script)
		})
	}
}

func TestBuiltinToolsErrorMessage(t *testing.T) {
	t.Run("anthropic uses search_content", func(t *testing.T) {
		msg := builtinToolsErrorMessage("anthropic")
		assert.Contains(t, msg, "find_references")
		assert.Contains(t, msg, "find_definition")
		assert.Contains(t, msg, "find_implementations")
		assert.Contains(t, msg, "search_symbols")
		assert.Contains(t, msg, "outline_file")
		assert.Contains(t, msg, "describe_symbol")
		assert.Contains(t, msg, "search_content")
		assert.NotContains(t, msg, "grep_files")
	})

	t.Run("openai uses grep_files", func(t *testing.T) {
		msg := builtinToolsErrorMessage("openai")
		assert.Contains(t, msg, "grep_files")
		assert.NotContains(t, msg, "search_content")
	})

	t.Run("codex uses grep_files", func(t *testing.T) {
		msg := builtinToolsErrorMessage("codex")
		assert.Contains(t, msg, "grep_files")
		assert.NotContains(t, msg, "search_content")
	})

	t.Run("gemini uses antigravity-native names", func(t *testing.T) {
		msg := builtinToolsErrorMessage("gemini")
		assert.Contains(t, msg, "grep_search")
		assert.Contains(t, msg, "codebase_search")
		assert.Contains(t, msg, "view_file_outline")
		assert.NotContains(t, msg, "search_content")
		assert.NotContains(t, msg, "search_symbols")
		assert.NotContains(t, msg, "outline_file")
	})
}

// errConfig is a memConfig variant whose GetBool returns a non-ErrNotFound
// error, exercising the slog.Warn fallthrough in decideGrep.
type errConfig struct {
	memConfig
	err error
}

func (e *errConfig) GetBool(string) configedit.Bool {
	return configedit.NewBool(func(context.Context) (bool, error) {
		return false, e.err
	})
}

func TestNewGrepGuard_PanicsOnNilConfig(t *testing.T) {
	assert.PanicsWithValue(t,
		"agentools: grepGuard cfg must not be nil",
		func() { _ = newGrepGuard(nil) })
}

func TestNewGrepGuard_StoresCfgAndBoolView(t *testing.T) {
	cfg := withForce(true)
	g := newGrepGuard(cfg)
	require.NotNil(t, g.cfg)
	v, err := g.forceBuiltinTools.Resolve(context.Background())
	require.NoError(t, err)
	assert.True(t, v)
}

func TestDecideGrep(t *testing.T) {
	const promptTitle = "Allow grep via shell?"

	t.Run("config true rejects without prompting", func(t *testing.T) {
		mp := &mockPrompter{}
		ctx := agent.WithPrompter(context.Background(), mp)
		g := newGrepGuard(withForce(true))
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.True(t, rejected)
		assert.Empty(t, mp.calls)
	})

	t.Run("config false allows without prompting", func(t *testing.T) {
		mp := &mockPrompter{}
		ctx := agent.WithPrompter(context.Background(), mp)
		g := newGrepGuard(withForce(false))
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.False(t, rejected)
		assert.Empty(t, mp.calls)
	})

	t.Run("config error other than ErrNotFound falls through to prompt", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"yes"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		g := newGrepGuard(&errConfig{err: errors.New("boom")})
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.False(t, rejected)
		require.Len(t, mp.calls, 1)
	})

	t.Run("absent and nil prompter falls back to allow undecided", func(t *testing.T) {
		g := newGrepGuard(newMemConfig())
		rejected, decided := g.decideGrep(context.Background())
		assert.False(t, decided)
		assert.False(t, rejected)
	})

	t.Run("prompter error rejects single call", func(t *testing.T) {
		mp := &mockPrompter{err: errors.New("dismissed")}
		ctx := agent.WithPrompter(context.Background(), mp)
		g := newGrepGuard(newMemConfig())
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.True(t, rejected)
		require.Len(t, mp.calls, 1)
		assert.Equal(t, promptTitle, mp.calls[0].Title)
	})

	t.Run("yes allows, does not persist", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"yes"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := newMemConfig()
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.False(t, rejected)
		assert.Empty(t, cfg.setCalls)
	})

	t.Run("no rejects, does not persist", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"no"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := newMemConfig()
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.True(t, rejected)
		assert.Empty(t, cfg.setCalls)
	})

	t.Run("always persists false and allows", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"always"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := newMemConfig()
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.False(t, rejected)
		require.Len(t, cfg.setCalls, 1)
		assert.False(t, cfg.setCalls[0])
	})

	t.Run("never persists true and rejects", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"never"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := newMemConfig()
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.True(t, rejected)
		require.Len(t, cfg.setCalls, 1)
		assert.True(t, cfg.setCalls[0])
	})

	t.Run("session_yes stores ephemeral and allows", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"session_yes"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := newMemConfig()
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.False(t, rejected)
		assert.Empty(t, cfg.setCalls)
		require.Len(t, cfg.ephemeralCalls, 1)
		assert.False(t, cfg.ephemeralCalls[0])
	})

	t.Run("session_no stores ephemeral and rejects", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"session_no"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := newMemConfig()
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.True(t, rejected)
		assert.Empty(t, cfg.setCalls)
		require.Len(t, cfg.ephemeralCalls, 1)
		assert.True(t, cfg.ephemeralCalls[0])
	})

	t.Run("unknown choice rejects", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"maybe"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		g := newGrepGuard(newMemConfig())
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.True(t, rejected)
	})

	t.Run("empty response values rejects", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: nil}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		g := newGrepGuard(newMemConfig())
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.True(t, rejected)
	})

	t.Run("always with cfg SetBool error still allows", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"always"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := &failingSetCfg{err: errors.New("disk full")}
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.False(t, rejected)
	})

	t.Run("never with cfg SetBool error still rejects", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"never"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := &failingSetCfg{err: errors.New("disk full")}
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.True(t, rejected)
	})

	t.Run("session_yes with cfg SetBool error still allows", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"session_yes"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := &failingSetCfg{err: errors.New("oom")}
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.False(t, rejected)
	})

	t.Run("session_no with cfg SetBool error still rejects", func(t *testing.T) {
		mp := &mockPrompter{
			responses: []agent.PromptResponse{{Values: []string{"session_no"}}},
		}
		ctx := agent.WithPrompter(context.Background(), mp)
		cfg := &failingSetCfg{err: errors.New("oom")}
		g := newGrepGuard(cfg)
		rejected, decided := g.decideGrep(ctx)
		assert.True(t, decided)
		assert.True(t, rejected)
	})
}

// failingSetCfg is a memConfig whose SetBool returns an error, used to
// exercise the slog.Warn branches in the always/never code paths.
type failingSetCfg struct {
	memConfig
	err error
}

func (f *failingSetCfg) GetBool(string) configedit.Bool {
	return configedit.NewBool(func(context.Context) (bool, error) {
		return false, configedit.ErrNotFound
	})
}

func (f *failingSetCfg) SetBool(context.Context, string, bool, bool) error {
	return f.err
}

// The remaining tests target defensive nil-guards on internal helpers
// that the parser does not currently produce but which the code is
// written to tolerate. They are exercised directly to lock the
// guarantees in place.

func TestStmtMentionsGrep_NilStmt(t *testing.T) {
	assert.False(t, stmtMentionsGrep(nil))
}

func TestCommandMentionsGrep_NilInterface(t *testing.T) {
	assert.False(t, commandMentionsGrep(nil))
}

func TestCommandMentionsGrep_UnknownCommandType(t *testing.T) {
	// ArithmCmd is reachable as the parser type but our switch
	// intentionally has no case for it, so it falls to the default
	// branch.
	assert.False(t, commandMentionsGrep(&syntax.ArithmCmd{}))
}

func TestCallExprMentionsGrep_Nil(t *testing.T) {
	assert.False(t, callExprMentionsGrep(nil))
}

func TestLiteralProgramName(t *testing.T) {
	t.Run("nil word", func(t *testing.T) {
		_, ok := literalProgramName(nil)
		assert.False(t, ok)
	})
	t.Run("empty parts", func(t *testing.T) {
		_, ok := literalProgramName(&syntax.Word{})
		assert.False(t, ok)
	})
	t.Run("dblquoted containing non-literal inner part", func(t *testing.T) {
		// "$(grep)" is a DblQuoted whose inner part is a CmdSubst,
		// not a Lit; literalProgramName must reject the word.
		w := &syntax.Word{Parts: []syntax.WordPart{
			&syntax.DblQuoted{Parts: []syntax.WordPart{
				&syntax.CmdSubst{},
			}},
		}}
		_, ok := literalProgramName(w)
		assert.False(t, ok)
	})
	t.Run("unknown word part type", func(t *testing.T) {
		w := &syntax.Word{Parts: []syntax.WordPart{
			&syntax.ParamExp{},
		}}
		_, ok := literalProgramName(w)
		assert.False(t, ok)
	})
}
