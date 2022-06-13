package termutil

import (
	"bufio"
	"io"

	log "github.com/sirupsen/logrus"
)

func (t *Terminal) handleANSI() (renderRequired, exit bool) {
	// if the byte is an escape character, read the next byte to determine which one
	r, _, err := t.reader.ReadRune()
	if err == io.EOF {
		return false, true
	}

	// log.Tracef("Terminal.handleANSI: %c 0x%X", r.Rune, r.Rune)

	t.mu.Lock()
	defer t.mu.Unlock()

	switch r {
	case '[':
		return t.handleCSI()
	case ']':
		return t.handleOSC()
	case '(':
		return t.handleSCS0() // select character set into G0
	case ')':
		return t.handleSCS1() // select character set into G1
	case '*':
		return swallowHandler(1, t.reader) // character set bullshit
	case '+':
		return swallowHandler(1, t.reader) // character set bullshit
	case '>':
		return false, false // numeric char selection
	case '=':
		return false, false // alt char selection
	case '7':
		t.GetActiveBuffer().saveCursor()
	case '8':
		t.GetActiveBuffer().restoreCursor()
	case 'D':
		t.GetActiveBuffer().index()
	case 'E':
		t.GetActiveBuffer().newLineEx(true)
	case 'H':
		t.GetActiveBuffer().tabSetAtCursor()
	case 'M':
		t.GetActiveBuffer().reverseIndex()
	case 'P': // sixel
		return false, false
	case 'c':
		t.GetActiveBuffer().clear()
	case '#':
		return t.handleScreenState()
	case '^':
		return t.handlePrivacyMessage()
	default:
		log.Warnf("UNKNOWN ESCAPE SEQUENCE: 0x%X", r)
		return false, false
	}

	return true, false
}

func swallowHandler(size int, reader *bufio.Reader) (bool, bool) {
	for i := 0; i < size; i++ {
		_, _, err := reader.ReadRune()
		if err == io.EOF {
			return false, true
		}
	}
	return false, false
}

func (t *Terminal) handleScreenState() (bool, bool) {
	r, _, err := t.reader.ReadRune()
	if err == io.EOF {
		return false, true
	}
	switch r {
	case '8': // DECALN -- Screen Alignment Pattern

		// hide cursor?
		buffer := t.GetActiveBuffer()
		buffer.resetVerticalMargins(uint(buffer.viewHeight))
		buffer.SetScrollOffset(0)

		// Fill the whole screen with E's
		count := buffer.ViewHeight() * buffer.ViewWidth()
		for count > 0 {
			buffer.write(MeasuredRune{Rune: 'E', Width: 1})
			count--
			if count > 0 && !buffer.modes.AutoWrap && count%buffer.ViewWidth() == 0 {
				buffer.index()
				buffer.carriageReturn()
			}
		}
		// restore cursor
		buffer.setPosition(0, 0)
	default:
		return false, false
	}
	return true, false
}

func (t *Terminal) handlePrivacyMessage() (bool, bool) {
	isEscaped := false
	for {
		r, _, err := t.reader.ReadRune()
		if err == io.EOF {
			return false, true
		}
		if r == 0x18 /*CAN*/ || r == 0x1a /*SUB*/ || (r == 0x5c /*backslash*/ && isEscaped) {
			break
		}
		if isEscaped {
			isEscaped = false
		} else if r == 0x1b {
			isEscaped = true
			continue
		}
	}
	return false, false
}
