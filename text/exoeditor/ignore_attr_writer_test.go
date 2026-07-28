// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package exoeditor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// setCellRecorder captures every SetCell call so the test can assert
// which Cell payload reached the underlying writer through
// ignoreAttrWriter.
type setCellRecorder struct {
	calls []setCellCall
}

type setCellCall struct {
	pos  term.Coordinates
	cell term.Cell
}

func (w *setCellRecorder) SetCell(pos term.Coordinates, c term.Cell) {
	w.calls = append(w.calls, setCellCall{pos: pos, cell: c})
}

func (w *setCellRecorder) UnionAttributes(term.Coordinates, term.Attributes) {}
func (w *setCellRecorder) Context() context.Context                          { return context.Background() }

func TestIgnoreAttrWriterSetCellPreservesBgAndReverse(t *testing.T) {
	red := term.NewColor(255, 0, 0)
	blue := term.NewColor(0, 0, 255)

	tests := []struct {
		name     string
		in       term.Attributes
		wantAttr term.AttrMask
		wantBg   term.Color
	}{
		{
			name:     "reverse preserved on its own",
			in:       term.Attributes{Attrs: term.AttrReverse},
			wantAttr: term.AttrReverse,
		},
		{
			name:     "reverse preserved alongside other attrs",
			in:       term.Attributes{Fg: red, Bg: blue, Attrs: term.AttrReverse | term.AttrBold | term.AttrItalic | term.AttrUnderline | term.AttrBlink},
			wantAttr: term.AttrReverse,
			wantBg:   blue,
		},
		{
			name:     "bg preserved without reverse, other attrs dropped",
			in:       term.Attributes{Fg: red, Bg: blue, Attrs: term.AttrBold | term.AttrItalic | term.AttrUnderline | term.AttrBlink | term.AttrDim | term.AttrStrikeThrough},
			wantAttr: 0,
			wantBg:   blue,
		},
		{
			name:     "zero attributes stay zero",
			in:       term.Attributes{},
			wantAttr: 0,
		},
		{
			name:     "fg dropped, bg preserved",
			in:       term.Attributes{Fg: red, Bg: blue},
			wantAttr: 0,
			wantBg:   blue,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := &setCellRecorder{}
			w := ignoreAttrWriter{Writer: rec}

			pos := term.Coordinates{X: 3, Y: 7}
			in := term.NewCell('x', 0, tc.in)
			w.SetCell(pos, in)

			if assert.Len(t, rec.calls, 1) {
				got := rec.calls[0]
				assert.Equal(t, pos, got.pos)
				assert.Equal(t, in.Ch, got.cell.Ch)
				assert.Equal(t, tc.wantAttr, got.cell.Attrs)
				assert.Equal(t, term.Color(0), got.cell.Fg)
				assert.Equal(t, tc.wantBg, got.cell.Bg)
			}
		})
	}
}
