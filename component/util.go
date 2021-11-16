package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

type stringComp struct {
	width, height int
	cells         [][]term.Cell
	attr          term.Attributes
}

func (s *stringComp) Resize(width, height int) {
	s.width, s.height = width, height
}

func (s *stringComp) Draw(w term.Writer) {
	for y, r := range s.cells {
		if y >= s.height {
			break
		}
		for x, c := range r {
			if x >= s.width {
				break
			}
			if c.Ch == 0 {
				continue
			}
			w.SetCell(term.Coordinates{X: x, Y: y},
				term.Cell{Bg: s.attr.Bg, Fg: s.attr.Fg, Ch: c.Ch})
		}
	}
}

func (s *stringComp) SetAttr(attr term.Attributes) {
	s.attr = attr
}

// used to wrap Background and provide SetAttr to underlying stringComp
type backgroundStrWrapper struct {
	frame bool
	pad   bool
	*Background
}

func (s backgroundStrWrapper) SetAttr(attr term.Attributes) {
	spanContent := s.Background.Content().(*Span).Content()
	if s.frame {
		if s.pad {
			spanContent.(*Frame).Content().(backgroundStrWrapper).SetAttr(attr)
			return
		}
		spanContent.(*Frame).Content().(*stringComp).SetAttr(attr)
		return
	}
	spanContent.(*stringComp).SetAttr(attr)
}

// StringBackgroundAttr converts str into a static tui.Compontent with attr as attributes,
// and uses c as the background rune, battr as its attributes. It processes newlines and so draws
// the string multi line if applicable. It also centers the string vertically
// and horizontally.
func StringBackgroundAttr(
	str string, attr term.Attributes, c rune, battr term.Attributes,
) WithAttributes {
	return stringBackgroundAttrFrame(str, attr, c, battr, FrameCharSet{}, 0, 0)
}

func centerStrComp(
	comp tui.Component, height, width int, background term.Cell,
	frame, pad bool,
) WithAttributes {
	return backgroundStrWrapper{
		frame: frame,
		pad:   pad,
		Background: WithBackground(NewSpan(comp, SpanConfig{
			PadVertical:   -height,
			PadHorizontal: -width,
		}), background),
	}
}

func stringBackgroundAttrFrame(
	str string, attr term.Attributes, c rune, battr term.Attributes,
	frameCharSet FrameCharSet, padWidth, padHeight int,
) WithAttributes {
	var comp WithAttributes

	cells := cell.StringToCells(str)
	comp = &stringComp{cells: cells, attr: attr}

	var width, height int
	for _, row := range cells {
		if len(row) > width {
			width = len(row)
		}
	}

	background := term.Cell{Ch: c, Fg: battr.Fg, Bg: battr.Bg}
	height = len(cells)
	shouldFrame := frameCharSet != (FrameCharSet{})
	shouldPad := padWidth != 0 || padHeight != 0

	if shouldFrame {
		// if inner pad is provided, center text
		if shouldPad {
			comp = centerStrComp(comp, height, width, background, false, false)
		}
		width += 2 + padWidth
		height += 2 + padHeight

		frame := NewFrame(comp)
		frame.FrameCharSet = frameCharSet
		comp = frame
	}

	return centerStrComp(comp, height, width, background, shouldFrame, shouldPad)
}

// StringBackground converts a string into a static tui.Compontent,
// and uses c as the background rune. It processes newlines and so draws
// the string multi line if applicable. It also centers the string vertically
// and horizontally.
func StringBackground(str string, c rune) tui.Component {
	return StringBackgroundAttr(str, term.Attributes{}, c, term.Attributes{})
}

// StringCentered converts a string into a static tui.Compontent.
// It processes newlines and so draws the string multi line if applicable.
// It also centers the string vertically and horizontally.
func StringCentered(str string) tui.Component {
	return StringBackgroundAttr(str, term.Attributes{}, 0, term.Attributes{})
}

// String converts a string into a very efficient top left centered one line tui.Component
// which draws the given string. If the string needs to be centered dynamically,
// or drawn multi-line use StringCentered instead.
func String(str string) tui.Component {
	row := make([]term.Cell, len(str))
	for i, r := range str {
		row[i] = term.Cell{Ch: r}
	}
	return &stringComp{cells: [][]term.Cell{row}}
}

// StringAttr converts a string into a very efficient top left centered multi-line tui.Component
// which draws str along with attr. See String for more information.
func StringAttr(str string, attr term.Attributes) WithAttributes {
	row := make([]term.Cell, len(str))
	for i, r := range str {
		row[i] = term.Cell{Ch: r}
	}
	return &stringComp{cells: [][]term.Cell{row}, attr: attr}
}
