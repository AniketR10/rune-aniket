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

package main

import (
	"context"
	"image"
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
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
