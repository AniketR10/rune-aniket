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

func TestIsBareGrepCommand(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   bool
	}{
		{"plain grep", "grep foo .", true},
		{"plain rg", "rg foo", true},
		{"plain ag", "ag foo", true},
		{"plain ack", "ack foo", true},
		{"grep with quoted args", `grep "hello world" file.go`, true},
		{"cd then grep", "cd /tmp && grep foo .", true},
		{"cd then grep with quoted path", `cd "/tmp" && grep foo .`, true},
		{"absolute path grep", "/usr/bin/grep foo .", true},
		{"grep with semicolon after cd via two stmts", "cd /tmp; grep foo .", true},

		{"piped grep", "grep foo | head", false},
		{"grep with stdout redirect", "grep foo > out", false},
		{"grep with stdin redirect", "grep foo < in", false},
		{"two forked commands", "ls && grep foo", false},
		{"nested via bash -c", `bash -c "grep foo ."`, false},
		{"grep inside command substitution arg", "echo $(grep foo)", false},
		{"grep with command substitution arg", "grep $(echo foo) .", false},
		{"or chain", "grep foo || true", false},
		{"empty script", "", false},
		{"only whitespace", "   \t\n", false},
		{"only cd", "cd /tmp", false},
		{"malformed shell", "grep 'unterminated", false},
		{"non-grep command", "ls -la", false},
		{"two greps via &&", "grep foo && grep bar", false},
		{"leading env assignment then grep", "FOO=bar grep foo", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBareGrepCommand(tt.script)
			assert.Equal(t, tt.want, got, "isBareGrepCommand(%q)", tt.script)
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
