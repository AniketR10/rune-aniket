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

package asciiart

import (
	"image"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/internal/cell"
	"unstable.build/rune/internal/component"
)

// NewComponent returns a tui.Component that renders the given image
// as ascii art.
func NewComponent(img image.Image, config Config) tui.Component {
	buf := new(cell.Buffer)
	buf.InitPerformance(100, 200, ' ')
	scroll := new(component.Scroll)
	scroll.InitPerformance(buf)
	return &imgComp{
		img:    img,
		config: config,
		scroll: scroll,
		dirty:  true,
	}
}

type imgComp struct {
	img           image.Image
	config        Config
	scroll        *component.Scroll
	dirty         bool
	width, height int
}

func (c *imgComp) Draw(w term.Writer) {
	if c.dirty {
		c.scroll.Buffer().Reset()
		Encode(c.scroll.Buffer(), c.width, c.height, c.img, c.config)
		c.dirty = false
	}
	c.scroll.Draw(w)
}

func (c *imgComp) Resize(width, height int) {
	c.dirty = true
	c.width = width
	c.height = height
	c.scroll.Resize(width, height)
}
