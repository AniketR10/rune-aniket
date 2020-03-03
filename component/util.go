package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// String converts a string into a static tui.Compontent.
func String(str string) tui.Component {
	var buf cell.Buffer
	var scroll Scroll
	buf.Init()
	buf.WriteString(str)
	scroll.InitWithBuffer(&buf)

	background := term.Cell{Bg: scroll.Attributes.Bg, Fg: scroll.Attributes.Fg}
	span := NewSpan(WithBackground(&scroll, background))

	span.Padding.Vertical = -buf.Height()
	span.Padding.Horizontal = -buf.Width()

	return span
}
