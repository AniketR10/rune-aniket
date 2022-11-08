package color

/*
 * Copyright (c) 2008 Nicholas Marriott <nicholas.marriott@gmail.com>
 * Copyright (c) 2016 Avi Halachmi <avihpit@yahoo.com>
 *
 * Permission to use, copy, modify, and distribute this software for any
 * purpose with or without fee is hereby granted, provided that the above
 * copyright notice and this permission notice appear in all copies.
 *
 * THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES
 * WITH REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF
 * MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR
 * ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
 * WHATSOEVER RESULTING FROM LOSS OF MIND, USE, DATA OR PROFITS, WHETHER
 * IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING
 * OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

 * Ported from tmux/colour.c
 */

import (
	"unstable.build/go-tui/term"
)

func colorTo6Cube(v uint8) int {
	if v < 48 {
		return 0
	}
	if v < 114 {
		return 1
	}
	return int(((v - 35) / 40))
}

func colorDistanceSq(R, G, B, r, g, b int) int {
	return ((R-r)*(R-r) + (G-g)*(G-g) + (B-b)*(B-b))
}

// RGBToAttribute converts an RGB triplet to the xterm(1) 256 color palette.
//
// xterm provides a 6x6x6 colour cube (16 - 231) and 24 greys (232 - 255). We
// map our RGB colour to the closest in the cube, also work out the closest
// grey, and use the nearest of the two.
//
// Note that the xterm has much lower resolution for darker colours (they are
// not evenly spread out), so our 6 levels are not evenly spread: 0x0, 0x5f
// (95), 0x87 (135), 0xaf (175), 0xd7 (215) and 0xff (255). Greys are more
// evenly spread (8, 18, 28 ... 238).
func RGBToAttribute(r, g, b uint8) term.Attribute {
	type rgb struct {
		r, g, b uint8
	}
	// map the first 8 colors manually
	switch (rgb{r, g, b}) {
	case rgb{0, 0, 0}:
		return term.ColorBlack
	case rgb{255, 0, 0}:
		return term.ColorRed
	case rgb{0, 255, 0}:
		return term.ColorGreen
	case rgb{255, 255, 0}:
		return term.ColorYellow
	case rgb{0, 0, 255}:
		return term.ColorBlue
	case rgb{255, 0, 255}:
		return term.ColorMagenta
	case rgb{0, 255, 255}:
		return term.ColorCyan
	case rgb{255, 255, 255}:
		return term.ColorWhite
	default:
		return rgbToAttribute(r, g, b)
	}
}

func rgbToAttribute(r, g, b uint8) term.Attribute {
	var q2c [6]int = [6]int{0x00, 0x5f, 0x87, 0xaf, 0xd7, 0xff}
	var qr, qg, qb, cr, cg, cb, d, idx int
	var grey_avg, grey_idx, grey int
	qr = colorTo6Cube(r)
	cr = q2c[qr]
	qg = colorTo6Cube(g)
	cg = q2c[qg]
	qb = colorTo6Cube(b)
	cb = q2c[qb]

	// return early if we have hit the exact color
	if cr == int(r) && cg == int(g) && cb == int(b) {
		return term.Attribute((16 + (36 * qr) + (6 * qg) + qb)) + 1
	}

	// Work out the closest grey (average of RGB).
	grey_avg = int((r + g + b) / 3)
	if grey_avg > 238 {
		grey_idx = 23
	} else {
		grey_idx = (grey_avg - 3) / 10
	}
	grey = 8 + (10 * grey_idx)

	// Is grey or 6x6x6 colour closest?
	d = colorDistanceSq(cr, cg, cb, int(r), int(g), int(b))
	if colorDistanceSq(grey, grey, grey, int(r), int(g), int(b)) < d {
		idx = 232 + grey_idx
	} else {
		idx = 16 + (36 * qr) + (6 * qg) + qb
	}
	return term.Attribute(idx) + 1
}
