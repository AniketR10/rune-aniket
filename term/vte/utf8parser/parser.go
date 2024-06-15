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
