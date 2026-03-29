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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestParseLenient verifies the parser tolerates common LLM mistakes:
// missing delimiters, preamble text, and trailing commentary.
//
// Test inputs are built with strings.Join to avoid confusing the
// apply_patch tool with lines that look like patch directives.
func TestParseLenient(t *testing.T) {
	join := strings.Join

	tests := []struct {
		name    string
		input   string
		want    Patch
		wantErr string
	}{
		{
			name: "missing begin marker tolerated",
			input: join([]string{
				"*** Add File: foo.txt",
				"+content",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type:  OpAdd,
					Path:  "foo.txt",
					Lines: []Line{{Kind: LineAdd, Content: "content"}},
				},
			}},
		},
		{
			name: "missing end marker tolerated",
			input: join([]string{
				"*** Begin Patch",
				"*** Add File: foo.txt",
				"+content",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type:  OpAdd,
					Path:  "foo.txt",
					Lines: []Line{{Kind: LineAdd, Content: "content"}},
				},
			}},
		},
		{
			name: "both delimiters missing tolerated",
			input: join([]string{
				"*** Add File: new.txt",
				"+hello world",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type:  OpAdd,
					Path:  "new.txt",
					Lines: []Line{{Kind: LineAdd, Content: "hello world"}},
				},
			}},
		},
		{
			name: "preamble text before begin marker skipped",
			input: join([]string{
				"Here is the patch:",
				"",
				"*** Begin Patch",
				"*** Delete File: old.txt",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{Type: OpDelete, Path: "old.txt"},
			}},
		},
		{
			name: "preamble text before file directive skipped",
			input: join([]string{
				"I'll update the file now:",
				"*** Update File: main.go",
				"@@",
				" package main",
				"-// old",
				"+// new",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type: OpUpdate,
					Path: "main.go",
					Hunks: []Hunk{
						{
							Lines: []Line{
								{Kind: LineContext, Content: "package main"},
								{Kind: LineRemove, Content: "// old"},
								{Kind: LineAdd, Content: "// new"},
							},
						},
					},
				},
			}},
		},
		{
			name: "trailing text after end patch ignored",
			input: join([]string{
				"*** Begin Patch",
				"*** Delete File: old.txt",
				"*** End Patch",
				"",
				"That should fix the issue.",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{Type: OpDelete, Path: "old.txt"},
			}},
		},
		{
			name:    "empty input errors",
			input:   "",
			wantErr: "empty patch",
		},
		{
			name:    "only preamble text errors",
			input:   "just some commentary\nnothing useful here",
			wantErr: "empty patch",
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
			require.NoError(t, err, "input:\n%s", tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
