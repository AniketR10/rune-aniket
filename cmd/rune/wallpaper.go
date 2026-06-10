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

package main

import (
	"image"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/asciiart"
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
