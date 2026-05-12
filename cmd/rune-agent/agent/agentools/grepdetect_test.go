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
		// Plain forms.
		{"plain grep", "grep foo .", true},
		{"plain rg", "rg foo", true},
		{"plain ag", "ag foo", true},
		{"plain ack", "ack foo", true},
		{"grep with quoted args", `grep "hello world" file.go`, true},
		{"cd then grep", "cd /tmp && grep foo .", true},
		{"cd then grep with quoted path", `cd "/tmp" && grep foo .`, true},
		{"absolute path grep", "/usr/bin/grep foo .", true},
		{"cd; grep via two stmts", "cd /tmp; grep foo .", true},

		// Evasion patterns: trivial wrappers around grep that the
		// model was using to slip past the previous "bare grep only"
		// detector. All of these must now be intercepted.
		{"piped grep", "grep foo | head", true},
		{"grep then sort", "grep -rl foo . 2>/dev/null | sort -u", true},
		{"grep with stdout redirect", "grep foo > out", true},
		{"grep with stderr redirect", "grep foo 2>/dev/null", true},
		{"grep with stdin redirect", "grep foo < in", true},
		{"ls && grep", "ls && grep foo", true},
		{"grep && grep", "grep foo && grep bar", true},
		{"or chain", "grep foo || true", true},
		{"env assignment then grep", "FOO=bar grep foo", true},
		{"grep inside command substitution", "echo $(grep foo)", true},
		{"grep with command substitution arg", "grep $(echo foo) .", true},
		{"cat | grep", "cat file | grep foo", true},
		{"find | xargs grep", "find . | xargs grep foo", true},

		// Negatives.
		{"empty script", "", false},
		{"only whitespace", "   \t\n", false},
		{"only cd", "cd /tmp", false},
		{"only sort", "sort file", false},
		{"non-grep pipeline", "cat foo | head", false},
		{"malformed shell", "grep 'unterminated", false},
		{"non-grep command", "ls -la", false},
		{"grep as literal arg to echo", "echo grep foo", false},
		// Known evasion limitation: we don't re-parse the body of
		// `bash -c "..."`, so this stays false. Documenting it here
		// to make the gap explicit.
		{"bash -c wraps grep (known gap)", `bash -c "grep foo ."`, false},
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
