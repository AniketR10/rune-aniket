package browser

import (
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
)

func newLogSpan(buf *cell.Buffer, bgAttr term.Attributes) handler.Virtual {
	scroll := component.NewScroll()
	scroll.InitWithBuffer(buf)
	scroll.Attributes = bgAttr

	background := term.Cell{Bg: bgAttr.Bg, Fg: bgAttr.Fg}
	content := component.WithBackground(scroll, background)
	return handler.Virtual{Virtual: component.Virtual{C: content}}
}

func resizeLogSpan(width, height int, logVirt *handler.Virtual) {
	if width > 1 && height > 0 {
		logVirt.Resize(width-2, 1)
		busPos := term.Coordinates{X: 1, Y: height - 2}
		logVirt.Move(busPos)
	} else {
		logVirt.Resize(0, 0)
	}
}
