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

package component

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/term"
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

func TestWindowManagerSplit(t *testing.T) {
	w := term.NewStringWriter(20, 8)

	h1 := TestComponent{Ch: 'A'}
	wm, w1 := NewWindowManager(&h1, DefaultWindowManagerConfig())
	wm.Resize(20, 8)

	var w2, w3 Window
	var prevFloating tui.Component
	var ok bool
	h2 := TestComponent{Ch: 'B'}
	hnop := TestComponent{Ch: 0}
	h3 := TestComponent{Ch: 'C'}

	tests := []comptest.TestCase{
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
			assert.False(t, w2.Closed())
			assert.NoError(t, w2.Close())
			assert.True(t, w2.Closed())
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
			assert.False(t, w1.Closed())
			assert.NoError(t, w1.Close())
			assert.True(t, w1.Closed())
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
			assert.True(t, w1.Closed())
			floating := StaticFloating(&h2, 2, 2)
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentLeft | SpanAlignmentTop,
					Offset:    term.Coordinates{X: 1, Y: 1},
				},
			)
		}, `
┌──────────────────┐
│┌──┐CCCCCCCCCCCCCC│
││BB│CCCCCCCCCCCCCC│
││BB│CCCCCCCCCCCCCC│
│└──┘CCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			floating := StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentRight | SpanAlignmentTop,
					Offset:    term.Coordinates{X: 1, Y: 1},
				},
			)
		}, `
┌──────────────────┐
│CCCCCCCCCCCCCC┌──┐│
│CCCCCCCCCCCCCC│BB││
│CCCCCCCCCCCCCC│BB││
│CCCCCCCCCCCCCC└──┘│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			floating := StaticFloating(&h2, 2, 2)
			assert.False(t, w2.Closed())
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentRight | SpanAlignmentBottom,
					Offset:    term.Coordinates{X: 1, Y: 1},
				},
			)
		}, `
┌──────────────────┐
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCC┌──┐│
│CCCCCCCCCCCCCC│BB││
│CCCCCCCCCCCCCC│BB││
│CCCCCCCCCCCCCC└──┘│
└──────────────────┘`,
		}, {func() {
			floating := StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentLeft | SpanAlignmentBottom,
					Offset:    term.Coordinates{X: 1, Y: 1},
				},
			)
		}, `
┌──────────────────┐
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│┌──┐CCCCCCCCCCCCCC│
││BB│CCCCCCCCCCCCCC│
││BB│CCCCCCCCCCCCCC│
│└──┘CCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			floating := StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentHorizontallyCentered,
					Offset:    term.Coordinates{X: 1, Y: 1}, // offset.X is ignored
				},
			)
		}, `
┌──────────────────┐
│CCCCCCC┌──┐CCCCCCC│
│CCCCCCC│BB│CCCCCCC│
│CCCCCCC│BB│CCCCCCC│
│CCCCCCC└──┘CCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			floating := StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentVerticallyCentered,
					Offset:    term.Coordinates{X: 1, Y: 1}, // offset.Y is ignored
				},
			)
		}, `
┌──────────────────┐
│CCCCCCCCCCCCCCCCCC│
│┌──┐CCCCCCCCCCCCCC│
││BB│CCCCCCCCCCCCCC│
││BB│CCCCCCCCCCCCCC│
│└──┘CCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			floating := StaticFloating(&hnop, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentCentered,
					Offset:    term.Coordinates{X: 1, Y: 1}, // offset is ignored
				},
			)
		}, `
┌──────────────────┐
│CCCCCCCCCCCCCCCCCC│
│CCCCCCC┌──┐CCCCCCC│
│CCCCCCC│  │CCCCCCC│
│CCCCCCC│  │CCCCCCC│
│CCCCCCC└──┘CCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			floating := StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentBottom,
					Offset:    term.Coordinates{X: 400, Y: 500},
				},
			)
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
			floating := StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentTop,
					Offset:    term.Coordinates{X: 400, Y: 500},
				},
			)
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
			floating := StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentLeft,
					Offset:    term.Coordinates{X: 400, Y: 500},
				},
			)
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
			floating := StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: SpanAlignmentRight,
					Offset:    term.Coordinates{X: 400, Y: 500},
				},
			)
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
			floating := StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{},
			)
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
			floating.width = 3
			floating.height = 4
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

	comptest.TestComponent(t, wm, w, tests)
}

func TestWindowManagerMinimize(t *testing.T) {
	w := term.NewStringWriter(20, 8)

	h1 := TestComponent{Ch: 'A'}
	wm, _ := NewWindowManager(&h1, DefaultWindowManagerConfig())
	wm.Resize(20, 8)

	var fwin Window
	tests := []comptest.TestCase{
		{
			func() {
				floating := StaticFloating(&TestComponent{Ch: '1'}, 2, 2)
				fwin = wm.FloatingWindow(floating,
					FloatingConfig{
						Alignment: SpanAlignmentCentered,
					},
				)
				assert.True(t, fwin.MinimizeUp())
			}, `
┌──────────────────┐
┌──────────────────┐
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`,
		}, {func() {
			w := wm.FloatingWindow(StaticFloating(&TestComponent{Ch: '2'}, 2, 2),
				FloatingConfig{
					Alignment: SpanAlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeLeft())
			assert.False(t, w.MinimizeLeft())

			w = wm.FloatingWindow(StaticFloating(&TestComponent{Ch: '3'}, 2, 2),
				FloatingConfig{
					Alignment: SpanAlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeRight())
			assert.False(t, w.MinimizeRight())

			w = wm.FloatingWindow(StaticFloating(&TestComponent{Ch: '4'}, 2, 2),
				FloatingConfig{
					Alignment: SpanAlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeDown())
			assert.False(t, w.MinimizeDown())
			assert.False(t, w.MinimizeLeft())
			assert.False(t, w.MinimizeRight())
			assert.False(t, w.MinimizeUp())
		}, `

┌──────────────────┐
┌┌────────────────┐┐
││AAAAAAAAAAAAAAAA││
││AAAAAAAAAAAAAAAA││
││AAAAAAAAAAAAAAAA││
││AAAAAAAAAAAAAAAA││
└└────────────────┘┘
└──────────────────┘`,
		}, {func() {
			assert.True(t, fwin.Unminimize())
			assert.False(t, fwin.Unminimize())
		}, `
┌┌────────────────┐┐
││AAAAA┌──┐AAAAAAA││
││AAAAA│11│AAAAAAA││
││AAAAA│11│AAAAAAA││
││AAAAA└──┘AAAAAAA││
││AAAAAAAAAAAAAAAA││
└└────────────────┘┘
└──────────────────┘`,
		}, {func() {
			assert.True(t, fwin.MinimizeUp())
			w := wm.FloatingWindow(StaticFloating(&TestComponent{Ch: '5'}, 2, 2),
				FloatingConfig{
					Alignment: SpanAlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeUp())
			w = wm.FloatingWindow(StaticFloating(&TestComponent{Ch: '6'}, 2, 2),
				FloatingConfig{
					Alignment: SpanAlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeLeft())

			w = wm.FloatingWindow(StaticFloating(&TestComponent{Ch: '7'}, 2, 2),
				FloatingConfig{
					Alignment: SpanAlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeRight())

			w = wm.FloatingWindow(StaticFloating(&TestComponent{Ch: '8'}, 2, 2),
				FloatingConfig{
					Alignment: SpanAlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeDown())
		}, `
┌──────────────────┐
┌──────────────────┐
┌┌┌──────────────┐┐┐
│││AAAAAAAAAAAAAA│││
│││AAAAAAAAAAAAAA│││
└└└──────────────┘┘┘
└──────────────────┘
└──────────────────┘`,
		}, {func() {
			require.NoError(t, fwin.Close())
		}, `
┌──────────────────┐
┌┌┌──────────────┐┐┐
│││AAAAAAAAAAAAAA│││
│││AAAAAAAAAAAAAA│││
│││AAAAAAAAAAAAAA│││
└└└──────────────┘┘┘
└──────────────────┘
└──────────────────┘`,
		}, {func() {
			win, ok := wm.WindowAt(term.Coordinates{X: 2, Y: 1})
			assert.True(t, ok)
			assert.Equal(t, 'A', win.Content().(*TestComponent).Ch)

			win, ok = wm.WindowAt(term.Coordinates{X: 0, Y: 0})
			assert.True(t, ok)
			assert.Equal(t, '5', win.Content().(*staticFloating).Component.(*TestComponent).Ch)

			win, ok = wm.WindowAt(term.Coordinates{X: 0, Y: 1})
			assert.True(t, ok)
			assert.Equal(t, '2', win.Content().(*staticFloating).Component.(*TestComponent).Ch)

			win, ok = wm.WindowAt(term.Coordinates{X: 19, Y: 1})
			assert.True(t, ok)
			assert.Equal(t, '7', win.Content().(*staticFloating).Component.(*TestComponent).Ch)

			win, ok = wm.WindowAt(term.Coordinates{X: 19, Y: 7})
			assert.True(t, ok)
			assert.Equal(t, '8', win.Content().(*staticFloating).Component.(*TestComponent).Ch)
		}, `
┌──────────────────┐
┌┌┌──────────────┐┐┐
│││AAAAAAAAAAAAAA│││
│││AAAAAAAAAAAAAA│││
│││AAAAAAAAAAAAAA│││
└└└──────────────┘┘┘
└──────────────────┘
└──────────────────┘`,
		},
	}

	comptest.TestComponent(t, wm, w, tests)
}
