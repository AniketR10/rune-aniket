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

package text

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
)

// TestMouseDelegateSelectWordAtInvertedScrollAboveBuffer reproduces
// crash report 787830382: "runtime error: index out of range [-3]".
// vte terminals run with Scroll.InvertOffset=true, which makes
// WindowToScrollCoordinates legitimately return a negative Y for
// window positions that sit above the bottom-anchored buffer content
// (tall window, sparse buffer). The mouse delegate must treat those
// out-of-buffer positions as "nothing to select" rather than handing
// the negative coordinate to Scroll.WordAt, which would panic indexing
// rawCells.cells[-3].
func TestMouseDelegateSelectWordAtInvertedScrollAboveBuffer(t *testing.T) {
	scroll := component.NewScroll(cell.NewBuffer())
	scroll.InvertOffset = true
	_, _ = scroll.Buffer().ReadFrom(strings.NewReader("hello\n"))
	// Window is much taller than the populated buffer rows, so
	// convertedOffset().Y is negative and clicks near the top of the
	// window translate to negative scroll coordinates.
	scroll.Resize(80, 24)

	cur := NewCursor(scroll, nil)
	delegate := CursorMouseDelegate(cur)

	assert.NotPanics(t, func() {
		delegate.SelectWordAt(term.Coordinates{X: 0, Y: 0})
	})
	_, hasSelection := cur.SelectionMode()
	assert.False(t, hasSelection,
		"clicks above the buffer must not start a selection")
}

// TestScrollWordAtOutOfBounds locks in the documented "empty string if
// token at the given position is not a word" contract for inputs that
// fall outside the buffer. This is a defense-in-depth guard: every
// other call site that routes window coordinates through
// WindowToScrollCoordinates and into WordAt is protected here.
func TestScrollWordAtOutOfBounds(t *testing.T) {
	scroll := component.NewScroll(cell.NewBuffer())
	_, _ = scroll.Buffer().ReadFrom(strings.NewReader("hello world\n"))
	scroll.Resize(80, 24)

	cases := []struct {
		name string
		at   term.Coordinates
	}{
		{"negative Y", term.Coordinates{X: 0, Y: -3}},
		{"negative X", term.Coordinates{X: -1, Y: 0}},
		{"negative both", term.Coordinates{X: -2, Y: -5}},
		{"Y past last row", term.Coordinates{X: 0, Y: 1 << 20}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				_, _, got := scroll.WordAt(tc.at)
				assert.Equal(t, "", got)
			})
		})
	}
}
