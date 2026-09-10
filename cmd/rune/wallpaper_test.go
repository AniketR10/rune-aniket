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

package main

import (
	"context"
	"image"
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/rune/internal/cell"
)

var allLogoThemes = []string{
	"default", "furman", "romero", "carmack", "reynolds", "brevik",
	"thompson", "pike", "hopper", "kernighan", "wozniak", "auge",
	"ritchie", "kelleher", "sanfilippo", "greig", "mullen",
}

func TestLogoForThemeReturnsVariant(t *testing.T) {
	for _, theme := range allLogoThemes {
		t.Run(theme, func(t *testing.T) {
			img := logoForTheme(theme)
			require.NotNil(t, img)
			require.Equal(t, image.Rect(0, 0, 304, 304), img.Bounds())
		})
	}
}

func TestLogoForThemeFallsBackToBase(t *testing.T) {
	for _, theme := range []string{"", "default", "carmack", "does-not-exist"} {
		require.Same(t, logoImageBase, logoForTheme(theme))
	}
}

func TestLogoForThemeHopperIsDistinctVariant(t *testing.T) {
	require.Same(t, logoImageHopper, logoForTheme("hopper"))
	require.NotSame(t, logoImageBase, logoForTheme("hopper"))
}

func TestLogoForThemeReturnsStableGlobal(t *testing.T) {
	for _, theme := range allLogoThemes {
		require.Same(t, logoForTheme(theme), logoForTheme(theme))
	}
}

func drawThemedWallpaper(t *testing.T, w *themedWallpaper) {
	t.Helper()
	w.Resize(80, 24)
	writer := cell.NewBufferWriter(context.Background(), 80, 24)
	w.Draw(writer)
}

func TestThemedWallpaperSwapsLogoOnThemeChange(t *testing.T) {
	theme := "carmack"
	w := &themedWallpaper{themeName: func() string { return theme }}

	drawThemedWallpaper(t, w)
	require.Equal(t, "carmack", w.current)
	first := w.child
	require.NotNil(t, first)

	drawThemedWallpaper(t, w)
	require.Same(t, first, w.child, "child should be reused when theme is unchanged")

	theme = "hopper"
	drawThemedWallpaper(t, w)
	require.Equal(t, "hopper", w.current)
	require.NotSame(t, first, w.child, "child should be rebuilt on theme change")
}

func TestThemedWallpaperNilThemeNameUsesDefault(t *testing.T) {
	w := &themedWallpaper{}
	require.Equal(t, logoThemeDefault, w.resolveTheme())

	drawThemedWallpaper(t, w)
	require.Equal(t, logoThemeDefault, w.current)
	require.NotNil(t, w.child)
}
