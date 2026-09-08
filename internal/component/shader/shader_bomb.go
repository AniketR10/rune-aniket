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

package shader

import (
	"math"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/component/asciiart"
	"unstable.build/rune/internal/component/shader/shaderutils"
)

// Bomb shows an expansive ring expanding from the center outwards displacing
// the characters under the ring outwards making it look like magnifier.
func Bomb(params BombParams, defaultAttrs term.Attributes) Shader {
	b := &bomb{
		BombParams:  params,
		ringScale:   1.0 / (1 - params.RingStart - (1 - params.RingEnd)),
		defaultAttr: defaultAttrs,
	}

	return b
}

// BombParams defines the parameters used by the Bomb shader.
type BombParams struct {
	RingCol         term.Color
	RingStart       float64
	RingEnd         float64
	CharBandwidth   float64
	ShiftCharacters bool
}

// DefaultBombParams return a set of sane BombParams.
func DefaultBombParams() BombParams {
	return BombParams{
		RingCol:         term.NewRGBColor(255, 0, 0),
		RingStart:       0.5,
		RingEnd:         0.8,
		CharBandwidth:   0.3,
		ShiftCharacters: true,
	}
}

type bomb struct {
	BombParams
	defaultAttr term.Attributes
	ringScale   float64
}

func (s *bomb) Shade(epoch, total int, cells [][]term.Cell) {
	if epoch >= total {
		return
	}

	cAR := 1 / asciiart.HeightToWidthCellAspectRatio
	cARSq := cAR * cAR

	cols := len(cells)
	rows := len(cells[0])

	wh := float64(rows) / 2.0
	hh := float64(cols) / 2.0

	t := (float64(epoch) / float64(total-1))

	// rL would be squared hypothenuse (a² = b² + c²)
	rl := float64(wh*wh) + float64(hh*hh)/cARSq
	rl *= 1.5

	for y, row := range cells {
		for x := range row {
			xx := float64(x)
			yy := float64(y)

			vx := (xx - wh)
			vy := (yy - hh)

			// vL would be squared hypothenuse (a² = b² + c²)
			vl := float64(vx*vx + vy*vy/cARSq)

			// no need to get the real hypothenuses if we only want to compare
			if vl <= (rl * t) {
				// shade the ring using a falloff
				gr := math.Max(0, math.Min(1, (vl/rl)+(2*((1-t)-0.5))))
				res := math.Max(0, math.Min(1, s.ringScale*(gr-(1-s.RingEnd))))
				bg := shaderutils.InterpolateColor(res, cells[y][x].Bg, s.RingCol, s.defaultAttr.Bg)
				cells[y][x].Bg = bg

				// show text in the inner circle from certain threshold
				if s.ShiftCharacters && vl >= (rl*(t-s.CharBandwidth)) {
					i := int(wh + vx)
					j := int(hh + vy)

					// step one unit towards the vector direction. We would add
					// the unit vector int((vx-w_h)/(vx-w_h)) but to avoid the
					// expensive division, since we are stepping only one unit
					// we are only interested in the sign of that vector
					vx_wh := int(vx - wh)
					if vx_wh != 0 {
						if vx_wh > 0 {
							i = i + 1
						} else {
							i = i - 1
						}
					}
					vy_hh := int(vy - hh)
					if vy_hh != 0 {
						if vy_hh > 0 {
							j = j - 1
						} else {
							j = j + 1
						}
					}

					j = int(math.Max(0, math.Min(float64(cols-1), float64(j))))
					if cells[j] == nil {
						continue
					}

					i = int(math.Max(0, math.Min(float64(rows-1), float64(i))))
					if i >= len(cells[j]) {
						continue
					}

					cells[y][x].Ch = cells[j][i].Ch
					cells[y][x].Width = 1
				}
			}
		}
	}
}
