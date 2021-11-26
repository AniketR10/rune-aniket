package component

import (
	"math"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
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

// StringResponsive returns a Responsive implementation of
// a string tui.Component.
func StringResponsive(str string, cfg StringConfig) Responsive {
	return &respStr{
		cfg: cfg,
		in:  cell.StringToCells(str),
	}
}

type respStr struct {
	cfg StringConfig
	in  [][]term.Cell
	out tui.Component
}

// Height satisfies Responsive.
func (s *respStr) Height(width int) int {
	height := len(s.in)
	for _, col := range s.in {
		height += (len(col) - 1) / width
	}
	return height
}

// Resize satisfies tui.Component.
func (s *respStr) Resize(width, height int) {
	var outRaw [][]term.Cell
	for _, col := range s.in {
		for len(col) > 0 {
			chunkLen := int(math.Min(float64(len(col)), float64(width)))
			if chunkLen == 0 {
				break
			}
			outRaw = append(outRaw, col[:chunkLen])
			col = col[chunkLen:]
		}
	}
	s.out = newStringComp(outRaw, s.cfg.Attributes, 0,
		s.cfg.Attributes, s.cfg.FrameCharSet,
		0, 0, s.cfg.Alignment)
	s.out.Resize(width, height)
}

// Draw satisfies tui.Component.
func (s *respStr) Draw(w term.Writer) {
	s.out.Draw(w)
}
