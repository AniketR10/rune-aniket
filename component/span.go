package component

import (
	"math"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

// Alignment represents the content alignment.
type Alignment int

const (
	// SpanAlignmentLeft horizontally aligns content to the left.
	SpanAlignmentLeft Alignment = 1 << iota
	// SpanAlignmentRight horizontally aligns content to the right.
	SpanAlignmentRight
	// SpanAlignmentHorizontallyCentered horizontally aligns content to the center.
	SpanAlignmentHorizontallyCentered
	// SpanAlignmentTop vertically aligns content to the top.
	SpanAlignmentTop
	// SpanAlignmentBottom vertically aligns content to the bottom.
	SpanAlignmentBottom
	// SpanAlignmentVerticallyCentered vertically aligns content to center.
	SpanAlignmentVerticallyCentered

	// SpanAlignmentCentered vertically and horizontally aligns content to center.
	SpanAlignmentCentered = SpanAlignmentHorizontallyCentered | SpanAlignmentVerticallyCentered
)

// SpanConfig represents the configuration of a Span.
//
// Padding can be configured as an absolute number of cells (PadHorizontal/PadVerical)
// or as a percentage of the available space (PadHorizontalPerc/PadVerticalPerc).
// Either PadHorizontal/PadVertical or PadHorizontalPerc/PadVerticalPerc should be set;
// If both are set, then Horizontal/Vertical take precedence.
//
// Negative padding on PadHorizontal/PadVertical indicates that the padding should be
// automatically calculated based on the available height/width. For instance,
// a Horizontal padding of -1, indicates that the padding needs to be set such
// that the inner component is exactly 1 cell.
type SpanConfig struct {
	PadHorizontal int
	PadVertical   int

	PadHorizontalPerc float64
	PadVerticalPerc   float64

	ContentAlignment Alignment
}

// Span is a component that takes another component and handles padding and
// alignment.
type Span struct {
	content       Virtual
	width, height int
	cfg           SpanConfig
}

// DefaultSpanConfig returns the default span configuration wich is no padding,
// and content alignment centered.
func DefaultSpanConfig() (ret SpanConfig) {
	ret.ContentAlignment = SpanAlignmentCentered
	return
}

// NewSpan returns an initialized Span. See Span.Init for more info.
func NewSpan(content tui.Component, cfg SpanConfig) *Span {
	s := new(Span)
	s.Init(content, cfg)
	return s
}

// Init initializes this Span with content.
func (s *Span) Init(content tui.Component, cfg SpanConfig) {
	if cfg.PadHorizontalPerc < 0 || cfg.PadHorizontalPerc > 1 ||
		cfg.PadVerticalPerc < 0 || cfg.PadVerticalPerc > 1 {
		panic("padding percentage must be between range [0, 1]")
	}

	s.cfg = cfg
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
	var hPadding, vPadding int

	if width < 3 {
		hPadding = 0
	} else if s.cfg.PadHorizontal == 0 {
		hPadding = int(s.cfg.PadHorizontalPerc * float64(width))
	} else if s.cfg.PadHorizontal < 0 {
		hPadding = int(math.Max(0, float64(width+s.cfg.PadHorizontal)))
	} else {
		hPadding = s.cfg.PadHorizontal
	}

	if height < 3 {
		vPadding = 0
	} else if s.cfg.PadVertical == 0 {
		vPadding = int(s.cfg.PadVerticalPerc * float64(height))
	} else if s.cfg.PadVertical < 0 {
		vPadding = int(math.Max(0, float64(height+s.cfg.PadVertical)))
	} else {
		vPadding = s.cfg.PadVertical
	}

	alignContent(&s.content, width, height, hPadding,
		vPadding, s.cfg.ContentAlignment)

	s.width, s.height = width, height
}

// Draw : Component
func (s *Span) Draw(w term.Writer) {
	s.content.Draw(w)
}

// SetContent updates the underlying component and resizes it
// to conform to this frame's width and height.
func (s *Span) SetContent(content tui.Component) {
	s.content.C = content
	s.Resize(s.width, s.height)
}

// Content returns this Span's underlying content.
func (s *Span) Content() tui.Component {
	return s.content.C
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
