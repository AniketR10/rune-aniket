package utf8parser

type Receiver interface {
	Codepoint(c rune)
	InvalidSequence()
}

type Parser struct {
	point int
	state
	receiver Receiver
}

func NewParser(receiver Receiver) *Parser {
	return &Parser{point: 0, state: Ground, receiver: receiver}
}

func (p *Parser) Advance(b byte) {
	state, action := p.state.advance(b)
	p.performAction(b, action)
	p.state = state
}

func (p *Parser) performAction(b byte, action Action) {
	switch action {
	case InvalidSequence:
		p.point = 0
		p.receiver.InvalidSequence()
	case EmitByte:
		p.receiver.Codepoint(rune(b))
	case SetByte1:
		point := p.point | int(b&0b0011_1111)
		c := rune(point)
		p.point = 0
		p.receiver.Codepoint(c)
	case SetByte2:
		p.point |= int(b&0b0011_1111) << 6
	case SetByte2Top:
		p.point |= int(b&0b0001_1111) << 6
	case SetByte3:
		p.point |= int(b&0b0011_1111) << 12
	case SetByte3Top:
		p.point |= int(b&0b0000_1111) << 12
	case SetByte4:
		p.point |= int(b&0b0000_0111) << 18
	}
}
