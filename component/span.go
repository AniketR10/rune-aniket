package component

import (
	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/term"
)

type Alignment int

const (
	SpanAlignmentLeft Alignment = 1 << iota
	SpanAlignmentRight
	SpanAlignmentHorizontallyCentered
	SpanAlignmentTop
	SpanAlignmentBottom
	SpanAlignmentVerticallyCentered

	DefaultSpanFlags = SpanAlignmentHorizontallyCentered | SpanAlignmentVerticallyCentered
)

// Span is a component that takes another component and handles padding and
// alignment.
type Span struct {
	content       VirtualComponent
	width, height int
	// Padding represents horizontal and vertical padding as a
	// percentual point from 0 to 1.
	Padding struct {
		Horizontal float64
		Vertical   float64
	}
	ContentAlignment Alignment
}

// NewSpan returns an initialized Span. See Span.Init for more info.
func NewSpan(content fractal.Component) *Span {
	s := new(Span)
	s.Init(content)
	return s
}

// Init initializes this Span with content.
func (s *Span) Init(content fractal.Component) {
	s.content.C = content
}

func calculateContentOffset(
	horizontalPadding, verticalPadding int, flags Alignment,
) (offset term.Coordinates) {
	if flags&SpanAlignmentVerticallyCentered != 0 {
		offset.Y = verticalPadding / 2
	} else if flags&SpanAlignmentBottom != 0 {
		offset.Y = verticalPadding
	}

	if flags&SpanAlignmentHorizontallyCentered != 0 {
		offset.X = horizontalPadding / 2
	} else if flags&SpanAlignmentRight != 0 {
		offset.X = horizontalPadding
	}

	return
}

func alignContent(
	content *VirtualComponent,
	width, height int,
	horizontalPadding, verticalPadding int,
	flags Alignment) {

	offset := calculateContentOffset(horizontalPadding, verticalPadding, flags)
	contentWidth := width - horizontalPadding
	contentHeight := height - verticalPadding

	content.Resize(contentWidth, contentHeight)
	content.Move(offset)
}

// Resize : Component
func (s *Span) Resize(width, height int) {
	if s.Padding.Horizontal < 0 || s.Padding.Vertical < 0 ||
		s.Padding.Horizontal > 1 || s.Padding.Vertical > 1 {
		panic("invalid padding")
	}

	var hPadding, vPadding int
	if width < 3 || height < 3 {
		hPadding, vPadding = 0, 0
	} else {
		hPadding = int(s.Padding.Horizontal * float64(width))
		vPadding = int(s.Padding.Vertical * float64(height))
	}

	alignContent(&s.content, width, height, hPadding,
		vPadding, s.ContentAlignment)

	s.width, s.height = width, height
}

// Draw : Component
func (s *Span) Draw(w fractal.Writer) {
	s.content.Draw(w)
}

// SetContent updates the underlying component and resizes it
// to conform to this frame's width and height.
func (s *Span) SetContent(content fractal.Component) {
	s.content.C = content
	s.Resize(s.width, s.height)
}
