package component

import (
	"fmt"
	"math"

	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
)

// Responsive components implement a backpressure mechanism (Height) for
// aggregate components to dynamically resize children based on their contents.
// See Height for more details.
type Responsive interface {
	tui.Component
	// Height allows for children components to return a height hint
	// given a width so a parent component can compose accordingly.
	// The returned height can be overriden at the parent's discretion
	// (i.e. there's simply no height left on the screen)
	// so implementers should expect that on calls to Resize.
	Height(width int) int
}

type StringResponsiveConfig struct {
	// NoSplitWords instructs the underlying string responsive component
	// to attempt to not split words in half when possible.
	NoSplitWords bool
	StringConfig
}

// StringResponsive returns a Responsive implementation of
// a string tui.Component.
//
// Deprecated: use NewResponsiveString.
func StringResponsive(str string, cfg StringResponsiveConfig) Responsive {
	return NewResponsiveString(str, cfg)
}

// NewResponsiveString allocates storage for a new ResponsiveString based on str and cfg.
func NewResponsiveString(str string, cfg StringResponsiveConfig) *ResponsiveString {
	return NewResponsiveStringFromCells(cell.StringToCells(str, cfg.Tabspaces), cfg)
}

// NewResponsiveStringFromCells returns a Responsive implementation for a matrix of cells.
func NewResponsiveStringFromCells(cells [][]term.Cell, cfg StringResponsiveConfig) *ResponsiveString {
	ret := new(ResponsiveString)
	ret.cfg = cfg
	ret.in = cells
	ret.Resize(0, 0) // initialize ret.out
	return ret
}

// Buffer wraps a cell.Buffer and returns a tui.Component which satisfies
// Responsive. Note that this is not the most efficient implementation of tui.Component
// for a cell.Buffer. See component.Scroll for more details.
func Buffer(buf *cell.Buffer, cfg StringResponsiveConfig) Responsive {
	return &respBuf{buf: buf, ResponsiveString: ResponsiveString{cfg: cfg}}
}

// NopResponsive returns a Responsive tui.Component that draws nothing.
func NopResponsive() Responsive {
	return nopResponsive{Component: Nop()}
}

// FuncResponsive wraps a tui.Component that satisfies Responsive's Height
// by calling heightFn.
func FuncResponsive(c tui.Component, heightFn func(width int) int) Responsive {
	return respFn{Component: c, heightFn: heightFn}
}

type nopResponsive struct {
	tui.Component
}

func (f nopResponsive) Height(width int) int {
	return 0
}

type respFn struct {
	tui.Component
	heightFn func(int) int
}

func (f respFn) Height(width int) int {
	return f.heightFn(width)
}

// ResponsiveString is a String component that also satisfies Responsive.
type ResponsiveString struct {
	cfg    StringResponsiveConfig
	in     [][]term.Cell
	out    floatingWithAttributes
	width  int
	height int
}

var _ WithAttributes = (*ResponsiveString)(nil)
var _ Responsive = (*ResponsiveString)(nil)
var _ fmt.Stringer = (*ResponsiveString)(nil)

var _ WithAttributes = (*respBuf)(nil)
var _ Responsive = (*respBuf)(nil)
var _ fmt.Stringer = (*respBuf)(nil)

type respBuf struct {
	ResponsiveString
	buf           *cell.Buffer
	width, height int
}

func (b *respBuf) Height(width int) int {
	b.ResponsiveString.in = b.buf.RawCells()
	return b.ResponsiveString.Height(width)
}

func (b *respBuf) Resize(width, height int) {
	b.width, b.height = width, height
}

func (b *respBuf) Draw(w term.Writer) {
	b.ResponsiveString.in = b.buf.RawCells()
	b.ResponsiveString.Resize(b.width, b.height)
	b.ResponsiveString.Draw(w)
}

// Height satisfies Responsive.
func (s *ResponsiveString) Height(width int) int {
	if width <= 0 {
		return 0
	}
	height := len(s.massageInput(width))
	height += s.cfg.PaddingVertical
	if s.cfg.FrameCharSet != (FrameCharSet{}) {
		height += 2
	}
	return height
}

// Resize satisfies tui.Component.
func (s *ResponsiveString) Resize(width, height int) {
	s.width = width
	s.height = height
	outRaw := s.massageInput(width)
	s.out = newStringComp(outRaw, s.cfg.Attributes, 0,
		s.cfg.BackgroundAttributes, s.cfg.FrameCharSet,
		s.cfg.PaddingHorizontal, s.cfg.PaddingVertical, s.cfg.Alignment, s.cfg.MinWidth)
	s.out.Resize(width, height)
}

// Draw satisfies tui.Component.
func (s *ResponsiveString) Draw(w term.Writer) {
	s.out.Draw(w)
}

// SetAttr satisfies WithAttributes.
func (s *ResponsiveString) SetAttr(attr term.Attributes) term.Attributes {
	s.cfg.Attributes = attr
	s.cfg.BackgroundAttributes = attr
	return s.out.SetAttr(attr)
}

func (s *ResponsiveString) massageInput(width int) [][]term.Cell {
	effectiveWidth := width
	if effectiveWidth > 2 && s.cfg.FrameCharSet != (FrameCharSet{}) {
		effectiveWidth -= 2
	}
	if effectiveWidth > s.cfg.PaddingHorizontal {
		effectiveWidth -= s.cfg.PaddingHorizontal
	}
	var outRaw [][]term.Cell
	for _, col := range s.in {
		if len(col) == 0 {
			outRaw = append(outRaw, col[:])
			continue
		}
		for len(col) > 0 {
			chunkLen := int(math.Min(float64(len(col)), float64(effectiveWidth)))
			if chunkLen == 0 {
				break
			}
			origChunkLen := chunkLen
			// do not split word in half
			for s.cfg.NoSplitWords && origChunkLen != len(col) && chunkLen > 1 && col[chunkLen-1].Ch != ' ' {
				chunkLen--
			}
			// word doesn't fit, split word
			if chunkLen == 1 {
				chunkLen = origChunkLen
			}
			outRaw = append(outRaw, col[:chunkLen])
			col = col[chunkLen:]
		}
	}
	return outRaw
}

// String satisfies fmt.Stringer.
func (s *ResponsiveString) String() string {
	return s.out.String()
}
