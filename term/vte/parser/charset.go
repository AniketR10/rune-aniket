// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package parser

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
