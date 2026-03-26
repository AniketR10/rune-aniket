// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package dialoguetui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
)

var _ mouse.Delegate = (*mouseDelegate)(nil)

// drawGrid creates a cellGrid, resizes and draws the component into it.
func drawGrid(comp *Component, width, height int) *cellGrid {
	var g cellGrid
	g.Resize(width, height)
	comp.Draw(&g)
	return &g
}

func TestMouseDelegateSelection(t *testing.T) {
	const (
		width  = 30
		height = 10
	)

	suite := []struct {
		desc    string
		setup   func(*Component)
		input   string // mouse key sequence via term.ParseKeys
		coords  [][2]int
		wantSel string
		wantOK  bool
	}{
		{
			desc: "click-drag on send message",
			setup: func(c *Component) {
				c.AddSendMessage("Hello send")
			},
			input:   "<mouse-left><mouse-left><mouse-release>",
			coords:  [][2]int{{0, 0}, {4, 0}, {4, 0}},
			wantSel: "Hello",
			wantOK:  true,
		},
		{
			desc: "click-drag on received markdown message",
			setup: func(c *Component) {
				c.AddReceiveMessage("Hello markdown")
			},
			input:   "<mouse-left><mouse-left><mouse-release>",
			coords:  [][2]int{{0, 0}, {13, 0}, {13, 0}},
			wantSel: "Hello markdown",
			wantOK:  true,
		},
		{
			desc: "click-drag on empty area yields no selection",
			setup: func(c *Component) {
				c.AddSendMessage("Hello send")
			},
			input:   "<mouse-left><mouse-left><mouse-release>",
			coords:  [][2]int{{0, 5}, {5, 5}, {5, 5}},
			wantSel: "",
			wantOK:  false,
		},
		{
			desc: "negative coordinates do not panic",
			setup: func(c *Component) {
				c.AddSendMessage("Hello send")
			},
			input:   "<mouse-left><mouse-left><mouse-release>",
			coords:  [][2]int{{-12, -4}, {-1, -1}, {-1, -1}},
			wantSel: "",
			wantOK:  false,
		},
		{
			desc: "cross-element selection across two send messages",
			setup: func(c *Component) {
				c.AddSendMessage("msg1")
				c.AddSendMessage("msg2")
			},
			input:   "<mouse-left><mouse-left><mouse-release>",
			coords:  [][2]int{{0, 0}, {3, 1}, {3, 1}},
			wantSel: "msg1\nmsg2",
			wantOK:  true,
		},
		{
			desc: "cross-element drag upward",
			setup: func(c *Component) {
				c.AddSendMessage("msg1")
				c.AddSendMessage("msg2")
			},
			input:   "<mouse-left><mouse-left><mouse-release>",
			coords:  [][2]int{{3, 1}, {0, 0}, {0, 0}},
			wantSel: "msg1\nmsg2",
			wantOK:  true,
		},
	}

	for _, tc := range suite {
		t.Run(tc.desc, func(t *testing.T) {
			comp := NewComponent(ComponentConfig{})
			tc.setup(comp)
			comp.Resize(width, height)

			grid := drawGrid(comp, width, height)
			d := newMouseDelegate(grid, &comp.messages)
			m := mouse.New(d)

			keys, err := term.ParseKeys(tc.input)
			require.NoError(t, err)
			require.Len(t, keys, len(tc.coords), "coords must match parsed keys")

			for i, kc := range keys {
				m.Handle(term.Event{
					Type:   term.EventMouse,
					Key:    kc.Key,
					Mod:    kc.Mod,
					MouseX: tc.coords[i][0],
					MouseY: tc.coords[i][1],
				})
			}

			text, ok := d.Selection()
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantSel, text)
		})
	}
}

func TestMouseDelegateSelectWordAt(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.AddSendMessage("Hello send")
	comp.Resize(30, 10)

	grid := drawGrid(comp, 30, 10)
	d := newMouseDelegate(grid, &comp.messages)

	// SelectWordAt on X=2 selects the word "Hello".
	d.SelectWordAt(term.Coordinates{X: 2, Y: 0})

	text, ok := d.Selection()
	assert.True(t, ok)
	assert.Equal(t, "Hello", text)
}

func TestMouseDelegateSelectLine(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.AddSendMessage("Hello send")
	comp.Resize(30, 10)

	grid := drawGrid(comp, 30, 10)
	d := newMouseDelegate(grid, &comp.messages)

	// SelectLine on row 0 selects the full line content.
	d.SelectLine(0)

	text, ok := d.Selection()
	assert.True(t, ok)
	assert.Equal(t, "Hello send", text)
}

func TestMouseDelegateScrollN(t *testing.T) {
	const (
		width  = 30
		height = 5
	)

	comp := NewComponent(ComponentConfig{})
	// Add enough messages to make the list scrollable.
	for i := range 20 {
		comp.AddSendMessage("message " + string(rune('A'+i)))
	}
	comp.Resize(width, height)

	grid := drawGrid(comp, width, height)
	d := newMouseDelegate(grid, &comp.messages)

	// Scroll to the top so we can measure downward scrolls.
	for d.ScrollUp(1) {
	}

	// ScrollDown(1) three times should move the same distance as ScrollDown(3) once.
	for range 3 {
		d.ScrollDown(1)
	}

	// Count how many ScrollUp(1) calls to get back to the top.
	countAfterThreeSingles := 0
	for d.ScrollUp(1) {
		countAfterThreeSingles++
	}

	// Now test ScrollDown(3) — a single call should move the same distance.
	ok := d.ScrollDown(3)
	require.True(t, ok)

	countAfterOneBatch := 0
	for d.ScrollUp(1) {
		countAfterOneBatch++
	}
	assert.Equal(t, countAfterThreeSingles, countAfterOneBatch,
		"ScrollDown(3) should scroll the same distance as three ScrollDown(1) calls")
}

func TestMouseDelegateClearAndReselect(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.AddSendMessage("Hello send")
	comp.Resize(30, 10)

	grid := drawGrid(comp, 30, 10)
	d := newMouseDelegate(grid, &comp.messages)

	// Select, then clear, then re-select.
	d.SetSelectionStart(term.Coordinates{X: 0, Y: 0})
	d.SetSelectionEnd(term.Coordinates{X: 4, Y: 0})

	text, ok := d.Selection()
	require.True(t, ok)
	require.Equal(t, "Hello", text)

	d.ClearSelection()
	text, ok = d.Selection()
	assert.False(t, ok)
	assert.Equal(t, "", text)

	// Re-select should work without issues.
	d.SetSelectionStart(term.Coordinates{X: 0, Y: 0})
	d.SetSelectionEnd(term.Coordinates{X: 4, Y: 0})

	text, ok = d.Selection()
	assert.True(t, ok)
	assert.Equal(t, "Hello", text)
}
