// Copyright (C) 2017-2026 The Rune Authors
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

package emoji

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"unstable.build/rune/internal/term/gui/font/builtinfont"
)

func newTestFace(t *testing.T) *Face {
	t.Helper()
	f, err := NewFace(builtinfont.EmojiTTF)
	require.NoError(t, err)
	require.NotNil(t, f)
	return f
}

// TestHasDistinguishesColorEmoji asserts Has is the single source of
// truth for "this rune is a color emoji we can render": true for color
// emoji present in the font, false for ordinary text the monochrome
// path must keep handling.
func TestHasDistinguishesColorEmoji(t *testing.T) {
	f := newTestFace(t)

	for _, r := range []rune{'😀', '🚀', '🎉'} {
		assert.Truef(t, f.Has([]rune{r}), "%c must be a color emoji", r)
	}
	// '❤' (U+2764) has default *text* presentation (width 1), so it must
	// stay on the monochrome path; only its variation-selected form '❤️'
	// is emoji-presented. Colorizing the bare heart would regress
	// ordinary text, so it belongs with the non-emoji runes.
	for _, r := range []rune{'A', '中', ' ', '1', 'z', '❤'} {
		assert.Falsef(t, f.Has([]rune{r}), "%c must not be treated as a color emoji", r)
	}
}

// TestHasComposesMultiRuneClusters asserts a full grapheme cluster —
// skin-tone modifier, ZWJ sequence, or emoji variation selector — is
// recognized as a single color emoji, since the terminal delivers the
// whole cluster in one cell (Ch + Combining). Rendering only the base
// rune would drop the modifier and mangle the emoji.
func TestHasComposesMultiRuneClusters(t *testing.T) {
	f := newTestFace(t)

	colored := [][]rune{
		{'\U0001F91F', '\U0001F3FC'},                                   // rock-on + medium-light skin tone
		{'\U0001F468', '\u200D', '\U0001F469', '\u200D', '\U0001F467'}, // family (ZWJ)
		{'\u2764', '\uFE0F'},                                           // heart + emoji variation selector
		{'\U0001F1EB', '\U0001F1F7'},                                   // regional-indicator flag
	}
	for _, cl := range colored {
		assert.Truef(t, f.Has(cl), "%q must be a color emoji cluster", string(cl))
	}
}

// TestHasRejectsTextClustersWithCombining asserts clusters whose grapheme
// width is 1 stay on the monochrome path even though the font would
// happily return a color bitmap for the base rune. Keycap sequences are
// width 1 (the grid reserves one cell) and, together with the bare heart
// and digits, must not be colorized or ordinary text regresses.
func TestHasRejectsTextClustersWithCombining(t *testing.T) {
	f := newTestFace(t)

	for _, cl := range [][]rune{
		{'1', '\uFE0F', '\u20E3'}, // keycap "1" (width 1)
		{'❤'},                     // bare heart, text presentation
		{'1'},                     // bare digit
	} {
		assert.Falsef(t, f.Has(cl), "%q must not be a color emoji cluster", string(cl))
	}
}

// TestHasRejectsClusterFontCannotLigate asserts a ZWJ sequence the font
// has no single glyph for stays on the monochrome path: shaping yields
// one color bitmap per component, and drawing only the first would
// render a different emoji than the one typed.
func TestHasRejectsClusterFontCannotLigate(t *testing.T) {
	f := newTestFace(t)

	assert.False(t, f.Has([]rune{'\U0001F468', '\u200D', '\U0001F469'}))
}

// systemEmojiFace opens the host's color-emoji font, skipping when there
// is none. The bundled Noto consumes a variation selector while shaping,
// so only a system face exercises fonts that keep it as its own glyph.
func systemEmojiFace(t *testing.T) *Face {
	t.Helper()
	for _, path := range []string{
		"/System/Library/Fonts/Apple Color Emoji.ttc",
		"/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf",
		"/usr/share/fonts/noto/NotoColorEmoji.ttf",
	} {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if f, err := NewFaceFromFile(path); err == nil {
			return f
		}
	}
	t.Skip("no system color-emoji font on this host")
	return nil
}

// TestHasAcceptsClusterWhenFontKeepsIgnorableGlyph asserts an
// emoji-presented cluster is recognized even when the font shapes the
// variation selector into a separate blank glyph rather than consuming
// it. Apple Color Emoji does exactly that, so demanding a single shaped
// glyph pushed every variation-selected emoji down the monochrome path
// on macOS.
func TestHasAcceptsClusterWhenFontKeepsIgnorableGlyph(t *testing.T) {
	f := systemEmojiFace(t)

	for _, cl := range [][]rune{
		{'\U0001F577', '\uFE0F'}, // spider, default text presentation
		{'\u23F1', '\uFE0F'},     // stopwatch
		{'\u2764', '\uFE0F'},     // heart
	} {
		assert.Truef(t, f.Has(cl), "%q must be a color emoji cluster", string(cl))
	}
}

// TestGlyphProducesColorRGBA asserts Glyph returns a decoded,
// downscaled, premultiplied RGBA that is actually multi-color — the
// exact property the monochrome mask path fails to preserve.
func TestGlyphProducesColorRGBA(t *testing.T) {
	f := newTestFace(t)

	const cellW, cellH = 12, 24
	img, ok := f.Glyph([]rune{'😀'}, cellW, cellH)
	require.True(t, ok)
	require.NotNil(t, img)

	// Fits within the cell box and is non-degenerate.
	assert.LessOrEqual(t, img.Bounds().Dx(), cellW)
	assert.LessOrEqual(t, img.Bounds().Dy(), cellH)
	assert.Positive(t, img.Bounds().Dx())
	assert.Positive(t, img.Bounds().Dy())
	// Tightly packed so it can be uploaded as one contiguous run.
	assert.Equal(t, 4*img.Bounds().Dx(), img.Stride)

	var chroma, opaque int
	for i := 0; i+3 < len(img.Pix); i += 4 {
		r, g, b, a := img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3]
		// Premultiplied invariant: no channel may exceed alpha.
		require.LessOrEqual(t, r, a)
		require.LessOrEqual(t, g, a)
		require.LessOrEqual(t, b, a)
		if a > 0 {
			opaque++
		}
		if r != g || g != b {
			chroma++
		}
	}
	assert.Positive(t, opaque, "glyph must have visible pixels")
	assert.Positive(t, chroma, "glyph must be multi-color, not grayscale")
}

// TestGlyphComposesClusterBitmap asserts the composed glyph for a ZWJ
// family sequence differs from the base rune's glyph, proving the shaper
// resolves the whole cluster rather than falling back to the first rune.
func TestGlyphComposesClusterBitmap(t *testing.T) {
	f := newTestFace(t)
	const cellW, cellH = 24, 24

	family := []rune{'👨', '\u200d', '👩', '\u200d', '👧'}
	composed, ok := f.Glyph(family, cellW, cellH)
	require.True(t, ok)
	require.NotNil(t, composed)

	base, ok := f.Glyph([]rune{'👨'}, cellW, cellH)
	require.True(t, ok)

	assert.NotEqual(t, base.Pix, composed.Pix,
		"composed family glyph must differ from the base man glyph")
}

// TestGlyphUnknownRune asserts a rune with no color glyph reports
// absence rather than returning a bogus image, so the renderer can fall
// back to the monochrome path.
func TestGlyphUnknownRune(t *testing.T) {
	f := newTestFace(t)
	img, ok := f.Glyph([]rune{'A'}, 12, 24)
	assert.False(t, ok)
	assert.Nil(t, img)
}

// TestNewFaceRejectsCorruptFont asserts a non-font byte slice fails
// cleanly at construction so the renderer disables the color path
// instead of panicking later.
func TestNewFaceRejectsCorruptFont(t *testing.T) {
	f, err := NewFace([]byte("not a font"))
	assert.Error(t, err)
	assert.Nil(t, f)

	f, err = NewFace(nil)
	assert.Error(t, err)
	assert.Nil(t, f)
}

// TestGlyphResultIsCopy asserts consecutive Glyph calls do not alias one
// shared scratch buffer, since the atlas layer keeps the returned pixels
// while packing later glyphs.
func TestGlyphResultIsCopy(t *testing.T) {
	f := newTestFace(t)
	a, ok := f.Glyph([]rune{'😀'}, 12, 24)
	require.True(t, ok)
	first := append([]byte(nil), a.Pix...)

	b, ok := f.Glyph([]rune{'🚀'}, 12, 24)
	require.True(t, ok)

	assert.NotSame(t, a, b)
	assert.Equal(t, first, a.Pix, "first glyph must be untouched by the second decode")
}

// writeBundledFont writes the embedded Noto Color Emoji bytes to a temp
// file so NewFaceFromFile can be exercised against the file/collection
// path without depending on any system-installed emoji font.
func writeBundledFont(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "NotoColorEmoji.ttf")
	require.NoError(t, os.WriteFile(path, builtinfont.EmojiTTF, 0o600))
	return path
}

// TestNewFaceFromFileLoadsColorFont asserts the file/collection path
// parses a bundled color-emoji font and produces a Face that recognizes
// color emoji, proving the color-bitmap selection accepts a usable face.
func TestNewFaceFromFileLoadsColorFont(t *testing.T) {
	f, err := NewFaceFromFile(writeBundledFont(t))
	require.NoError(t, err)
	require.NotNil(t, f)

	assert.True(t, f.Has([]rune{'😀'}))
	assert.False(t, f.Has([]rune{'A'}))

	img, ok := f.Glyph([]rune{'😀'}, 12, 24)
	require.True(t, ok)
	require.NotNil(t, img)
}

// TestNewFaceFromFileMissing asserts a missing path fails cleanly so the
// resolver can move on to the next candidate or the bundled fallback.
func TestNewFaceFromFileMissing(t *testing.T) {
	f, err := NewFaceFromFile(filepath.Join(t.TempDir(), "does-not-exist.ttf"))
	assert.Error(t, err)
	assert.Nil(t, f)
}

// TestNewFaceFromFileRejectsNonFont asserts a non-font file fails cleanly
// rather than yielding a Face that cannot render color emoji.
func TestNewFaceFromFileRejectsNonFont(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notafont.ttf")
	require.NoError(t, os.WriteFile(path, []byte("not a font"), 0o600))

	f, err := NewFaceFromFile(path)
	assert.Error(t, err)
	assert.Nil(t, f)
}
