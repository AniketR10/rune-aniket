package component

import (
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
)

// StringBackground converts a string into a static tui.Compontent,
// and uses c as the background rune.
func StringBackground(str string, c rune) tui.Component {
	var buf cell.Buffer
	var scroll Scroll
	buf.Init()
	buf.WriteString(str)
	scroll.InitWithBuffer(&buf)

	background := term.Cell{
		Ch: c,
		Bg: scroll.Attributes.Bg,
		Fg: scroll.Attributes.Fg,
	}

	span := NewSpan(WithBackground(&scroll, background), SpanConfig{
		PadVertical:   -buf.Height(),
		PadHorizontal: -buf.Width(),
	})

	return span
}

// String converts a string into a static tui.Compontent.
func String(str string) tui.Component {
	return StringBackground(str, 0)
}
