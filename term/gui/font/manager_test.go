// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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

package font

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadDefaultFont(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)
	assert.NotPanics(t, func() {
		assert.NotNil(t, m.RegularFontFace())
	})
}

func TestPixelAndCellCalculation(t *testing.T) {
	t.Run("doesn't panic", func(t *testing.T) {
		m, err := NewManager(100, 2)
		require.NoError(t, err)
		assert.NotPanics(t, func() {
			m.PixelX(-10)
			m.PixelY(-10)
			m.CellX(-10)
			m.CellY(-10)
		})
	})

	t.Run("PixelX/Y returns the pixel corresponding to the given cell", func(t *testing.T) {
		m, err := NewManager(2, 1)
		require.NoError(t, err)
		setTestCharSize(m, 21, 43)
		assert.NotNil(t, m.RegularFontFace())
		assert.Equal(t, float64(190), m.PixelX(10))
		assert.Equal(t, float64(420), m.PixelY(10))
	})

	t.Run("CellX/Y returns the cell corresponding to the given pixel", func(t *testing.T) {
		m, err := NewManager(2, 1)
		require.NoError(t, err)
		setTestCharSize(m, 21, 43)
		assert.NotNil(t, m.RegularFontFace())
		assert.Equal(t, 10, m.CellX(190))
		assert.Equal(t, 10, m.CellY(420))
	})
}

// TestSetOffsetXYMixup is a regression test for RUNE-51. Manager.SetOffset
// previously forwarded the y argument to setOffsetX, silently corrupting the
// horizontal offset whenever SetOffset was called.
func TestSetOffsetXYMixup(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)
	// ensure font is loaded so SetOffset's ReloadFont path is exercised.
	require.NotNil(t, m.RegularFontFace())

	const wantX = 3.0
	const wantY = 7.0
	require.NoError(t, m.SetOffset(wantX, wantY))

	assert.Equal(t, wantX, fixedToFloat64(m.offset.X),
		"SetOffset must store the x argument in offset.X, not the y argument")
	assert.Equal(t, wantY, fixedToFloat64(m.offset.Y),
		"SetOffset must store the y argument in offset.Y")
}

func TestSetOffsetAxisPreservesOtherAxis(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)
	// ensure font is loaded so SetOffset's ReloadFont path is exercised.
	require.NotNil(t, m.RegularFontFace())

	require.NoError(t, m.SetOffset(3, 7))
	require.NoError(t, m.SetOffsetX(5))
	assert.Equal(t, 5.0, fixedToFloat64(m.offset.X))
	assert.Equal(t, 7.0, fixedToFloat64(m.offset.Y))

	require.NoError(t, m.SetOffsetY(9))
	assert.Equal(t, 5.0, fixedToFloat64(m.offset.X))
	assert.Equal(t, 9.0, fixedToFloat64(m.offset.Y))
}

func TestCellWidthOffsetPreservesLineHeightOffset(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)
	// ensure font is loaded so SetOffset's ReloadFont path is exercised.
	require.NotNil(t, m.RegularFontFace())

	require.NoError(t, m.SetOffset(3, 7))
	require.NoError(t, m.IncreaseCellWidth())
	assert.Equal(t, 4.0, fixedToFloat64(m.offset.X))
	assert.Equal(t, 7.0, fixedToFloat64(m.offset.Y))

	require.NoError(t, m.DecreaseCellWidth())
	assert.Equal(t, 3.0, fixedToFloat64(m.offset.X))
	assert.Equal(t, 7.0, fixedToFloat64(m.offset.Y))
}

// TestSetOffsetInvalidMetricsRollsBack is a regression test for RUNE-51.
// When the new offset produces degenerate glyph metrics (charSize.X <= 0),
// SetOffset must return an error and restore the previous offset by
// reloading the font normally, leaving the manager in a consistent state.
func TestSetOffsetInvalidMetricsRollsBack(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)
	require.NotNil(t, m.RegularFontFace())

	goodCharSize := m.CharSize()
	prevOffsetX := fixedToFloat64(m.offset.X)
	prevOffsetY := fixedToFloat64(m.offset.Y)

	// An offset more negative than the widest glyph advance collapses
	// charSize.X to zero, which is rejected by setFaceMetrics.
	err = m.SetOffset(-1000, 0)
	require.Error(t, err,
		"SetOffset must surface the invalid-metrics error instead of panicking")
	assert.Contains(t, err.Error(), "invalid font metrics")

	// Offset must be rolled back to the previous value.
	assert.Equal(t, prevOffsetX, fixedToFloat64(m.offset.X),
		"offset.X must be restored after rollback")
	assert.Equal(t, prevOffsetY, fixedToFloat64(m.offset.Y),
		"offset.Y must be restored after rollback")

	// Metrics must be restored to the previously-known-good values, so
	// subsequent CellsWidth/Height calls are safe.
	assert.Equal(t, goodCharSize, m.CharSize(),
		"charSize must be restored after rollback")
	assert.NotPanics(t, func() {
		_ = m.CellsWidth(1200)
		_ = m.CellsHeight(900)
	})
}

// TestSetFontByFamilyNameRestoresOnError is a regression test for
// RUNE-51. When loading a font by name fails (e.g. an unknown family),
// the manager must restore the previously configured font by running
// the normal load path, not leave itself in a half-initialized state.
func TestSetFontByFamilyNameRestoresOnError(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)
	require.NotNil(t, m.RegularFontFace())
	goodCharSize := m.CharSize()

	err = m.SetFontByFamilyName("this-font-family-does-not-exist-RUNE-51")
	require.Error(t, err)

	// Manager must remain usable with the previous (builtin) font.
	assert.Equal(t, "", m.FontFamily(),
		"family must be rolled back to builtin fallback")
	assert.Equal(t, goodCharSize, m.CharSize(),
		"charSize must be the builtin fallback's metrics after rollback")
	assert.NotPanics(t, func() {
		_ = m.CellsWidth(1200)
		_ = m.CellsHeight(900)
	})
}

func setTestCharSize(m *Manager, x, y float64) {
	m.ensureFontLoaded()
	m.charSize.X = x
	m.charSize.Y = y
}

// TestSymbolFallbackResolvesGapGlyphs verifies that glyphs Claude Code
// emits which are absent from the builtin user font, the braille font,
// and the Meslo fallback (e.g. ⏺ ⏸ ※ ⑂) are still resolved by the
// embedded Symbola symbol fallback, so they never render as tofu
// regardless of the user-selected font.
func TestSymbolFallbackResolvesGapGlyphs(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)
	face := m.RegularFontFace()
	require.NotNil(t, face)

	gaps := map[string]rune{
		"BLACK_CIRCLE_MAC (U+23FA)": '⏺',
		"PAUSE (U+23F8)":            '⏸',
		"REFERENCE_MARK (U+203B)":   '※',
		"FORK (U+2442)":             '⑂',
	}
	for name, r := range gaps {
		_, ok := face.GlyphAdvance(r)
		assert.True(t, ok, "glyph %s must resolve through the font fallback chain", name)
	}
}

func TestDefaultSizeForScale(t *testing.T) {
	cases := []struct {
		scale float64
		want  float64
	}{
		{0.75, 17},
		{1.0, 17},
		{1.5, 15},
		{1.75, 14},
		{2.0, 13},
		{3.0, 13},
	}
	for _, tc := range cases {
		assert.InDelta(t, tc.want, defaultSizeForScale(tc.scale), 1e-9,
			"scale %v", tc.scale)
	}
}

func TestSetSizeZeroResolvesDefaultForDeviceScale(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)

	m.SetDeviceScale(1)
	require.NoError(t, m.SetSize(0))
	assert.Equal(t, float64(17), m.size)

	m.SetDeviceScale(2)
	require.NoError(t, m.SetSize(0))
	assert.Equal(t, float64(13), m.size)
}

func TestSetSizeExplicitIgnoresDeviceScale(t *testing.T) {
	m, err := NewManager(0, 0)
	require.NoError(t, err)

	m.SetDeviceScale(1)
	require.NoError(t, m.SetSize(13))
	assert.Equal(t, float64(13), m.size)
}
