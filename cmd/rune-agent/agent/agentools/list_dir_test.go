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
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

func TestListDir_definition(t *testing.T) {
	tool := NewListDir(localFS{}, dirURI("/workspace"))
	def := tool.Definition()
	assert.Equal(t, llm.ToolTypeFunction, def.Type)
	assert.Equal(t, "list_dir", def.Function.Name)
	assert.NotEmpty(t, def.Function.Description)
	assert.NotNil(t, def.Function.Parameters)
}

func TestListDir(t *testing.T) {
	tests := []struct {
		name     string
		args     string
		setup    func(t *testing.T, dir string)
		assertFn func(t *testing.T, result string, dir string)
	}{
		{
			name: "lists files and directories with defaults",
			args: `{"dir_path": "%s"}`,
			assertFn: func(t *testing.T, result string, dir string) {
				assert.Contains(t, result, "Absolute path: "+dir)
				assert.Contains(t, result, "hello.txt")
				assert.Contains(t, result, "sub/")
			},
		},
		{
			name: "depth 1 shows only immediate children",
			args: `{"dir_path": "%s", "depth": 1}`,
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub", "deep"), 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, "sub", "deep", "inner.txt"),
					[]byte("inner"), 0o644))
			},
			assertFn: func(t *testing.T, result string, dir string) {
				assert.Contains(t, result, "hello.txt")
				assert.Contains(t, result, "sub/")
				// Children of sub should NOT appear at depth 1.
				assert.NotContains(t, result, "nested.go")
				assert.NotContains(t, result, "inner.txt")
			},
		},
		{
			name: "depth 2 shows children of subdirectories",
			args: `{"dir_path": "%s", "depth": 2}`,
			assertFn: func(t *testing.T, result string, dir string) {
				assert.Contains(t, result, "hello.txt")
				assert.Contains(t, result, "sub/")
				assert.Contains(t, result, "  nested.go")
			},
		},
		{
			name: "offset skips entries",
			args: `{"dir_path": "%s", "depth": 1, "offset": 2}`,
			assertFn: func(t *testing.T, result string, dir string) {
				// With depth=1, setupWorkspace gives: hello.txt, sub/
				// Sorted: hello.txt, sub/ — offset 2 skips hello.txt.
				lines := strings.Split(result, "\n")
				// First line is "Absolute path: ..."
				require.True(t, len(lines) >= 2)
				assert.Contains(t, lines[1], "sub/")
				assert.NotContains(t, result, "hello.txt")
			},
		},
		{
			name: "limit caps results",
			args: `{"dir_path": "%s", "depth": 1, "limit": 1}`,
			assertFn: func(t *testing.T, result string, dir string) {
				assert.Contains(t, result, "More than 1 entries found")
				// Only one entry besides the header and the "More" line.
				lines := strings.Split(result, "\n")
				// line 0: Absolute path, line 1: first entry, line 2: More than...
				assert.Equal(t, 3, len(lines))
			},
		},
		{
			name: "non-existent directory returns error",
			args: `{"dir_path": "%s/nonexistent"}`,
			assertFn: func(t *testing.T, result string, dir string) {
				assert.Contains(t, result, "error:")
			},
		},
		{
			name: "invalid JSON returns error",
			args: `bad json`,
			assertFn: func(t *testing.T, result string, dir string) {
				assert.Contains(t, result, "error: invalid arguments")
			},
		},
		{
			name: "indentation increases with depth",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755))
				require.NoError(t, os.WriteFile(
					filepath.Join(dir, "a", "b", "deep.txt"),
					[]byte("deep"), 0o644))
			},
			args: `{"dir_path": "%s", "depth": 3}`,
			assertFn: func(t *testing.T, result string, dir string) {
				lines := strings.Split(result, "\n")
				var found bool
				for _, line := range lines {
					if strings.HasSuffix(line, "deep.txt") {
						// depth 3 entry → 4 spaces indent
						assert.True(t, strings.HasPrefix(line, "    "),
							"expected 4-space indent, got: %q", line)
						found = true
					}
				}
				assert.True(t, found, "deep.txt not found in output")
			},
		},
		{
			name: "entries sorted alphabetically",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "z.txt"), []byte("z"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
			},
			args: `{"dir_path": "%s", "depth": 1}`,
			assertFn: func(t *testing.T, result string, dir string) {
				aIdx := strings.Index(result, "a.txt")
				zIdx := strings.Index(result, "z.txt")
				assert.Less(t, aIdx, zIdx, "a.txt should appear before z.txt")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := setupWorkspace(t)
			tool := NewListDir(localFS{root: dir}, dirURI(dir))

			if tt.setup != nil {
				tt.setup(t, dir)
			}

			args := tt.args
			if strings.Contains(args, "%s") {
				args = strings.ReplaceAll(args, "%s", dir)
			}

			result := tool.Execute(context.Background(), args)

			if tt.name == "invalid JSON returns error" || tt.name == "non-existent directory returns error" {
				assert.True(t, result.IsError)
			} else {
				assert.False(t, result.IsError, "unexpected error: %s", result.Content)
			}

			tt.assertFn(t, result.Content, dir)
		})
	}
}

func TestPathDepth(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{"", 0},
		{".", 0},
		{"file.txt", 1},
		{"sub/file.txt", 2},
		{"a/b/c", 3},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, pathDepth(tt.path), "pathDepth(%q)", tt.path)
	}
}
