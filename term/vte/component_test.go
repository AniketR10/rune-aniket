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
	"context"
	"sync"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/vte/vteparser"
	"unstable.build/go-tui/workspace/workspacetest"
)

func TestIntegrationComponent(t *testing.T) {
	t.Parallel()

	suite := []struct {
		desc      string
		altBuffer bool
		sut       func(*testing.T, *Component, *mockTabManager)
	}{
		{
			desc:      "primary scroll down 0 rows does nothing",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				p.ScrollUp(0)
				assertDraw(t, comp, "a    \nb    \nc    \nd    \ne    ")
				assert.False(t, comp.ScrollUp(0))
				assertDraw(t, comp, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "primary scroll ScrollOffset/MaxOffset rows <= height",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				assert.Equal(t, 0, comp.ScrollOffset())
				assert.Equal(t, 0, comp.MaxScrollOffset())
			},
		},
		{
			desc:      "primary scroll ScrollOffset/MaxOffset rows > height",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \nf    \ng    ")
				assert.Equal(t, 2, comp.ScrollOffset())
				assert.Equal(t, 2, comp.MaxScrollOffset())
			},
		},
		{
			desc:      "primary scroll up 0 rows does nothing",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    ")
				p.ScrollDown(0)
				assertDraw(t, comp, "a    \nb    \nc    \nd    \ne    ")
				assert.False(t, comp.ScrollDown(0))
				assertDraw(t, comp, "a    \nb    \nc    \nd    \ne    ")
			},
		},
		{
			desc:      "primary input mixed with user scrolls",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				for range 6 {
					p.Input('a')
					p.CarriageReturn()
					p.Linefeed()
				}
				require.True(t, comp.ScrollUp(1))
				p.Input('b')
				require.True(t, comp.ScrollUp(1))
				p.CarriageReturn()
				p.Linefeed()
				comp.ScrollUp(1)
				comp.ScrollDown(3)
				assertDraw(t, comp, "a    \na    \na    \nb    \n     ")
			},
		},
		{
			desc:      "primary scroll ScrollOffset/MaxOffset after scroll",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \nf    \ng    ")
				require.False(t, comp.ScrollDown(1))
				assert.Equal(t, 2, comp.ScrollOffset())
				assert.Equal(t, 2, comp.MaxScrollOffset())
				require.True(t, comp.ScrollUp(1))
				assert.Equal(t, 1, comp.ScrollOffset())
				assert.Equal(t, 2, comp.MaxScrollOffset())
				require.True(t, comp.ScrollUp(1))
				assert.Equal(t, 0, comp.ScrollOffset())
				assert.Equal(t, 2, comp.MaxScrollOffset())
				require.False(t, comp.ScrollUp(1))
				assert.Equal(t, 0, comp.ScrollOffset())
				assert.Equal(t, 2, comp.MaxScrollOffset())
				require.True(t, comp.ScrollDown(1))
				assert.Equal(t, 1, comp.ScrollOffset())
				assert.Equal(t, 2, comp.MaxScrollOffset())
				require.True(t, comp.ScrollDown(1))
				assert.Equal(t, 2, comp.ScrollOffset())
				assert.Equal(t, 2, comp.MaxScrollOffset())
				require.False(t, comp.ScrollDown(1))
				assert.Equal(t, 2, comp.ScrollOffset())
				assert.Equal(t, 2, comp.MaxScrollOffset())
			},
		},
		{
			desc:      "primary scroll ScrollOffset/MaxOffset after ScrollBottom",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \nf    \ng    ")
				assertDraw(t, comp, "c    \nd    \ne    \nf    \ng    ")
				require.True(t, comp.ScrollUp(100))
				require.True(t, comp.ScrollBottom())
				assertDraw(t, comp, "c    \nd    \ne    \nf    \ng    ")
				assert.Equal(t, 2, comp.ScrollOffset())
				assert.Equal(t, 2, comp.MaxScrollOffset())
			},
		},
		{
			desc:      "selection after scroll",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \nf    \ng    ")
				require.True(t, comp.ScrollUp(1))
				assertDraw(t, comp, "b    \nc    \nd    \ne    \nf    ")

				comp.SelectWordAt(term.Coordinates{})
				data, ok := comp.Selection()
				assert.True(t, ok)
				assert.Equal(t, "b", data)

				comp.Unselect()

				comp.Select(term.Coordinates{})
				comp.SelectEnd(term.Coordinates{})
				data, ok = comp.Selection()
				assert.True(t, ok)
				assert.Equal(t, "b", data)

				comp.Unselect()

				comp.SelectLine(term.Coordinates{})
				data, ok = comp.Selection()
				assert.True(t, ok)
				assert.Equal(t, "b    \n", data)
			},
		},
		{
			desc:      "selection strips null characters",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				p.Input('a')
				p.Input('b')
				p.Input('c')
				p.Input('\x00')

				comp.Select(term.Coordinates{})
				comp.SelectEnd(term.Coordinates{X: 4})
				data, ok := comp.Selection()
				assert.True(t, ok)
				assert.Equal(t, "abc", data)
			},
		},
		{
			desc:      "primary scroll up/down with cap",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				p := comp.parserHandler
				comp.Resize(5, 5)
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \nf    \ng    ")
				assertDraw(t, comp, "c    \nd    \ne    \nf    \ng    ")

				assert.False(t, comp.ScrollDown(1))
				assertDraw(t, comp, "c    \nd    \ne    \nf    \ng    ")

				assert.True(t, comp.ScrollUp(1))
				assertDraw(t, comp, "b    \nc    \nd    \ne    \nf    ")

				assert.True(t, comp.ScrollUp(1))
				assertDraw(t, comp, "a    \nb    \nc    \nd    \ne    ")

				assert.False(t, comp.ScrollUp(1))
				assertDraw(t, comp, "a    \nb    \nc    \nd    \ne    ")

				assert.True(t, comp.ScrollDown(1))
				assertDraw(t, comp, "b    \nc    \nd    \ne    \nf    ")

				assert.True(t, comp.ScrollDown(1))
				assertDraw(t, comp, "c    \nd    \ne    \nf    \ng    ")

				assert.False(t, comp.ScrollDown(1))
				assertDraw(t, comp, "c    \nd    \ne    \nf    \ng    ")
			},
		},
		{
			desc:      "primary clear mode saved",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				comp.Resize(5, 5)
				p := comp.parserHandler
				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertDraw(t, comp, "d    \ne    \n$ .  \nout  \n$    ")

				p.ClearScreen(vteparser.ClearModeSaved)
				assertDraw(t, comp, "d    \ne    \n$ .  \nout  \n$    ")

				assert.False(t, comp.ScrollUp(1))
				assertDraw(t, comp, "d    \ne    \n$ .  \nout  \n$    ")

				assert.False(t, comp.ScrollDown(1))
				assertDraw(t, comp, "d    \ne    \n$ .  \nout  \n$    ")
			},
		},
		{
			desc:      "shell cltr-l with scrollback history",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				comp.Resize(5, 5)
				p := comp.parserHandler

				resetBuffer(t, p, "a    \nb    \nc    \nd    \ne    \n$ .  \nout  \n$    ")
				assertDraw(t, comp, "d    \ne    \n$ .  \nout  \n$    ")
				assert.Equal(t, 3, comp.ScrollOffset())
				assert.Equal(t, 3, comp.MaxScrollOffset())

				p.Goto(0, 0)
				p.ClearScreen(vteparser.ClearModeAll)
				p.ClearScreen(vteparser.ClearModeBelow)
				p.Input('$')
				p.ClearLine(vteparser.LineClearModeRight)
				assertDraw(t, comp, "$    \n     \n     \n     \n     ")
				assert.Equal(t, 8, comp.ScrollOffset())
				assert.Equal(t, 8, comp.MaxScrollOffset())

				// simulate user scrolling
				assert.True(t, comp.ScrollUp(1))
				assertDraw(t, comp, "$    \n$    \n     \n     \n     ")
				assert.Equal(t, 7, comp.ScrollOffset())
				assert.Equal(t, 8, comp.MaxScrollOffset())
				comp.Select(term.Coordinates{})
				comp.SelectEnd(term.Coordinates{})
				data, ok := comp.Selection()
				assert.True(t, ok)
				assert.Equal(t, "$", data)

				assert.True(t, comp.ScrollUp(1))
				assertDraw(t, comp, "out  \n$    \n$    \n     \n     ")
				assert.Equal(t, 6, comp.ScrollOffset())
				assert.Equal(t, 8, comp.MaxScrollOffset())

				assert.True(t, comp.ScrollUp(2))
				assertDraw(t, comp, "e    \n$ .  \nout  \n$    \n$    ")
				assert.Equal(t, 4, comp.ScrollOffset())
				assert.Equal(t, 8, comp.MaxScrollOffset())

				assert.True(t, comp.ScrollUp(100))
				assertDraw(t, comp, "a    \nb    \nc    \nd    \ne    ")
				assert.Equal(t, 0, comp.ScrollOffset())
				assert.Equal(t, 8, comp.MaxScrollOffset())

				assert.True(t, comp.ScrollDown(100))
				assertDraw(t, comp, "e    \n$ .  \nout  \n$    \n$    ")
				assert.Equal(t, 4, comp.ScrollOffset())
				assert.Equal(t, 8, comp.MaxScrollOffset())

				comp.SelectWordAt(term.Coordinates{})
				data, ok = comp.Selection()
				assert.True(t, ok)
				assert.Equal(t, "e", data)

				comp.Unselect()

				comp.Select(term.Coordinates{})
				comp.SelectEnd(term.Coordinates{})
				data, ok = comp.Selection()
				assert.True(t, ok)
				assert.Equal(t, "e", data)

				comp.Unselect()

				comp.SelectLine(term.Coordinates{})
				data, ok = comp.Selection()
				assert.True(t, ok)
				assert.Equal(t, "e    \n", data)

				require.True(t, comp.ScrollBottom())
				assertDraw(t, comp, "$    \n     \n     \n     \n     ")
			},
		},
		{
			desc:      "Input + Linefeed + CarriageReturn hit max scrollback history",
			altBuffer: false,
			sut: func(t *testing.T, comp *Component, tm *mockTabManager) {
				comp.Resize(5, 5)
				p := comp.parserHandler
				p.maxScrollLength = 6
				p.ResetState()
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
				assertDraw(t, comp, "a    \nb    \nc    \nd    \ne    ")

				p.Linefeed()
				p.CarriageReturn()
				p.Input('f')
				assertDraw(t, comp, "b    \nc    \nd    \ne    \nf    ")

				p.CarriageReturn()
				p.Linefeed()
				p.Input('g')
				assertDraw(t, comp, "c    \nd    \ne    \nf    \ng    ")
				assert.Equal(t, p.maxScrollLength, p.sync.buf.Rows())
			},
		},
	}

	for _, test := range suite {
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()
			tm := mockTabManager{}

			cfg := DefaultConfig()
			comp, err := NewComponent(&testExecutor{}, &testExecutor{}, &tm, cfg)
			require.NoError(t, err)

			ph := comp.parserHandler
			ph.sync.primBuf.SetDefaultChar(' ')
			ph.sync.altBuf.SetDefaultChar(' ')

			if test.altBuffer {
				ph.SetPrivateMode(vteparser.PrivateModeSwapScreenAndSetRestoreCursor)
			} else {
				ph.UnsetPrivateMode(vteparser.PrivateModeSwapScreenAndSetRestoreCursor)
			}

			test.sut(t, comp, &tm)
		})
	}
}

type testExecutor struct {
}

func (e *testExecutor) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	return 0, nil
}

func (e *testExecutor) Signal(pid workspaceapi.Pid, signal syscall.Signal) error {
	return nil
}

func (e *testExecutor) Close() error {
	return nil
}

func (e *testExecutor) NewPty(context.Context) (workspaceapi.Pty, error) {
	mockPtyFile := workspacetest.File{}
	return workspaceapi.Pty{Master: &mockPtyFile, Slave: &mockPtyFile}, nil
}

func (e *testExecutor) SetPtySize(p workspaceapi.Pty, width, height int) error {
	return nil
}

// recordingExecutor captures the workspaceapi.Cmd passed to StartCommand
// so tests can assert on env, path, args, etc.
type recordingExecutor struct {
	testExecutor
	mu         sync.Mutex
	cmd        workspaceapi.Cmd
	setPtySize []ptySize
}

func (e *recordingExecutor) StartCommand(
	ctx context.Context, cmd workspaceapi.Cmd,
) (workspaceapi.Pid, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.cmd = cmd
	return 0, nil
}

func (e *recordingExecutor) snapshotCmd() workspaceapi.Cmd {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.cmd
}

// TestComponentCreatePtySetsTerminalEnv pins the contract that the vte
// component injects a sane TERM/COLORTERM into every command it spawns.
// Without it, an SSH-workspace runesvc inherits TERM="" or "dumb" from
// its non-PTY SSH session; vim then falls back to its built-in "ansi"
// terminfo, which has no Ss/Se entries, never emits DECSCUSR, and the
// host renders CursorStyleDefault as a bar instead of the block vim
// would otherwise request.
func TestComponentCreatePtySetsTerminalEnv(t *testing.T) {
	t.Parallel()

	exec := &recordingExecutor{}
	cfg := DefaultConfig()
	_, err := NewComponent(exec, exec, &mockTabManager{}, cfg)
	require.NoError(t, err)

	got := exec.snapshotCmd()
	assert.Contains(t, got.Env, "TERM=xterm",
		"vte.Component must pin TERM=xterm (not xterm-256color) so "+
			"the spawned program (e.g. vim) emits SGR 30-37/90-97 "+
			"ANSI colors and lets the editor theme govern the "+
			"palette, while still finding Ss/Se in terminfo for "+
			"DECSCUSR. Got Env=%v", got.Env)
	for _, e := range got.Env {
		assert.NotEqual(t, "TERM=xterm-256color", e,
			"vte.Component must NOT advertise xterm-256color; "+
				"that would let programs use SGR 38;5;N which "+
				"bypasses the editor theme. Got Env=%v", got.Env)
	}
	assert.Contains(t, got.Env, "COLORTERM=",
		"vte.Component must clear COLORTERM so spawned programs "+
			"render with the 256-color ANSI palette and the "+
			"editor's theme remains the single source of truth "+
			"for colors. Got Env=%v", got.Env)
	for _, e := range got.Env {
		assert.NotEqual(t, "COLORTERM=truecolor", e,
			"vte.Component must NOT advertise truecolor; that "+
				"would let programs bypass the editor theme. "+
				"Got Env=%v", got.Env)
	}
}

// TestComponentAlternateScroll verifies AlternateScroll mirrors
// PrimaryScroll for the alternate buffer: callers can reach the rendered
// cells via Scroll.Buffer().RawCells() and the cursor via
// CursorAtScreen, without any cloning. The inference layer
// (term/vte/vteprobe) and BYOE's editorHandler rely on these two
// accessors plus IsAltBuffer to read the rendered grid of whichever
// screen the embedded program is drawing into.
func TestComponentAlternateScroll(t *testing.T) {
	t.Parallel()

	t.Run("primary", func(t *testing.T) {
		t.Parallel()
		comp, err := NewComponent(&testExecutor{}, &testExecutor{},
			&mockTabManager{}, DefaultConfig())
		require.NoError(t, err)
		ph := comp.parserHandler
		ph.sync.primBuf.SetDefaultChar(' ')
		ph.sync.altBuf.SetDefaultChar(' ')
		require.NoError(t, comp.Resize(5, 3))

		resetBuffer(t, ph, "abcde\nfg   \n     ")
		ph.setCursorAtScreen(term.Coordinates{X: 2, Y: 1}, false)

		assert.False(t, comp.IsAltBuffer(),
			"primary must be the active buffer by default")
		scroll, mu := comp.PrimaryScroll()
		require.NotNil(t, scroll)
		require.NotNil(t, mu)
		assert.Equal(t, term.Coordinates{X: 2, Y: 1},
			comp.CursorAtScreen(),
			"primary cursor must match what was set")
		assert.Equal(t, "abcde\nfg   \n     ",
			term.CellsToString(scroll.Buffer().RawCells()),
			"primary cells must match the active buffer contents")
	})

	t.Run("alternate", func(t *testing.T) {
		t.Parallel()
		comp, err := NewComponent(&testExecutor{}, &testExecutor{},
			&mockTabManager{}, DefaultConfig())
		require.NoError(t, err)
		ph := comp.parserHandler
		ph.sync.primBuf.SetDefaultChar(' ')
		ph.sync.altBuf.SetDefaultChar(' ')
		require.NoError(t, comp.Resize(5, 3))

		// Swap to the alternate screen first, then populate it.
		ph.SetPrivateMode(vteparser.PrivateModeSwapScreenAndSetRestoreCursor)
		require.True(t, comp.IsAltBuffer(),
			"alt buffer must be active after DECSET 1049")
		resetBuffer(t, ph, "ALT  \nBUF  \n     ")
		ph.setCursorAtScreen(term.Coordinates{X: 3, Y: 0}, false)

		scroll, mu := comp.AlternateScroll()
		require.NotNil(t, scroll)
		require.NotNil(t, mu)
		assert.Equal(t, term.Coordinates{X: 3, Y: 0},
			comp.CursorAtScreen(),
			"alt cursor must match what was set")
		assert.Equal(t, "ALT  \nBUF  \n     ",
			term.CellsToString(scroll.Buffer().RawCells()),
			"alt cells must match the active buffer contents")
	})
}

func assertDraw(t *testing.T, comp *Component, expected string) {
	t.Helper()
	writer := term.NewStringWriter(comp.width, comp.height)
	comp.Draw(writer)
	writer.Flush()
	assert.Equal(t, expected, writer.String())
}

// ptySize captures a single SetPtySize call so tests can assert the
// component drives the pty winsize via the schemeapi.Terminal contract.
type ptySize struct {
	width, height int
}

func (e *recordingExecutor) SetPtySize(p workspaceapi.Pty, width, height int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.setPtySize = append(e.setPtySize, ptySize{width: width, height: height})
	return nil
}

// TestComponentRestoreFromSnapshotDrivesSetPtySize reproduces a bug where
// a freshly-restored Component would keep its pre-restore width/height (e.g.
// the warm-reservoir WidthHint or a stale workspace size propagated via
// Facility.Resize) without driving that value into the pty via SetPtySize.
// When the window manager subsequently called Resize with dimensions that
// happened to match those stale values (a common case: snapshot was taken
// at the same workspace size), Component.Resize early-returned and never
// reached SetPtySize. The pty winsize stayed at the kernel default, the
// shell wrote 1-column output into primBuf, and the user saw a vertical
// "main\n?\n)\nblue\n…" cascade until they manually resized the tile.
//
// The fix unifies restore through the regular Resize path: after restoring
// the cells, RestoreFromSnapshot drives the snapshot dimensions through
// Component.Resize so SetPtySize is invoked exactly once via the same
// well-tested code path used by the window manager.
func TestComponentRestoreFromSnapshotDrivesSetPtySize(t *testing.T) {
	t.Parallel()

	tm := mockTabManager{}
	exe := &recordingExecutor{}
	cfg := DefaultConfig()
	comp, err := NewComponent(exe, exe, &tm, cfg)
	require.NoError(t, err)

	const w, h = 80, 24

	// simulate the warm reservoir / Facility.Resize path: live dimensions
	// are already (w, h) before RestoreFromSnapshot is called.
	require.NoError(t, comp.Resize(w, h))
	require.Equal(t, w, comp.width)
	require.Equal(t, h, comp.height)

	exe.setPtySize = nil

	snap := Snapshot{
		Version: terminalSnapshotVersion,
		Width:   w,
		Height:  h,
		Primary: ScreenSnapshot{
			Cells:  [][]term.Cell{{{Ch: 'h'}, {Ch: 'i'}}},
			Cursor: term.Coordinates{X: 2, Y: 0},
		},
	}

	_, err = comp.RestoreFromSnapshot(snap)
	require.NoError(t, err)

	// RestoreFromSnapshot must drive the snapshot's dimensions through
	// the regular Resize path so SetPtySize is called even when the
	// live width/height already match the snapshot dimensions. Without
	// this, the pty stays at its kernel-default winsize and the shell
	// renders into a 0/1-column buffer.
	require.NotEmpty(t, exe.setPtySize,
		"RestoreFromSnapshot must call SetPtySize via the regular Resize path")
	assert.Equal(t, ptySize{width: w, height: h},
		exe.setPtySize[len(exe.setPtySize)-1])
	assert.Equal(t, w, comp.width)
	assert.Equal(t, h, comp.height)

	// The window manager's follow-up Resize at the tile's dimensions
	// (matching the snapshot dimensions) is now a legitimate no-op.
	exe.setPtySize = nil
	require.NoError(t, comp.Resize(w, h))
	assert.Empty(t, exe.setPtySize,
		"follow-up Resize at the same size is a legitimate no-op")
}

// TestComponentResizeIsSerialized pins that Component.Resize takes
// Component.mu when mutating t.width/t.height. Other Component
// accessors (CursorVisible, ScrollDown, drawSelection via Selection,
// Snapshot) read t.width/t.height under t.mu, so writing them off-lock
// from Resize races the IDE event-loop reads. The race detector trips
// when Resize touches shared state without locking.
func TestComponentResizeIsSerialized(t *testing.T) {
	t.Parallel()

	comp, err := NewComponent(&testExecutor{}, &testExecutor{},
		&mockTabManager{}, DefaultConfig())
	require.NoError(t, err)
	require.NoError(t, comp.Resize(20, 10))

	const iterations = 500
	stop := make(chan struct{})
	done := make(chan struct{})

	// Reader: takes Component.mu and reads t.height (CursorVisible at
	// component.go:305 references t.height under t.mu).
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = comp.CursorVisible()
		}
	}()

	// Writer: Component.Resize writes t.width/t.height. Must take
	// t.mu to serialize with the reader.
	for i := range iterations {
		w, h := 20, 10
		if i%2 == 0 {
			w, h = 30, 12
		}
		require.NoError(t, comp.Resize(w, h))
	}
	close(stop)
	<-done
}
