package browser

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	browserapi "unstable.build/go-tui/api/browser"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

func TestWindowDraw(t *testing.T) {
	noFrameNoDim := DefaultConfig()
	noFrameNoDim.Dim = false
	noFrameNoDim.Frame = false

	noFrameDim := DefaultConfig()
	noFrameDim.Dim = true
	noFrameDim.Frame = false
	suite := []Config{
		noFrameNoDim,
		noFrameDim,
	}

	for _, cfg := range suite {
		cfg := cfg
		t.Run(fmt.Sprintf("%#v", cfg), func(t *testing.T) {
			// create a decent mix of components and UI elements
			b := NewComponent(cfg)
			b.Split(browserapi.OrientationRight, b.Focus(), newTestHandler())
			b.Split(browserapi.OrientationBottom, b.Focus(), newTestHandler())
			b.Split(browserapi.OrientationTop, b.Focus(), newTestHandler())
			b.Split(browserapi.OrientationLeft, b.Focus(), newTestHandler())
			uri1, err := workspaceapi.ParseURI("file:///a")
			require.NoError(t, err)
			h := newTestHandler()
			b.NewTab(uri1, "a", h, h)
			b.Bar(browserapi.OrientationTop, newTestHandler())
			b.Bar(browserapi.OrientationBottom, newTestHandler())
			b.Bar(browserapi.OrientationLeft, newTestHandler())
			b.Bar(browserapi.OrientationRight, newTestHandler())
			b.Floating(newTestHandler(), component.FloatingConfig{
				Alignment: component.SpanAlignmentHorizontallyCentered,
			})

			width, height := 12, 8
			writer1 := term.NewStringWriter(width, height)
			writer2 := term.NewStringWriter(width, height)

			b.Resize(width, height)
			b.Draw(writer1)

			// draw first union (everything), and then windows on top
			// and it matches Draw, then DrawWindow is correct.
			b.tabs.ResetFocus()
			for id, t := range b.buffers {
				if !t.free {
					b.tabs.SetFocus(id)
				}
			}
			b.union.Draw(writer2)
			b.wm.Iterate(func(w handler.Window) {
				win, _ := b.findWindow(w.ID())
				b.DrawWindow(win, writer2)
			})
			b.overwriteFocusWindowUnion(writer2)

			writer1.Flush()
			writer2.Flush()
			assert.Equal(t, writer1.String(), writer2.String())
		})
	}
}

func TestWindowClosedOnClose(t *testing.T) {
	b := NewComponent(DefaultConfig())
	h := newTestHandler()
	var win Window
	win = b.Floating(FuncFloatingHandler(h, func() error {
		if !win.Closed() {
			_ = win.Close()
		}
		return h.Close()
	}), component.FloatingConfig{})
	require.NoError(t, win.Close())
}

type nopHandler struct {
	handler.TestHandler
}

func (n nopHandler) Dimensions() (int, int) {
	return 12, 8
}

func (n nopHandler) Close() error {
	return nil
}

func newTestHandler() *nopHandler {
	return &nopHandler{}
}
