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
	"bytes"
	"encoding/base64"
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/tcell/v3"
	browserapi "unstable.build/go-tui/api/browser"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/clipboard"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/vte/vteparser"
	"unstable.build/go-tui/term/vte/vtescreen"
	"unstable.build/go-tui/workspace/workspacetest"
)

func TestIntegrationParserHandler(t *testing.T) {
	testURI, err := workspaceapi.ParseURI("memory:///radical")
	require.NoError(t, err)

	suite := []struct {
		desc      string
		altBuffer bool
		sut       func(*testing.T, *parserHandler, *mockTabManager, *workspacetest.File)
	}{
		{
			desc:      "primary input after carriage return and line feed",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				p.Input('a')
				p.CarriageReturn()
				p.Linefeed()
				p.Input('b')
				assertEqualBuf(t, p, "a    \nb    \n     \n     \n     ")
			},
		},
		{
			desc:      "alt input after carriage return and line feed",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				p.Input('a')
				p.CarriageReturn()
				p.Linefeed()
				p.Input('b')
				assertEqualBuf(t, p, "a    \nb    \n     \n     \n     ")
			},
		},
		{
			desc:      "primary input after clear right of line and goto",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				p.Input('a')
				p.Goto(0, 1)
				p.ClearLine(vteparser.LineClearModeRight)
				p.Goto(1, 0)
				p.Input('b')
				assertEqualBuf(t, p, "a    \nb    \n     \n     \n     ")
			},
		},
		{
			desc:      "alt input after clear right of line and goto",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				p.Input('a')
				p.Goto(0, 1)
				p.ClearLine(vteparser.LineClearModeRight)
				p.Goto(1, 0)
				p.Input('b')
				assertEqualBuf(t, p, "a    \nb    \n     \n     \n     ")
			},
		},
		{
			desc:      "scrolling region change + linefeed scrolls up on margin",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \n     \n     \n     ")

				p.SetScrollingRegion(2, 4, false)
				p.Goto(3, 0)
				p.CarriageReturn()
				p.Linefeed()
				p.SetScrollingRegion(1, 5, false)
				assertEqualBuf(t, p, "a    \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "vi delete a line 'dd'",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \n     \n     \n     ")

				p.SetScrollingRegion(2, 4, false)
				p.Goto(3, 0)
				p.CarriageReturn()
				p.Linefeed()
				p.SetScrollingRegion(1, 5, false)
				p.Goto(3, 0)
				p.Input(' ')
				p.Input(' ')
				p.Input(' ')
				p.Input(' ')
				p.Input(' ')
				p.Goto(4, 0)
				p.ClearLine(vteparser.LineClearModeRight)
				p.Goto(1, 0)
				assertEqualBuf(t, p, "a    \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "vi visual delete multiple lines'",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 6)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n     ")

				p.UnsetPrivateMode(25)
				// do not scroll command bar
				p.SetScrollingRegion(1, 5, false)
				p.Goto(0, 0)
				p.DeleteLines(2)
				p.SetScrollingRegion(1, 6, false)
				p.Goto(3, 0)
				p.Input('X')
				p.CarriageReturn()
				p.Linefeed()
				p.Input('Y')
				p.Goto(0, 0)
				p.SetPrivateMode(25)

				assertEqualBuf(t, p, "c    \nd    \ne    \nX    \nY    \n     ")
			},
		},
		{
			desc:      "primary scroll down non-capped",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \nf    ")
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \nf    ")
				p.ScrollDown(100)
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "primary scroll up non-capped",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \nf    ")
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \nf    ")
				p.ScrollDown(100)
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")
				p.ScrollUp(2)
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")
				p.ScrollUp(100)
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")
				p.ScrollDown(1)
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "primary scroll down 0 rows does nothing",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				p.ScrollDown(0)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
				assert.False(t, p.scrollDown(0, true))
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "alternate scroll down 0 rows does nothing",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				p.ScrollDown(0)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "primary scroll up 0 rows does nothing",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				p.ScrollUp(0)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
				assert.False(t, p.scrollUp(0, true))
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "alternate scroll up 0 rows does nothing",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				p.ScrollUp(0)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "primary scroll up/down with cap",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \nf    \ng    ")
				assertEqualBuf(t, p, "c    \nd    \ne    \nf    \ng    ")

				assert.False(t, p.scrollUp(1, true))
				assertEqualBuf(t, p, "c    \nd    \ne    \nf    \ng    ")

				assert.True(t, p.scrollDown(1, true))
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \nf    ")

				assert.True(t, p.scrollDown(1, true))
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")

				assert.False(t, p.scrollDown(1, true))
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")

				assert.True(t, p.scrollUp(1, true))
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \nf    ")

				assert.True(t, p.scrollUp(1, true))
				assertEqualBuf(t, p, "c    \nd    \ne    \nf    \ng    ")

				assert.False(t, p.scrollUp(1, true))
				assertEqualBuf(t, p, "c    \nd    \ne    \nf    \ng    ")
			},
		},
		{
			desc:      "shell scroll back, then write next command",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  ")
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \n$ .  ")
				p.CarriageReturn()
				p.CarriageReturn()
				p.Linefeed()
				p.Input('o')
				p.Input('u')
				p.Input('t')
				p.CarriageReturn()
				p.Linefeed()
				p.CarriageReturn()
				p.Linefeed()
				p.ClearScreen(vteparser.ClearModeBelow)
				p.Input('$')
				p.Input(' ')
				p.ClearLine(vteparser.LineClearModeRight)
				assertEqualBuf(t, p, "e    \n$ .  \nout  \n     \n$    ")
			},
		},
		{
			desc:      "shell resize + move up does not oob",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  ")
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \n$ .  ")
				p.Resize(4, 4)
				p.ClearScreen(vteparser.ClearModeAll)
				p.Goto(0, 0)
				p.Input('$')
				assertEqualBuf(t, p, "$   \n    \n    \n    ")
			},
		},
		{
			desc:      "shell cltr-l exactly all screen",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  ")
				p.Goto(0, 0)
				p.ClearScreen(vteparser.ClearModeAll)
				p.ClearScreen(vteparser.ClearModeBelow)
				p.Input('$')
				p.ClearLine(vteparser.LineClearModeRight)
				assertEqualBuf(t, p, "$    \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "shell cltr-l with scrollback history",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")
				p.Goto(0, 0)
				p.ClearScreen(vteparser.ClearModeAll)
				p.ClearScreen(vteparser.ClearModeBelow)
				p.Input('$')
				p.ClearLine(vteparser.LineClearModeRight)
				assertEqualBuf(t, p, "$    \n     \n     \n     \n     ")

				// simulate user scrolling
				assert.True(t, p.scrollDown(1, true))
				assertEqualBuf(t, p, "$    \n$    \n     \n     \n     ")

				assert.True(t, p.scrollDown(1, true))
				assertEqualBuf(t, p, "out  \n$    \n$    \n     \n     ")

				assert.True(t, p.scrollDown(2, true))
				assertEqualBuf(t, p, "e    \n$ .  \nout  \n$    \n$    ")

				assert.True(t, p.scrollDown(100, true))
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")

				assert.True(t, p.scrollUp(100, true))
				assertEqualBuf(t, p, "$    \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "shell input wrap around and scroll down",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")
				for i := 0; i < 7; i++ {
					p.Input('a')
				}
				assertEqualBuf(t, p, "$ .  \nout  \n$    \naaaaa\naa   ")
			},
		},
		{
			desc:      "primary resize maintains cursor position at content",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assert.Equal(t, term.Coordinates{Y: 7, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())

				p.Resize(1, 1)
				assert.Equal(t, term.Coordinates{Y: 7, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: 0, X: 4}, p.sync.buf.CursorAtScreen())

				p.Resize(5, 5)
				assert.Equal(t, term.Coordinates{Y: 7, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())

				p.Resize(0, 0)
				assert.Equal(t, term.Coordinates{Y: 7, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: -1, X: 4}, p.sync.buf.CursorAtScreen())

				p.Resize(5, 5)
				assert.Equal(t, term.Coordinates{Y: 7, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())
			},
		},
		{
			desc:      "alternate resize maintains cursor position at content",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())

				p.Resize(1, 1)
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())

				p.Resize(5, 5)
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())

				p.Resize(0, 0)
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())

				p.Resize(5, 5)
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScroll())
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())
			},
		},
		{
			desc:      "primary resize negative does not panic",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(-1, -1)
			},
		},
		{
			desc:      "alternate resize negative does not panic",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(-1, -1)
			},
		},
		{
			desc:      "primary clear mode above",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())
				p.ClearScreen(vteparser.ClearModeAbove)
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "primary clear mode saved",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")

				p.ClearScreen(vteparser.ClearModeSaved)
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")

				assert.True(t, p.scrollDown(1, true))
				assertEqualBuf(t, p, "     \nd    \ne    \n$ .  \nout  ")

				assert.True(t, p.scrollUp(1, true))
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")
			},
		},
		{
			desc:      "primary clear mode saved with no history does nothing",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")

				p.ClearScreen(vteparser.ClearModeSaved)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "secondary clear mode saved, does nothing, because there's no history",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")

				p.ScrollDown(1)
				assertEqualBuf(t, p, "     \na    \nb    \nc    \nd    ")

				p.ScrollUp(1)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n     ")

				p.ClearScreen(vteparser.ClearModeSaved)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n     ")

				p.ScrollDown(1)
				assertEqualBuf(t, p, "     \na    \nb    \nc    \nd    ")
			},
		},
		{
			desc:      "primary clear mode saved with scroll offset oob",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")

				p.sync.primBuf.SetOffset(term.Coordinates{Y: 999})
				p.ClearScreen(vteparser.ClearModeSaved)
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")

				assert.True(t, p.scrollDown(1, true))
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")

				assert.True(t, p.scrollUp(2, true))
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "unknown clipboard does nothing",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)

				data := base64Data(t, "1234\n5678@hello")

				p.ClipboardStore(1, data)
				p.ClipboardLoad(1, "TERM")
				require.Len(t, pty.Writes, 0)
			},
		},
		{
			desc:      "clipboard load/store",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)

				data := base64Data(t, "1234\n5678@hello")

				p.ClipboardStore(int('c'), data)

				p.ClipboardLoad(int('p'), "TERM")
				require.Len(t, pty.Writes, 0)

				p.ClipboardLoad(int('c'), "TERM")
				assertWriteToPty(t, pty, fmt.Sprintf("\x1b]52;c;%sTERM", data))
			},
		},
		{
			desc:      "sh ls usage of put tab",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(48, 2)
				resetBuffer(t, p, "sh-3.2$ ls                                      \n                                                ")

				p.CarriageReturn()

				for _, ch := range "LICENSE" {
					p.Input(ch)
				}
				p.PutTab()
				p.PutTab()
				for _, ch := range "cpu.out" {
					p.Input(ch)
				}
				p.PutTab()
				p.PutTab()
				for _, ch := range "plugin" {
					p.Input(ch)
				}
				assertEqualBuf(t, p, "sh-3.2$ ls                                      \n"+
					"LICENSE         cpu.out         plugin          ")

			},
		},
		{
			desc:      "Input + Linefeed + CarriageReturn hit max scrollback history",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				p.maxScrollLength = 6
				p.Input('a')
				p.Linefeed()
				p.CarriageReturn()
				p.Input('b')
				p.Linefeed()
				p.CarriageReturn()
				p.Input('c')
				p.Linefeed()
				p.CarriageReturn()
				p.Input('d')
				p.Linefeed()
				p.CarriageReturn()
				p.Input('e')
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")

				p.Linefeed()
				p.CarriageReturn()
				p.Input('f')
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \nf    ")

				p.CarriageReturn()
				p.Linefeed()
				p.Input('g')
				assertEqualBuf(t, p, "c    \nd    \ne    \nf    \ng    ")

				// simulate user scrolling
				assert.True(t, p.scrollDown(1, true))
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \nf    ")

				assert.False(t, p.scrollDown(100, true))
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \nf    ")

				assert.True(t, p.scrollUp(100, true))
				assertEqualBuf(t, p, "c    \nd    \ne    \nf    \ng    ")

				assert.False(t, p.scrollUp(100, true))
				assertEqualBuf(t, p, "c    \nd    \ne    \nf    \ng    ")

				assert.Equal(t, p.maxScrollLength, p.sync.buf.Rows())
			},
		},
		{
			desc:      "reverse index usage of git log on primary buffer",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \n:    ")
				p.CarriageReturn()
				p.ClearLine(0)
				p.Goto(0, 0)
				p.ReverseIndex()
				p.Input('X')
				p.CarriageReturn()
				p.Linefeed()
				p.Goto(4, 0)
				p.CarriageReturn()
				p.ClearLine(0)
				p.Input(':')
				p.ClearLine(0)
				assertEqualBuf(t, p, "X    \na    \nb    \nc    \n:    ")
			},
		},
		{
			desc:      "reverse index usage of git log on primary buffer with history",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "com  \nlog  \na    \nb    \nc    \nd    \n:    ")
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n:    ")
				p.CarriageReturn()
				p.ClearLine(0)
				p.Goto(0, 0)
				p.ReverseIndex()
				p.Input('X')
				p.CarriageReturn()
				p.Linefeed()
				p.Goto(4, 0)
				p.CarriageReturn()
				p.ClearLine(0)
				p.Input(':')
				p.ClearLine(0)
				assertEqualBuf(t, p, "X    \na    \nb    \nc    \n:    ")
			},
		},
		{
			desc:      "multiple reverse index after sefveral 'scroll down' on primary buffer with history",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "com  \nlog  \na    \nb    \nc    \nd    \n:    ")
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n:    ")

				// scroll down with git log
				p.Goto(4, 0)
				for i := 0; i < 3; i++ {
					p.CarriageReturn()
					p.ClearLine(0)
					p.Input([]rune(strconv.Itoa(i))[0])
					p.CarriageReturn()
					p.Linefeed()
					p.Input(':')
					p.ClearLine(0)
				}
				assertEqualBuf(t, p, "d    \n0    \n1    \n2    \n:    ")

				for i := 0; i < 3; i++ {
					p.CarriageReturn()
					p.ClearLine(0)
					p.Goto(0, 0)
					p.ReverseIndex()
					switch i {
					case 0:
						p.Input('c')
					case 1:
						p.Input('b')
					case 2:
						p.Input('a')
					}
					p.CarriageReturn()
					p.Linefeed()
					p.Goto(4, 0)
					p.CarriageReturn()
					p.ClearLine(0)
					p.Input(':')
					p.ClearLine(0)
				}
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n:    ")
			},
		},
		{
			desc:      "zsh delete a character in vi mode",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "log  \na    \nb    \nc    \nd    \n$ xaa")
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n$ xaa")
				p.Backspace()
				p.Backspace()
				p.DeleteChars(1)
				p.MoveForward(2)
				p.Input(' ')
				p.MoveBackward(2)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n$ aa ")
				p.Input('X')
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n$ Xa ")
			},
		},
		{
			desc:      "zsh delete a character in vi mode with wrap around line",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "log  \na    \nb    \nc    \nd    \n$ xaa")
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n$ xaa")
				p.Backspace()
				p.Backspace()
				p.DeleteChars(1)
				p.MoveForward(2)
				p.ClearLine(0)
				p.MoveDown(1)
				p.CarriageReturn()
				p.ClearLine(0)
				p.MoveUp(1)
				p.MoveForward(2)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n$ aa ")
				p.Input('X')
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n$ Xa ")
			},
		},
		{
			desc:      "bell is called if idle (first created)",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				assert.False(t, tm.belled)
				p.Bell()
				assert.True(t, tm.belled)
				assert.Zero(t, tm.setName)
				assert.Zero(t, tm.toUri)
				assert.Zero(t, tm.setAttr)
			},
		},
		{
			desc:      "bell is called if in focus",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				assert.False(t, tm.belled)
				p.onFocusChange(false)
				p.onFocusChange(true)
				p.Bell()
				assert.True(t, tm.belled)
				assert.Zero(t, tm.setName)
				assert.Zero(t, tm.toUri)
				assert.Zero(t, tm.setAttr)
			},
		},
		{
			desc:      "bell is not called if not in focus, but needs attention attrs are set if mode urgency hints is set",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				assert.False(t, tm.belled)
				p.onFocusChange(false)
				p.Bell()
				assert.False(t, tm.belled)
				assert.Equal(t, testURI, tm.toUri)
				assert.Equal(t, "radical", tm.setName)
				assert.Equal(t, term.Attributes{Attrs: tcell.AttrBlink}, tm.setAttr)
			},
		},
		{
			desc:      "bell is not called if not in focus, and needs attention attrs are not set if mode urgency hints is disabled",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				assert.False(t, tm.belled)
				p.onFocusChange(false)
				p.UnsetPrivateMode(vteparser.PrivateModeUrgencyHints)
				p.Bell()
				assert.False(t, tm.belled)
				assert.Zero(t, tm.setName)
				assert.Zero(t, tm.toUri)
				assert.Zero(t, tm.setAttr)
			},
		},
		{
			desc:      "bell is called whether mode urgency hints is disabled or not",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				assert.False(t, tm.belled)
				p.onFocusChange(true)
				p.UnsetPrivateMode(vteparser.PrivateModeUrgencyHints)
				p.Bell()
				assert.True(t, tm.belled)
				assert.Zero(t, tm.setName)
				assert.Zero(t, tm.toUri)
				assert.Zero(t, tm.setAttr)
			},
		},
		{
			desc:      "needs attention attrs are cleared if on focus true is triggered",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				assert.False(t, tm.belled)
				p.onFocusChange(false)
				p.Bell()
				p.onFocusChange(true)
				assert.Equal(t, testURI, tm.toUri)
				assert.Equal(t, "radical", tm.setName)
				assert.Equal(t, term.Attributes{}, tm.setAttr)
			},
		},
		{
			desc:      "git show scroll up and down should not leave lines below",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, tm *mockTabManager, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")
				p.Goto(0, 0)
				p.ReverseIndex()
				p.Input('1')
				p.Goto(0, 0)
				p.ReverseIndex()
				p.Input('2')

				assertEqualBuf(t, p, "2    \n1    \nd    \ne    \n$ .  ")

				p.scrollUp(2, true)
				assertEqualBuf(t, p, "2    \n1    \nd    \ne    \n$ .  ")

				p.scrollUp(2, false)
				assertEqualBuf(t, p, "d    \ne    \n$ .  \n     \n     ")

				p.scrollDown(4, true)
				assertEqualBuf(t, p, "b    \nc    \n2    \n1    \nd    ")
			},
		},
	}

	for _, test := range suite {
		t.Run(test.desc, func(t *testing.T) {
			mockPtyFile := workspacetest.File{}
			tm := mockTabManager{}
			attrs := DefaultConfig().NeedsAttentionAttributes
			pty := workspaceapi.Pty{Master: &mockPtyFile, Slave: &mockPtyFile}
			ph := newParserHandler(new(sync.Mutex), pty, &tm, clipboard.NewInMemory(), tm.bell, testURI, attrs)
			ph.sync.primBuf.SetDefaultChar(' ')
			ph.sync.altBuf.SetDefaultChar(' ')

			if test.altBuffer {
				ph.SetPrivateMode(vteparser.PrivateModeSwapScreenAndSetRestoreCursor)
			} else {
				ph.UnsetPrivateMode(vteparser.PrivateModeSwapScreenAndSetRestoreCursor)
			}

			test.sut(t, ph, &tm, &mockPtyFile)
		})
	}
}

func assertEqualBuf(t *testing.T, p *parserHandler, expected string) {
	assertEqualScreenBuf(t, p.sync.buf, expected)
}

func assertEqualScreenBuf(t *testing.T, p screenBuffer, expected string) {
	if prim, ok := p.(*vtescreen.PrimaryBuffer); ok {
		width, height := prim.Dimensions()
		writer := term.NewStringWriter(width, height)
		prim.Draw(writer)
		writer.Flush()
		assert.Equal(t, expected, writer.String())
	} else {
		assert.Equal(t, expected, cell.CellsToString(p.(*vtescreen.AltBuffer).Cells.RawCells()))
	}
}

func resetBuffer(t *testing.T, p *parserHandler, to string) {
	writeToBuffer(p, to)
	if prim, ok := p.sync.buf.(*vtescreen.PrimaryBuffer); ok {
		require.Equal(t, to, cell.CellsToString(prim.Cells.RawCells()))
	} else {
		require.Equal(t, to, cell.CellsToString(p.sync.buf.(*vtescreen.AltBuffer).Cells.RawCells()))
	}
}

func writeToBuffer(p *parserHandler, str string) {
	for _, ch := range str {
		if ch == '\n' {
			p.CarriageReturn()
			p.Linefeed()
		} else {
			p.Input(ch)
		}
	}
}

func assertWriteToPty(t *testing.T, pty *workspacetest.File, data string) {
	require.Len(t, pty.Writes, 1)
	assert.Equal(t, string(pty.Writes[0]), data)
}

func base64Data(t *testing.T, data string) []byte {
	var buf bytes.Buffer
	e := base64.NewEncoder(base64.StdEncoding, &buf)
	_, err := e.Write([]byte(data))
	require.NoError(t, err)
	require.NoError(t, e.Close())
	return buf.Bytes()
}

type mockTabManager struct {
	belled  bool
	toUri   workspaceapi.URI
	setName string
	setAttr term.Attributes
}

func (tm *mockTabManager) Tab(
	uri workspaceapi.URI, icon rune, name string, h browserapi.Handler,
) (
	browserapi.Handler, error,
) {
	panic("not in use")
}

func (tm *mockTabManager) SetTabName(uri workspaceapi.URI, name string, attr term.Attributes) error {
	tm.toUri = uri
	tm.setName = name
	tm.setAttr = attr
	return nil
}

func (tm *mockTabManager) bell() {
	tm.belled = true
}
