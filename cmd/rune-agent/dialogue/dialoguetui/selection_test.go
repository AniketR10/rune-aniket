// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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
	"github.com/unstablebuild/rune-go-sdk/component/comptest"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func newTestSelection(multiSelect bool) *Selection {
	return NewSelection(
		"Pick a database", "Database",
		[]SelectionOption{
			{Label: "PostgreSQL", Description: "Mature RDBMS"},
			{Label: "SQLite", Description: "Lightweight"},
			{Label: "MySQL"},
		},
		multiSelect,
		SelectionConfig{},
	)
}

func TestSelectionHeight(t *testing.T) {
	sel := newTestSelection(false)
	// title (1) + blank (1) + 3 options (label + desc, label + desc, label) = 1+1+2+2+1 = 7
	assert.Equal(t, 7, sel.Height(30))
}

func TestSelectionHeightZeroWidth(t *testing.T) {
	sel := newTestSelection(false)
	assert.Equal(t, 0, sel.Height(0))
}

func TestSelectionRender(t *testing.T) {
	sel := newTestSelection(false)
	w := term.NewStringWriter(30, 7)

	comptest.TestComponent(t, sel, w, []comptest.TestCase{
		{
			Action: func() {
				sel.Resize(30, 7)
			},
			Expected: "Pick a database     [Database]" +
				"\n                              " +
				"\n> PostgreSQL                  " +
				"\n      Mature RDBMS            " +
				"\n  SQLite                      " +
				"\n      Lightweight             " +
				"\n  MySQL                       ",
		},
	})
}

func TestSelectionCursorMoveDown(t *testing.T) {
	sel := newTestSelection(false)
	sel.Resize(30, 7)

	sel.MoveDown()
	assert.Equal(t, []string{"SQLite"}, sel.Selected())

	sel.MoveDown()
	assert.Equal(t, []string{"MySQL"}, sel.Selected())

	// Wrap around
	sel.MoveDown()
	assert.Equal(t, []string{"PostgreSQL"}, sel.Selected())
}

func TestSelectionCursorMoveUp(t *testing.T) {
	sel := newTestSelection(false)
	sel.Resize(30, 7)

	// Wrap from top to bottom
	sel.MoveUp()
	assert.Equal(t, []string{"MySQL"}, sel.Selected())

	sel.MoveUp()
	assert.Equal(t, []string{"SQLite"}, sel.Selected())
}

func TestSelectionSelectedSingleSelect(t *testing.T) {
	sel := newTestSelection(false)
	require.Equal(t, []string{"PostgreSQL"}, sel.Selected())

	sel.MoveDown()
	require.Equal(t, []string{"SQLite"}, sel.Selected())
}

func TestSelectionMultiSelectToggle(t *testing.T) {
	sel := newTestSelection(true)

	// Initially nothing checked
	assert.Empty(t, sel.Selected())

	// Toggle first item
	sel.Toggle()
	assert.Equal(t, []string{"PostgreSQL"}, sel.Selected())

	// Move down and toggle second
	sel.MoveDown()
	sel.Toggle()
	assert.Equal(t, []string{"PostgreSQL", "SQLite"}, sel.Selected())

	// Untoggle first
	sel.MoveUp()
	sel.Toggle()
	assert.Equal(t, []string{"SQLite"}, sel.Selected())
}

func TestSelectionMultiSelectRender(t *testing.T) {
	sel := newTestSelection(true)
	sel.Toggle() // check PostgreSQL
	w := term.NewStringWriter(30, 7)

	comptest.TestComponent(t, sel, w, []comptest.TestCase{
		{
			Action: func() {
				sel.Resize(30, 7)
			},
			Expected: "Pick a database     [Database]" +
				"\n                              " +
				"\n>[x] PostgreSQL               " +
				"\n      Mature RDBMS            " +
				"\n[ ] SQLite                    " +
				"\n      Lightweight             " +
				"\n[ ] MySQL                     ",
		},
	})
}

func TestSelectionDescriptionWraps(t *testing.T) {
	sel := NewSelection(
		"Choose", "",
		[]SelectionOption{
			{Label: "Option A", Description: "This is a really long description text"},
		},
		false,
		SelectionConfig{},
	)
	// width=20, indent=6, usable=14. 37 chars -> ceil(37/14) = 3 lines for desc
	// total: title(1) + blank(1) + label(1) + desc(3) = 6
	assert.Equal(t, 6, sel.Height(20))
}

func TestSelectionToggleNoOpForSingleSelect(t *testing.T) {
	sel := newTestSelection(false)
	sel.Toggle() // should be a no-op
	assert.Equal(t, []string{"PostgreSQL"}, sel.Selected())
}

func TestSelectionTitleWrapsHeight(t *testing.T) {
	sel := NewSelection(
		"What is your favorite color?", "", // 28 chars, no chip
		[]SelectionOption{
			{Label: "Red"},
			{Label: "Blue"},
		},
		false,
		SelectionConfig{},
	)
	// width=20: 28 chars wraps to 2 lines
	// title(2) + blank(1) + 2 options = 5
	assert.Equal(t, 5, sel.Height(20))
}

func TestSelectionTitleWrapsRender(t *testing.T) {
	sel := NewSelection(
		"What is your favorite color?", "Colors",
		[]SelectionOption{
			{Label: "Red"},
			{Label: "Blue"},
		},
		false,
		SelectionConfig{},
	)
	// width=20: title wraps to 2 lines, chip "[Colors]" can't fit on first line
	w := term.NewStringWriter(20, 5)

	comptest.TestComponent(t, sel, w, []comptest.TestCase{
		{
			Action: func() {
				sel.Resize(20, 5)
			},
			Expected: "What is your favorit" +
				"\ne color?            " +
				"\n                    " +
				"\n> Red               " +
				"\n  Blue              ",
		},
	})
}
