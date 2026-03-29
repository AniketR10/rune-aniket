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

package applypatch

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func dirURI(dir string) workspaceapi.URI {
	u, _ := workspaceapi.ParseURI("file://" + dir)
	return u
}

// osFS implements workspaceapi.FileSystem for testing against a real directory.
type osFS struct{ root string }

func (f osFS) resolve(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(f.root, path)
}

func (f osFS) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + f.resolve(path))
}

func (f osFS) OpenFile(path string, flag int, mode os.FileMode) (workspaceapi.File, error) {
	return os.OpenFile(path, flag, mode)
}

func (f osFS) Remove(path string) error        { return os.Remove(path) }
func (f osFS) Stat(path string) (os.FileInfo, error) { return os.Stat(path) }
func (f osFS) ReadDir(name string) ([]os.DirEntry, error) { return os.ReadDir(name) }
func (f osFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }

func TestApply(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, dir string)
		patch     Patch
		wantApply int
		wantErrs  int
		verify    func(t *testing.T, dir string)
	}{
		{
			name: "add new file",
			patch: Patch{Ops: []FileOp{
				{
					Type: OpAdd,
					Path: "hello.txt",
					Lines: []Line{
						{Kind: LineAdd, Content: "hello world"},
					},
				},
			}},
			wantApply: 1,
			verify: func(t *testing.T, dir string) {
				data, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
				require.NoError(t, err)
				assert.Equal(t, "hello world", string(data))
			},
		},
		{
			name: "add nested file",
			patch: Patch{Ops: []FileOp{
				{
					Type: OpAdd,
					Path: "a/b/c.txt",
					Lines: []Line{
						{Kind: LineAdd, Content: "deep"},
					},
				},
			}},
			wantApply: 1,
			verify: func(t *testing.T, dir string) {
				data, err := os.ReadFile(filepath.Join(dir, "a", "b", "c.txt"))
				require.NoError(t, err)
				assert.Equal(t, "deep", string(data))
			},
		},
		{
			name: "add file already exists errors",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "exists.txt"), []byte("old"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{
					Type:  OpAdd,
					Path:  "exists.txt",
					Lines: []Line{{Kind: LineAdd, Content: "new"}},
				},
			}},
			wantApply: 0,
			wantErrs:  1,
		},
		{
			name: "add multi-line file",
			patch: Patch{Ops: []FileOp{
				{
					Type: OpAdd,
					Path: "multi.txt",
					Lines: []Line{
						{Kind: LineAdd, Content: "line1"},
						{Kind: LineAdd, Content: "line2"},
						{Kind: LineAdd, Content: "line3"},
					},
				},
			}},
			wantApply: 1,
			verify: func(t *testing.T, dir string) {
				data, err := os.ReadFile(filepath.Join(dir, "multi.txt"))
				require.NoError(t, err)
				assert.Equal(t, "line1\nline2\nline3", string(data))
			},
		},
		{
			name: "delete file",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "del.txt"), []byte("bye"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{Type: OpDelete, Path: "del.txt"},
			}},
			wantApply: 1,
			verify: func(t *testing.T, dir string) {
				_, err := os.Stat(filepath.Join(dir, "del.txt"))
				assert.True(t, os.IsNotExist(err))
			},
		},
		{
			name: "delete nonexistent errors",
			patch: Patch{Ops: []FileOp{
				{Type: OpDelete, Path: "nope.txt"},
			}},
			wantApply: 0,
			wantErrs:  1,
		},
		{
			name: "single hunk update exact match",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"),
					[]byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "main.go",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineContext, Content: "func main() {"},
								{Kind: LineRemove, Content: "\tfmt.Println(\"hello\")"},
								{Kind: LineAdd, Content: "\tfmt.Println(\"world\")"},
								{Kind: LineContext, Content: "}"},
							},
						},
					},
				},
			}},
			wantApply: 1,
			verify: func(t *testing.T, dir string) {
				data, err := os.ReadFile(filepath.Join(dir, "main.go"))
				require.NoError(t, err)
				assert.Contains(t, string(data), "world")
				assert.NotContains(t, string(data), "hello")
			},
		},
		{
			name: "single hunk fuzzy match trailing whitespace",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "ws.txt"),
					[]byte("alpha  \nbeta\t\ngamma\n"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "ws.txt",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineContext, Content: "alpha"},
								{Kind: LineRemove, Content: "beta"},
								{Kind: LineAdd, Content: "BETA"},
							},
						},
					},
				},
			}},
			wantApply: 1,
			verify: func(t *testing.T, dir string) {
				data, err := os.ReadFile(filepath.Join(dir, "ws.txt"))
				require.NoError(t, err)
				assert.Contains(t, string(data), "BETA")
			},
		},
		{
			name: "multi hunk update",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "multi.go"),
					[]byte("package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hi\")\n}\n"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "multi.go",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineRemove, Content: "import \"fmt\""},
								{Kind: LineAdd, Content: "import \"log\""},
							},
						},
						{
							Lines: []Line{
								{Kind: LineContext, Content: "func main() {"},
								{Kind: LineRemove, Content: "\tfmt.Println(\"hi\")"},
								{Kind: LineAdd, Content: "\tlog.Println(\"hi\")"},
								{Kind: LineContext, Content: "}"},
							},
						},
					},
				},
			}},
			wantApply: 1,
			verify: func(t *testing.T, dir string) {
				data, err := os.ReadFile(filepath.Join(dir, "multi.go"))
				require.NoError(t, err)
				s := string(data)
				assert.Contains(t, s, "import \"log\"")
				assert.Contains(t, s, "log.Println")
				assert.NotContains(t, s, "fmt")
			},
		},
		{
			name: "hunk not found errors",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "nope.go"),
					[]byte("package main\n"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "nope.go",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineContext, Content: "does not exist"},
								{Kind: LineRemove, Content: "nope"},
							},
						},
					},
				},
			}},
			wantApply: 0,
			wantErrs:  1,
			verify: func(t *testing.T, dir string) {},
		},
		{
			name: "hunk not found error includes divergence details",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "diverge.go"),
					[]byte("package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "diverge.go",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineContext, Content: "func main() {"},
								{Kind: LineRemove, Content: "\tlog.Println(\"hello\")"},
								{Kind: LineContext, Content: "}"},
							},
						},
					},
				},
			}},
			wantApply: 0,
			wantErrs:  1,
			verify: func(t *testing.T, dir string) {},
		},
		{
			name: "hunk not found error includes context hint",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "hint.go"),
					[]byte("package main\n\nfunc main() {\n}\n"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "hint.go",
					Hunks: []Hunk{
						{
							ContextHint: "func helper",
							Lines: []Line{
								{Kind: LineContext, Content: "func helper() {"},
								{Kind: LineRemove, Content: "\treturn nil"},
								{Kind: LineContext, Content: "}"},
							},
						},
					},
				},
			}},
			wantApply: 0,
			wantErrs:  1,
			verify: func(t *testing.T, dir string) {},
		},
		{
			name: "update with move to",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "old.go"),
					[]byte("package old\n// comment\n"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{
					Type:   OpUpdate,
					Path:   "old.go",
					MoveTo: "new.go",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineRemove, Content: "package old"},
								{Kind: LineAdd, Content: "package new"},
							},
						},
					},
				},
			}},
			wantApply: 1,
			verify: func(t *testing.T, dir string) {
				_, err := os.Stat(filepath.Join(dir, "old.go"))
				assert.True(t, os.IsNotExist(err))

				data, err := os.ReadFile(filepath.Join(dir, "new.go"))
				require.NoError(t, err)
				assert.Contains(t, string(data), "package new")
			},
		},
		{
			name:      "empty patch",
			patch:     Patch{},
			wantApply: 0,
		},
		{
			name: "mixed patch",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "keep.txt"),
					[]byte("line one\nline two\nline three\n"), 0o644))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "remove.txt"),
					[]byte("goodbye"), 0o644))
			},
			patch: Patch{Ops: []FileOp{
				{
					Type:  OpAdd,
					Path:  "created.txt",
					Lines: []Line{{Kind: LineAdd, Content: "fresh"}},
				},
				{Type: OpDelete, Path: "remove.txt"},
				{
					Type: OpUpdate,
					Path: "keep.txt",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineContext, Content: "line one"},
								{Kind: LineRemove, Content: "line two"},
								{Kind: LineAdd, Content: "line TWO"},
							},
						},
					},
				},
			}},
			wantApply: 3,
			verify: func(t *testing.T, dir string) {
				data, err := os.ReadFile(filepath.Join(dir, "created.txt"))
				require.NoError(t, err)
				assert.Equal(t, "fresh", string(data))

				_, err = os.Stat(filepath.Join(dir, "remove.txt"))
				assert.True(t, os.IsNotExist(err))

				data, err = os.ReadFile(filepath.Join(dir, "keep.txt"))
				require.NoError(t, err)
				assert.Contains(t, string(data), "line TWO")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fs := osFS{root: dir}

			if tt.setup != nil {
				tt.setup(t, dir)
			}

			result := Apply(fs, dirURI(dir), tt.patch)
			assert.Equal(t, tt.wantApply, result.Applied)
			assert.Equal(t, tt.wantErrs, len(result.Errors), "errors: %v", result.Errors)

			if tt.verify != nil {
				tt.verify(t, dir)
			}
		})
	}
}

func TestSplitJoinRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"single line no newline", "hello"},
		{"single line with newline", "hello\n"},
		{"multi lines", "a\nb\nc\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := splitLines(tt.input)
			assert.Equal(t, tt.input, joinLines(lines))
		})
	}
}

func TestHunkMatchError(t *testing.T) {
	tests := []struct {
		name     string
		file     string
		hunks    []Hunk
		wantMsgs []string
	}{
		{
			name: "partial match shows expected vs got",
			file: "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
			hunks: []Hunk{
				{
					Lines: []Line{
						{Kind: LineContext, Content: "func main() {"},
						{Kind: LineRemove, Content: "\tlog.Println(\"hello\")"},
						{Kind: LineContext, Content: "}"},
					},
				},
			},
			wantMsgs: []string{
				"hunk 1: no match found",
				"best partial match",
				"1/3 lines matched",
				"expected",
				"got",
			},
		},
		{
			name: "no match at all shows first expected line",
			file: "package main\n",
			hunks: []Hunk{
				{
					Lines: []Line{
						{Kind: LineContext, Content: "does not exist anywhere"},
						{Kind: LineRemove, Content: "nope"},
					},
				},
			},
			wantMsgs: []string{
				"hunk 1: no match found",
				"could not match any context lines",
				"first expected line",
			},
		},
		{
			name: "context hint is included",
			file: "package main\n\nfunc main() {\n}\n",
			hunks: []Hunk{
				{
					ContextHint: "func helper",
					Lines: []Line{
						{Kind: LineContext, Content: "func helper() {"},
						{Kind: LineRemove, Content: "\treturn nil"},
						{Kind: LineContext, Content: "}"},
					},
				},
			},
			wantMsgs: []string{
				"hunk 1: no match found",
				"near \"func helper\"",
			},
		},
		{
			name: "past eof indicated",
			file: "alpha\nbeta",
			hunks: []Hunk{
				{
					Lines: []Line{
						{Kind: LineContext, Content: "alpha"},
						{Kind: LineContext, Content: "beta"},
						{Kind: LineRemove, Content: "gamma"},
					},
				},
			},
			wantMsgs: []string{
				"hunk 1: no match found",
				"2/3 lines matched",
				"reached end of file",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			fs := osFS{root: dir}

			require.NoError(t, os.WriteFile(filepath.Join(dir, "test.go"),
				[]byte(tt.file), 0o644))

			result := Apply(fs, dirURI(dir), Patch{Ops: []FileOp{
				{Type: OpUpdate, Path: "test.go", Hunks: tt.hunks},
			}})
			require.Equal(t, 1, len(result.Errors), "expected exactly 1 error")
			for _, msg := range tt.wantMsgs {
				assert.Contains(t, result.Errors[0], msg)
			}
		})
	}
}
