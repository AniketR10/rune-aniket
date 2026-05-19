// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package text

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
)

// TestWordAtCursor covers the identifier-boundary scanner used to
// derive $WORD. The exit conditions (out-of-bounds, non-word cell)
// must return the empty string rather than panic, since dispatched
// commands invoke wordAtCursor opportunistically and a panic here
// would tear down DispatchCommand.
func TestWordAtCursor(t *testing.T) {
	buf := cell.NewBuffer()
	buf.WriteString("foo bar.baz\nhello_world  \n")

	cases := []struct {
		name string
		pos  term.Coordinates
		want string
	}{
		{"middle of identifier", term.Coordinates{Y: 0, X: 1}, "foo"},
		{"start of identifier", term.Coordinates{Y: 0, X: 4}, "bar"},
		{"after dot", term.Coordinates{Y: 0, X: 8}, "baz"},
		{"on separator", term.Coordinates{Y: 0, X: 3}, ""},
		{"snake_case treated as one word",
			term.Coordinates{Y: 1, X: 3}, "hello_world"},
		{"row out of range", term.Coordinates{Y: 99}, ""},
		{"column out of range", term.Coordinates{Y: 0, X: 99}, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, wordAtCursor(buf, tc.pos))
		})
	}
}

// TestWordAtCursorNilView guards the cell.View nil path. Tabs whose
// browser handler is not a text.Handler (terminals, plugin floats)
// must not feed a nil cell.View into the scanner.
func TestWordAtCursorNilView(t *testing.T) {
	assert.Equal(t, "", wordAtCursor(nil, term.Coordinates{}))
}
