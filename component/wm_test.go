package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestComponentWindowZeroValue(t *testing.T) {
	t.Run("Close", func(t *testing.T) {
		var win Window
		assert.NotPanics(t, func() {
			win.Close()
		})
	})
	t.Run("Content", func(t *testing.T) {
		var win Window
		assert.PanicsWithValue(t, errCalledZeroValuedWin, func() {
			_ = win.Content()
		})
	})
	t.Run("SetContent", func(t *testing.T) {
		var win Window
		assert.PanicsWithValue(t, errCalledZeroValuedWin, func() {
			win.SetContent(NewString("ballz"))
		})
	})
	t.Run("Size", func(t *testing.T) {
		var win Window
		assert.PanicsWithValue(t, errCalledZeroValuedWin, func() {
			win.Size()
		})
	})
	t.Run("TileDirection", func(t *testing.T) {
		var win Window
		assert.PanicsWithValue(t, errCalledZeroValuedWin, func() {
			win.TileDown()
		})
	})
}

func TestComponentWindowSplit(t *testing.T) {
	w := term.NewStringWriter(20, 8)

	h1 := TestComponent{Ch: 'A'}
	wm, w1 := NewWindowManager(&h1, DefaultWindowManagerConfig())
	wm.Resize(20, 8)

	var w2, w3 Window
	var prevFloating tui.Component
	var ok bool
	h2 := TestComponent{Ch: 'B'}
	h3 := TestComponent{Ch: 'C'}

	tests := []testutil.ComponentTestCase{
		{
			nil, `
┌──────────────────┐
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`,
		}, {func() {
			w2, ok = wm.SplitHorizontal(w1, &h2)
			assert.True(t, ok)
		}, `
┌──────────────────┐
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘
┌──────────────────┐
│BBBBBBBBBBBBBBBBBB│
│BBBBBBBBBBBBBBBBBB│
└──────────────────┘`,
		}, {func() {
			w3, ok = wm.SplitVertical(w2, &h3)
			assert.True(t, ok)
		}, `
┌──────────────────┐
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘
┌────────┐┌────────┐
│BBBBBBBB││CCCCCCCC│
│BBBBBBBB││CCCCCCCC│
└────────┘└────────┘`,
		}, {func() {
			assert.NoError(t, w2.Close())
		}, `
┌──────────────────┐
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘
┌──────────────────┐
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			assert.NoError(t, w1.Close())
		}, `
┌──────────────────┐
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			assert.Error(t, w1.Close())
			floating := StaticFloating(&h2, 4, 4)
			w2 = wm.FloatingWindow(floating, term.Coordinates{})
		}, `
┌──┐───────────────┐
│BB│CCCCCCCCCCCCCCC│
│BB│CCCCCCCCCCCCCCC│
└──┘CCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			floating := w2.Content().(*staticFloating)
			floating.width = 5
			floating.height = 6
		}, `
┌───┐──────────────┐
│BBB│CCCCCCCCCCCCCC│
│BBB│CCCCCCCCCCCCCC│
│BBB│CCCCCCCCCCCCCC│
│BBB│CCCCCCCCCCCCCC│
└───┘CCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			// test setting non floating component
			prevFloating = w2.SetContent(&TestComponent{Ch: '5'})
		}, `
┌───┐──────────────┐
│555│CCCCCCCCCCCCCC│
│555│CCCCCCCCCCCCCC│
│555│CCCCCCCCCCCCCC│
│555│CCCCCCCCCCCCCC│
└───┘CCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			// test return of SetContent is always what we expect
			prevFloating = w2.SetContent(prevFloating)
			assert.Equal(t, '5', prevFloating.(*TestComponent).Ch)
		}, `
┌───┐──────────────┐
│BBB│CCCCCCCCCCCCCC│
│BBB│CCCCCCCCCCCCCC│
│BBB│CCCCCCCCCCCCCC│
│BBB│CCCCCCCCCCCCCC│
└───┘CCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			wx, ok := wm.WindowAt(term.Coordinates{})
			assert.True(t, ok)
			assert.Equal(t, w2, wx)

			wx, ok = wm.WindowAt(term.Coordinates{X: 6})
			assert.True(t, ok)
			assert.Equal(t, w3, wx)

			wx, ok = wm.WindowAt(term.Coordinates{Y: 7})
			assert.True(t, ok)
			assert.Equal(t, w3, wx)

			assert.NoError(t, w2.Close())
			wx, ok = wm.WindowAt(term.Coordinates{})
			assert.True(t, ok)
			assert.Equal(t, w3, wx)
		}, `
┌──────────────────┐
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			wx, ok := wm.WindowAt(term.Coordinates{})
			require.True(t, ok)
			comp := TestComponent{Ch: '#'}
			comp.Resize(3, 3)
			wx.SetContentResize(&comp, true)
		}, `
┌──────────────────┐
│##################│
│##################│
│##################│
│##################│
│##################│
│##################│
└──────────────────┘`,
		}, {func() {
			wx, ok := wm.WindowAt(term.Coordinates{})
			require.True(t, ok)
			comp := TestComponent{Ch: '$'}
			comp.Resize(3, 3)
			wx.SetContentResize(&comp, false)
		}, `
┌──────────────────┐
│$$$               │
│$$$               │
│$$$               │
│                  │
│                  │
│                  │
└──────────────────┘`,
		},
	}

	testutil.TestComponent(t, wm, w, tests)
}
