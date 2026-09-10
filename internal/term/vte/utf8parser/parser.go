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

// Receiver gets called every time a sequence of bytes has been processed.
type Receiver interface {
	Codepoint(c rune)
	InvalidSequence()
}

// Parser implements a utf8 parser.
type Parser struct {
	point int
	state
	receiver Receiver
}

// NewParser allocates storage for a Parser and initializes it.
func NewParser(receiver Receiver) *Parser {
	ret := new(Parser)
	ret.Init(receiver)
	return ret
}

// Init initializes this Parser with the given receiver.
func (p *Parser) Init(receiver Receiver) {
	p.receiver = receiver
	p.point = 0
	p.state = ground
}

// Advance advances the parser to the next byte.
func (p *Parser) Advance(b byte) {
	state, action := p.state.advance(b)
	p.performAction(b, action)
	p.state = state
}

func (p *Parser) performAction(b byte, action action) {
	switch action {
	case invalidSequence:
		p.point = 0
		p.receiver.InvalidSequence()
	case emitByte:
		p.receiver.Codepoint(rune(b))
	case setByte1:
		point := p.point | int(b&0b0011_1111)
		c := rune(point)
		p.point = 0
		p.receiver.Codepoint(c)
	case setByte2:
		p.point |= int(b&0b0011_1111) << 6
	case setByte2Top:
		p.point |= int(b&0b0001_1111) << 6
	case setByte3:
		p.point |= int(b&0b0011_1111) << 12
	case setByte3Top:
		p.point |= int(b&0b0000_1111) << 12
	case setByte4:
		p.point |= int(b&0b0000_0111) << 18
	}
}
