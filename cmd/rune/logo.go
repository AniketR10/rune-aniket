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
	"image"
)

//go:generate go run ./logogen

// logo is the default Rune logo used by the single-logo wallpaper.
var logo image.Image = logoImageBase

const logoThemeDefault = "default"

// logoVariants maps a GUI theme name to its logo art. Only themes with art
// that differs from the base logo need an entry; every other theme falls back
// to the shared base logo via logoForTheme.
var logoVariants = map[string]image.Image{
	"hopper": logoImageHopper,
}

// logoForTheme returns the logo art for the given theme name, falling back to
// the base logo when the theme has no dedicated art.
func logoForTheme(theme string) image.Image {
	if img, ok := logoVariants[theme]; ok {
		return img
	}
	return logoImageBase
}
