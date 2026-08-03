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

package gui

import (
	"image"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/benchdraw"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/term/gui/font"
)

// spyEmojiFace is a colorEmojiFace that records the grapheme clusters
// routed to the color path (as strings), so a renderer test can assert
// an emoji cell takes the color path (Glyph is queried for the whole
// cluster) while ordinary text does not.
type spyEmojiFace struct {
	present    map[string]bool
	glyphCalls []string
}

func (s *spyEmojiFace) Has(cluster []rune) bool {
	return s.present[string(cluster)]
}

func (s *spyEmojiFace) Glyph(cluster []rune, cellW, cellH int) (*image.RGBA, bool) {
	s.glyphCalls = append(s.glyphCalls, string(cluster))
	if !s.present[string(cluster)] {
		return nil, false
	}
	img := image.NewRGBA(image.Rect(0, 0, cellW, cellH))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	return img, true
}

func newEmojiTestRenderer(t *testing.T) *renderer {
	t.Helper()
	m, err := font.NewManager(1, 1)
	require.NoError(t, err)
	const w, h = 320, 120
	return newRenderer(w, h, 1, m, 1, 1, false, term.Attributes{}, term.Attributes{})
}

// TestRendererRoutesEmojiToColorPath asserts a cell holding a color
// emoji is rasterized through the color face (Glyph queried for that
// rune) while an adjacent ASCII cell is not, proving the renderer routes
// emoji to the color path and leaves ordinary text on the mask path.
func TestRendererRoutesEmojiToColorPath(t *testing.T) {
	r := newEmojiTestRenderer(t)
	spy := &spyEmojiFace{present: map[string]bool{"😀": true}}
	r.emojiFace = spy

	cells := [][]term.Cell{{
		{Ch: 'A', Width: 1},
		{Ch: '😀', Width: 2},
	}}
	img := ebiten.NewImage(int(r.font.CellSize.X*8)+8, int(r.font.CellSize.Y)+8)

	benchdraw.BeginFrame(t)
	r.Draw(img, cells, false, term.Coordinates{}, term.CursorStyleDefault, 0, 0)
	benchdraw.EndFrame(t)

	assert.Contains(t, spy.glyphCalls, "😀", "emoji cell must reach the color path")
	assert.NotContains(t, spy.glyphCalls, "A", "ASCII must not reach the color path")
}

// TestRendererRoutesClusterToColorPath asserts a cell holding a
// multi-rune emoji (base rune + combining runes) routes the whole
// cluster to the color path, so composed emoji reach the shaper rather
// than only their base rune.
func TestRendererRoutesClusterToColorPath(t *testing.T) {
	r := newEmojiTestRenderer(t)
	family := "👨\u200d👩\u200d👧"
	spy := &spyEmojiFace{present: map[string]bool{family: true}}
	r.emojiFace = spy

	cell := term.Cell{Ch: '👨', Width: 2}
	cell.SetCombining([]rune{'\u200d', '👩', '\u200d', '👧'})
	cells := [][]term.Cell{{cell}}
	img := ebiten.NewImage(int(r.font.CellSize.X*8)+8, int(r.font.CellSize.Y)+8)

	benchdraw.BeginFrame(t)
	r.Draw(img, cells, false, term.Coordinates{}, term.CursorStyleDefault, 0, 0)
	benchdraw.EndFrame(t)

	assert.Contains(t, spy.glyphCalls, family,
		"the full cluster must reach the color path")
	assert.NotContains(t, spy.glyphCalls, "👨",
		"the base rune alone must not be routed")
}

// TestRendererNilEmojiFaceFallsBack asserts that with no color face the
// renderer draws every cell — emoji included — without panicking, i.e.
// the color path degrades cleanly to the monochrome mask path.
func TestRendererNilEmojiFaceFallsBack(t *testing.T) {
	r := newEmojiTestRenderer(t)
	r.emojiFace = nil

	cells := [][]term.Cell{{
		{Ch: '😀', Width: 2},
		{Ch: 'x', Width: 1},
	}}
	img := ebiten.NewImage(int(r.font.CellSize.X*8)+8, int(r.font.CellSize.Y)+8)

	benchdraw.BeginFrame(t)
	assert.NotPanics(t, func() {
		r.Draw(img, cells, false, term.Coordinates{}, term.CursorStyleDefault, 0, 0)
	})
	benchdraw.EndFrame(t)
}
