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
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/component/comptest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
)

func TestTabsDraw(t *testing.T) {
	l := NewTabs()
	l.Resize(20, 4)
	fb := component.FrameCharSetDefault()
	fb.BottomLeft, fb.BottomRight = '├', '┤'
	l.SetFrameCharSet(fb)

	w := term.NewStringWriter(20, 9)

	tests := []comptest.TestCase{
		{
			nil, `
┌──────────────────┐
│                  │
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.Add('X', "Atzari") }, `
┌━━━━━━━━──────────┐
│X Atzari          │
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() { l.Resize(20, 9) }, `
┌━━━━━━━━──────────┐
│                  │
│                  │
│                  │
│X Atzari          │
│                  │
│                  │
│                  │
├──────────────────┤`,
		}, {
			func() {
				l.Add('$', "Saturn")
				l.Resize(20, 4)
			}, `
┌━━━━━━━━──────────┐
│X Atzari  $ Saturn│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() {
				assert.True(t, l.MoveLeft(1))
				assert.False(t, l.MoveLeft(0))
			}, `
┌──────────━━━━━━━━┐
│$ Saturn  X Atzari│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() {
				assert.True(t, l.MoveTo(1, 0))
			}, `
┌━━━━━━━━──────────┐
│X Atzari  $ Saturn│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() {
				assert.True(t, l.MoveTo(0, 1))
			}, `
┌──────────━━━━━━━━┐
│$ Saturn  X Atzari│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		}, {
			func() {
				assert.True(t, l.MoveRight(0))
				assert.False(t, l.MoveRight(1))
			}, `
┌━━━━━━━━──────────┐
│X Atzari  $ Saturn│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		},
		// ----------------------------------------------------------------
		// Resize algorithm: when not all tabs fit at full width, the
		// focused tab keeps its full label and the rest are truncated to
		// share the remaining inner width. No scrolling, no "..", every
		// tab stays visible.
		// ----------------------------------------------------------------
		// State at this point: tabs = ["X Atzari" (focused), "$ Saturn"],
		// width=20, inner=18.
		// Add "X Other" — three tabs of full widths 8, 8, 7 + 2 separators
		// (2 chars each) = 27. Doesn't fit in 18. Focused (idx 0) keeps
		// full=8; remaining = 18 - 8 - 2*2 = 6 split equally between the
		// two non-focused tabs (3 each → "$ S", "X O").
		{
			func() { l.Add('X', "Other") }, `
┌━━━━━━━━──────────┐
│X Atzari  $ S  X O│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		},
		// Add "X Things" — 4 tabs, focus still idx 0. Focused keeps
		// full=8; remaining = 18 - 8 - 3*2 = 4 over 3 non-focused →
		// 4/3 = 1 with 1 extra column assigned to the leftmost
		// non-focused tab. Widths: Saturn=2, Other=1, Things=1.
		{
			func() { l.Add('X', "Things") }, `
┌━━━━━━━━──────────┐
│X Atzari  $   X  X│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		},
		// Move focus to "X Other" (idx 2, full=7). Remaining = 18 - 7 - 6
		// = 5 over 3 → 5/3 = 1 with 2 extras to the two leftmost
		// non-focused tabs. Widths: Atzari=2, Saturn=2, Things=1.
		{
			func() { l.SetFocus(2) }, `
┌────────━━━━━━━───┐
│X   $   X Other  X│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		},
		// Add two more tabs (focus still on idx 2, "X Other", full=7).
		// All 6 at full width can't fit at min=1. The visible window
		// shrinks until it does, dropping tabs farther from the focused
		// index first (ties shrink from the left). Window becomes
		// [1..4]: Saturn, Other(focused), Things, Morsins. Remaining =
		// 18 - 7 - 3*2 = 5 over 3 → widths 2,2,1.
		{
			func() { l.Add('#', "Morsins"); l.Add('#', "Morsillonins") }, `
┌────━━━━━━━───────┐
│$   X Other  X   #│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		},
		// Move focus to the longest tab (Morsillonins, idx 5, full=14).
		// Window shrinks to [4..5]; Morsins gets remaining = 18 - 14 -
		// 2 = 2 columns.
		{
			func() { l.SetFocus(5) }, `
┌────━━━━━━━━━━━━━━┐
│#   # Morsillonins│
│                  │
├──────────────────┤
                    
                    
                    
                    
                    `,
		},
		// ----------------------------------------------------------------
		// Borderless mode: same algorithm but inner width = full width.
		// ----------------------------------------------------------------
		// width=20, no border, innerW=20. Focused idx 5 (Morsillonins,
		// full=14). Window shrinks to [3..5]: Things, Morsins,
		// Morsillonins. Remaining = 20 - 14 - 2*2 = 2 over 2 → 1,1.
		{
			func() { l.SetBorder(false); l.Resize(20, 1) }, `
X  #  # Morsillonins
                    
                    
                    
                    
                    
                    
                    
                    `,
		},
		// Focus shifts to Things (idx 3, full=8). Window [1..5]:
		// Saturn, Other, Things(focused), Morsins, Morsillonins.
		// Remaining = 20 - 8 - 4*2 = 4 over 4 → 1,1,1,1.
		{
			func() { l.SetFocus(3) }, `
$  X  X Things  #  #
                    
                    
                    
                    
                    
                    
                    
                    `,
		},
		{
			func() { l.SetBorder(false); l.Resize(4, 0) }, `
                    
                    
                    
                    
                    
                    
                    
                    
                    `,
		},
	}

	comptest.TestComponent(t, l, w, tests)
}

// TestTabsDrawFocusHighlightBorderless verifies that in borderless mode at
// h>=2 (the production tab-bar config: TabBarHeight=2, frame=false), tab
// labels render on y=1 and the focused-tab columns on y=0 get the focus
// highlight rune. The non-focused tab columns (and the separator gap) on
// y=0 stay blank.
func TestTabsDrawFocusHighlightBorderless(t *testing.T) {
	l := NewTabs()
	l.SetBorder(false)
	l.Resize(20, 2)

	w := term.NewStringWriter(20, 2)

	tests := []comptest.TestCase{
		// Single focused tab "A alpha" (full=7) at innerW=20.
		// Fast path: 7 ≤ 20. Layout: A alpha + 13 trailing pad.
		// Highlight on y=0 covers columns [0..6].
		{
			func() { l.Add('A', "alpha") }, `
━━━━━━━             
A alpha             `,
		},
		// Two tabs A alpha (focused) + B beta. innerW=20, fast path
		// (7+6+2=15 ≤ 20). Highlight columns [0..6].
		{
			func() { l.Add('B', "beta") }, `
━━━━━━━             
A alpha  B beta     `,
		},
		// Focus moves to B beta (idx 1). Cell starts at column 9
		// (7 + 2 sep), width=6 → highlight columns [9..14].
		{
			func() { l.SetFocus(1) }, `
         ━━━━━━     
A alpha  B beta     `,
		},
	}

	comptest.TestComponent(t, l, w, tests)
}

// TestTabsDrawFocusHighlightCharAttr verifies SetFocusFrameChar overrides
// the highlight rune and that the focusFrame attr passed to SetAttr is
// applied to the highlight cells (and only those — non-focused columns on
// the highlight row stay unattributed).
func TestTabsDrawFocusHighlightCharAttr(t *testing.T) {
	l := NewTabs()
	l.SetBorder(false)
	l.Resize(20, 2)

	focusFrameAttr := term.Attributes{Fg: tcell.ColorRed, Attrs: tcell.AttrBold}
	l.SetAttr(term.Attributes{}, term.Attributes{},
		term.Attributes{}, term.Attributes{},
		focusFrameAttr, term.Attributes{}, term.Attributes{})
	l.SetFocusFrameChar('▀')

	l.Add('A', "alpha")
	l.Add('B', "beta")
	l.SetFocus(1)

	w := term.NewStringWriter(20, 2)
	l.Draw(w)
	cells := w.Cells()

	// y=0, columns [9..14] hold the focus highlight; outside that range
	// the row is left untouched by Tabs.Draw (Ch=0).
	for x := 0; x < 20; x++ {
		c := cells[x]
		if x >= 9 && x <= 14 {
			assert.Equal(t, '▀', c.Ch, "expected highlight rune at x=%d", x)
			assert.Equal(t, focusFrameAttr, c.Attributes,
				"expected focus-frame attr at x=%d", x)
		} else {
			assert.NotEqual(t, '▀', c.Ch,
				"unexpected highlight at x=%d", x)
			assert.Equal(t, term.Attributes{}, c.Attributes,
				"expected no attr at x=%d on highlight row", x)
		}
	}

	// y=1 carries the labels (sanity).
	assert.Equal(t, 'A', cells[20+0].Ch)
	assert.Equal(t, 'B', cells[20+9].Ch)
}

func TestTabsDrawIconAttr(t *testing.T) {
	l := NewTabs()
	l.SetBorder(false)
	l.Resize(30, 1)

	focusAttr := term.Attributes{Fg: tcell.ColorWhite}
	nonFocusAttr := term.Attributes{Fg: tcell.ColorBlue}
	iconAttr := term.Attributes{Bg: tcell.ColorGreen, Attrs: tcell.AttrBold}
	l.SetAttr(focusAttr, nonFocusAttr, term.Attributes{}, term.Attributes{},
		term.Attributes{}, term.Attributes{}, term.Attributes{})

	l.Add('A', "alpha")
	l.Add('B', "beta")
	l.ResetFocus()
	l.SetFocus(0)
	l.SetIconAttr(1, iconAttr)

	w := term.NewStringWriter(30, 1)
	l.Draw(w)
	cells := w.Cells()

	assert.Equal(t, 'A', cells[0].Ch)
	assert.Equal(t, term.Attributes{}, cells[0].Attributes)
	assert.Equal(t, 'a', cells[2].Ch)
	assert.Equal(t, focusAttr, cells[2].Attributes)

	assert.Equal(t, 'B', cells[9].Ch)
	assert.Equal(t, iconAttr, cells[9].Attributes)
	assert.Equal(t, term.Attributes{}, cells[10].Attributes)
	assert.Equal(t, 'b', cells[11].Ch)
	assert.Equal(t, nonFocusAttr, cells[11].Attributes)
}

// TestTabsSetAttrIconAttrs verifies SetAttr's focus/non-focus icon attributes
// are applied to tab icons that don't have a per-tab icon attr override, and
// that per-tab overrides take precedence (via AttributesUnion semantics).
func TestTabsSetAttrIconAttrs(t *testing.T) {
	l := NewTabs()
	l.SetBorder(false)
	l.Resize(30, 1)

	focusAttr := term.Attributes{Fg: tcell.ColorWhite}
	nonFocusAttr := term.Attributes{Fg: tcell.ColorBlue}
	focusIconAttr := term.Attributes{Fg: tcell.ColorGreen, Attrs: tcell.AttrBold}
	nonFocusIconAttr := term.Attributes{Fg: tcell.ColorRed}
	l.SetAttr(focusAttr, nonFocusAttr, focusIconAttr, nonFocusIconAttr,
		term.Attributes{}, term.Attributes{}, term.Attributes{})

	l.Add('A', "alpha")
	l.Add('B', "beta")
	l.ResetFocus()
	l.SetFocus(0)

	w := term.NewStringWriter(30, 1)
	l.Draw(w)
	cells := w.Cells()

	assert.Equal(t, 'A', cells[0].Ch)
	assert.Equal(t, focusIconAttr, cells[0].Attributes)
	assert.Equal(t, 'a', cells[2].Ch)
	assert.Equal(t, focusAttr, cells[2].Attributes)

	assert.Equal(t, 'B', cells[9].Ch)
	assert.Equal(t, nonFocusIconAttr, cells[9].Attributes)
	assert.Equal(t, 'b', cells[11].Ch)
	assert.Equal(t, nonFocusAttr, cells[11].Attributes)
}

func TestTabsDrawCustomSeparator(t *testing.T) {
	l := NewTabs()
	l.Resize(20, 4)
	l.SetNameSeparator(" | ")

	w := term.NewStringWriter(20, 9)

	tests := []comptest.TestCase{
		{
			nil, `
┌──────────────────┐
│                  │
│                  │
└──────────────────┘
                    
                    
                    
                    
                    `,
		}, {
			func() { l.Add('#', "Atzari") }, `
┌━━━━━━━━──────────┐
│# Atzari          │
│                  │
└──────────────────┘
                    
                    
                    
                    
                    `,
		}, {
			// Custom separator is honored when both tabs fit at full width.
			// (innerW=18; "# Atzari" + " | " + "# Saturn" = 8+3+8 = 19,
			// doesn't fit — so the resize path runs with the custom
			// 3-char separator. Focused (Atzari, full=8) keeps full;
			// Saturn gets 18 - 8 - 3 = 7 columns, hard-cut to "# Satur".)
			func() {
				l.Add('#', "Saturn")
			}, `
┌━━━━━━━━──────────┐
│# Atzari | # Satur│
│                  │
└──────────────────┘
                    
                    
                    
                    
                    `,
		},
	}

	comptest.TestComponent(t, l, w, tests)
}

func setupOneTab(width, height int) *Tabs {
	l := NewTabs()
	l.Resize(width, height)

	const name = "blah"
	l.Add('#', name)

	return l
}

// TestTabsDrawResize exercises the resize algorithm at a width wide enough
// that the algorithm's individual cases — full-fit, fair-share with
// give-back, leftover distributed leftmost-first, window pruning when even
// min width doesn't fit, and tie-breaking shrink direction — each show up
// in their own inline literal. Tab full widths chosen for clarity:
//
//	"A alpha"=7, "B beta"=6, "C gamma"=7, "D delta"=7,
//	"E epsilon"=9, "F zeta"=6.
//
// Separator is 2 chars. Width 40 → innerW=38; width 30 → 28; width 20 → 18.
func TestTabsDrawResize(t *testing.T) {
	l := NewTabs()
	l.Resize(40, 4)

	w := term.NewStringWriter(40, 4)

	tests := []comptest.TestCase{
		// 4 tabs, focused idx 0. Full sum 7+6+7+7=27 + 3*2 seps = 33 ≤ 38.
		// Everyone renders full width; remaining 5 columns become
		// trailing pad.
		{
			func() {
				l.Add('A', "alpha")
				l.Add('B', "beta")
				l.Add('C', "gamma")
				l.Add('D', "delta")
			}, `
┌━━━━━━━───────────────────────────────┐
│A alpha  B beta  C gamma  D delta     │
│                                      │
└──────────────────────────────────────┘`,
		},
		// Add "E epsilon". Total full 42 + 4*2 seps = 50 > 38. Focused
		// (A, 7) keeps full; remaining 23 over 4 non-focused. fairShare
		// = 23/4 = 5 with 3 leftover columns. The 3 leftover are given
		// to the first 3 non-focused tabs (B, C, D), but each width is
		// capped at its full label width:
		//   B: 5+1 = 6 → cap 6 (full)
		//   C: 5+1 = 6 → cap 7 → 6
		//   D: 5+1 = 6 → cap 7 → 6
		//   E: 5     = 5 → cap 9 → 5
		// Widths: A=7, B=6, C=6, D=6, E=5.
		{
			func() { l.Add('E', "epsilon") }, `
┌━━━━━━━───────────────────────────────┐
│A alpha  B beta  C gamm  D delt  E eps│
│                                      │
└──────────────────────────────────────┘`,
		},
		// Shift focus to E (idx 4, full=9). remaining=38-9-8=21 over 4.
		// fairShare = 21/4 = 5, leftover 1 → leftmost (A) gets +1.
		//   A: 6 (cap 7), B: 5 (cap 6), C: 5 (cap 7), D: 5 (cap 7).
		// Widths: A=6, B=5, C=5, D=5, E=9.
		{
			func() { l.ResetFocus(); l.SetFocus(4) }, `
┌─────────────────────────────━━━━━━━━━┐
│A alph  B bet  C gam  D del  E epsilon│
│                                      │
└──────────────────────────────────────┘`,
		},
		// Add "F zeta" (idx 5). 6 tabs, focus still E (idx 4, full=9).
		// remaining = 38 - 9 - 5*2 = 19 over 5. fairShare = 19/5 = 3,
		// leftover 4 → A,B,C,D each get +1.
		//   A=4, B=4, C=4, D=4, F=3 (all under caps).
		{
			func() { l.Add('F', "zeta") }, `
┌────────────────────────━━━━━━━━━─────┐
│A al  B be  C ga  D de  E epsilon  F z│
│                                      │
└──────────────────────────────────────┘`,
		},
		// Resize to width=30, innerW=28. Focus still E. Writer is 40
		// wide so each rendered row is padded with 10 trailing spaces.
		// Window [0..5] at min fits: 9 + 5*1 + 5*2 = 24 ≤ 28.
		// remaining = 28 - 9 - 10 = 9 over 5. fairShare = 9/5 = 1,
		// leftover 4 → A,B,C,D get +1.
		//   A=2, B=2, C=2, D=2, F=1.
		{
			func() { l.Resize(30, 4) }, `
┌────────────────━━━━━━━━━───┐          
│A   B   C   D   E epsilon  F│          
│                            │          
└────────────────────────────┘          `,
		},
		// Resize to width=20, innerW=18. Focus still E (idx 4).
		// Window [0..5] at min: 9+5+10=24 > 18 → prune. focusIdx=4 is
		// closer to end than start (dist 4 vs 1), so start++.
		// Window [1..5] min 9+4+8=21 > 18, start++.
		// Window [2..5] min 9+3+6=18 ≤ 18 ✓.
		// Visible: C, D, E (focused), F. remaining = 18-9-6 = 3 over 3.
		// fairShare = 1, leftover 0 → C=1, D=1, F=1.
		{
			func() { l.Resize(20, 4) }, `
┌──────━━━━━━━━━───┐                    
│C  D  E epsilon  F│                    
│                  │                    
└──────────────────┘                    `,
		},
		// Focus to A (idx 0, full=7) at width=20.
		// Window [0..5] at min 22 > 18 → prune. focusIdx=0: dist 0 vs 5
		// → end--. Window [0..4] min 19 > 18, end--.
		// Window [0..3] min 16 ≤ 18 ✓.
		// remaining = 18-7-6 = 5 over 3. fairShare = 1, leftover 2 →
		// B,C get +1.
		//   B=2, C=2, D=1.
		{
			func() { l.ResetFocus(); l.SetFocus(0) }, `
┌━━━━━━━───────────┐                    
│A alpha  B   C   D│                    
│                  │                    
└──────────────────┘                    `,
		},
		// Focus to C (idx 2, full=7) at width=20 — exercises ties in
		// the window-shrink direction. Window [0..5] min 22 > 18 →
		// prune. focusIdx=2: dist 2 vs 3, end--. Window [0..4] min 19 >
		// 18; dist 2 vs 2 → tie, start++. Window [1..4] min 16 ≤ 18 ✓.
		// Visible: B, C(focused), D, E. remaining = 18-7-6 = 5 over 3.
		// fairShare = 1, leftover 2 → B,D get +1.
		//   B=2, D=2, E=1.
		{
			func() { l.ResetFocus(); l.SetFocus(2) }, `
┌────━━━━━━━───────┐                    
│B   C gamma  D   E│                    
│                  │                    
└──────────────────┘                    `,
		},
	}

	comptest.TestComponent(t, l, w, tests)
}

func TestTabsTabAt(t *testing.T) {
	t.Run("should return false if no tab in list", func(t *testing.T) {
		l := NewTabs()
		_, ok := l.TabAt(term.Coordinates{})
		assert.False(t, ok)
	})

	t.Run("should return first tab if coordinates is zero", func(t *testing.T) {
		l := setupOneTab(4, 4)

		idx, ok := l.TabAt(term.Coordinates{})
		require.True(t, ok)
		assert.Equal(t, "blah", l.tabs[idx].name)
	})

	t.Run("should return first tab if size is 0", func(t *testing.T) {
		l := setupOneTab(0, 0)

		l.TabAt(term.Coordinates{})
		idx, ok := l.TabAt(term.Coordinates{})
		require.True(t, ok)
		assert.Equal(t, "blah", l.tabs[idx].name)
	})

	t.Run("should resolve the focused tab when not all tabs fit", func(t *testing.T) {
		l := setupOneTab(4, 4)

		l.Add(0, "1111")
		l.Add(0, "2222222222222222222")
		l.Add(0, "3")
		l.SetFocus(l.Add(0, "4"))

		// force calculating offsets
		w := term.NewStringWriter(4, 4)
		l.Draw(w)
		require.NoError(t, w.Flush())

		idx, ok := l.TabAt(term.Coordinates{})
		require.True(t, ok)
		assert.Equal(t, "4", l.tabs[idx].name)
	})
}

// TestTabsDrawFocusLast regression-tests the case where the focused tab is
// the rightmost tab in the list and the bar doesn't have enough room for
// every tab at full width. The focused tab must keep its full label and
// non-focused tabs must shrink to fit; visually the focused (rightmost)
// tab should NOT be truncated while preceding non-focused tabs are.
func TestTabsDrawFocusLast(t *testing.T) {
	l := NewTabs()
	l.SetBorder(false)
	l.Resize(60, 1)

	w := term.NewStringWriter(60, 1)

	// Tab full widths (icon + space + name; we use icon=0 so full = name
	// width):
	//   ".golangci.yml" = 13
	//   "context.go"    = 10
	//   "events.go"     =  9
	//   "file.go"       =  7
	//   "handler_test.go" = 15
	//   "component.go"  = 12
	// Σ = 66, + 5 seps · 2 = 76 > 60 → resize path.
	// Focus on the last tab (idx 5, full=12). innerW=60, sepLen=2.
	// Window [0..5] min: 12 + 5·1 + 5·2 = 27 ≤ 60 ✓ — no pruning.
	// remaining = 60 − 12 − 10 = 38 over 5 non-focused.
	// fairShare = 38/5 = 7, leftover = 3 → leftmost 3 get +1:
	//   .golangci.yml: 8 (cap 13) → 8
	//   context.go:    8 (cap 10) → 8
	//   events.go:     8 (cap  9) → 8
	//   file.go:       7 (cap  7) → 7 (capped, surplus 0)
	//   handler_test:  7 (cap 15) → 7
	// Used = 8+8+8+7+7 = 38, surplus = 0.
	// Layout: .golangc + sep + context. + sep + events.g + sep + file.go
	//         + sep + handler + sep + component.go
	tests := []comptest.TestCase{
		{
			func() {
				l.Add(0, ".golangci.yml")
				l.Add(0, "context.go")
				l.Add(0, "events.go")
				l.Add(0, "file.go")
				l.Add(0, "handler_test.go")
				l.Add(0, "component.go")
				l.ResetFocus()
				l.SetFocus(5)
			}, `
.golangc  context.  events.g  file.go  handler  component.go`,
		},
	}

	comptest.TestComponent(t, l, w, tests)
}
