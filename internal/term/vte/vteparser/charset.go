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

package vteparser

// StandardCharset represents an optional mapping of certain character representations.
type StandardCharset int

const (
	// StandardCharsetASCII does no mapping of characters.
	StandardCharsetASCII StandardCharset = iota
	// StandardCharsetSpecialCharacterAndLineDrawing maps certain
	// characters to a different representation.
	StandardCharsetSpecialCharacterAndLineDrawing
)

// Map maps c to its StandardCharset representation.
func (charset StandardCharset) Map(c rune) rune {
	switch charset {
	case StandardCharsetASCII:
		return c
	case StandardCharsetSpecialCharacterAndLineDrawing:
		switch c {
		case '_':
			return ' '
		case '`':
			return '◆'
		case 'a':
			return '▒'
		case 'b':
			return '\u2409' // Symbol for horizontal tabulation
		case 'c':
			return '\u240c' // Symbol for form feed
		case 'd':
			return '\u240d' // Symbol for carriage return
		case 'e':
			return '\u240a' // Symbol for line feed
		case 'f':
			return '°'
		case 'g':
			return '±'
		case 'h':
			return '\u2424' // Symbol for newline
		case 'i':
			return '\u240b' // Symbol for vertical tabulation
		case 'j':
			return '┘'
		case 'k':
			return '┐'
		case 'l':
			return '┌'
		case 'm':
			return '└'
		case 'n':
			return '┼'
		case 'o':
			return '⎺'
		case 'p':
			return '⎻'
		case 'q':
			return '─'
		case 'r':
			return '⎼'
		case 's':
			return '⎽'
		case 't':
			return '├'
		case 'u':
			return '┤'
		case 'v':
			return '┴'
		case 'w':
			return '┬'
		case 'x':
			return '│'
		case 'y':
			return '≤'
		case 'z':
			return '≥'
		case '{':
			return 'π'
		case '|':
			return '≠'
		case '}':
			return '£'
		case '~':
			return '·'
		default:
			return c
		}
	default:
		return c
	}
}
