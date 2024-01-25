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

func (p *Parser) performAction(b byte, action Action) {
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
