package parser

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/tcell/v3"
	log "github.com/sirupsen/logrus"
)

// C0 set of 7-bit control characters (from ANSI X3.4-1977).
const (
	c0NUL  = 0x00 // Null filler, terminal should ignore this character.
	c0SOH  = 0x01 // Start of Header.
	c0STX  = 0x02 // Start of Text, implied end of header.
	c0ETX  = 0x03 // End of Text, causes some terminal to respond with ACK or NAK.
	c0EOT  = 0x04 // End of Transmission.
	c0ENQ  = 0x05 // Enquiry, causes terminal to send ANSWER-BACK ID.
	c0ACK  = 0x06 // Acknowledge, usually sent by terminal in response to ETX.
	c0BEL  = 0x07 // Bell, triggers the bell, buzzer, or beeper on the terminal.
	c0BS   = 0x08 // Backspace, can be used to define overstruck characters.
	c0HT   = 0x09 // Horizontal Tabulation, move to next predetermined position.
	c0LF   = 0x0A // Linefeed, move to same position on next line (see also NL).
	c0VT   = 0x0B // Vertical Tabulation, move to next predetermined line.
	c0FF   = 0x0C // Form Feed, move to next form or page.
	c0CR   = 0x0D // Carriage Return, move to first character of current line.
	c0SO   = 0x0E // Shift Out, switch to G1 (other half of character set).
	c0SI   = 0x0F // Shift In, switch to G0 (normal half of character set).
	c0DLE  = 0x10 // Data Link Escape, interpret next control character specially.
	c0XON  = 0x11 // (DC1) Terminal is allowed to resume transmitting.
	c0DC2  = 0x12 // Device Control 2, causes ASR-33 to activate paper-tape reader.
	c0XOFF = 0x13 // (DC2) Terminal must pause and refrain from transmitting.
	c0DC4  = 0x14 // Device Control 4, causes ASR-33 to deactivate paper-tape reader.
	c0NAK  = 0x15 // Negative Acknowledge, used sometimes with ETX and ACK.
	c0SYN  = 0x16 // Synchronous Idle, used to maintain timing in Sync communication.
	c0ETB  = 0x17 // End of Transmission block.
	c0CAN  = 0x18 // Cancel (makes VT100 abort current escape sequence if any).
	c0EM   = 0x19 // End of Medium.
	c0SUB  = 0x1A // Substitute (VT100 uses this to display parity errors).
	c0ESC  = 0x1B // Prefix to an escape sequence.
	c0FS   = 0x1C // File Separator.
	c0GS   = 0x1D // Group Separator.
	c0RS   = 0x1E // Record Separator (sent by VT132 in block-transfer mode).
	c0US   = 0x1F // Unit Separator.
	c0DEL  = 0x7F // Delete, should be ignored by terminal.
)

type driver struct {
	handler Handler
	state   *parserState
}

func newDriver(state *parserState, h Handler) *driver {
	ret := new(driver)
	ret.init(state, h)
	return ret
}

func (p *driver) init(state *parserState, h Handler) {
	p.handler = h
	p.state = state
}

func (p *driver) Print(r rune) {
	p.handler.Input(r)
	p.state.precedingChar = r
}

func (p *driver) Execute(ch byte) {
	switch ch {
	case c0HT:
		p.handler.PutTab()
	case c0BS:
		p.handler.Backspace()
	case c0CR:
		p.handler.CarriageReturn()
	case c0LF, c0VT, c0FF:
		p.handler.Linefeed()
	case c0BEL:
		p.handler.Bell()
	case c0SUB:
		p.handler.Substitute()
	case c0SI:
		p.handler.SetActiveCharset(CharsetIndexG0)
	case c0SO:
		p.handler.SetActiveCharset(CharsetIndexG1)
	default:
		p.log(log.DebugLevel, "unhandled execute byte=%02x", ch)
	}
}

func (p *driver) Hook(params [][]uint16, intermediates []byte, ignore bool, action rune) {
	p.log(log.DebugLevel, "unhandled hook params=%v, ints: %v, ignore: %v, action: %v",
		params, intermediates, ignore, action)
}

func (p *driver) Put(ch byte) {
	p.log(log.DebugLevel, "unhandled put byte=%v", ch)
}

func (p *driver) Unhook() {
	p.log(log.DebugLevel, "unhandled unhook")
}

func (p *driver) OSCDispatch(params [][]byte, bellTerminated bool) {
	terminator := "\x1b\\"
	if bellTerminated {
		terminator = "\x07"
	}

	if len(params) == 0 || len(params[0]) == 0 {
		return
	}

	switch string(params[0]) {
	case "0", "2":
		if len(params) >= 2 {
			var title strings.Builder
			for _, x := range params[1:] {
				title.WriteString(string(x))
				title.WriteByte(';')
			}
			titleStr := strings.TrimSpace(title.String())
			p.handler.SetTitle(titleStr)
			return
		}
		p.logUnhandledOSC(params)

	case "4":
		/* unsupported setting color value of indexed color, or responding to dynamic color query */
	case "8":
		if len(params) > 2 {
			linkParams := params[1]
			var uri strings.Builder

			uri.WriteString(string(params[2]))
			for _, param := range params[3:] {
				uri.WriteByte(';')
				uri.WriteString(string(param))
			}

			if uri.Len() == 0 {
				p.handler.SetHyperlink(nil)
				return
			}

			var id string
			for _, kv := range bytes.Split(linkParams, []byte(":")) {
				if bytes.HasPrefix(kv, []byte("id=")) {
					id = string(kv[3:])
					break
				}
			}

			p.handler.SetHyperlink(&Hyperlink{ID: id, URI: uri.String()})
		}

	case "10", "11", "12":
		/* unsupported setting color value of indexed color, or responding to dynamic color query */
	case "22":
		/* unsupported setting of cursor icon */
	case "50":
		if len(params) >= 2 && len(params[1]) >= 13 &&
			bytes.HasPrefix(params[1], []byte("CursorShape=")) {
			shape := CursorShapeBlock
			switch params[1][12] {
			case '0':
				shape = CursorShapeBlock
			case '1':
				shape = CursorShapeBeam
			case '2':
				shape = CursorShapeUnderline
			default:
				p.logUnhandledOSC(params)
				return
			}
			p.handler.SetCursorShape(shape)
			return
		}
		p.logUnhandledOSC(params)

	case "52":
		if len(params) < 3 {
			p.logUnhandledOSC(params)
			return
		}

		register := int('c')
		if len(params[1]) != 0 {
			register = int(params[1][0])
		}
		switch string(params[2]) {
		case "?":
			p.handler.ClipboardLoad(register, terminator)
		default:
			p.handler.ClipboardStore(register, params[2])
		}

	case "104", "110", "111", "112":
	/* unsupported color CSI dispatch */
	default:
		p.logUnhandledOSC(params)
	}
}

func (p *driver) CSIDispatch(
	params [][]uint16, intermediates []byte,
	hasIgnoredIntermediates bool, action rune,
) {
	if hasIgnoredIntermediates || len(intermediates) > 2 {
		p.logUnhandledCSI(params, intermediates, action)
		return
	}

	handler := p.handler
	nextParamOr := func(defaultValue int) int {
		if len(params) == 0 {
			return defaultValue
		}
		head := params[0]
		params = params[1:]
		if head[0] == 0 {
			return defaultValue
		}
		ret := head[0]
		return int(ret)
	}

	// TODO consider doing the same as ESC where we keep a tight control over intermediates.

	switch action {
	case '@':
		handler.InsertBlank(nextParamOr(1))
	case 'A':
		handler.MoveUp(nextParamOr(1))
	case 'B', 'e':
		handler.MoveDown(nextParamOr(1))
	case 'b':
		if c := p.state.precedingChar; c != 0 {
			count := nextParamOr(1)
			for i := 0; i < count; i++ {
				handler.Input(c)
			}
		} else {
			p.log(log.TraceLevel, "tried to repeat with no preceding char")
		}
	case 'C', 'a':
		handler.MoveForward(nextParamOr(1))
	case 'c':
		if nextParamOr(0) == 0 {
			identifySecondary := len(intermediates) != 0 && intermediates[0] == '>'
			handler.IdentifyTerminal(identifySecondary)
		} else {
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'D':
		handler.MoveBackward(nextParamOr(1))
	case 'd':
		handler.GotoLine(nextParamOr(1) - 1)
	case 'E':
		handler.MoveDownAndCR(nextParamOr(1))
	case 'F':
		handler.MoveUpAndCR(nextParamOr(1))
	case 'G', '`':
		handler.GotoCol(nextParamOr(1) - 1)
	case 'g':
		mode := nextParamOr(0)
		switch mode {
		case 0:
			handler.ClearTabs(TabulationClearModeCurrent)
		case 3:
			handler.ClearTabs(TabulationClearModeAll)
		default:
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'H', 'f':
		y := nextParamOr(1)
		x := nextParamOr(1)
		handler.Goto(y-1, x-1)
	case 'h':
		if len(intermediates) == 1 && intermediates[0] == '?' {
			for _, param := range params {
				if len(param) == 0 {
					p.logUnexpectedParamsLength(params, intermediates, action)
				} else {
					if PrivateMode(param[0]) == PrivateModeSyncUpdate {
						p.state.syncState.timeout.SetTimeout(syncUpdateTimeout)
					}
					p.handler.SetPrivateMode(NewPrivateMode(param[0]))
				}
			}
		} else if len(intermediates) == 0 {
			for _, param := range params {
				if len(param) == 0 {
					p.logUnexpectedParamsLength(params, intermediates, action)
				} else {
					p.handler.SetMode(NewMode(param[0]))
				}
			}
		} else {
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'I':
		handler.MoveForwardTabs(nextParamOr(1))
	case 'J':
		mode := nextParamOr(0)
		switch mode {
		case 0:
			handler.ClearScreen(ClearModeBelow)
		case 1:
			handler.ClearScreen(ClearModeAbove)
		case 2:
			handler.ClearScreen(ClearModeAll)
		case 3:
			handler.ClearScreen(ClearModeSaved)
		default:
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'K':
		mode := nextParamOr(0)
		switch mode {
		case 0:
			handler.ClearLine(LineClearModeRight)
		case 1:
			handler.ClearLine(LineClearModeLeft)
		case 2:
			handler.ClearLine(LineClearModeAll)
		default:
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'L':
		handler.InsertBlankLines(nextParamOr(1))
	case 'l':
		if len(intermediates) == 1 && intermediates[0] == '?' {
			for _, param := range params {
				if len(param) == 0 {
					p.logUnexpectedParamsLength(params, intermediates, action)
				} else {
					p.handler.UnsetPrivateMode(NewPrivateMode(param[0]))
				}
			}
		} else if len(intermediates) == 0 {
			for _, param := range params {
				if len(param) == 0 {
					p.logUnexpectedParamsLength(params, intermediates, action)
				} else {
					p.handler.UnsetMode(NewMode(param[0]))
				}
			}
		} else {
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'M':
		handler.DeleteLines(nextParamOr(1))
	case 'm':
		if len(intermediates) == 1 && intermediates[0] == '>' {
			if nextParamOr(1) == 4 {
				switch nextParamOr(0) {
				case 0:
					p.handler.SetModifyOtherKeys(ModifyOtherKeysReset)
				case 1:
					p.handler.SetModifyOtherKeys(ModifyOtherKeysEnableExceptWellDefined)
				case 2:
					p.handler.SetModifyOtherKeys(ModifyOtherKeysEnableAll)
				default:
					p.logUnhandledCSI(params, intermediates, action)
				}
			} else {
				p.logUnhandledCSI(params, intermediates, action)
			}
		} else if len(intermediates) == 1 && intermediates[0] == '?' {
			if nextParamOr(0) == 4 {
				p.handler.ReportModifyOtherKeys()
			} else {
				p.logUnhandledCSI(params, intermediates, action)
			}
		} else if len(intermediates) == 0 {
			if len(params) == 0 {
				handler.TerminalAttribute(Attr{Type: ResetAttr})
			} else {
				for _, attr := range p.attrsFromSgrParameters(params) {
					handler.TerminalAttribute(attr)
				}
			}
		} else {
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'n':
		handler.DeviceStatus(nextParamOr(0))
	case 'P':
		handler.DeleteChars(nextParamOr(1))
	case 'p':
		switch {
		case len(intermediates) == 1 && intermediates[0] == '$':
			p.handler.ReportMode(NewMode(uint16(nextParamOr(0))))
		case len(intermediates) == 2 && intermediates[0] == '?' && intermediates[1] == '$':
			p.handler.ReportPrivateMode(NewPrivateMode(uint16(nextParamOr(0))))
		default:
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'q':
		if len(intermediates) == 1 && intermediates[0] == ' ' {
			// DECSCUSR (CSI Ps SP q) -- Set Cursor Style.
			i := nextParamOr(0)
			var shape CursorShape
			switch i {
			case 0, 1, 2:
				shape = CursorShapeBlock
			case 3, 4:
				shape = CursorShapeUnderline
			case 5, 6:
				shape = CursorShapeBeam
			default:
				p.logUnhandledCSI(params, intermediates, action)
			}
			blinking := i%2 == 1
			handler.SetCursorStyle(CursorStyle{Shape: shape, Blinking: blinking})
		} else {
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'r':
		top := nextParamOr(1)
		var bottom int
		var end bool
		if len(params) == 1 && len(params[0]) == 1 && params[0][0] != 0 {
			bottom = int(params[0][0])
		} else {
			end = true
		}
		handler.SetScrollingRegion(top, bottom, end)
	case 'S':
		handler.ScrollUp(nextParamOr(1))
	case 's':
		handler.SaveCursorPosition()
	case 'T':
		handler.ScrollDown(nextParamOr(1))
	case 't':
		param := int(nextParamOr(1))
		switch param {
		case 14:
			handler.TextAreaSizePixels()
		case 18:
			handler.TextAreaSizeChars()
		case 22:
			handler.PushTitle()
		case 23:
			handler.PopTitle()
		default:
			p.logUnhandledCSI(params, intermediates, action)
		}
	case 'u':
		if len(intermediates) == 0 {
			handler.RestoreCursorPosition()
		} else {
			switch intermediates[0] {
			case '?':
				handler.ReportKeyboardMode()
			case '=':
				mode := NewKeyboardMode(uint8(nextParamOr(0)))
				var behavior KeyboardModesApplyBehavior
				switch nextParamOr(1) {
				case 3:
					behavior = KeyboardModesApplyBehaviorDifference
				case 2:
					behavior = KeyboardModesApplyBehaviorUnion
				default:
					behavior = KeyboardModesApplyBehaviorReplace
				}
				handler.SetKeyboardMode(mode, behavior)
			case '>':
				handler.PushKeyboardMode(NewKeyboardMode(uint8(nextParamOr(0))))
			case '<':
				// default is 1
				handler.PopKeyboardModes(nextParamOr(1))
			}
		}
	case 'X':
		handler.EraseChars(nextParamOr(1))
	case 'Z':
		handler.MoveBackwardTabs(nextParamOr(1))
	default:
		p.logUnhandledCSI(params, intermediates, action)
	}
}
func (p *driver) configureCharset(charset StandardCharset, intermediates []byte, b byte) {
	index := CharsetIndex(0)
	switch string(intermediates) {
	case "(":
		index = CharsetIndexG0
	case ")":
		index = CharsetIndexG1
	case "*":
		index = CharsetIndexG2
	case "+":
		index = CharsetIndexG3
	default:
		p.logUnhandledESC(intermediates, b)
		return
	}
	p.handler.ConfigureCharset(index, charset)
}

func (p *driver) ESCDispatch(intermediates []byte, ignore bool, b byte) {
	if len(intermediates) == 0 {
		switch b {
		case 'D':
			p.handler.Linefeed()
		case 'E':
			p.handler.Linefeed()
			p.handler.CarriageReturn()
		case 'H':
			p.handler.SetHorizontalTabstop()
		case 'M':
			p.handler.ReverseIndex()
		case 'Z':
			p.handler.IdentifyTerminal(false)
		case 'c':
			p.handler.ResetState()
		case '7':
			p.handler.SaveCursorPosition()
		case '8':
			p.handler.RestoreCursorPosition()
		case '=':
			p.handler.SetKeypadApplicationMode()
		case '>':
			p.handler.UnsetKeypadApplicationMode()
		case '\\':
		// string terminator, do nothing (parser handles as string terminator).
		default:
			p.logUnhandledESC(intermediates, b)
		}
		return
	}

	switch b {
	case 'B':
		p.configureCharset(StandardCharsetASCII, intermediates, b)
	case '0':
		p.configureCharset(StandardCharsetSpecialCharacterAndLineDrawing, intermediates, b)
	case '8':
		if len(intermediates) == 1 && intermediates[0] == '#' {
			p.handler.Decaln()
		} else {
			p.logUnhandledESC(intermediates, b)
		}
	default:
		p.logUnhandledESC(intermediates, b)
	}
}

func (p *driver) attrsFromSgrParameters(params [][]uint16) []Attr {
	attrs := make([]Attr, 0, len(params))

	for i := 0; i < len(params); i++ {
		param := params[i]
		if len(param) == 0 || len(param) > 2 {
			p.log(log.DebugLevel,
				"unhandled sgr parameter in CSI dispatch: params=%v", params)
			continue
		}

		if len(param) == 2 && param[0] == 4 {
			switch param[1] {
			case 0:
				attrs = append(attrs, Attr{Type: CancelUnderlineAttr})
			case 2:
				attrs = append(attrs, Attr{Type: DoubleUnderlineAttr})
			case 3:
				attrs = append(attrs, Attr{Type: UndercurlAttr})
			case 4:
				attrs = append(attrs, Attr{Type: DottedUnderlineAttr})
			case 5:
				attrs = append(attrs, Attr{Type: DashedUnderlineAttr})
			default:
				attrs = append(attrs, Attr{Type: UnderlineAttr})
			}
			continue
		}

		switch param[0] {
		case 0:
			attrs = append(attrs, Attr{Type: ResetAttr})
		case 1:
			attrs = append(attrs, Attr{Type: BoldAttr})
		case 2:
			attrs = append(attrs, Attr{Type: DimAttr})
		case 3:
			attrs = append(attrs, Attr{Type: ItalicAttr})
		case 4:
			attrs = append(attrs, Attr{Type: UnderlineAttr})
		case 5:
			attrs = append(attrs, Attr{Type: BlinkSlowAttr})
		case 6:
			attrs = append(attrs, Attr{Type: BlinkFastAttr})
		case 7:
			attrs = append(attrs, Attr{Type: ReverseAttr})
		case 8:
			attrs = append(attrs, Attr{Type: HiddenAttr})
		case 9:
			attrs = append(attrs, Attr{Type: StrikeAttr})
		case 21:
			attrs = append(attrs, Attr{Type: CancelBoldAttr})
		case 22:
			attrs = append(attrs, Attr{Type: CancelBoldDimAttr})
		case 23:
			attrs = append(attrs, Attr{Type: CancelItalicAttr})
		case 24:
			attrs = append(attrs, Attr{Type: CancelUnderlineAttr})
		case 25:
			attrs = append(attrs, Attr{Type: CancelBlinkAttr})
		case 27:
			attrs = append(attrs, Attr{Type: CancelReverseAttr})
		case 28:
			attrs = append(attrs, Attr{Type: CancelHiddenAttr})
		case 29:
			attrs = append(attrs, Attr{Type: CancelStrikeAttr})
		case 30:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorBlack})
		case 31:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorMaroon})
		case 32:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorGreen})
		case 33:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorOlive})
		case 34:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorNavy})
		case 35:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorPurple})
		case 36:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorTeal})
		case 37:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorSilver})
		case 38, 48, 58:
			var attr AttrType
			switch param[0] {
			case 38:
				attr = ForegroundAttr
			case 48:
				attr = BackgroundAttr
			case 58:
				attr = UnderlineAttr
			}
			if len(param) == 1 {
				// FIXME this needs to consume all the params
				n, color, ok := parseSGRColor(mapParamsToHeadParam(params[i+1:]))
				if ok {
					i += n - 1
					attrs = append(attrs, Attr{Type: attr, Color: color})
					continue
				}
			} else if len(param) > 1 {
				n, color, ok := handleColonRGB(param[1:])
				if ok {
					i += n - 1
					attrs = append(attrs, Attr{Type: attr, Color: color})
					continue
				}
			}
			p.logUnhandledAttribute(params)
		case 39:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorDefault})
		case 40:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorBlack})
		case 41:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorMaroon})
		case 42:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorGreen})
		case 43:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorOlive})
		case 44:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorNavy})
		case 45:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorPurple})
		case 46:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorTeal})
		case 47:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorSilver})
		case 49:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorDefault})
		case 59:
			attrs = append(attrs, Attr{Type: UnderlineColorAttr, Color: tcell.ColorBlack})
		case 90:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorGray})
		case 91:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorRed})
		case 92:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorLime})
		case 93:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorYellow})
		case 94:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorBlue})
		case 95:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorFuchsia})
		case 96:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorAqua})
		case 97:
			attrs = append(attrs, Attr{Type: ForegroundAttr, Color: tcell.ColorWhite})
		case 100:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorGray})
		case 101:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorRed})
		case 102:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorLime})
		case 103:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorYellow})
		case 104:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorBlue})
		case 105:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorFuchsia})
		case 106:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorAqua})
		case 107:
			attrs = append(attrs, Attr{Type: BackgroundAttr, Color: tcell.ColorWhite})
		default:
			p.logUnhandledAttribute(params)
		}
	}

	return attrs
}

func (p *driver) log(level log.Level, msg string, args ...any) {
	log.WithField(logging.KeyClass, "parser.driver").
		Logf(level, msg, args...)
}

func (p *driver) logUnhandledCSI(
	params [][]uint16, intermediates []byte, action rune,
) {
	p.log(log.DebugLevel, "Unhandled CSI action=%q, params=%v, intermediates=%v",
		action, params, intermediates)
}

func (p *driver) logUnexpectedParamsLength(
	params [][]uint16, intermediates []byte, action rune,
) {
	p.log(log.DebugLevel, "Unexpected params length in CSI action=%q, "+
		"params=%v, intermediates=%v", action, params, intermediates)
}

func (p *driver) logUnhandledESC(
	intermediates []byte, b byte,
) {
	p.log(log.DebugLevel, "Unhandled ESC ints=%v, byte=%c", intermediates, b)
}

func (p *driver) logUnhandledOSC(params [][]byte) {
	var buf strings.Builder
	for _, items := range params {
		buf.WriteByte('[')
		for _, item := range items {
			buf.WriteString(strconv.QuoteRune(rune(item)))
		}
		buf.WriteString("],")
	}
	p.log(log.DebugLevel, "unhandled osc_dispatch: [%s]", buf.String())
}

func (p *driver) logUnhandledAttribute(params [][]uint16) {
	p.log(log.DebugLevel, "Unhandled Attribute in CSI dispatch: params=%v", params)
}

// handleColonRGB handles colon separated RGB color escape sequence.
func handleColonRGB(params []uint16) (int, tcell.Color, bool) {
	var rgbStart int
	if len(params) > 4 {
		rgbStart = 2
	} else {
		rgbStart = 1
	}

	newLen := len(params) - rgbStart
	newParams := make([]uint16, newLen)
	copy(newParams, params[rgbStart:])
	n, color, ok := parseSGRColor(newParams)
	if !ok {
		return 0, 0, false
	}
	return n + rgbStart, color, ok
}

// parse a color
func parseSGRColor(params []uint16) (int, tcell.Color, bool) {
	if len(params) == 0 {
		return 0, 0, false
	}
	switch params[0] {
	case 2:
		// rgb color
		var r, g, b int32
		for i := 1; i < len(params) && i < 4; i++ {
			switch i {
			case 1:
				r = int32(params[i])
			case 2:
				g = int32(params[i])
			case 3:
				b = int32(params[i])
			}
		}
		return 4, tcell.NewRGBColor(r, g, b), true
	case 5:
		// indexed color
		if len(params) > 1 {
			return 2, tcell.PaletteColor(int(params[1])), true
		}
		return 1, tcell.ColorBlack, true
	default:
		return 0, 0, false
	}
}

func mapParamsToHeadParam(params [][]uint16) (ret []uint16) {
	for _, param := range params {
		if len(param) != 0 {
			ret = append(ret, param[0])
		}
	}
	return
}
