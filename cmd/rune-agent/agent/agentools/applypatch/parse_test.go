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
