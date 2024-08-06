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

package utf8parser

type Action int

const (
	invalidSequence Action = iota
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
func (s state) advance(b byte) (state, Action) {
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
