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
		{
			// Reproduces a real failure from the "lucky-goshawk"
			// conversation: the model emitted the terminator as a
			// "+"-prefixed content line, which used to be written into
			// the file verbatim instead of ending the add block.
			name: "add block with plus-prefixed end marker errors",
			input: join([]string{
				"*** Begin Patch",
				"*** Add File: setup-gcs-audit.sh",
				"+#!/usr/bin/env bash",
				"+echo done",
				"+SQL",
				"+*** End Patch",
			}, "\n"),
			wantErr: prefixEnd,
		},
		{
			name: "update hunk with minus-prefixed end marker errors",
			input: join([]string{
				"*** Begin Patch",
				"*** Update File: setup-downloads-rollup.sh",
				"@@",
				"   ORDER BY total DESC",
				" SQL",
				"-*** End Patch",
				"*** End Patch",
			}, "\n"),
			wantErr: prefixEnd,
		},
		{
			name: "update hunk with plus-prefixed end marker errors",
			input: join([]string{
				"*** Begin Patch",
				"*** Update File: foo.txt",
				"@@",
				" keep",
				"+*** End Patch",
				"*** End Patch",
			}, "\n"),
			wantErr: prefixEnd,
		},
		{
			name: "add block with plus-prefixed begin marker errors",
			input: join([]string{
				"*** Begin Patch",
				"*** Add File: foo.txt",
				"+real content",
				"+*** Begin Patch",
			}, "\n"),
			wantErr: prefixBegin,
		},
		{
			name: "add block with plus-prefixed file directive errors",
			input: join([]string{
				"*** Begin Patch",
				"*** Add File: foo.txt",
				"+real content",
				"+*** Add File: bar.txt",
			}, "\n"),
			wantErr: "*** Add File:",
		},
		{
			name: "marker with surrounding whitespace as content errors",
			input: join([]string{
				"*** Begin Patch",
				"*** Add File: foo.txt",
				"+content",
				"+   *** End Patch   ",
			}, "\n"),
			wantErr: prefixEnd,
		},
		{
			name: "marker as substring is content",
			input: join([]string{
				"*** Begin Patch",
				"*** Add File: doc.md",
				"+The *** End Patch *** marker terminates a patch.",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type: OpAdd,
					Path: "doc.md",
					Lines: []Line{
						{Kind: LineAdd, Content: "The *** End Patch *** marker terminates a patch."},
					},
				},
			}},
		},
		{
			name: "blank lines inside add block preserved",
			input: join([]string{
				"*** Begin Patch",
				"*** Add File: spaced.txt",
				"+first",
				"",
				"+third",
				"*** End Patch",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type: OpAdd,
					Path: "spaced.txt",
					Lines: []Line{
						{Kind: LineAdd, Content: "first"},
						{Kind: LineAdd, Content: ""},
						{Kind: LineAdd, Content: "third"},
					},
				},
			}},
		},
		{
			name: "implicit end with single add line tolerated",
			input: join([]string{
				"*** Add File: note.txt",
				"+just one line",
			}, "\n"),
			want: Patch{Ops: []FileOp{
				{
					Type:  OpAdd,
					Path:  "note.txt",
					Lines: []Line{{Kind: LineAdd, Content: "just one line"}},
				},
			}},
		},
		{
			name: "update missing hunk header errors",
			input: join([]string{
				"*** Begin Patch",
				"*** Update File: main.go",
				" package main",
				"-old",
				"+new",
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
			require.NoError(t, err, "input:\n%s", tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}
