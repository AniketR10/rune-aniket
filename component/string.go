package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// StringConfig defines options for StringConfig and StringResponsive
// constructors.
type StringConfig struct {
	Alignment
	term.Attributes
	FrameCharSet
	BackgroundAttributes term.Attributes
	BackgroundRune       rune
}

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

func withBackgroundWrapper(
	comp tui.Component, height, width int, background term.Cell,
	frame, pad bool, alignment Alignment,
) WithAttributes {
	return backgroundStrWrapper{
		frame: frame,
		pad:   pad,
		Background: WithBackground(NewSpan(comp, SpanConfig{
			PadVertical:      -height,
			PadHorizontal:    -width,
			ContentAlignment: alignment,
		}), background),
	}
}

func newStringComp(
	cells [][]term.Cell, attr term.Attributes, c rune, battr term.Attributes,
	frameCharSet FrameCharSet, padWidth, padHeight int, alg Alignment,
) WithAttributes {
	var comp WithAttributes

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
			comp = withBackgroundWrapper(comp, height, width, background, false, false, alg)
		}
		width += 2 + padWidth
		height += 2 + padHeight

		frame := NewFrame(comp)
		frame.Attributes = attr
		frame.FrameCharSet = frameCharSet
		comp = frame
	}

	return withBackgroundWrapper(comp, height, width, background, shouldFrame, shouldPad, alg)
}

// StringWithConfig converts a string into a static tui.Compontent with
// background/foreground attributes, content alignment and a frame,
// all configurable through cfg. The returned component is significantly
// slower to Draw and Resize than the component returned by String.
func StringWithConfig(str string, cfg StringConfig) WithAttributes {
	cells := cell.StringToCells(str)
	return newStringComp(cells, cfg.Attributes, cfg.BackgroundRune,
		cfg.BackgroundAttributes, cfg.FrameCharSet, 0, 0, cfg.Alignment)
}

// String converts a string into a very efficient top left centered one line tui.Component
// which draws the given string. If the string needs to be centered dynamically,
// or drawn multi-line use StringWithConfig instead.
func String(str string) WithAttributes {
	row := make([]term.Cell, len(str))
	for i, r := range str {
		row[i] = term.Cell{Ch: r}
	}
	return &stringComp{cells: [][]term.Cell{row}}
}
