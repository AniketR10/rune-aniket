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

package term

import (
	"context"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// recordingWriter captures the last cell forwarded to it.
type recordingWriter struct {
	pos  term.Coordinates
	cell term.Cell
	attr term.Attributes
}

func (w *recordingWriter) SetCell(pos term.Coordinates, c term.Cell) {
	w.pos = pos
	w.cell = c
}

func (w *recordingWriter) UnionAttributes(pos term.Coordinates, attr term.Attributes) {
	w.pos = pos
	w.attr = attr
}

func (w *recordingWriter) Context() context.Context { return context.Background() }

func gray(c term.Color) term.Color {
	r, g, b := c.RGB()
	lum := int32(math.Round(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)))
	return term.NewRGBColor(lum, lum, lum)
}

func TestBWWriterSetCell(t *testing.T) {
	red := term.NewRGBColor(255, 0, 0)
	blue := term.NewRGBColor(0, 0, 255)

	cases := []struct {
		name   string
		in     term.Cell
		wantFg term.Color
		wantBg term.Color
	}{
		{
			name:   "rgb fg and bg grayscaled",
			in:     term.Cell{Ch: 'x', Attributes: term.Attributes{Fg: red, Bg: blue}},
			wantFg: gray(red),
			wantBg: gray(blue),
		},
		{
			name:   "default colors unchanged",
			in:     term.Cell{Ch: 'x', Attributes: term.Attributes{Fg: term.ColorDefault, Bg: term.ColorDefault}},
			wantFg: term.ColorDefault,
			wantBg: term.ColorDefault,
		},
		{
			name:   "background rune grayscales both fg and bg",
			in:     term.Cell{Ch: '█', Attributes: term.Attributes{Fg: red, Bg: blue}},
			wantFg: gray(red),
			wantBg: gray(blue),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &recordingWriter{}
			BWWriter(rec, term.Attributes{}).SetCell(term.Coordinates{X: 2, Y: 3}, tc.in)
			assert.Equal(t, term.Coordinates{X: 2, Y: 3}, rec.pos)
			assert.Equal(t, tc.wantFg, rec.cell.Attributes.Fg)
			assert.Equal(t, tc.wantBg, rec.cell.Attributes.Bg)
			assert.Equal(t, tc.in.Ch, rec.cell.Ch)
		})
	}
}

// TestBWWriterResolvesDefaultForeground pins that a ColorDefault
// foreground is resolved to the constructor's defaultFg before
// grayscaling in SetCell (every cell has a concrete fg), while a
// ColorDefault foreground in a union overlay stays a no-op so it does
// not repaint cells the caller meant to leave alone.
func TestBWWriterResolvesDefaultForeground(t *testing.T) {
	tan := term.NewRGBColor(210, 180, 140)

	t.Run("SetCell resolves default fg, leaves default bg", func(t *testing.T) {
		rec := &recordingWriter{}
		BWWriter(rec, term.Attributes{Fg: tan}).SetCell(term.Coordinates{X: 4, Y: 5}, term.Cell{
			Ch:         'x',
			Attributes: term.Attributes{Fg: term.ColorDefault, Bg: term.ColorDefault},
		})
		assert.Equal(t, gray(tan), rec.cell.Attributes.Fg)
		assert.Equal(t, term.ColorDefault, rec.cell.Attributes.Bg)
	})

	t.Run("UnionAttributes leaves default fg untouched", func(t *testing.T) {
		rec := &recordingWriter{}
		BWWriter(rec, term.Attributes{Fg: tan}).UnionAttributes(term.Coordinates{}, term.Attributes{
			Fg:    term.ColorDefault,
			Bg:    term.ColorDefault,
			Attrs: term.AttrBold,
		})
		assert.Equal(t, term.ColorDefault, rec.attr.Fg)
		assert.Equal(t, term.ColorDefault, rec.attr.Bg)
		assert.Equal(t, term.AttrBold, rec.attr.Attrs)
	})

	t.Run("UnionAttributes still grayscales an explicit fg", func(t *testing.T) {
		rec := &recordingWriter{}
		red := term.NewRGBColor(255, 0, 0)
		BWWriter(rec, term.Attributes{Fg: tan}).UnionAttributes(term.Coordinates{}, term.Attributes{
			Fg: red,
		})
		assert.Equal(t, gray(red), rec.attr.Fg)
	})
}

// TestBWWriterUnionAttributes guards against syntax-highlight color
// leaking through: highlighters overlay token colors via
// UnionAttributes, so those colors must be grayscaled too.
func TestBWWriterUnionAttributes(t *testing.T) {
	red := term.NewRGBColor(255, 0, 0)
	blue := term.NewRGBColor(0, 0, 255)

	t.Run("rgb colors grayscaled", func(t *testing.T) {
		rec := &recordingWriter{}
		BWWriter(rec, term.Attributes{}).UnionAttributes(term.Coordinates{X: 1, Y: 1},
			term.Attributes{Fg: red, Bg: blue, Attrs: term.AttrBold})
		assert.Equal(t, gray(red), rec.attr.Fg)
		assert.Equal(t, gray(blue), rec.attr.Bg)
		assert.Equal(t, term.AttrBold, rec.attr.Attrs)
	})

	t.Run("default colors pass through", func(t *testing.T) {
		rec := &recordingWriter{}
		BWWriter(rec, term.Attributes{}).UnionAttributes(term.Coordinates{},
			term.Attributes{Fg: term.ColorDefault, Bg: term.ColorDefault, Attrs: term.AttrReverse})
		assert.Equal(t, term.ColorDefault, rec.attr.Fg)
		assert.Equal(t, term.ColorDefault, rec.attr.Bg)
		assert.Equal(t, term.AttrReverse, rec.attr.Attrs)
	})
}
