package utf8parser

// / Action to take when receiving a byte
// Action to take when receiving a byte
type Action int

// Action constants
const (
	InvalidSequence Action = iota
	EmitByte
	SetByte1
	SetByte2
	SetByte2Top
	SetByte3
	SetByte3Top
	SetByte4
)

// state is the state the parser can be in.
type state int

// state constants
const (
	Ground state = iota
	Tail3
	Tail2
	Tail1
	U3_2_e0
	U3_2_ed
	Utf8_4_3_f0
	Utf8_4_3_f4
)

// Advance the parser state.
func (s state) advance(b byte) (state, Action) {
	switch s {
	case Ground:
		switch {
		case b <= 0x7f:
			return Ground, EmitByte
		case b >= 0xc2 && b <= 0xdf:
			return Tail1, SetByte2Top
		case b == 0xe0:
			return U3_2_e0, SetByte3Top
		case b >= 0xe1 && b <= 0xec:
			return Tail2, SetByte3Top
		case b == 0xed:
			return U3_2_ed, SetByte3Top
		case b >= 0xee && b <= 0xef:
			return Tail2, SetByte3Top
		case b == 0xf0:
			return Utf8_4_3_f0, SetByte4
		case b >= 0xf1 && b <= 0xf3:
			return Tail3, SetByte4
		case b == 0xf4:
			return Utf8_4_3_f4, SetByte4
		default:
			return Ground, InvalidSequence
		}
	case U3_2_e0:
		switch {
		case b >= 0xa0 && b <= 0xbf:
			return Tail1, SetByte2
		default:
			return Ground, InvalidSequence
		}
	case U3_2_ed:
		switch {
		case b >= 0x80 && b <= 0x9f:
			return Tail1, SetByte2
		default:
			return Ground, InvalidSequence
		}
	case Utf8_4_3_f0:
		switch {
		case b >= 0x90 && b <= 0xbf:
			return Tail2, SetByte3
		default:
			return Ground, InvalidSequence
		}
	case Utf8_4_3_f4:
		switch {
		case b >= 0x80 && b <= 0x8f:
			return Tail2, SetByte3
		default:
			return Ground, InvalidSequence
		}
	case Tail3:
		switch {
		case b >= 0x80 && b <= 0xbf:
			return Tail2, SetByte3
		default:
			return Ground, InvalidSequence
		}
	case Tail2:
		switch {
		case b >= 0x80 && b <= 0xbf:
			return Tail1, SetByte2
		default:
			return Ground, InvalidSequence
		}
	case Tail1:
		switch {
		case b >= 0x80 && b <= 0xbf:
			return Ground, SetByte1
		default:
			return Ground, InvalidSequence
		}
	default:
		panic("unknown utf8 parse state")
	}
}
