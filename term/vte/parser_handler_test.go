package vte

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/term/vte/parser"
	"unstable.build/go-tui/term/vte/screen"
	"unstable.build/go-tui/text/clipboard"
	workspacetest "unstable.build/go-tui/workspace/test"
)

func TestIntegrationParserHandler(t *testing.T) {
	suite := []struct {
		desc      string
		altBuffer bool
		sut       func(*testing.T, *parserHandler, *workspacetest.File)
	}{
		{
			desc:      "primary input after carriage return and line feed",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				p.Input('a')
				p.Goto(0, 1)
				p.ClearLine(parser.LineClearModeRight)
				p.Goto(1, 0)
				p.Input('b')
				assertEqualBuf(t, p, "a    \nb    \n     \n     \n     ")
			},
		},
		{
			desc:      "alt input after clear right of line and goto",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				p.Input('a')
				p.Goto(0, 1)
				p.ClearLine(parser.LineClearModeRight)
				p.Goto(1, 0)
				p.Input('b')
				assertEqualBuf(t, p, "a    \nb    \n     \n     \n     ")
			},
		},
		{
			desc:      "scrolling region change + linefeed scrolls up on margin",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
				p.ClearLine(parser.LineClearModeRight)
				p.Goto(1, 0)
				assertEqualBuf(t, p, "a    \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "vi visual delete multiple lines'",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				p.ScrollDown(0)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "primary scroll up 0 rows does nothing",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				p.ScrollUp(0)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "primary scroll up/down with cap",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
				p.ClearScreen(parser.ClearModeBelow)
				p.Input('$')
				p.Input(' ')
				p.ClearLine(parser.LineClearModeRight)
				assertEqualBuf(t, p, "e    \n$ .  \nout  \n     \n$    ")
			},
		},
		{
			desc:      "shell resize + move up does not oob",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  ")
				assertEqualBuf(t, p, "b    \nc    \nd    \ne    \n$ .  ")
				p.Goto(0, 0)
				p.Resize(4, 4)
				p.MoveUp(1)
				p.ClearScreen(parser.ClearModeAll)
				p.Input('$')
				assertEqualBuf(t, p, "$   \n    \n    \n    ")
			},
		},
		{
			desc:      "shell cltr-l exactly all screen",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  ")
				p.Goto(0, 0)
				p.ClearScreen(parser.ClearModeAll)
				p.ClearScreen(parser.ClearModeBelow)
				p.Input('$')
				p.ClearLine(parser.LineClearModeRight)
				assertEqualBuf(t, p, "$    \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "shell cltr-l with scrollback history",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")
				p.Goto(0, 0)
				p.ClearScreen(parser.ClearModeAll)
				p.ClearScreen(parser.ClearModeBelow)
				p.Input('$')
				p.ClearLine(parser.LineClearModeRight)
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
				assertEqualBuf(t, p, "e    \n$ .  \nout  \n$    \n$    ")
			},
		},
		{
			desc:      "shell input wrap around and scroll down",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(-1, -1)
			},
		},
		{
			desc:      "alternate resize negative does not panic",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(-1, -1)
			},
		},
		{
			desc:      "primary clear mode above",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")
				assert.Equal(t, term.Coordinates{Y: 4, X: 4}, p.sync.buf.CursorAtScreen())
				p.ClearScreen(parser.ClearModeAbove)
				assertEqualBuf(t, p, "     \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "primary clear mode saved",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")

				p.ClearScreen(parser.ClearModeSaved)
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")

				p.ClearScreen(parser.ClearModeSaved)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "secondary clear mode saved, does nothing, because there's no history",
			altBuffer: true,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \ne    ")

				p.ScrollDown(1)
				assertEqualBuf(t, p, "     \na    \nb    \nc    \nd    ")

				p.ScrollUp(1)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n     ")

				p.ClearScreen(parser.ClearModeSaved)
				assertEqualBuf(t, p, "a    \nb    \nc    \nd    \n     ")

				p.ScrollDown(1)
				assertEqualBuf(t, p, "     \na    \nb    \nc    \nd    ")
			},
		},
		{
			desc:      "primary clear mode saved with scroll offset oob",
			altBuffer: false,
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertEqualBuf(t, p, "d    \ne    \n$ .  \nout  \n$    ")

				p.sync.primBuf.SetOffset(term.Coordinates{Y: 999})
				p.ClearScreen(parser.ClearModeSaved)
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
				p.Resize(48, 2)
				resetBuffer(t, p, "sh-3.2$ ls                                      ")

				p.Linefeed()
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
			sut: func(t *testing.T, p *parserHandler, pty *workspacetest.File) {
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
	}

	for _, test := range suite {
		t.Run(test.desc, func(t *testing.T) {
			mockPtyFile := workspacetest.File{}
			pty := workspaceapi.Pty{Master: &mockPtyFile, Slave: &mockPtyFile}
			ph := newParserHandler(new(sync.Mutex), pty, clipboard.NewInMemory())

			if test.altBuffer {
				ph.SetPrivateMode(parser.PrivateModeSwapScreenAndSetRestoreCursor)
			} else {
				ph.UnsetPrivateMode(parser.PrivateModeSwapScreenAndSetRestoreCursor)
			}

			test.sut(t, ph, &mockPtyFile)
		})
	}
}

func assertEqualBuf(t *testing.T, p *parserHandler, expected string) {
	assertEqualScreenBuf(t, p.sync.buf, expected)
}

func assertEqualScreenBuf(t *testing.T, p screenBuffer, expected string) {
	if prim, ok := p.(*screen.PrimaryBuffer); ok {
		width, height := prim.Dimensions()
		writer := term.NewStringWriter(width, height)
		prim.Draw(writer)
		writer.Flush()
		assert.Equal(t, expected, writer.String())
	} else {
		assert.Equal(t, expected, cell.CellsToString(p.(*screen.AltBuffer).Cells.RawCells()))
	}
}

func resetBuffer(t *testing.T, p *parserHandler, to string) {
	writeToBuffer(p, to)
	if prim, ok := p.sync.buf.(*screen.PrimaryBuffer); ok {
		require.Equal(t, to, cell.CellsToString(prim.Cells.RawCells()))
	} else {
		require.Equal(t, to, cell.CellsToString(p.sync.buf.(*screen.AltBuffer).Cells.RawCells()))
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
