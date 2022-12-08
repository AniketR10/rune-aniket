package browser

import (
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

// satisfies sync.Locker
type nopLocker struct{}

func (l nopLocker) Lock() {
}
func (l nopLocker) Unlock() {
}

// newMessageSpan returns a virtual message bar suitable for use with
// ResizeMessageSpan.
func newMessageSpan(buf *cell.Buffer, bgAttr term.Attributes) handler.Virtual {
	responsive := component.Buffer(buf, component.StringConfig{
		BackgroundAttributes: bgAttr,
		Attributes:           bgAttr,
	})
	return handler.Virtual{Virtual: component.Virtual{C: responsive}}
}

// resizeMessageSpan resizes a handler.Virtual Message span returned by
// NewMessageSpan. It positions the handler.Virtual at the bottom
// of the available space with a 1 cell padding left and bottom.
func resizeMessageSpan(logVirt *handler.Virtual, width, height int, frame bool) {
	if width <= 1 || height <= 0 {
		logVirt.Resize(0, 0)
		return
	}

	move := term.Coordinates{}
	maxWidth := width
	if frame {
		maxWidth -= 2
		move.X = 1
	}
	minHeight := logVirt.C.(component.Responsive).Height(maxWidth)
	if minHeight > height {
		// if ideal height cannot be used because msg would be truncated
		// then revert to old behaviour of truncating beyond first line
		minHeight = 1
	}

	move.Y = height - minHeight - 1

	logVirt.Resize(maxWidth, minHeight)
	logVirt.Move(move)
}
