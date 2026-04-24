// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

package shop

import (
	"bytes"
	_ "embed"
	"image"
	"image/png"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/asciiart"
)

//go:embed unstable_build_logo.png
var logoBytes []byte

var logo image.Image

func init() {
	img, err := png.Decode(bytes.NewReader(logoBytes))
	if err != nil {
		panic("shop: decode logo: " + err.Error())
	}
	logo = img
}

// makeWallpaper returns the Unstable Build wallpaper used for empty
// browser windows. Mirrors cmd/rune/wallpaper.go so the storefront
// shares the editor's look.
func makeWallpaper() browser.Wallpaper {
	return browser.Wallpaper{
		NewComponent: func() tui.Component {
			cfg := asciiart.DefaultConfig()
			cfg.Color = true
			cfg.MaintainAspectRatio = true
			cfg.DensityCharacters = "\u2009▓▓▓▓▓▓▓▓▓"
			img := asciiart.NewComponent(logo, cfg)
			return component.NewSpan(img, component.SpanConfig{
				PadHorizontalPerc: 0.4,
				PadVerticalPerc:   0.2,
				ContentAlignment:  component.AlignmentCentered,
			})
		},
	}
}
