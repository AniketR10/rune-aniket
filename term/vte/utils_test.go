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

package vte

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
)

func TestLastPromptLine(t *testing.T) {
	t.Parallel()
	suite := []struct {
		description string
		content     string
		width       int
		// cursorY=0 (zero value) defaults to -1 (legacy
		// bottom-up search). Set setCursorY explicitly to
		// anchor the prompt block at a specific row.
		cursorY        int
		setCursorY     bool
		considerSpaces bool
		expectedStart  term.Coordinates
		expectedEnd    term.Coordinates
	}{
		{
			description:   "empty just returns zero",
			content:       "",
			width:         10,
			expectedStart: term.Coordinates{},
			expectedEnd:   term.Coordinates{},
		},
		{
			description:   "exactly one line shell",
			content:       "$",
			width:         10,
			expectedStart: term.Coordinates{},
			expectedEnd:   term.Coordinates{X: 1},
		},
		{
			description: "one line shell with history",
			content: `
blablab  
$ echo`,
			width:         10,
			expectedStart: term.Coordinates{Y: 2},
			expectedEnd:   term.Coordinates{Y: 2, X: 6},
		},
		{
			description: "empty last line should return everything since last shell",
			content: `
blablabla
$ echo    
       `,
			width:         10,
			expectedStart: term.Coordinates{Y: 2},
			expectedEnd:   term.Coordinates{Y: 3, X: 7},
		},
		{
			description: "multiline simple",
			content: `
blablabla
$ echo bla
aaaaaaaaaa`,
			width:         10,
			expectedStart: term.Coordinates{Y: 2},
			expectedEnd:   term.Coordinates{Y: 3, X: 10},
		},
		{
			description: "multiline with space (previous less than width)",
			content: `
blablabla
$ echo
$ aaaaaaa `,
			width:         10,
			expectedStart: term.Coordinates{Y: 3},
			expectedEnd:   term.Coordinates{Y: 3, X: 10},
		},
		{
			description: "multiline with space (previous filled with blank chars)",
			content: "\n" +
				"blablabla\n" +
				"$ echo\x00\x00\x00\x00\n" +
				"$ aaaaaaa \n",
			width:         10,
			expectedStart: term.Coordinates{Y: 3},
			expectedEnd:   term.Coordinates{Y: 3, X: 10},
		},
		{
			description: "AMBIGUOUS multiline",
			content: `
blablabla
$ echo bla
$ aaaaaaaa`,
			width:         10,
			expectedStart: term.Coordinates{Y: 2},
			expectedEnd:   term.Coordinates{Y: 3, X: 10},
		},
		{
			description: "single line with blank lines under",
			content: "blablabla \n" +
				"$ echo\n" +
				"$ aaaaaaa\x00\x00\n" +
				"\x00\x00\x00",
			width:         10,
			expectedStart: term.Coordinates{Y: 2},
			expectedEnd:   term.Coordinates{Y: 2, X: 9},
		},
		{
			description: "multiline with blank lines under",
			content: "blablabla\n" +
				"$ echo aaa\n" +
				"aaaaaaaaa\x00\x00\n" +
				"\x00\x00\x00",
			width:         10,
			expectedStart: term.Coordinates{Y: 1},
			expectedEnd:   term.Coordinates{Y: 2, X: 9},
		},
		{
			// RUNE-193: zsh menu-select tab completion below the
			// prompt leaves an orphan `$ ` row below the real
			// prompt row after <esc>. Without a prompt anchor,
			// lastPromptLine picked the orphan as the prompt and
			// excluded the real prompt row — vi edits on the real
			// prompt row would then be rejected with a bell.
			// Anchoring at the PTY cursor row (the shell prompt
			// start) makes lastPromptLine return the real prompt
			// row instead.
			description: "RUNE-193 orphan prompt row below the real prompt",
			// row 0: `$ ls` + 16 NULs (real prompt, cursor here)
			// row 1: `$ ` + 18 NULs (orphan from menu cleanup)
			content: "$ ls" + strings.Repeat("\x00", 16) + "\n" +
				"$ " + strings.Repeat("\x00", 18),
			width:         20,
			cursorY:       0,
			setCursorY:    true,
			expectedStart: term.Coordinates{Y: 0, X: 0},
			expectedEnd:   term.Coordinates{Y: 0, X: 4},
		},
		{
			description: "considering spaces, multiline simple",
			content: `
blablabla
$ echo bla
aaaaaaaaaa`,
			width:          10,
			expectedStart:  term.Coordinates{Y: 2},
			expectedEnd:    term.Coordinates{Y: 3, X: 10},
			considerSpaces: true,
		},
		{
			description: "considering spaces multiline with space (previous less than width)",
			content: `
blablabla
$ echo
$ aaaaaaa `,
			width:          10,
			expectedStart:  term.Coordinates{Y: 3},
			expectedEnd:    term.Coordinates{Y: 3, X: 9},
			considerSpaces: true,
		},
		{
			description: "considering spaces, AMBIGUOUS multiline",
			content: `
blablabla
$ echo bla
$ aaaaaaaa`,
			width:          10,
			expectedStart:  term.Coordinates{Y: 2},
			expectedEnd:    term.Coordinates{Y: 3, X: 10},
			considerSpaces: true,
		},
		{
			description: "considering spaces, single line with blank lines under",
			content: "blablabla \n" +
				"$ echo\n" +
				"$ aaaaa  \x00\x00\n" +
				"\x00\x00\x00",
			width:          10,
			expectedStart:  term.Coordinates{Y: 2},
			expectedEnd:    term.Coordinates{Y: 2, X: 7},
			considerSpaces: true,
		},
		{
			description: "considering spaces, multiline with blank lines under",
			content: "blablabla\n" +
				"$ echo aaa\n" +
				"aaaaaaa  \x00\x00\n" +
				"\x00\x00\x00",
			width:          10,
			expectedStart:  term.Coordinates{Y: 1},
			expectedEnd:    term.Coordinates{Y: 2, X: 7},
			considerSpaces: true,
		},
	}

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			t.Parallel()
			buf := cell.NewBuffer()
			// do not use ReadFrom or InsertString as null characters
			// will be elided.
			var next term.Coordinates
			for _, ch := range test.content {
				next = buf.Insert(next, ch)
			}

			// sut
			cursorY := -1
			if test.setCursorY {
				cursorY = test.cursorY
			}
			actualStart, actualEnd := lastPromptLine(buf, test.width, cursorY, test.considerSpaces)
			assert.Equal(t, test.expectedStart, actualStart, "start")
			assert.Equal(t, test.expectedEnd, actualEnd, "end")
		})
	}
}
