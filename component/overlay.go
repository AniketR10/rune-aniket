package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
)

type overlay struct {
	background tui.Component
	cover      tui.Component
}

// Overlay overlays one component over another one, with potentially padding
// and/or alignment, specified by config.
func Overlay(background, cover tui.Component, config SpanConfig) tui.Component {
	// clean cells before drawing on top
	cover = WithBackground(cover, term.Cell{})
	cover = NewSpan(cover, config)

	return &overlay{
		background: background,
		cover:      cover,
	}
}

func (o overlay) Resize(width, height int) {
	o.background.Resize(width, height)
	o.cover.Resize(width, height)
}

func (o overlay) Draw(w term.Writer) {
	o.background.Draw(w)
	o.cover.Draw(w)
}
