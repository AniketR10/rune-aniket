package main

import (
	"strconv"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler/search"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text"
	"github.com/ernestrc/go-tui/workspace"
)

type mode uint8

const (
	normalMode = iota
	logsMode
)

var (
	defaultTextAttr        = term.Attributes{}
	defaultPinAttr         = term.Attributes{Fg: term.AttrBold}
	defaultMatchedTextAttr = term.Attributes{Fg: term.ColorRed}
)

type logsHandler struct {
	mode
	ed             tui.Handler
	width, height  int
	moveMultiplier []rune

	l struct {
		*search.List
		component.Virtual
	}

	matchedTextAttr term.Attributes
	pinAttr         term.Attributes
	textAttr        term.Attributes

	scroll int
	pinned component.Virtual
}

func newLogsHandler(
	l *search.List, textAttr, matchedTextAttr, pinAttr *term.Attributes,
) tui.Handler {
	ret := new(logsHandler)

	ret.l.List = l
	ret.l.C = ret.l.List
	if matchedTextAttr == nil {
		matchedTextAttr = &defaultMatchedTextAttr
	}
	ret.matchedTextAttr = *matchedTextAttr
	if pinAttr == nil {
		pinAttr = &defaultPinAttr
	}
	ret.pinAttr = *pinAttr
	if textAttr == nil {
		textAttr = &defaultTextAttr
	}
	ret.textAttr = *textAttr

	ret.pinned.C = component.StringResponsive("", component.StringConfig{})

	buf := l.Buffer()
	ret.ed, _ = text.SimpleEditor(true).Edit(workspace.URI{}, buf)

	ret.l.C = ret.withBackground(ret.l.C)

	return ret
}

func (s *logsHandler) Handle(ev term.Event) (exit, handled bool) {
	switch s.mode {
	case normalMode:
		return s.handleNormal(ev)
	case logsMode:
		return s.handleFilter(ev)
	default:
		panic("unknown mode")
	}
}

func (s *logsHandler) positionPinned() {
	pos := term.Coordinates{Y: s.l.FocusOffset() - s.l.Offset()}
	s.pinned.Move(pos)
}

func (s *logsHandler) setTokenAttr(
	match search.Match, offset int, attrSetter *component.AttrSetter, attr term.Attributes,
) {
	for _, idx := range match.Tokens() {
		attrSetter.SetAttrAt(term.Coordinates{X: idx - offset}, attr)
	}
}

func (s *logsHandler) newPinnedResponsive(match search.Match) {
	cfg := s.defaultStringConfig()

	str := component.StringResponsive(string(match.Data()), cfg)
	attrSetter := component.WithAttrSetter(str)
	s.setTokenAttr(match, 0, attrSetter, s.matchedTextAttr)
	comp := s.withBackground(attrSetter)
	s.pinned.C = comp

	height := str.Height(s.width)
	s.pinned.Resize(s.width, height)
	s.positionPinned()
}

func (s *logsHandler) withBackground(comp tui.Component) tui.Component {
	return component.WithBackground(comp,
		term.Cell{Ch: ' ', Fg: s.textAttr.Fg, Bg: s.textAttr.Bg})
}

func (s *logsHandler) newScrolled(match search.Match) {
	cfg := s.defaultStringConfig()

	str := component.StringResponsive(string(match.Data()[s.scroll:]), cfg)
	attrSetter := component.WithAttrSetter(str)
	s.setTokenAttr(match, s.scroll, attrSetter, s.matchedTextAttr)
	comp := s.withBackground(attrSetter)
	s.pinned.C = comp

	height := s.l.ElementHeight()
	s.pinned.Resize(s.width, height)
	s.positionPinned()
}

func (s *logsHandler) defaultStringConfig() component.StringConfig {
	return component.StringConfig{
		Alignment:      component.SpanAlignmentLeft,
		BackgroundRune: ' ',
		Attributes:     s.pinAttr,
	}
}

func (s *logsHandler) newScrolledRight(match search.Match) bool {
	data := match.Data()
	if len(data) < s.width || s.scroll == len(data)-1-s.width {
		return false
	}
	s.scroll++
	s.newScrolled(match)
	return true
}
func (s *logsHandler) newScrolledLeft(match search.Match) bool {
	if s.scroll <= 1 {
		s.resetPinned()
		return false
	}
	s.scroll--
	s.newScrolled(match)
	return true
}

func (s *logsHandler) scrollLeft() bool {
	data, ok := s.l.Focus()
	if ok {
		return s.newScrolledLeft(data)
	}
	return false
}

func (s *logsHandler) scrollRight() bool {
	data, ok := s.l.Focus()
	if ok {
		return s.newScrolledRight(data)
	}
	return false
}

func (s *logsHandler) resetPinned() {
	s.scroll = 0
	s.pinned.Resize(0, 0)
}

func (s *logsHandler) setFilterMode() {
	s.mode = logsMode
	s.resetPinned()
}

func (s *logsHandler) moveMultiply() int {
	if len(s.moveMultiplier) == 0 {
		return 1
	}
	n, _ := strconv.Atoi(string(s.moveMultiplier))
	return n
}

func (s *logsHandler) focusUp() bool {
	ret := s.l.FocusUp()
	s.resetPinned()
	return ret
}

func (s *logsHandler) focusDown() bool {
	ret := s.l.FocusDown()
	s.resetPinned()
	return ret
}

func (s *logsHandler) focusStart() bool {
	ret := s.l.FocusStart()
	s.resetPinned()
	return ret
}

func (s *logsHandler) focusEnd() bool {
	ret := s.l.FocusEnd()
	s.resetPinned()
	return ret
}

func (s *logsHandler) handleNormal(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return
	}

	switch ev.Key {
	case term.KeyTab:
		s.toggleCaseSensitivity()
		handled = true
	case term.KeyArrowRight:
		handled = s.scrollRight()
	case term.KeyArrowLeft:
		handled = s.scrollLeft()
	case term.KeyEnter:
		s.l.Wait()
		data, ok := s.l.Focus()
		if ok {
			handled = true
			s.newPinnedResponsive(data)
		}
	case term.KeyCtrlJ, term.KeyArrowDown:
		handled = s.focusDown()
	case term.KeyCtrlK, term.KeyArrowUp:
		handled = s.focusUp()
	default:
		multiplier := s.moveMultiply()
		switch ev.Ch {
		case '/':
			handled = true
			s.setFilterMode()
		case 'g':
			handled = s.focusStart()
		case 'G':
			handled = s.focusEnd()
		case 'l':
			handled = s.scrollRight()
		case 'h':
			handled = s.scrollLeft()
		case '0':
			for handled = true; handled; handled = s.scrollLeft() {
			}
		case '$':
			for handled = true; handled; handled = s.scrollRight() {
			}
		case 'k':
			ok := true
			for i := 0; ok && i < multiplier; i++ {
				ok = s.focusUp()
				if !ok {
					break
				}
				handled = true
			}
		case 'j':
			ok := true
			for i := 0; ok && i < multiplier; i++ {
				ok = s.focusDown()
				if !ok {
					break
				}
				handled = true
			}
		default:
			if ev.Ch >= '0' && ev.Ch <= '9' {
				s.moveMultiplier = append(s.moveMultiplier, ev.Ch)
				return
			}
		}
	}
	s.moveMultiplier = s.moveMultiplier[:0]

	return
}

func (s *logsHandler) toggleCaseSensitivity() {
	s.l.ToggleCaseSensitivity()
}

func (s *logsHandler) handleFilter(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return
	}
	switch ev.Key {
	case term.KeyCtrlJ, term.KeyArrowDown:
		handled = s.focusDown()
	case term.KeyCtrlK, term.KeyArrowUp:
		handled = s.focusUp()
	case term.KeyEnter:
		s.mode = normalMode
		handled = true
	case term.KeyTab:
		s.toggleCaseSensitivity()
		handled = true
	case term.KeyBackspace, term.KeyBackspace2:
		_, handled = s.currentEd().Handle(ev)
		if handled {
			return
		}
		fallthrough
	case term.KeyEsc:
		handled = true
		s.mode = normalMode
		s.l.Buffer().Reset()
	default:
		_, handled = s.currentEd().Handle(ev)
	}
	return
}

func (s *logsHandler) Cursor() (term.Coordinates, bool) {
	if s.mode == normalMode {
		return term.Coordinates{}, false
	}
	c, ok := s.currentEd().Cursor()
	// NOTE: assumes bottomSearchBar is true
	c.Y += (s.l.Height() - s.l.InputHeight())
	return c, ok
}

func (s *logsHandler) Man() tui.Manual {
	panic("TODO")
}

func (s *logsHandler) currentEd() tui.Handler {
	return s.ed
}

func (s *logsHandler) Resize(width, height int) {
	s.width, s.height = width, height
	s.l.Virtual.Resize(width, height)
	inputHeight := s.l.InputHeight()
	s.ed.Resize(width, inputHeight)
	s.resetPinned()
}
func (s *logsHandler) Draw(w term.Writer) {
	s.l.Virtual.Draw(w)
	s.pinned.Draw(w)
}
