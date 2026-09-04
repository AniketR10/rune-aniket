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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestBomb(t *testing.T) {
	t.Run("the smaller the ring start the greater the ring scale is", func(t *testing.T) {
		params := DefaultBombParams()
		sh := Bomb(params, term.Attributes{}).(*bomb)
		ringScale1 := sh.ringScale
		params.RingStart -= 0.1
		sh = Bomb(params, term.Attributes{}).(*bomb)
		ringScale2 := sh.ringScale
		assert.Less(t, ringScale2, ringScale1)
	})

	t.Run("the greater the ring start the smaller the ring scale is", func(t *testing.T) {
		params := DefaultBombParams()
		sh := Bomb(params, term.Attributes{}).(*bomb)
		ringScale1 := sh.ringScale
		params.RingStart += 0.1
		sh = Bomb(params, term.Attributes{}).(*bomb)
		ringScale2 := sh.ringScale
		assert.Greater(t, ringScale2, ringScale1)
	})

	t.Run("the smaller the ring end the smaller the ring scale is", func(t *testing.T) {
		params := DefaultBombParams()
		sh := Bomb(params, term.Attributes{}).(*bomb)
		ringScale1 := sh.ringScale
		params.RingEnd -= 0.1
		sh = Bomb(params, term.Attributes{}).(*bomb)
		ringScale2 := sh.ringScale
		assert.Greater(t, ringScale2, ringScale1)
	})

	t.Run("the greater the ring end the greater the ring scale is", func(t *testing.T) {
		params := DefaultBombParams()
		sh := Bomb(params, term.Attributes{}).(*bomb)
		ringScale1 := sh.ringScale
		params.RingEnd += 0.1
		sh = Bomb(params, term.Attributes{}).(*bomb)
		ringScale2 := sh.ringScale
		assert.Less(t, ringScale2, ringScale1)
	})

}
