// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package browser

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/browserapi"
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
			b.NewTab(uri1, 'a', "a", h, h)
			cfg := browserapi.BarConfig{Size: 1, Orientation: browserapi.OrientationTop}
			b.Bar(cfg, newTestHandler())
			cfg.Orientation = browserapi.OrientationBottom
			b.Bar(cfg, newTestHandler())
			cfg.Orientation = browserapi.OrientationLeft
			b.Bar(cfg, newTestHandler())
			cfg.Orientation = browserapi.OrientationRight
			b.Bar(cfg, newTestHandler())
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
	assert.NoError(t, win.Close())
}

func TestBrowserScrollable(t *testing.T) {
	t.Run("create floating window", func(t *testing.T) {
		b := NewComponent(DefaultConfig())
		scrollable := newTestScrollableHandler()

		win := b.Floating(scrollable, component.FloatingConfig{})
		content, err := win.Content()
		require.NoError(t, err)

		_, ok := content.(component.Scrollable)
		assert.True(t, ok)
		assert.NoError(t, win.Close())
	})

	t.Run("create split window", func(t *testing.T) {
		b := NewComponent(DefaultConfig())
		scrollable := newTestScrollableHandler()

		win, ok := b.Split(browserapi.OrientationLeft, b.Focus(), scrollable)
		require.True(t, ok)

		content, err := win.Content()
		require.NoError(t, err)

		_, ok = content.(component.Scrollable)
		assert.True(t, ok)
		assert.NoError(t, win.Close())
	})

	t.Run("update content of window", func(t *testing.T) {
		b := NewComponent(DefaultConfig())

		win, ok := b.Split(browserapi.OrientationLeft, b.Focus(), newTestHandler())
		require.True(t, ok)

		scrollable := newTestScrollableHandler()
		err := win.SetContent(scrollable)
		require.NoError(t, err)

		content, err := win.Content()
		require.NoError(t, err)

		_, ok = content.(component.Scrollable)
		assert.True(t, ok)
		assert.NoError(t, win.Close())
	})

	t.Run("new tab", func(t *testing.T) {
		b := NewComponent(DefaultConfig())

		uri1, err := workspaceapi.ParseURI("file:///a")
		require.NoError(t, err)
		h := newTestScrollableHandler()
		tab := b.NewTab(uri1, 'x', "a", h, h)

		content := tab.Handler()
		_, ok := content.(component.Scrollable)
		assert.True(t, ok)
	})
}

func TestComponentCloseOtherWindows(t *testing.T) {
	t.Run("fails if there's only one window", func(t *testing.T) {
		b := NewComponent(DefaultConfig())
		require.Error(t, b.CloseOtherWindows(b.Focus()))
		assert.Equal(t, 1, b.Tiles())
		assert.Equal(t, 0, b.FloatingWindows())
	})
	t.Run("fails if there's one tiled window and one floating window", func(t *testing.T) {
		b := NewComponent(DefaultConfig())
		_ = b.Floating(newTestHandler(), component.FloatingConfig{
			Alignment: component.SpanAlignmentHorizontallyCentered,
		})
		require.Error(t, b.CloseOtherWindows(b.Focus()))
		assert.Equal(t, 1, b.Tiles())
		assert.Equal(t, 1, b.FloatingWindows())
	})
	t.Run("fails if called on floating window", func(t *testing.T) {
		b := NewComponent(DefaultConfig())
		win := b.Floating(newTestHandler(), component.FloatingConfig{
			Alignment: component.SpanAlignmentHorizontallyCentered,
		})
		require.Error(t, b.CloseOtherWindows(win))
		assert.Equal(t, 1, b.Tiles())
		assert.Equal(t, 1, b.FloatingWindows())
	})
	t.Run("closes all floating and non-floating windows except focus", func(t *testing.T) {
		b := NewComponent(DefaultConfig())
		orig := b.Focus()
		_ = b.Floating(newTestHandler(), component.FloatingConfig{
			Alignment: component.SpanAlignmentHorizontallyCentered,
		})
		win, ok := b.Split(browserapi.OrientationDefault, orig, newTestHandler())
		require.True(t, ok)
		require.NoError(t, b.CloseOtherWindows(win))
		assert.Equal(t, 1, b.Tiles())
		assert.Equal(t, 0, b.FloatingWindows())

		require.Error(t, b.CloseOtherWindows(win))
		assert.Equal(t, 1, b.Tiles())
		assert.Equal(t, 0, b.FloatingWindows())
	})
}

func TestRemoveInactiveTabs(t *testing.T) {
	b := NewComponent(DefaultConfig())

	tabDefs := []struct {
		name   string
		active bool
	}{
		{"a", false}, {"b", true}, {"c", true}, {"d", false}}

	for _, tabDef := range tabDefs {
		uri, err := workspaceapi.ParseURI("file:///" + tabDef.name)
		require.NoError(t, err)
		h := newTestHandler()
		tab := b.NewTab(uri, 'o', tabDef.name, h, h)
		if tabDef.active {
			b.Split(browserapi.OrientationRight, b.Focus(), tab)
		}
	}

	tabNames := make([]string, 0)
	for _, tab := range b.Tabs() {
		tabName, _, _ := b.TabName(tab.URI())
		tabNames = append(tabNames, tabName)
	}
	assert.Equal(t, []string{"a", "b", "c", "d"}, tabNames)

	b.RemoveInactiveTabs()
	tabNames = tabNames[:0]

	for _, tab := range b.Tabs() {
		tabName, _, _ := b.TabName(tab.URI())
		tabNames = append(tabNames, tabName)
	}
	assert.Equal(t, []string{"b", "c"}, tabNames)
}

var _ component.Scrollable = (*nopScrollableHandler)(nil)

type nopScrollableHandler struct {
	nopHandler
}

func (b *nopScrollableHandler) SeekUp() bool {
	return false
}

func (b *nopScrollableHandler) SeekDown() bool {
	return false
}

func (b *nopScrollableHandler) SeekOffset() int {
	return 0
}

func (b *nopScrollableHandler) MaxSeekOffset() int {
	return 0
}

func newTestScrollableHandler() *nopScrollableHandler {
	return &nopScrollableHandler{}
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
