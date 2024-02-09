package component

import (
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

// Overlay overlays one component over another one, with potentially padding
// and/or alignment, specified by SpanConfig.
type Overlay struct {
	background tui.Component
	Span
}

// NewOverlay allocates storage for a new overlay and initializes it.
func NewOverlay(
	background, cover tui.Component, backAttr term.Attributes, config SpanConfig,
) *Overlay {
	ret := new(Overlay)
	ret.Init(background, cover, backAttr, config)
	return ret
}

func (o *Overlay) Init(
	background, cover tui.Component, backAttr term.Attributes, config SpanConfig,
) {
	// clean cells before drawing on top
	cover = WithBackground(cover, term.Cell{Attributes: backAttr})

	o.Span.Init(cover, config)
	o.background = background
}

func (o *Overlay) Resize(width, height int) {
	o.background.Resize(width, height)
	o.Span.Resize(width, height)
}

func (o *Overlay) Draw(w term.Writer) {
	o.background.Draw(w)
	o.Span.Draw(w)
}
