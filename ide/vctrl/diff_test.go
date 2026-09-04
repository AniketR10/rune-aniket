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

package vctrl

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/cell"
	"unstable.build/rune/component"
)

func TestDiffUtils(t *testing.T) {
	suite := []struct {
		src, dst    string
		expChanges  []diffmatchpatch.Diff
		expFileDiff FileDiff
	}{
		{
			src:         "",
			dst:         "",
			expChanges:  []diffmatchpatch.Diff{},
			expFileDiff: FileDiff{},
		},
		{
			src: "a",
			dst: "a",
			expChanges: []diffmatchpatch.Diff{
				{
					Type: 0,
					Text: "a",
				},
			},
			expFileDiff: FileDiff{
				Hunks: nil,
			},
		},
		{
			src: "",
			dst: "abc\ncba",
			expChanges: []diffmatchpatch.Diff{
				{
					Type: 1,
					Text: "abc\ncba",
				},
			},
			expFileDiff: FileDiff{
				Hunks: []Hunk{
					{
						NewStartLine:  1,
						OrigStartLine: 1,
						OrigLines:     0,
						NewLines:      2,
						Body:          "+abc\n+cba",
					},
				},
			},
		},
		{
			src: "",
			dst: "abc\n\ncba",
			expChanges: []diffmatchpatch.Diff{
				{
					Type: 1,
					Text: "abc\n\ncba",
				},
			},
			expFileDiff: FileDiff{
				Hunks: []Hunk{
					{
						NewStartLine:  1,
						OrigStartLine: 1,
						OrigLines:     0,
						NewLines:      3,
						Body:          "+abc\n+\n+cba",
					},
				},
			},
		},
		{
			src: "abc\ncba",
			dst: "",
			expChanges: []diffmatchpatch.Diff{
				{
					Type: -1,
					Text: "abc\ncba",
				},
			},
			expFileDiff: FileDiff{
				Hunks: []Hunk{
					{
						NewStartLine:  1,
						OrigStartLine: 1,
						OrigLines:     2,
						NewLines:      0,
						Body:          "-abc\n-cba",
					},
				},
			},
		},
		{
			src: "abc\n\ncba",
			dst: "",
			expChanges: []diffmatchpatch.Diff{
				{
					Type: -1,
					Text: "abc\n\ncba",
				},
			},
			expFileDiff: FileDiff{
				Hunks: []Hunk{
					{
						NewStartLine:  1,
						OrigStartLine: 1,
						OrigLines:     3,
						NewLines:      0,
						Body:          "-abc\n-\n-cba",
					},
				},
			},
		},
		{
			src: "abc\nbcd\ncde",
			dst: "000\nabc\n111\nBCD\n",
			expChanges: []diffmatchpatch.Diff{
				{Type: 1, Text: "000\n"},
				{Type: 0, Text: "abc\n"},
				{Type: -1, Text: "bcd\ncde"},
				{Type: 1, Text: "111\nBCD\n"},
			},
			expFileDiff: FileDiff{
				Hunks: []Hunk{
					{
						OrigStartLine: 1,
						OrigLines:     0,
						NewStartLine:  1,
						NewLines:      1,
						Body:          "+000\n",
					},
					{
						OrigStartLine: 3,
						OrigLines:     2,
						NewStartLine:  3,
						NewLines:      0,
						Body:          "-bcd\n-cde",
					},
					{
						OrigStartLine: 3,
						OrigLines:     0,
						NewStartLine:  3,
						NewLines:      2,
						Body:          "+111\n+BCD\n",
					},
				},
			},
		},
		{
			src: "A\nB\nC\nD\nE\nF\nG\nH\nI\nJ\nK\nL\nM\nN\nÑ\nO\nP\nQ\nR\nS\nT\nU\nV\nW\nX\nY\nZ",
			dst: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\nZ",
			expChanges: []diffmatchpatch.Diff{
				{Type: -1, Text: "A\n"},
				{Type: 0, Text: "B\nC\nD\nE\nF\nG\n"},
				{Type: -1, Text: "H\n"},
				{Type: 0, Text: "I\nJ\nK\nL\nM\nN\n"},
				{Type: -1, Text: "Ñ\n"},
				{Type: 0, Text: "O\nP\nQ\nR\nS\nT\n"},
				{Type: -1, Text: "U\n"},
				{Type: 0, Text: "V\nW\nX\nY\nZ"},
			},
			expFileDiff: FileDiff{
				Hunks: []Hunk{
					{
						OrigStartLine: 1,
						OrigLines:     1,
						NewStartLine:  1,
						NewLines:      0,
						Body:          "-A\n",
					},
					{
						OrigStartLine: 7,
						OrigLines:     1,
						NewStartLine:  7,
						NewLines:      0,
						Body:          "-H\n",
					},
					{
						OrigStartLine: 13,
						OrigLines:     1,
						NewStartLine:  13,
						NewLines:      0,
						Body:          "-Ñ\n",
					},
					{
						OrigStartLine: 19,
						OrigLines:     1,
						NewStartLine:  19,
						NewLines:      0,
						Body:          "-U\n",
					},
				},
			},
		},
		{
			src: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\nZ",
			dst: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\n",
			expChanges: []diffmatchpatch.Diff{
				{Type: 0, Text: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\n"},
				{Type: -1, Text: "Z"},
			},
			expFileDiff: FileDiff{
				Hunks: []Hunk{
					{
						OrigStartLine: 23,
						OrigLines:     1,
						NewStartLine:  23,
						NewLines:      0,
						Body:          "-Z",
					},
				},
			},
		},
		{
			src: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY\nZ",
			dst: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\nY",
			expChanges: []diffmatchpatch.Diff{
				{Type: 0, Text: "B\nC\nD\nE\nF\nG\nI\nJ\nK\nL\nM\nN\nO\nP\nQ\nR\nS\nT\nV\nW\nX\n"},
				{Type: -1, Text: "Y\nZ"},
				{Type: 1, Text: "Y"},
			},
			expFileDiff: FileDiff{
				Hunks: []Hunk{
					{
						OrigStartLine: 22,
						OrigLines:     2,
						NewStartLine:  22,
						NewLines:      0,
						Body:          "-Y\n-Z",
					},
					{
						OrigStartLine: 22,
						OrigLines:     0,
						NewStartLine:  22,
						NewLines:      1,
						Body:          "+Y",
					},
				},
			},
		},
	}

	for i, test := range suite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			uri, err := workspaceapi.ParseURI("file:///tmp")
			require.NoError(t, err)

			a := cell.NewBuffer()
			a.ReadFrom(strings.NewReader(test.src))
			b := cell.NewBuffer()
			b.ReadFrom(strings.NewReader(test.dst))

			ctx := context.Background()
			changes := Diff(ctx, a.String(), b.String())
			assert.Equal(t, test.expChanges, changes)

			t.Run("ApplyChanges", func(t *testing.T) {
				ApplyChanges(ctx, a, changes)
				assert.Equal(t, b.String(), a.String())
			})

			t.Run("ConvertChangesToFileDiff", func(t *testing.T) {
				diff := ConvertChangesToFileDiff(uri, changes)
				assert.Equal(t, "/tmp", diff.OrigName)
				assert.Equal(t, "/tmp", diff.NewName)
				diff.OrigName = ""
				diff.NewName = ""
				assert.Equal(t, test.expFileDiff, diff)
			})
		})
	}
}

func TestDiffBuffer(t *testing.T) {
	// Changed lines are tinted through the background only, so the
	// foreground stays available for syntax highlighting.
	wantAdd := term.Attributes{Bg: term.GetColor("darkgreen")}
	wantDel := term.Attributes{Bg: term.GetColor("darkred")}
	tests := []struct {
		name     string
		diffs    []diffmatchpatch.Diff
		wantText string
		// wantAttr maps a row to the attributes every cell in it carries.
		wantAttr map[int]term.Attributes
	}{
		{
			name:     "empty",
			diffs:    nil,
			wantText: "",
		},
		{
			name: "equal lines are unstyled and unprefixed",
			diffs: []diffmatchpatch.Diff{
				{Type: diffmatchpatch.DiffEqual, Text: "a\nb\n"},
			},
			wantText: "a\nb\n",
			wantAttr: map[int]term.Attributes{0: {}, 1: {}},
		},
		{
			name: "insert lines are prefixed and tinted dark green",
			diffs: []diffmatchpatch.Diff{
				{Type: diffmatchpatch.DiffInsert, Text: "a\nb\n"},
			},
			wantText: "+a\n+b\n",
			wantAttr: map[int]term.Attributes{
				0: wantAdd, 1: wantAdd,
			},
		},
		{
			name: "delete lines are prefixed and tinted dark red",
			diffs: []diffmatchpatch.Diff{
				{Type: diffmatchpatch.DiffDelete, Text: "a\nb\n"},
			},
			wantText: "-a\n-b\n",
			wantAttr: map[int]term.Attributes{
				0: wantDel, 1: wantDel,
			},
		},
		{
			name: "trailing line without newline keeps its prefix",
			diffs: []diffmatchpatch.Diff{
				{Type: diffmatchpatch.DiffInsert, Text: "a"},
			},
			wantText: "+a",
			wantAttr: map[int]term.Attributes{0: wantAdd},
		},
		{
			name: "mixed operations keep order",
			diffs: []diffmatchpatch.Diff{
				{Type: diffmatchpatch.DiffEqual, Text: " ctx\n"},
				{Type: diffmatchpatch.DiffDelete, Text: "old\n"},
				{Type: diffmatchpatch.DiffInsert, Text: "new\n"},
			},
			wantText: " ctx\n-old\n+new\n",
			wantAttr: map[int]term.Attributes{0: {}, 1: wantDel, 2: wantAdd},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := DiffBuffer(tt.diffs)
			assert.Equal(t, tt.wantText, buf.String())
			// DiffComponent and DiffString share the same buffer path.
			assert.Equal(t, tt.wantText, DiffString(tt.diffs))
			scroll, ok := DiffComponent(tt.diffs).(*component.Scroll)
			require.True(t, ok)
			assert.Equal(t, tt.wantText, scroll.Buffer().String())

			for y, attr := range tt.wantAttr {
				for x := 0; x < buf.Columns(y)-1; x++ {
					c, ok := buf.Cell(term.Coordinates{X: x, Y: y})
					require.Truef(t, ok, "no cell at %d,%d", x, y)
					assert.Equalf(t, attr,
						term.Attributes{Fg: c.Fg, Bg: c.Bg, Attrs: c.Attrs},
						"attributes at %d,%d (%q)", x, y, string(c.Ch))
				}
			}
		})
	}
}
