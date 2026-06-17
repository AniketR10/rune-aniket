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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/component/comptest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

func TestSetWidthHeight(t *testing.T) {
	w := term.NewStringWriter(20, 8)

	h1 := component.TestComponent{Ch: 'A'}
	wm, win1 := NewWindowManager(&h1, testWindowManagerConfig())
	wm.Resize(20, 8)

	var win2, win3 Window
	tests := []comptest.TestCase{
		{
			Action: func() {
				var ok bool
				win2, ok = wm.SplitVertical(win1, &component.TestComponent{Ch: 'B'})
				require.True(t, ok)
				win3, ok = wm.SplitHorizontal(win1, &component.TestComponent{Ch: 'C'})
				require.True(t, ok)
			}, Expected: `
┌────────┐┌────────┐
│AAAAAAAA││BBBBBBBB│
│AAAAAAAA││BBBBBBBB│
└────────┘│BBBBBBBB│
┌────────┐│BBBBBBBB│
│CCCCCCCC││BBBBBBBB│
│CCCCCCCC││BBBBBBBB│
└────────┘└────────┘`,
		},
		{
			Action: func() {
				assert.True(t, wm.SetHeight(win1, win1.MaxHeight()))
				assert.True(t, wm.SetWidth(win1, win1.MaxWidth()))
				assert.True(t, wm.SetHeight(win2, win2.MaxHeight()))
				assert.True(t, wm.SetWidth(win2, win2.MaxWidth()))
				assert.True(t, wm.SetHeight(win3, win3.MaxHeight()))
				assert.True(t, wm.SetWidth(win3, win3.MaxWidth()))
			}, Expected: `
┌───────────────┐┌─┐
│AAAAAAAAAAAAAAA││B│
└───────────────┘│B│
┌───────────────┐│B│
│CCCCCCCCCCCCCCC││B│
│CCCCCCCCCCCCCCC││B│
│CCCCCCCCCCCCCCC││B│
└───────────────┘└─┘`,
		},
	}

	comptest.TestComponent(t, wm, w, tests)
}

// splitRootLayout is one of the three initial layouts exercised by
// TestSplitRoot: a single tile (empty), a horizontal-then-vertical
// decomposition, or a vertical-then-horizontal decomposition.
type splitRootLayout int

const (
	splitRootLayoutEmpty splitRootLayout = iota
	splitRootLayoutHorizontalThenVertical
	splitRootLayoutVerticalThenHorizontal
)

// buildSplitRootLayout populates wm/win1 with the layout identified
// by l. The returned siblings are returned so the test can interact
// with them if needed; callers that don't care may discard them.
func buildSplitRootLayout(
	t *testing.T, wm *WindowManager, win1 Window, l splitRootLayout,
) (win2, win3 Window) {
	t.Helper()
	switch l {
	case splitRootLayoutEmpty:
		return
	case splitRootLayoutHorizontalThenVertical:
		var ok bool
		win2, ok = wm.SplitHorizontal(win1, &component.TestComponent{Ch: 'B'})
		require.True(t, ok)
		win3, ok = wm.SplitVertical(win1, &component.TestComponent{Ch: 'C'})
		require.True(t, ok)
		return
	case splitRootLayoutVerticalThenHorizontal:
		var ok bool
		win2, ok = wm.SplitVertical(win1, &component.TestComponent{Ch: 'B'})
		require.True(t, ok)
		win3, ok = wm.SplitHorizontal(win1, &component.TestComponent{Ch: 'C'})
		require.True(t, ok)
		return
	}
	t.Fatalf("unknown layout: %d", l)
	return
}

// TestSplitRoot is a table-driven regression suite for
// WindowManager.SplitRoot. It covers every supported Alignment
// (left, right, top, bottom) across three starting layouts:
//
//   - a single-tile root,
//   - an asymmetric split-horizontal-then-split-vertical layout,
//   - an asymmetric split-vertical-then-split-horizontal layout,
//
// and asserts the final terminal rendering as a literal string.
func TestSplitRoot(t *testing.T) {
	tests := []struct {
		name      string
		layout    splitRootLayout
		alignment component.Alignment
		expected  string
	}{
		{
			name:      "empty_left",
			layout:    splitRootLayoutEmpty,
			alignment: component.AlignmentLeft,
			expected: `
┌────────┐┌────────┐
│DDDDDDDD││AAAAAAAA│
│DDDDDDDD││AAAAAAAA│
│DDDDDDDD││AAAAAAAA│
│DDDDDDDD││AAAAAAAA│
│DDDDDDDD││AAAAAAAA│
│DDDDDDDD││AAAAAAAA│
└────────┘└────────┘`,
		},
		{
			name:      "empty_right",
			layout:    splitRootLayoutEmpty,
			alignment: component.AlignmentRight,
			expected: `
┌────────┐┌────────┐
│AAAAAAAA││DDDDDDDD│
│AAAAAAAA││DDDDDDDD│
│AAAAAAAA││DDDDDDDD│
│AAAAAAAA││DDDDDDDD│
│AAAAAAAA││DDDDDDDD│
│AAAAAAAA││DDDDDDDD│
└────────┘└────────┘`,
		},
		{
			name:      "empty_top",
			layout:    splitRootLayoutEmpty,
			alignment: component.AlignmentTop,
			expected: `
┌──────────────────┐
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
┌──────────────────┐
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`,
		},
		{
			name:      "empty_bottom",
			layout:    splitRootLayoutEmpty,
			alignment: component.AlignmentBottom,
			expected: `
┌──────────────────┐
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘
┌──────────────────┐
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘`,
		},
		{
			name:      "horizontal_then_vertical_left",
			layout:    splitRootLayoutHorizontalThenVertical,
			alignment: component.AlignmentLeft,
			expected: `
┌────────┐┌───┐┌───┐
│DDDDDDDD││AAA││CCC│
│DDDDDDDD││AAA││CCC│
│DDDDDDDD│└───┘└───┘
│DDDDDDDD│┌────────┐
│DDDDDDDD││BBBBBBBB│
│DDDDDDDD││BBBBBBBB│
└────────┘└────────┘`,
		},
		{
			name:      "horizontal_then_vertical_right",
			layout:    splitRootLayoutHorizontalThenVertical,
			alignment: component.AlignmentRight,
			expected: `
┌───┐┌───┐┌────────┐
│AAA││CCC││DDDDDDDD│
│AAA││CCC││DDDDDDDD│
└───┘└───┘│DDDDDDDD│
┌────────┐│DDDDDDDD│
│BBBBBBBB││DDDDDDDD│
│BBBBBBBB││DDDDDDDD│
└────────┘└────────┘`,
		},
		{
			name:      "horizontal_then_vertical_top",
			layout:    splitRootLayoutHorizontalThenVertical,
			alignment: component.AlignmentTop,
			// Note: existing tiles (A/B/C) lose their frame chrome
			// in this top/bottom flip because each ends up too
			// short to fit a top+bottom border plus a content row.
			expected: `
┌──────────────────┐
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
AAAAAAAAAACCCCCCCCCC
AAAAAAAAAACCCCCCCCCC
BBBBBBBBBBBBBBBBBBBB
BBBBBBBBBBBBBBBBBBBB`,
		},
		{
			name:      "horizontal_then_vertical_bottom",
			layout:    splitRootLayoutHorizontalThenVertical,
			alignment: component.AlignmentBottom,
			// Note: same frame-loss caveat as horizontal_then_vertical_top.
			expected: `
AAAAAAAAAACCCCCCCCCC
AAAAAAAAAACCCCCCCCCC
BBBBBBBBBBBBBBBBBBBB
BBBBBBBBBBBBBBBBBBBB
┌──────────────────┐
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘`,
		},
		{
			name:      "vertical_then_horizontal_left",
			layout:    splitRootLayoutVerticalThenHorizontal,
			alignment: component.AlignmentLeft,
			expected: `
┌────┐┌─────┐┌─────┐
│DDDD││AAAAA││BBBBB│
│DDDD││AAAAA││BBBBB│
│DDDD│└─────┘│BBBBB│
│DDDD│┌─────┐│BBBBB│
│DDDD││CCCCC││BBBBB│
│DDDD││CCCCC││BBBBB│
└────┘└─────┘└─────┘`,
		},
		{
			name:      "vertical_then_horizontal_right",
			layout:    splitRootLayoutVerticalThenHorizontal,
			alignment: component.AlignmentRight,
			expected: `
┌────┐┌─────┐┌─────┐
│AAAA││BBBBB││DDDDD│
│AAAA││BBBBB││DDDDD│
└────┘│BBBBB││DDDDD│
┌────┐│BBBBB││DDDDD│
│CCCC││BBBBB││DDDDD│
│CCCC││BBBBB││DDDDD│
└────┘└─────┘└─────┘`,
		},
		{
			name:      "vertical_then_horizontal_top",
			layout:    splitRootLayoutVerticalThenHorizontal,
			alignment: component.AlignmentTop,
			// Note: the existing A/C column loses its frame chrome
			// here for the same reason as horizontal_then_vertical_top —
			// each tile is too short for a full frame.
			expected: `
┌──────────────────┐
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
AAAAAAAAAA┌────────┐
AAAAAAAAAA│BBBBBBBB│
CCCCCCCCCC│BBBBBBBB│
CCCCCCCCCC└────────┘`,
		},
		{
			name:      "vertical_then_horizontal_bottom",
			layout:    splitRootLayoutVerticalThenHorizontal,
			alignment: component.AlignmentBottom,
			// Note: same frame-loss caveat as vertical_then_horizontal_top.
			expected: `
AAAAAAAAAA┌────────┐
AAAAAAAAAA│BBBBBBBB│
CCCCCCCCCC│BBBBBBBB│
CCCCCCCCCC└────────┘
┌──────────────────┐
│DDDDDDDDDDDDDDDDDD│
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := term.NewStringWriter(20, 8)
			wm, win1 := NewWindowManager(
				&component.TestComponent{Ch: 'A'}, testWindowManagerConfig(),
			)
			wm.Resize(20, 8)
			_, _ = buildSplitRootLayout(t, wm, win1, tc.layout)

			cases := []comptest.TestCase{
				{
					Action: func() {
						_, ok := wm.SplitRoot(
							tc.alignment,
							&component.TestComponent{Ch: 'D'},
						)
						require.True(t, ok)
					},
					Expected: tc.expected,
				},
			}

			comptest.TestComponent(t, wm, w, cases)
		})
	}
}

func TestWindowZeroValue(t *testing.T) {
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
			win.SetContent(component.NewString("ballz"))
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

	h1 := component.TestComponent{Ch: 'A'}
	wm, w1 := NewWindowManager(&h1, testWindowManagerConfig())
	wm.Resize(20, 8)

	var w2, w3 Window
	var prevFloating tui.Component
	var ok bool
	h2 := component.TestComponent{Ch: 'B'}
	hnop := component.TestComponent{Ch: 0}
	h3 := component.TestComponent{Ch: 'C'}

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
			floating := component.StaticFloating(&h2, 2, 2)
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentLeft | component.AlignmentTop,
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
			floating := component.StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentRight | component.AlignmentTop,
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
			floating := component.StaticFloating(&h2, 2, 2)
			assert.False(t, w2.Closed())
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentRight | component.AlignmentBottom,
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
			floating := component.StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentLeft | component.AlignmentBottom,
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
			floating := component.StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentHorizontallyCentered,
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
			floating := component.StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentVerticallyCentered,
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
			floating := component.StaticFloating(&hnop, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentCentered,
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
			floating := component.StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentBottom,
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
			floating := component.StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentTop,
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
			floating := component.StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentLeft,
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
			floating := component.StaticFloating(&h2, 2, 2)
			w2.Close()
			assert.True(t, w2.Closed())
			w2 = wm.FloatingWindow(floating,
				FloatingConfig{
					Alignment: component.AlignmentRight,
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
			floating := component.StaticFloating(&h2, 2, 2)
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
			// test setting non floating component
			prevFloating = w2.SetContent(&component.TestComponent{Ch: '5'})
		}, `
┌──┐───────────────┐
│55│CCCCCCCCCCCCCCC│
│55│CCCCCCCCCCCCCCC│
└──┘CCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
│CCCCCCCCCCCCCCCCCC│
└──────────────────┘`,
		}, {func() {
			// test return of SetContent is always what we expect
			prevFloating = w2.SetContent(prevFloating)
			assert.Equal(t, '5', prevFloating.(*component.TestComponent).Ch)
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
			comp := component.TestComponent{Ch: '#'}
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
			comp := component.TestComponent{Ch: '$'}
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

func TestWindowManagerRestoreTileLayout(t *testing.T) {
	tests := []struct {
		name        string
		frame       bool
		layout      TileLayout
		content     map[uint64]rune
		wantMapped  []uint64
		wantFloats  int
		assertExtra func(*testing.T, *WindowManager, map[uint64]Window)
	}{
		{
			name:       "single leaf no frame",
			layout:     TileLayout{WindowID: 1},
			content:    map[uint64]rune{1: 'A'},
			wantMapped: []uint64{1},
		},
		{
			name:       "single leaf with frame",
			frame:      true,
			layout:     TileLayout{WindowID: 1},
			content:    map[uint64]rune{1: 'B'},
			wantMapped: []uint64{1},
		},
		{
			name: "deep nested layout",
			layout: TileLayout{
				Split: SplitOrientationVertical,
				Children: []TileLayout{
					{WindowID: 1},
					{
						Split: SplitOrientationHorizontal,
						Children: []TileLayout{
							{WindowID: 2},
							{
								Split:    SplitOrientationVertical,
								Children: []TileLayout{{WindowID: 3}, {WindowID: 4}},
							},
						},
					},
				},
			},
			content:    map[uint64]rune{1: 'A', 2: 'B', 3: 'C', 4: 'D'},
			wantMapped: []uint64{1, 2, 3, 4},
		},
		{
			name: "nil content fallback is drawable",
			layout: TileLayout{
				Split:    SplitOrientationHorizontal,
				Children: []TileLayout{{WindowID: 1}, {WindowID: 2}},
			},
			content:    map[uint64]rune{2: 'Z'},
			wantMapped: []uint64{1, 2},
			assertExtra: func(t *testing.T, wm *WindowManager, restored map[uint64]Window) {
				assert.NotPanics(t, func() {
					writer := term.NewStringWriter(20, 8)
					wm.Draw(writer)
				})
				assertEqualTile(t, restored[2], 'Z')
			},
		},
		{
			name: "floating windows cleared",
			layout: TileLayout{
				Split:    SplitOrientationHorizontal,
				Children: []TileLayout{{WindowID: 1}, {WindowID: 2}},
			},
			content:    map[uint64]rune{1: 'L', 2: 'R'},
			wantMapped: []uint64{1, 2},
			wantFloats: 2,
			assertExtra: func(t *testing.T, wm *WindowManager, restored map[uint64]Window) {
				require.Equal(t, 0, wm.SizeFloating())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := testWindowManagerConfig()
			cfg.Frame = tt.frame
			wm, _ := NewWindowManager(&component.TestComponent{Ch: 'O'}, cfg)
			wm.Resize(20, 8)
			for i := 0; i < tt.wantFloats; i++ {
				wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'F'}, 4, 4),
					FloatingConfig{Alignment: component.AlignmentCentered})
			}

			restored := wm.RestoreTileLayout(tt.layout, func(windowID uint64) tui.Component {
				ch, ok := tt.content[windowID]
				if !ok {
					return nil
				}
				return &component.TestComponent{Ch: ch}
			})

			require.Len(t, restored, len(tt.wantMapped))
			for _, id := range tt.wantMapped {
				require.Contains(t, restored, id)
				require.NotEqual(t, id, restored[id].ID())
				if ch, ok := tt.content[id]; ok {
					assertEqualTile(t, restored[id], ch)
				}
			}
			assertWindowManagerLayoutShape(t, tt.layout, wm.TileLayout(), restored)
			if tt.assertExtra != nil {
				tt.assertExtra(t, wm, restored)
			}
		})
	}
}

func assertWindowManagerLayoutShape(
	t *testing.T,
	want TileLayout,
	got TileLayout,
	restored map[uint64]Window,
) {
	t.Helper()
	require.Equal(t, want.Split, got.Split)
	require.Len(t, got.Children, len(want.Children))
	if len(want.Children) == 0 {
		if want.WindowID == 0 {
			require.Equal(t, uint64(0), got.WindowID)
			return
		}
		require.Equal(t, restored[want.WindowID].ID(), got.WindowID)
		return
	}
	for i := range want.Children {
		assertWindowManagerLayoutShape(t, want.Children[i], got.Children[i], restored)
	}
}

func TestWindowManagerMinimize(t *testing.T) {
	w := term.NewStringWriter(20, 8)

	h1 := component.TestComponent{Ch: 'A'}
	wm, _ := NewWindowManager(&h1, testWindowManagerConfig())
	wm.Resize(20, 8)

	var fwin Window
	tests := []comptest.TestCase{
		{
			Action: func() {
				floating := component.StaticFloating(&component.TestComponent{Ch: 'u'}, 2, 2)
				fwin = wm.FloatingWindow(floating,
					FloatingConfig{
						Alignment: component.AlignmentCentered,
					},
				)
				assert.True(t, fwin.MinimizeUp(0))
			}, Expected: `
┌──────────────────┐
┌──────────────────┐
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
│AAAAAAAAAAAAAAAAAA│
└──────────────────┘`,
		}, {Action: func() {
			w := wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'l'}, 2, 2),
				FloatingConfig{
					Alignment: component.AlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeLeft(0))
			assert.False(t, w.MinimizeLeft(0))

			w = wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'r'}, 2, 2),
				FloatingConfig{
					Alignment: component.AlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeRight(0))
			assert.False(t, w.MinimizeRight(0))

			w = wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'd'}, 2, 2),
				FloatingConfig{
					Alignment: component.AlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeDown(0))
			assert.False(t, w.MinimizeDown(0))
			assert.False(t, w.MinimizeLeft(0))
			assert.False(t, w.MinimizeRight(0))
			assert.False(t, w.MinimizeUp(0))
		}, Expected: `

┌──────────────────┐
┌┌────────────────┐┐
││AAAAAAAAAAAAAAAA││
││AAAAAAAAAAAAAAAA││
││AAAAAAAAAAAAAAAA││
││AAAAAAAAAAAAAAAA││
└└────────────────┘┘
└──────────────────┘`,
		}, {Action: func() {
			assert.True(t, fwin.Unminimize())
			assert.False(t, fwin.Unminimize())
		}, Expected: `
┌┌────────────────┐┐
││AAAAAA┌──┐AAAAAA││
││AAAAAA│uu│AAAAAA││
││AAAAAA│uu│AAAAAA││
││AAAAAA└──┘AAAAAA││
││AAAAAAAAAAAAAAAA││
└└────────────────┘┘
└──────────────────┘`,
		}, {Action: func() {
			assert.True(t, fwin.MinimizeUp(0))
			w := wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'U'}, 2, 2),
				FloatingConfig{
					Alignment: component.AlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeUp(1))
			w = wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'L'}, 2, 2),
				FloatingConfig{
					Alignment: component.AlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeLeft(1))

			w = wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'R'}, 2, 2),
				FloatingConfig{
					Alignment: component.AlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeRight(1))

			w = wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'D'}, 2, 2),
				FloatingConfig{
					Alignment: component.AlignmentCentered,
				},
			)
			assert.True(t, w.MinimizeDown(1))
		}, Expected: `
┌──────────────────┐
┌──────────────────┐
│UUUUUUUUUUUUUUUUUU│
┌┌─AAAAAAAAAAAAAA─┐┐
└└─AAAAAAAAAAAAAA─┘┘
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
└──────────────────┘`,
		}, {Action: func() {
			require.NoError(t, fwin.Close())
		}, Expected: `
┌──────────────────┐
│UUUUUUUUUUUUUUUUUU│
┌┌─┌────────────┐─┐┐
││L│AAAAAAAAAAAA│R││
└└─└────────────┘─┘┘
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
└──────────────────┘`,
		}, {Action: func() {
			win, ok := wm.WindowAt(term.Coordinates{X: 3, Y: 2})
			require.True(t, ok)
			assert.Equal(t, 'A', win.Content().(*component.TestComponent).Ch)

			wof, ok := win.TileLeft()
			require.True(t, ok)
			assertEqualTile(t, wof, 'L')

			wof, ok = win.TileRight()
			require.True(t, ok)
			assertEqualTile(t, wof, 'R')

			wof, ok = win.TileUp()
			require.True(t, ok)
			assertEqualTile(t, wof, 'U')

			wof, ok = win.TileDown()
			require.True(t, ok)
			assertEqualTile(t, wof, 'D')

			win, ok = wm.WindowAt(term.Coordinates{X: 0, Y: 0})
			require.True(t, ok)
			assertEqualTile(t, win, 'U')

			wof, ok = win.TileLeft()
			assert.False(t, ok)

			wof, ok = win.TileRight()
			assert.False(t, ok)

			wof, ok = win.TileUp()
			assert.False(t, ok)

			wof, ok = win.TileDown()
			require.True(t, ok)
			assert.Equal(t, 'A', wof.Content().(*component.TestComponent).Ch)

			win, ok = wm.WindowAt(term.Coordinates{X: 0, Y: 2})
			assert.True(t, ok)
			assertEqualTile(t, win, 'l')

			wof, ok = win.TileLeft()
			assert.False(t, ok)

			wof, ok = win.TileRight()
			require.True(t, ok)
			assertEqualTile(t, wof, 'L')

			wof, ok = win.TileUp()
			require.True(t, ok)
			assertEqualTile(t, wof, 'U')

			wof, ok = win.TileDown()
			require.True(t, ok)
			assertEqualTile(t, wof, 'D')

			win, ok = wm.WindowAt(term.Coordinates{X: 19, Y: 2})
			require.True(t, ok)
			assertEqualTile(t, win, 'r')

			wof, ok = win.TileLeft()
			require.True(t, ok)
			assertEqualTile(t, wof, 'R')

			wof, ok = win.TileRight()
			assert.False(t, ok)

			wof, ok = win.TileUp()
			require.True(t, ok)
			assertEqualTile(t, wof, 'U')

			wof, ok = win.TileDown()
			require.True(t, ok)
			assertEqualTile(t, wof, 'D')

			win, ok = wm.WindowAt(term.Coordinates{X: 19, Y: 7})
			require.True(t, ok)
			assertEqualTile(t, win, 'd')

			wof, ok = win.TileLeft()
			assert.False(t, ok)

			wof, ok = win.TileRight()
			assert.False(t, ok)

			wof, ok = win.TileUp()
			require.True(t, ok)
			assertEqualTile(t, wof, 'D')

			wof, ok = win.TileDown()
			assert.False(t, ok)

		}, Expected: `
┌──────────────────┐
│UUUUUUUUUUUUUUUUUU│
┌┌─┌────────────┐─┐┐
││L│AAAAAAAAAAAA│R││
└└─└────────────┘─┘┘
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
└──────────────────┘`,
		}, {Action: func() {
			win, ok := wm.WindowAt(term.Coordinates{Y: 6})
			require.True(t, ok)
			assertEqualTile(t, win, 'D')

			require.True(t, win.Unminimize())
		}, Expected: `
┌──────────────────┐
│UUUUUUUUUUUUUUUUUU│
┌┌─┌────────────┐─┐┐
││L│AAAA┌──┐AAAA│R││
││L│AAAA│DD│AAAA│R││
││L│AAAA└──┘AAAA│R││
└└─└────────────┘─┘┘
└──────────────────┘`,
		}, {Action: func() {
			win, ok := wm.WindowAt(term.Coordinates{Y: 3, X: 9})
			require.True(t, ok)
			assertEqualTile(t, win, 'D')
			require.True(t, win.MinimizeDown(1))

			w := wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'X'}, 7, 100),
				FloatingConfig{
					Alignment: component.AlignmentCentered,
				},
			)
			wof, ok := w.TileRight()
			require.True(t, ok)
			assert.Equal(t, 'A', wof.Content().(*component.TestComponent).Ch)
		}, Expected: `
┌──────────────────┐
│UUUUUUUUUUUUUUUUUU│
┌┌─┌────────────┐─┐┐
││L│AXXXXXXXXXAA│R││
└└─└────────────┘─┘┘
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
└──────────────────┘`,
		}, {Action: func() {
			win, ok := wm.WindowAt(term.Coordinates{Y: 3, X: 6})
			require.True(t, ok)
			assertEqualTile(t, win, 'X')
			require.NoError(t, win.Close())

			w := wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'X'}, 100, 100),
				FloatingConfig{
					Alignment: component.AlignmentCentered,
				},
			)
			wof, ok := w.TileRight()
			require.True(t, ok)
			assertEqualTile(t, wof, 'A')

			wof, ok = w.TileLeft()
			require.True(t, ok)
			assertEqualTile(t, wof, 'A')

			wof, ok = w.TileUp()
			require.True(t, ok)
			assertEqualTile(t, wof, 'A')

			wof, ok = w.TileDown()
			require.True(t, ok)
			assertEqualTile(t, wof, 'A')
		}, Expected: `
┌──────────────────┐
│UUUUUUUUUUUUUUUUUU│
┌┌─┌────────────┐─┐┐
││L│XXXXXXXXXXXX│R││
└└─└────────────┘─┘┘
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
└──────────────────┘`,
		}, {Action: func() {
			assert.NotPanics(t, func() {
				wm.Resize(2, 2)
				wm.WindowAt(term.Coordinates{X: 0, Y: 0})
				wm.WindowAt(term.Coordinates{X: 0, Y: 1})
				wm.WindowAt(term.Coordinates{X: 1, Y: 0})
				wm.WindowAt(term.Coordinates{X: 1, Y: 1})
			})
		}, Expected: `

                    
                    
                    
    X               
                    
                    
                    
                    `,
		}, {Action: func() {
			wm.Resize(20, 8)
			win, ok := wm.WindowAt(term.Coordinates{Y: 3, X: 6})
			require.True(t, ok)
			assertEqualTile(t, win, 'X')
			require.NoError(t, win.Close())

			win, ok = wm.WindowAt(term.Coordinates{Y: 3, X: 6})
			require.True(t, ok)
			assert.Equal(t, 'A', win.Content().(*component.TestComponent).Ch)

			assert.True(t, win.MinimizeUp(0))
			_, ok = wm.SplitVertical(win, &component.TestComponent{Ch: 'a'})
			require.True(t, ok)

			require.True(t, win.MinimizeLeft(0))

			win, ok = wm.WindowAt(term.Coordinates{Y: 2, X: 6})
			require.True(t, ok)
			assert.Equal(t, 'a', win.Content().(*component.TestComponent).Ch)

			wof, ok := win.TileRight()
			require.True(t, ok)
			assertEqualTile(t, wof, 'R')

			wof, ok = win.TileLeft()
			require.True(t, ok)
			assert.Equal(t, 'A', wof.Content().(*component.TestComponent).Ch)

			wof, ok = win.TileUp()
			require.True(t, ok)
			assertEqualTile(t, wof, 'U')

			wof, ok = win.TileDown()
			require.True(t, ok)
			assertEqualTile(t, wof, 'D')
		}, Expected: `
┌──────────────────┐
│UUUUUUUUUUUUUUUUUU│
┌┌─┌─┐┌─────────┐─┐┐
││L│A││aaaaaaaaa│R││
└└─└─┘└─────────┘─┘┘
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
└──────────────────┘`,
		}, {Action: func() {
			win, ok := wm.WindowAt(term.Coordinates{Y: 4, X: 16})
			require.True(t, ok)
			assert.Equal(t, 'a', win.Content().(*component.TestComponent).Ch)

			win, ok = wm.SplitHorizontal(win, &component.TestComponent{Ch: 'b'})
			require.True(t, ok)

			require.True(t, win.MinimizeDown(0))

			win, ok = wm.WindowAt(term.Coordinates{Y: 2, X: 6})
			require.True(t, ok)
			assert.Equal(t, 'A', win.Content().(*component.TestComponent).Ch)

			wof, ok := win.TileRight()
			require.True(t, ok)
			assert.Equal(t, 'b', wof.Content().(*component.TestComponent).Ch)

			wof, ok = win.TileLeft()
			require.True(t, ok)
			assertEqualTile(t, wof, 'L')

			wof, ok = win.TileUp()
			require.True(t, ok)
			assertEqualTile(t, wof, 'U')

			wof, ok = win.TileDown()
			require.True(t, ok)
			assertEqualTile(t, wof, 'D')
		}, Expected: `
┌──────────────────┐
│UUUUUUUUUUUUUUUUUU│
┌┌─┌─────────┐aaa─┐┐
││L│AAAAAAAAA│bbbR││
└└─└─────────┘bbb─┘┘
│DDDDDDDDDDDDDDDDDD│
└──────────────────┘
└──────────────────┘`,
		},
	}

	comptest.TestComponent(t, wm, w, tests)
}

func TestExposedRootTileAt(t *testing.T) {
	h1 := component.TestComponent{Ch: 'A'}
	wm, w1 := NewWindowManager(&h1, testWindowManagerConfig())
	wm.Resize(20, 8)

	_, found := w1.TileUp()
	require.False(t, found)
	_, found = w1.TileLeft()
	require.False(t, found)
	_, found = w1.TileRight()
	require.False(t, found)
	_, found = w1.TileDown()
	require.False(t, found)

	h2 := component.TestComponent{Ch: 'B'}
	w2, ok := wm.SplitVertical(w1, &h2)
	require.True(t, ok)

	_, found = w2.TileUp()
	assert.False(t, found)

	require.NoError(t, w1.Close())

	_, found = w2.TileUp()
	assert.False(t, found)
	_, found = w2.TileLeft()
	assert.False(t, found)
	_, found = w2.TileRight()
	assert.False(t, found)
	_, found = w2.TileDown()
	assert.False(t, found)
}

func TestWindowManagerTileFocusFloating(t *testing.T) {
	w := term.NewStringWriter(20, 8)

	wm, w1 := NewWindowManager(&component.TestComponent{Ch: 'A'}, testWindowManagerConfig())
	wm.Resize(20, 8)

	_, ok := w1.TileUp()
	require.False(t, ok)

	w2, ok := wm.SplitVertical(w1, &component.TestComponent{Ch: 'B'})
	require.True(t, ok)
	wm.SplitHorizontal(w1, &component.TestComponent{Ch: 'C'})
	wm.SplitHorizontal(w2, &component.TestComponent{Ch: 'D'})

	var f1 Window
	tests := []comptest.TestCase{
		{
			Action: func() {
				floating := component.StaticFloating(&component.TestComponent{Ch: 'a'}, 2, 4)
				f1 = wm.FloatingWindow(floating,
					FloatingConfig{
						Alignment: component.AlignmentHorizontallyCentered,
					},
				)
				_, ok := f1.TileUp()
				require.False(t, ok)

				actual, ok := f1.TileLeft()
				require.True(t, ok)
				assertEqualTile(t, actual, 'A')

				actual, ok = f1.TileRight()
				require.True(t, ok)
				assertEqualTile(t, actual, 'B')

				actual, ok = f1.TileDown()
				require.True(t, ok)
				assertEqualTile(t, actual, 'A')
			}, Expected: `
┌───────┌──┐───────┐
│AAAAAAA│aa│BBBBBBB│
│AAAAAAA│aa│BBBBBBB│
└───────│aa│───────┘
┌───────│aa│───────┐
│CCCCCCC└──┘DDDDDDD│
│CCCCCCCC││DDDDDDDD│
└────────┘└────────┘`,
		},
		{
			Action: func() {
				require.NoError(t, f1.Close())

				floating := component.StaticFloating(&component.TestComponent{Ch: 'a'}, 2, 4)
				f1 = wm.FloatingWindow(floating,
					FloatingConfig{
						Alignment: component.AlignmentHorizontallyCentered | component.AlignmentBottom,
					},
				)
				_, ok := f1.TileDown()
				require.False(t, ok)

				actual, ok := f1.TileLeft()
				require.True(t, ok)
				assertEqualTile(t, actual, 'C')

				actual, ok = f1.TileRight()
				require.True(t, ok)
				assertEqualTile(t, actual, 'D')

				actual, ok = f1.TileUp()
				require.True(t, ok)
				assertEqualTile(t, actual, 'D')
			}, Expected: `
┌────────┐┌────────┐
│AAAAAAAA││BBBBBBBB│
│AAAAAAA┌──┐BBBBBBB│
└───────│aa│───────┘
┌───────│aa│───────┐
│CCCCCCC│aa│DDDDDDD│
│CCCCCCC│aa│DDDDDDD│
└───────└──┘───────┘`,
		},
		{
			Action: func() {
				require.NoError(t, f1.Close())

				floating := component.StaticFloating(&component.TestComponent{Ch: 'a'}, 12, 2)
				f1 = wm.FloatingWindow(floating,
					FloatingConfig{
						Alignment: component.AlignmentVerticallyCentered,
					},
				)
				_, ok := f1.TileLeft()
				require.False(t, ok)

				actual, ok := f1.TileUp()
				require.True(t, ok)
				assertEqualTile(t, actual, 'A')

				actual, ok = f1.TileDown()
				require.True(t, ok)
				assertEqualTile(t, actual, 'C')

				actual, ok = f1.TileRight()
				require.True(t, ok)
				assertEqualTile(t, actual, 'A')
			}, Expected: `
┌────────┐┌────────┐
│AAAAAAAA││BBBBBBBB│
┌────────────┐BBBBB│
│aaaaaaaaaaaa│─────┘
│aaaaaaaaaaaa│─────┐
└────────────┘DDDDD│
│CCCCCCCC││DDDDDDDD│
└────────┘└────────┘`,
		},
		{
			Action: func() {
				require.NoError(t, f1.Close())

				floating := component.StaticFloating(&component.TestComponent{Ch: 'a'}, 12, 2)
				f1 = wm.FloatingWindow(floating,
					FloatingConfig{
						Alignment: component.AlignmentVerticallyCentered | component.AlignmentRight,
					},
				)
				_, ok := f1.TileRight()
				require.False(t, ok)

				actual, ok := f1.TileUp()
				require.True(t, ok)
				assertEqualTile(t, actual, 'B')

				actual, ok = f1.TileDown()
				require.True(t, ok)
				assertEqualTile(t, actual, 'D')

				actual, ok = f1.TileLeft()
				require.True(t, ok)
				assertEqualTile(t, actual, 'D')
			}, Expected: `
┌────────┐┌────────┐
│AAAAAAAA││BBBBBBBB│
│AAAAA┌────────────┐
└─────│aaaaaaaaaaaa│
┌─────│aaaaaaaaaaaa│
│CCCCC└────────────┘
│CCCCCCCC││DDDDDDDD│
└────────┘└────────┘`,
		},
	}

	comptest.TestComponent(t, wm, w, tests)
}

func TestComponentWindowAt(t *testing.T) {
	h1 := &component.TestComponent{Ch: '1'}
	wm, w1 := NewWindowManager(h1, testWindowManagerConfig())
	wm.Resize(20, 8)

	h2 := &component.TestComponent{Ch: '2'}
	w2, ok := wm.SplitVertical(w1, h2)
	require.True(t, ok)

	h3 := &component.TestComponent{Ch: '3'}
	w3, ok := wm.SplitHorizontal(w2, h3)
	require.True(t, ok)

	h4 := &component.TestComponent{Ch: '4'}
	w4, ok := wm.SplitVertical(w3, h4)
	require.True(t, ok)

	wf := wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'f'}, 100, 100),
		FloatingConfig{
			Alignment: component.AlignmentCentered,
		},
	)
	require.True(t, wf.MinimizeDown(1))

	wF := wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'F'}, 2, 2),
		FloatingConfig{
			Alignment: component.AlignmentCentered,
		},
	)

	/*
			  ┌────────┐┌────────┐
		      │1111111┌──┐2222222│
		      │1111111│FF│───────┘
		      │1111111│FF│──┐┌───┐
		      │1111111└──┘33││444│
		      └────────┘└───┘└───┘
		      │ffffffffffffffffff│
		      └──────────────────┘
	*/

	suite := []struct {
		at               term.Coordinates
		expectedOut      Window
		expectedNotFound bool
	}{
		{
			at:          term.Coordinates{X: 10, Y: 3},
			expectedOut: wF,
		},
		{
			at:          term.Coordinates{X: 14, Y: 5},
			expectedOut: w3,
		},
		{
			at:          term.Coordinates{X: 12, Y: 3},
			expectedOut: w3,
		},
		{
			at:          term.Coordinates{X: 19, Y: 2},
			expectedOut: w2,
		},
		{
			at:          term.Coordinates{X: 10, Y: 0},
			expectedOut: w2,
		},
		{
			at:          term.Coordinates{X: 0, Y: 6},
			expectedOut: wf,
		},
		{
			at:          term.Coordinates{X: 0, Y: 7},
			expectedOut: wf,
		},
		{
			at:          term.Coordinates{X: 19, Y: 7},
			expectedOut: wf,
		},
		{
			at:          term.Coordinates{X: 19, Y: 6},
			expectedOut: wf,
		},
		{
			at:          term.Coordinates{X: 0, Y: 5},
			expectedOut: w1,
		},
		{
			at:          term.Coordinates{X: 9, Y: 0},
			expectedOut: w1,
		},
		{
			at:          term.Coordinates{X: 15, Y: 3},
			expectedOut: w4,
		},
		{
			at:          term.Coordinates{X: 19, Y: 5},
			expectedOut: w4,
		},
		{
			at:          term.Coordinates{X: 8, Y: 1},
			expectedOut: wF,
		},
		{
			at:          term.Coordinates{X: 11, Y: 4},
			expectedOut: wF,
		},
	}
	for i, test := range suite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			actualOut, actualOk := wm.WindowAt(test.at)
			require.Equal(t, !test.expectedNotFound, actualOk)
			assert.Equal(t, test.expectedOut, actualOut)
		})
	}

	wx := wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'x'}, 100, 100),
		FloatingConfig{
			Alignment: component.AlignmentCentered,
		},
	)
	require.True(t, wx.MinimizeUp(2))
	wy := wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'y'}, 100, 100),
		FloatingConfig{
			Alignment: component.AlignmentCentered,
		},
	)
	require.True(t, wy.MinimizeLeft(2))

	/*
		┌──────────────────┐
		│xxxxxxxxxxxxxxxxxx│
		│xxxxxxxxxxxxxxxxxx│
		┌──┌─────┌──┐2222222
		│yy│11111│FF│3344444
		└──└─────└──┘3344444
		│ffffffffffffffffff│
		└──────────────────┘
	*/

	suite = []struct {
		at               term.Coordinates
		expectedOut      Window
		expectedNotFound bool
	}{
		{
			at:          term.Coordinates{X: 10, Y: 4},
			expectedOut: wF,
		},
		{
			at:          term.Coordinates{X: 13, Y: 4},
			expectedOut: w3,
		},
		{
			at:          term.Coordinates{X: 14, Y: 5},
			expectedOut: w3,
		},
		{
			at:          term.Coordinates{X: 13, Y: 3},
			expectedOut: w2,
		},
		{
			at:          term.Coordinates{X: 19, Y: 3},
			expectedOut: w2,
		},
		{
			at:          term.Coordinates{X: 0, Y: 6},
			expectedOut: wf,
		},
		{
			at:          term.Coordinates{X: 0, Y: 7},
			expectedOut: wf,
		},
		{
			at:          term.Coordinates{X: 19, Y: 7},
			expectedOut: wf,
		},
		{
			at:          term.Coordinates{X: 19, Y: 6},
			expectedOut: wf,
		},
		{
			at:          term.Coordinates{X: 3, Y: 3},
			expectedOut: w1,
		},
		{
			at:          term.Coordinates{X: 8, Y: 5},
			expectedOut: w1,
		},
		{
			at:          term.Coordinates{X: 15, Y: 4},
			expectedOut: w4,
		},
		{
			at:          term.Coordinates{X: 19, Y: 5},
			expectedOut: w4,
		},
		{
			at:          term.Coordinates{X: 9, Y: 4},
			expectedOut: wF,
		},
		{
			at:          term.Coordinates{X: 12, Y: 4},
			expectedOut: wF,
		},
	}
	for i, test := range suite {
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			actualOut, actualOk := wm.WindowAt(test.at)
			require.Equal(t, !test.expectedNotFound, actualOk)
			assert.Equal(t, test.expectedOut, actualOut)
		})
	}
}

func TestFixedSizeWindows(t *testing.T) {
	h1 := &component.TestComponent{Ch: '1'}
	wm, w1 := NewWindowManager(h1, testWindowManagerConfig())
	wm.Resize(20, 8)

	var w2, w3, w4, wf, wF Window
	tests := []comptest.TestCase{
		{
			Action: func() {

				assert.False(t, wm.SetHeight(w1, 3))
				// it's the only window so it should fail
				require.False(t, wm.SetWidth(w1, 3))
				assert.Equal(t, 20, w1.MaxWidth())
				assert.Equal(t, 8, w1.MaxHeight())
				assert.Equal(t, 3, w1.MinWidth())
				assert.Equal(t, 3, w1.MinHeight())

				h2 := &component.TestComponent{Ch: '2'}
				var ok bool
				w2, ok = wm.SplitVertical(w1, h2)
				require.True(t, ok)

				assert.Equal(t, 17, w1.MaxWidth())
				assert.Equal(t, 8, w1.MaxHeight())
				assert.Equal(t, 17, w2.MaxWidth())
				assert.Equal(t, 8, w2.MaxHeight())

				assert.False(t, wm.SetHeight(w2, 3))
				require.True(t, wm.SetWidth(w1, 3))
				// should reset the fixed width of w1
				require.True(t, wm.SetWidth(w2, 3))

				// cannot resize to less than content size 1
				// with frame, that is less than 3.
				require.False(t, wm.SetWidth(w2, 2))

			}, Expected: `
┌───────────────┐┌─┐
│111111111111111││2│
│111111111111111││2│
│111111111111111││2│
│111111111111111││2│
│111111111111111││2│
│111111111111111││2│
└───────────────┘└─┘`,
		},
		{
			Action: func() {
				h3 := &component.TestComponent{Ch: '3'}
				var ok bool
				w3, ok = wm.SplitHorizontal(w2, h3)
				require.True(t, ok)
				assert.Equal(t, 17, w1.MaxWidth())
				assert.Equal(t, 8, w1.MaxHeight())
				assert.Equal(t, 17, w2.MaxWidth())
				assert.Equal(t, 5, w2.MaxHeight())
				assert.Equal(t, 17, w3.MaxWidth())
				assert.Equal(t, 5, w3.MaxHeight())
			}, Expected: `
┌───────────────┐┌─┐
│111111111111111││2│
│111111111111111││2│
│111111111111111│└─┘
│111111111111111│┌─┐
│111111111111111││3│
│111111111111111││3│
└───────────────┘└─┘`,
		},
		{
			Action: func() {
				assert.True(t, wm.SetHeight(w3, 3))
				assert.True(t, wm.SetWidth(w3, 13))
			}, Expected: `
┌─────┐┌───────────┐
│11111││22222222222│
│11111││22222222222│
│11111││22222222222│
│11111│└───────────┘
│11111│┌───────────┐
│11111││33333333333│
└─────┘└───────────┘`,
		},
		{
			Action: func() {
				h4 := &component.TestComponent{Ch: '4'}
				var ok bool
				w4, ok = wm.SplitVertical(w3, h4)
				require.True(t, ok)
				assert.Equal(t, 17, w1.MaxWidth()) // let's keep things simple
				assert.Equal(t, 8, w1.MaxHeight())
				assert.Equal(t, 17, w2.MaxWidth())
				assert.Equal(t, 5, w2.MaxHeight())
				assert.Equal(t, 10, w3.MaxWidth()) // ditto
				assert.Equal(t, 5, w3.MaxHeight())
				assert.Equal(t, 10, w4.MaxWidth()) // ditto
				assert.Equal(t, 5, w3.MaxHeight())
			}, Expected: `
┌─────┐┌───────────┐
│11111││22222222222│
│11111││22222222222│
│11111││22222222222│
│11111│└───────────┘
│11111│┌────┐┌─────┐
│11111││3333││44444│
└─────┘└────┘└─────┘`,
		},
		{
			Action: func() {
				wf = wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'f'}, 100, 100),
					FloatingConfig{
						Alignment: component.AlignmentCentered,
					},
				)
				require.True(t, wf.MinimizeDown(1))

				wF = wm.FloatingWindow(component.StaticFloating(&component.TestComponent{Ch: 'F'}, 2, 2),
					FloatingConfig{
						Alignment: component.AlignmentCentered,
					},
				)

				assert.Equal(t, 17, w1.MaxWidth())
				assert.Equal(t, 6, w1.MaxHeight())
				assert.Equal(t, 17, w2.MaxWidth())
				assert.Equal(t, 3, w2.MaxHeight())
				assert.Equal(t, 10, w3.MaxWidth())
				assert.Equal(t, 3, w3.MaxHeight())
				assert.Equal(t, 10, w4.MaxWidth())
				assert.Equal(t, 3, w3.MaxHeight())
			}, Expected: `
┌─────┐┌───────────┐
│11111││┌──┐2222222│
│11111│└│FF│───────┘
│11111│┌│FF│┐┌─────┐
│11111││└──┘││44444│
└─────┘└────┘└─────┘
│ffffffffffffffffff│
└──────────────────┘`,
		},
		{
			Action: func() {
				assert.False(t, wm.SetHeight(w4, w4.Height()+1))
				assert.True(t, wm.SetWidth(w4, w4.Width()+2))
			}, Expected: `
┌─────┐┌───────────┐
│11111││┌──┐2222222│
│11111│└│FF│───────┘
│11111│┌│FF│───────┐
│11111││└──┘4444444│
└─────┘└──┘└───────┘
│ffffffffffffffffff│
└──────────────────┘`,
		},
		{
			Action: func() {
				assert.False(t, wm.SetHeight(w2, w2.Height()+1))
				assert.True(t, wm.SetWidth(w2, w2.Width()+2))
				assert.False(t, wm.SetWidth(w2, w2.Width()+8))
			}, Expected: `
┌───┐┌─────────────┐
│111││22┌──┐2222222│
│111│└──│FF│───────┘
│111│┌──│FF│───────┐
│111││33└──┘4444444│
└───┘└────┘└───────┘
│ffffffffffffffffff│
└──────────────────┘`,
		},
		{
			Action: func() {
				assert.False(t, wm.SetHeight(wf, 5)) // is minimized
				assert.False(t, wm.SetWidth(wf, 5))  // is minimized
				assert.True(t, wm.SetHeight(wF, wF.Height()+1))
				assert.True(t, wm.SetWidth(wF, wF.Width()+1))

				assert.Equal(t, 4, wF.MinHeight())
				assert.Equal(t, 4, wF.MinWidth())
				assert.Equal(t, 6, wF.MaxHeight())
				assert.Equal(t, 20, wF.MaxWidth())
			}, Expected: `
┌───┐┌─────────────┐
│111││2┌───┐2222222│
│111│└─│FFF│───────┘
│111│┌─│FFF│───────┐
│111││3└───┘4444444│
└───┘└────┘└───────┘
│ffffffffffffffffff│
└──────────────────┘`,
		},
		{
			Action: func() {
				assert.True(t, wm.SetHeight(wF, 0)) // reset
				assert.True(t, wm.SetWidth(wF, 0))  // reset
			}, Expected: `
┌───┐┌─────────────┐
│111││22┌──┐2222222│
│111│└──│FF│───────┘
│111│┌──│FF│───────┐
│111││33└──┘4444444│
└───┘└────┘└───────┘
│ffffffffffffffffff│
└──────────────────┘`,
		},
		{
			Action: func() {
				require.NoError(t, w3.Close())
				require.NoError(t, w1.Close())
			}, Expected: `
┌──────────────────┐
│2222222┌──┐2222222│
└───────│FF│───────┘
┌───────│FF│───────┐
│4444444└──┘4444444│
└──────────────────┘
│ffffffffffffffffff│
└──────────────────┘`,
		},
		{
			Action: func() {
				require.NoError(t, wf.Close())
				require.NoError(t, wF.Close())
				require.NoError(t, w2.Close())
				require.Error(t, w4.Close())
			}, Expected: `
┌──────────────────┐
│444444444444444444│
│444444444444444444│
│444444444444444444│
│444444444444444444│
│444444444444444444│
│444444444444444444│
└──────────────────┘`,
		},
	}

	w := term.NewStringWriter(20, 8)
	comptest.TestComponent(t, wm, w, tests)
}

func TestSetFrameAttr(t *testing.T) {
	h1 := &component.TestComponent{Ch: '1'}
	cfg := testWindowManagerConfig()
	wm, w1 := NewWindowManager(h1, cfg)
	wm.Resize(20, 8)

	newAttr := term.Attributes{Fg: term.ColorGreen, Bg: term.ColorBlue}
	prev, ok := w1.SetFrameAttr(newAttr)
	assert.True(t, ok)
	assert.Equal(t, cfg.FrameAttr, prev)

	actualAttr, ok := w1.SetFrameAttr(prev)
	assert.True(t, ok)
	assert.Equal(t, newAttr, actualAttr)
}

func assertEqualTile(t *testing.T, win Window, expected rune) {
	t.Helper()
	if f, ok := win.Content().(interface{ Content() tui.Component }); ok {
		assert.Equal(t, string(expected), string(f.Content().(*component.TestComponent).Ch))
	} else {
		assert.Equal(t, string(expected), string(win.Content().(*component.TestComponent).Ch))
	}

}

func testWindowManagerConfig() WindowManagerConfig {
	ret := DefaultWindowManagerConfig()
	ret.NoMaxSize = true
	return ret
}

// TestWindowManagerIterateCloseDuringIteration guards Iterate's contract
// that op may close the visited floating window. closeFloatingWindow's
// slices.Delete shifts and zeroes the backing array of wm.float, and a
// naive for-range would either read a nil tail slot or skip the
// shifted-down neighbor.
func TestWindowManagerIterateCloseDuringIteration(t *testing.T) {
	h := component.TestComponent{Ch: 'A'}
	wm, _ := NewWindowManager(&h, testWindowManagerConfig())
	wm.Resize(20, 8)

	for range 3 {
		wm.FloatingWindow(
			component.StaticFloating(&component.TestComponent{Ch: 'f'}, 2, 2),
			FloatingConfig{Alignment: component.AlignmentCentered},
		)
	}

	var visited int
	assert.NotPanics(t, func() {
		wm.Iterate(func(w Window) {
			visited++
			_ = w.Close()
		})
	})
	// 1 root tile + 3 floating windows
	assert.Equal(t, 4, visited)
	assert.Equal(t, 0, wm.SizeFloating())
}

// TestWindowManagerIterateCloseTilesDuringIteration covers closing tiles
// from inside Iterate, as CloseOtherWindows does. Closing a sibling
// mutates the parent's children slice that Iterate ranges over, so
// without a snapshot some siblings were shifted past and never visited,
// leaking open tiles that the caller intended to close.
func TestWindowManagerIterateCloseTilesDuringIteration(t *testing.T) {
	h := component.TestComponent{Ch: 'A'}
	wm, keep := NewWindowManager(&h, testWindowManagerConfig())
	wm.Resize(40, 8)

	// Flat vertical layout of four sibling tiles, mirroring repeated
	// `windownew right` over the focused window.
	for i := range 3 {
		_, ok := wm.SplitVertical(keep, &component.TestComponent{Ch: rune('B' + i)})
		require.True(t, ok)
	}
	require.Equal(t, 4, wm.SizeTiles())

	// Mimic CloseOtherWindows(keep): visit every tile and close all but
	// the kept one.
	var visited int
	assert.NotPanics(t, func() {
		wm.Iterate(func(w Window) {
			visited++
			if w.node == keep.node {
				return
			}
			_ = w.Close()
		})
	})

	assert.Equal(t, 4, visited, "every tile must be visited exactly once")
	assert.Equal(t, 1, wm.SizeTiles(), "all sibling tiles except the kept one must be closed")

	// The survivor must still be positionable; a detached tile here is
	// what crashed the host on the next cursor calculation.
	assert.NotPanics(t, func() { _ = keep.Position() })
}
