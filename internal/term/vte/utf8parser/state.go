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

package utf8parser

type action int

const (
	invalidSequence action = iota
	emitByte
	setByte1
	setByte2
	setByte2Top
	setByte3
	setByte3Top
	setByte4
)

type state int

const (
	ground state = iota
	tail3
	tail2
	tail1
	u3_2_e0
	u3_2_ed
	utf8_4_3_f0
	utf8_4_3_f4
)

// Advance the parser state.
func (s state) advance(b byte) (state, action) {
	switch s {
	case ground:
		switch {
		case b <= 0x7f:
			return ground, emitByte
		case b >= 0xc2 && b <= 0xdf:
			return tail1, setByte2Top
		case b == 0xe0:
			return u3_2_e0, setByte3Top
		case b >= 0xe1 && b <= 0xec:
			return tail2, setByte3Top
		case b == 0xed:
			return u3_2_ed, setByte3Top
		case b >= 0xee && b <= 0xef:
			return tail2, setByte3Top
		case b == 0xf0:
			return utf8_4_3_f0, setByte4
		case b >= 0xf1 && b <= 0xf3:
			return tail3, setByte4
		case b == 0xf4:
			return utf8_4_3_f4, setByte4
		default:
			return ground, invalidSequence
		}
	case u3_2_e0:
		switch {
		case b >= 0xa0 && b <= 0xbf:
			return tail1, setByte2
		default:
			return ground, invalidSequence
		}
	case u3_2_ed:
		switch {
		case b >= 0x80 && b <= 0x9f:
			return tail1, setByte2
		default:
			return ground, invalidSequence
		}
	case utf8_4_3_f0:
		switch {
		case b >= 0x90 && b <= 0xbf:
			return tail2, setByte3
		default:
			return ground, invalidSequence
		}
	case utf8_4_3_f4:
		switch {
		case b >= 0x80 && b <= 0x8f:
			return tail2, setByte3
		default:
			return ground, invalidSequence
		}
	case tail3:
		switch {
		case b >= 0x80 && b <= 0xbf:
			return tail2, setByte3
		default:
			return ground, invalidSequence
		}
	case tail2:
		switch {
		case b >= 0x80 && b <= 0xbf:
			return tail1, setByte2
		default:
			return ground, invalidSequence
		}
	case tail1:
		switch {
		case b >= 0x80 && b <= 0xbf:
			return ground, setByte1
		default:
			return ground, invalidSequence
		}
	default:
		panic("unknown utf8 parse state")
	}
}
