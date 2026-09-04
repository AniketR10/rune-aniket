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

package main

import (
	"image"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/browser"
	"unstable.build/rune/component/asciiart"
)

func makeWallpaper() browser.Wallpaper {
	return browser.Wallpaper{
		NewComponent: func() tui.Component {
			return newLogoSpan(logo)
		},
	}
}

// newLogoSpan builds the centered ascii-art span used to render a logo as a
// wallpaper. It is shared by the single-logo and themed wallpapers.
func newLogoSpan(img image.Image) tui.Component {
	cfg := asciiart.DefaultConfig()
	cfg.Color = true
	cfg.MaintainAspectRatio = true
	cfg.DensityCharacters = "\u2009▓▓▓▓▓▓▓▓▓"
	// Uncomment below when debugging glslshader.Burning with the
	// DebugWallpaperBounds flag.
	// cfg.DensityCharacters = "x▓▓▓▓▓▓▓▓▓"
	art := asciiart.NewComponent(img, cfg)
	return component.NewSpan(art, component.SpanConfig{
		PadHorizontalPerc: 0.4,
		PadVerticalPerc:   0.2,
		ContentAlignment:  component.AlignmentCentered,
	})
}

// makeThemedWallpaper builds a wallpaper whose logo art follows the live GUI
// theme reported by themeName. When themeName is nil or returns an empty or
// unknown theme, the default logo is used.
func makeThemedWallpaper(themeName func() string) browser.Wallpaper {
	return browser.Wallpaper{
		NewComponent: func() tui.Component {
			return &themedWallpaper{themeName: themeName}
		},
	}
}

// themedWallpaper renders the logo for the current GUI theme, rebuilding its
// child span only when the resolved theme changes.
type themedWallpaper struct {
	themeName func() string
	current   string
	child     tui.Component
	width     int
	height    int
	drawn     bool
}

func (w *themedWallpaper) resolveTheme() string {
	if w.themeName == nil {
		return logoThemeDefault
	}
	if name := w.themeName(); name != "" {
		return name
	}
	return logoThemeDefault
}

func (w *themedWallpaper) Resize(width, height int) {
	w.width, w.height = width, height
	if w.child != nil {
		w.child.Resize(width, height)
	}
}

func (w *themedWallpaper) Draw(writer term.Writer) {
	theme := w.resolveTheme()
	if w.child == nil || !w.drawn || theme != w.current {
		w.current = theme
		w.child = newLogoSpan(logoForTheme(theme))
		w.child.Resize(w.width, w.height)
		w.drawn = true
	}
	w.child.Draw(writer)
}
