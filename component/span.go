package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

type Alignment int

const (
	SpanAlignmentLeft Alignment = 1 << iota
	SpanAlignmentRight
	SpanAlignmentHorizontallyCentered
	SpanAlignmentTop
	SpanAlignmentBottom
	SpanAlignmentVerticallyCentered

	SpanAlignmentCentered = SpanAlignmentHorizontallyCentered | SpanAlignmentVerticallyCentered
	DefaultSpanFlags      = SpanAlignmentCentered
)

// Span is a component that takes another component and handles padding and
// alignment.
type Span struct {
	content       Virtual
	width, height int
	// Padding represents horizontal and vertical padding. It can be represented
	// as an absolute number of cells (Horizontal/Verical) or as a percentage of
	// the available space (HorizontalPerc/VerticalPerc).
	// Either Horizontal/Vertical or HorizontalPerc/VerticalPerc can be set;
	// If both are set, then Horizontal/Vertical take precedence.
	//
	// Negative padding on Horizontal/Vertical indicates that the padding should be
	// automatically calculated based on the available height/width. For instance,
	// a Horizontal padding of -1, indicates that the padding needs to be set such
	// that the inner component is exactly 1 cell.
	Padding struct {
		Horizontal int
		Vertical   int

		HorizontalPerc float64
		VerticalPerc   float64
	}
	ContentAlignment Alignment
}

// NewSpan returns an initialized Span. See Span.Init for more info.
func NewSpan(content tui.Component) *Span {
	s := new(Span)
	s.Init(content)
	s.ContentAlignment = DefaultSpanFlags
	return s
}

// Init initializes this Span with content.
func (s *Span) Init(content tui.Component) {
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
	content *Virtual,
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
	if s.Padding.HorizontalPerc < 0 || s.Padding.VerticalPerc < 0 ||
		s.Padding.HorizontalPerc > 1 || s.Padding.VerticalPerc > 1 {
		panic("invalid padding")
	}

	var hPadding, vPadding int

	if width < 3 {
		hPadding = 0
	} else if s.Padding.Horizontal == 0 {
		hPadding = int(s.Padding.HorizontalPerc * float64(width))
	} else if s.Padding.Horizontal < 0 {
		hPadding = width + s.Padding.Horizontal
	} else {
		hPadding = s.Padding.Horizontal
	}

	if height < 3 {
		vPadding = 0
	} else if s.Padding.Vertical == 0 {
		vPadding = int(s.Padding.VerticalPerc * float64(height))
	} else if s.Padding.Vertical < 0 {
		vPadding = height + s.Padding.Vertical
	} else {
		vPadding = s.Padding.Vertical
	}

	alignContent(&s.content, width, height, hPadding,
		vPadding, s.ContentAlignment)

	s.width, s.height = width, height
}

// Draw : Component
func (s *Span) Draw(w tui.Writer) {
	s.content.Draw(w)
}

// SetContent updates the underlying component and resizes it
// to conform to this frame's width and height.
func (s *Span) SetContent(content tui.Component) {
	s.content.C = content
	s.Resize(s.width, s.height)
}

// ContentOffset returns the offset in term.Coordinates of the content inside the span.
func (s *Span) ContentOffset() term.Coordinates {
	offset := s.content.Position()
	if inner, ok := s.content.C.(*Span); ok {
		inner := inner.ContentOffset()
		return term.Coordinates{
			X: offset.X + inner.X,
			Y: offset.Y + inner.Y,
		}
	}
	return offset
}
