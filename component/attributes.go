package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// WithAttributes represents a tui.Component that can be set attributes.
type WithAttributes interface {
	tui.Component
	SetAttr(term.Attributes)
}

type attributesAt struct {
	term.Coordinates
	term.Attributes
}

// AttrSetter wraps another tui.Component to satisfy WithAttributes
// and add the ability to set the Attributes of arbitrary cells.
type AttrSetter struct {
	def    term.Attributes
	attr   []attributesAt
	comp   tui.Component
	width  int
	height int
}

// WithAttrSetter wraps a tui.Component to satisfy WithAttributes.
func WithAttrSetter(comp tui.Component) *AttrSetter {
	attr := make([]attributesAt, 0, 20)
	return &AttrSetter{term.Attributes{}, attr, comp, 0, 0}
}

// Reset resets all the previous calls to SetAttrAt and SetAttr.
func (s *AttrSetter) Reset() {
	s.attr = s.attr[:0]
	s.def = term.Attributes{}
}

// SetAttr sets the default attributes drawn by this component.
// This attributes will be set in each of the final term.Cells drawn.
func (s *AttrSetter) SetAttr(attr term.Attributes) {
	s.def = attr
}

// SetAttrAt sets the attributes at the given coordinates.
// If coordinates are out of bounds, this method will NOT panic, but
// the next call to Draw will.
func (s *AttrSetter) SetAttrAt(at term.Coordinates, attr term.Attributes) {
	s.attr = append(s.attr, attributesAt{at, attr})
}

// Resize satisfies tui.Component
func (s *AttrSetter) Resize(width, height int) {
	s.width, s.height = width, height
}

// Draw draws the underlying component along with
// the attributes.
func (s *AttrSetter) Draw(w term.Writer) {
	compWithAttr, is := s.comp.(WithAttributes)
	if is {
		compWithAttr.SetAttr(s.def)
	}

	var buf cell.BufferWriter
	buf.Init(s.width, s.height)
	s.comp.Resize(s.width, s.height)
	s.comp.Draw(&buf)
	cells := buf.RawCells()

	if !is {
		for y, row := range cells {
			for x := range row {
				cells[y][x].Bg |= s.def.Bg
				cells[y][x].Fg |= s.def.Fg
			}
		}
	}

	for _, attrAt := range s.attr {
		if attrAt.Y >= s.height || attrAt.Y < 0 ||
			attrAt.X >= s.width || attrAt.X < 0 {
			continue
		}
		cells[attrAt.Y][attrAt.X].Bg |= attrAt.Attributes.Bg
		cells[attrAt.Y][attrAt.X].Fg |= attrAt.Attributes.Fg
	}

	var sc Scroll
	sc.Init(&buf.Buffer)
	sc.Resize(s.width, s.height)
	sc.Draw(w)
}
