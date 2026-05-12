// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentools

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

		// --- awk / sed / perl: ad-hoc text-processor evasion -----------
		{"plain awk", "awk '/foo/' file", true},
		{"gawk", "gawk '{print}' file", true},
		{"mawk", "mawk '/foo/' file", true},
		{"nawk", "nawk '/foo/' file", true},
		{"plain sed", `sed -n '/foo/p' file`, true},
		{"gsed", `gsed -n '/foo/p' file`, true},
		{"sed substitute", `sed -i 's/foo/bar/g' file`, true},
		{"plain perl -ne", `perl -ne 'print if /foo/' file`, true},
		{"perl -pe", `perl -pe 's/foo/bar/' file`, true},

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
		{"sed first stage", `sed -n '/foo/p' file | sort`, true},
		{"awk first stage", `awk '/foo/' file | sort`, true},
		{"perl first stage", `perl -ne 'print if /foo/' file | sort`, true},
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
		{"find -exec sed", `find . -name '*.go' -exec sed -n '/foo/p' {} \;`, true},

		// --- negatives: must NOT flag legitimate non-search shell work -
		{"empty script", "", false},
		{"only whitespace", "   \t\n", false},
		{"only cd", "cd /tmp", false},
		{"only sort", "sort file", false},
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
}
