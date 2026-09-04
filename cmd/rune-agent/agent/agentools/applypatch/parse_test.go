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

package applypatch

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Patch
		wantErr string
	}{
		{
			name: "add file",
			input: `*** Begin Patch
*** Add File: src/hello.go
+package main
+
+func main() {}
*** End Patch`,
			want: Patch{Ops: []FileOp{
				{
					Type: OpAdd,
					Path: "src/hello.go",
					Lines: []Line{
						{Kind: LineAdd, Content: "package main"},
						{Kind: LineAdd, Content: ""},
						{Kind: LineAdd, Content: "func main() {}"},
					},
				},
			}},
		},
		{
			name: "delete file",
			input: `*** Begin Patch
*** Delete File: old.txt
*** End Patch`,
			want: Patch{Ops: []FileOp{
				{Type: OpDelete, Path: "old.txt"},
			}},
		},
		{
			name: "single hunk update",
			input: `*** Begin Patch
*** Update File: main.go
@@ func main
 func main() {
-	fmt.Println("hello")
+	fmt.Println("world")
 }
*** End Patch`,
			want: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "main.go",
					Hunks: []Hunk{
						{
							ContextHint: "func main",
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
		},
		{
			name: "multi hunk update",
			input: `*** Begin Patch
*** Update File: lib.go
@@ imports
 import (
-	"fmt"
+	"log"
 )
@@ func Run
 func Run() {
-	fmt.Println("run")
+	log.Println("run")
 }
*** End Patch`,
			want: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "lib.go",
					Hunks: []Hunk{
						{
							ContextHint: "imports",
							Lines: []Line{
								{Kind: LineContext, Content: "import ("},
								{Kind: LineRemove, Content: "\t\"fmt\""},
								{Kind: LineAdd, Content: "\t\"log\""},
								{Kind: LineContext, Content: ")"},
							},
						},
						{
							ContextHint: "func Run",
							Lines: []Line{
								{Kind: LineContext, Content: "func Run() {"},
								{Kind: LineRemove, Content: "\tfmt.Println(\"run\")"},
								{Kind: LineAdd, Content: "\tlog.Println(\"run\")"},
								{Kind: LineContext, Content: "}"},
							},
						},
					},
				},
			}},
		},
		{
			name: "update with move to",
			input: `*** Begin Patch
*** Update File: old.go
*** Move to: new.go
@@
 package main
-// old comment
+// new comment
*** End Patch`,
			want: Patch{Ops: []FileOp{
				{
					Type:   OpUpdate,
					Path:   "old.go",
					MoveTo: "new.go",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineContext, Content: "package main"},
								{Kind: LineRemove, Content: "// old comment"},
								{Kind: LineAdd, Content: "// new comment"},
							},
						},
					},
				},
			}},
		},
		{
			name: "mixed operations",
			input: `*** Begin Patch
*** Add File: new.txt
+hello
*** Delete File: old.txt
*** Update File: keep.txt
@@
 line one
-line two
+line TWO
*** End Patch`,
			want: Patch{Ops: []FileOp{
				{
					Type:  OpAdd,
					Path:  "new.txt",
					Lines: []Line{{Kind: LineAdd, Content: "hello"}},
				},
				{Type: OpDelete, Path: "old.txt"},
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
		},
		{
			name: "leading blank lines tolerated",
			input: `
*** Begin Patch
*** Delete File: x.txt
*** End Patch`,
			want: Patch{Ops: []FileOp{
				{Type: OpDelete, Path: "x.txt"},
			}},
		},
		{
			name: "add empty file",
			input: strings.Join([]string{
				"*** Begin Patch",
				"*** Add File: empty.txt",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{Type: OpAdd, Path: "empty.txt"},
			}},
		},
		{
			name: "path with spaces preserved",
			input: strings.Join([]string{
				"*** Begin Patch",
				"*** Add File: my docs/notes file.txt",
				"+hello",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type:  OpAdd,
					Path:  "my docs/notes file.txt",
					Lines: []Line{{Kind: LineAdd, Content: "hello"}},
				},
			}},
		},
		{
			name: "update move with no hunks",
			input: strings.Join([]string{
				"*** Begin Patch",
				"*** Update File: a.go",
				"*** Move to: b.go",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{Type: OpUpdate, Path: "a.go", MoveTo: "b.go"},
			}},
		},
		{
			name: "content line containing triple star kept",
			input: strings.Join([]string{
				"*** Begin Patch",
				"*** Add File: stars.txt",
				"+*** not a directive",
				"+normal",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type: OpAdd,
					Path: "stars.txt",
					Lines: []Line{
						{Kind: LineAdd, Content: "*** not a directive"},
						{Kind: LineAdd, Content: "normal"},
					},
				},
			}},
		},
		{
			name: "hunk with only additions",
			input: strings.Join([]string{
				"*** Begin Patch",
				"*** Update File: add.go",
				"@@ top",
				" package main",
				"+// new line",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "add.go",
					Hunks: []Hunk{
						{
							ContextHint: "top",
							Lines: []Line{
								{Kind: LineContext, Content: "package main"},
								{Kind: LineAdd, Content: "// new line"},
							},
						},
					},
				},
			}},
		},
		{
			name: "hunk with only removals",
			input: strings.Join([]string{
				"*** Begin Patch",
				"*** Update File: del.go",
				"@@",
				" keep",
				"-drop me",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "del.go",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineContext, Content: "keep"},
								{Kind: LineRemove, Content: "drop me"},
							},
						},
					},
				},
			}},
		},
		{
			name: "empty diff line treated as blank context",
			input: strings.Join([]string{
				"*** Begin Patch",
				"*** Update File: blank.go",
				"@@",
				" first",
				"",
				" third",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "blank.go",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineContext, Content: "first"},
								{Kind: LineContext, Content: ""},
								{Kind: LineContext, Content: "third"},
							},
						},
					},
				},
			}},
		},
		{
			name: "unexpected line in update errors",
			input: strings.Join([]string{
				"*** Begin Patch",
				"*** Update File: bad.go",
				"this is not a hunk header",
				"*** End Patch",
			}, "\n"),
			wantErr: "@@",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
