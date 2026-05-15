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
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"
	"go.uber.org/multierr"
	"mvdan.cc/sh/v3/shell"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/term/vte/vteparser"
	"unstable.build/go-tui/text"
)

// Component implements a vte terminal emulator tui.Component.
type Component struct {
	mu         sync.Mutex
	terminal   schemeapi.Terminal
	executor   schemeapi.Executor
	clipboard  clipboard.Register
	cfg        Config
	pty        workspaceapi.Pty
	cmdAndArgs []string
	watcher    workspaceapi.ProcessWatcher
	scroll     component.Scroll
	ctx        context.Context
	cancelCtx  func()
	uri        workspaceapi.URI
	remote     remote
	writech    chan []byte
	writeErr   atomic.Value

	width, height     int
	parserHandler     *parserHandler
	waitParserHandler *waitParserHandler
	parser            vteparser.Parser
	complete          bool
	selectionAttr     term.Attributes
	defAttr           term.Attributes
}

// NOTE: this is an integrator implementation, it shouldn't really do much other
// than creating a pty and initializing the vte parser and the parser handler.

// NewComponent allocates storage for a new Component and initializes it.
func NewComponent(
	t schemeapi.Terminal, e schemeapi.Executor,
	tm browser.TabManager, cfg Config,
) (*Component, error) {
	ret := new(Component)
	err := ret.Init(t, e, tm, cfg)
	return ret, err
}

// Init initializes it with the given dependencies and options.
func (t *Component) Init(
	term schemeapi.Terminal, e schemeapi.Executor,
	tm browser.TabManager, cfg Config,
) error {
	t.clipboard = cfg.Clipboard
	t.cmdAndArgs = cfg.CommandAndArgs
	t.watcher = cfg.Watcher
	t.defAttr = cfg.Attributes
	t.selectionAttr = cfg.SelectionAttributes
	t.terminal = term
	t.executor = e
	t.writech = make(chan []byte, 64)
	t.cfg = cfg

	t.ctx, t.cancelCtx = context.WithCancel(context.Background())
	err := t.createPty()
	if err != nil {
		return err
	}

	if cfg.ScheduleNextTick == nil || cfg.RingBell == nil {
		panic("nil schedule/bell function(s)")
	}

	t.parserHandler = newParserHandler(
		&t.mu, t.pty, tm, t.clipboard, cfg.scheduleBell, t.uri,
		cfg.NeedsAttentionAttributes, cfg.DynamicTabName, cfg.MaxLines, cfg.MinWidth)

	// start with pty slave file name as title
	var h vteparser.Handler = t.parserHandler
	if log.IsLevelEnabled(log.TraceLevel) {
		h = vteparser.HandlerWithLogging("vte.parserHandler", h)
	}
	t.scroll.InitPerformance(&t.parserHandler.sync.primBuf.Cells)
	t.scroll.InvertOffset = true
	t.scroll.SetTabspaces(1)

	t.remote = ptyWriterRemote(t, cfg.ScheduleNextTick)
	t.waitParserHandler = newWaitParserHandler(t.ctx, h)
	h = t.waitParserHandler
	t.waitParserHandler.useTrigger(t.triggerBell)

	t.parser.Init(h, new(vteparser.StdTimeout))
	t.SetDefaultAttributes(t.defAttr)
	return err
}

func (t *Component) triggerBell() {
	err := t.remote.triggerBell()
	if err != nil {
		t.log(log.WarnLevel, "trigger bell: %v", err)
	}
}

// Run must be called in a separate goroutine to start processing incoming
// data from the pty master.
func (t *Component) Run(updateChan chan struct{}) error {
	// interrupt at most at a reasonable fps. This improves
	// performance when program is dumping Kbs of content
	// into the terminal scroll.
	// buffer to 1, so we publish one last interrupt after
	// maxInterruptPerSecond since last interrupt
	ch := make(chan struct{}, 1)
	go debug.CapturePanicReport(func() {
		maxInterruptPerSecond := time.Duration(int(time.Second) / 30)
		timer := time.NewTimer(maxInterruptPerSecond)
		defer timer.Stop()
		defer close(updateChan)
		for {
			select {
			case <-ch:
			case <-t.ctx.Done():
				return
			}
			select {
			case updateChan <- struct{}{}:
			case <-t.ctx.Done():
				return
			}
			timer.Reset(maxInterruptPerSecond)
			select {
			case <-timer.C:
			case <-t.ctx.Done():
				return
			}
		}
	})
	go debug.CapturePanicReport(func() {
		for {
			select {
			case data := <-t.writech:
				_, err := t.pty.Master.Write(data)
				if err != nil {
					t.log(log.ErrorLevel, "write to pty: %v", err)
					t.writeErr.Store(err)
					return
				}
			case <-t.ctx.Done():
				return
			}
		}
	})

	return t.run(ch)
}

// Title returns the Title of this Component.
func (t *Component) Title() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.parserHandler.title
}

// URI returns the raw URI of this terminal emulator.
func (t *Component) URI() workspaceapi.URI {
	return t.uri
}

// WriteToPty writes the given data to the underlying pty master.
func (t *Component) WriteToPty(data []byte) error {
	if err := t.writeErr.Load(); err != nil {
		return err.(error)
	}
	// t.log(log.TraceLevel, "WriteToPty: %s", string(data))
	select {
	case t.writech <- data:
		t.log(log.TraceLevel, "written %d byte(s) to the pty", len(data))
	default:
		return errors.New("pty write queue is full")
	}
	return nil
}

// Resize resizes this component and returns an error if
// the call to resize the underlying pty failed.
func (t *Component) Resize(width, height int) error {
	if t.pty.Master == nil {
		return fmt.Errorf("terminal is not running")
	}

	if t.width == width && t.height == height {
		// some programs will not re-print if width and height
		// are the same, but resizing buffers does clear all the content
		// so we would be left with an empty screen buffer.
		return nil
	}

	err := t.terminal.SetPtySize(t.pty, width, height)
	if err != nil {
		return err
	}

	t.width = width
	t.height = height

	t.parserHandler.Resize(width, height)
	t.scroll.Resize(width, height)

	return nil
}

// ModeBracketedPaste returns whether bracketed paste mode is set.
func (t *Component) ModeBracketedPaste() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.parserHandler.modeBracketedPaste
}

// MouseModeReportMouseClicks returns whether PrivateMode 1000 (MouseModeVT200) is set.
func (t *Component) MouseModeReportMouseClicks() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.parserHandler.modeReportMouseClicks
}

// MouseModeReportCellMouseMotion returns whether PrivateMode 1002 (MouseModeButtonEvent) is set.
func (t *Component) MouseModeReportCellMouseMotion() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.parserHandler.modeReportCellMouseMotion
}

// MouseModeReportAllMouseMotion returns whether PrivateMode 1003 (MouseModeAnyEvent) is set.
func (t *Component) MouseModeReportAllMouseMotion() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.parserHandler.modeReportAllMouseMotion
}

// MouseModeUtf8Mouse returns whether PrivateMode 1005 (MouseExtUTF) is set.
func (t *Component) MouseModeUtf8Mouse() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.parserHandler.modeUtf8Mouse
}

// MouseModeSgrMouse returns whether PrivateMode 1006 (MouseExtSGR) is set.
func (t *Component) MouseModeSgrMouse() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.parserHandler.modeSgrMouse
}

// CursorVisible returns whether the cursor should be rendered or not.
func (t *Component) CursorVisible() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return !t.parserHandler.cursorHidden &&
		t.scroll.Offset().Y == 0 &&
		t.parserHandler.modeShowCursor &&
		t.parserHandler.sync.buf.CursorAtScreen().Y < t.height
}

// CursorAtScreen returns the current coordinates of the cursor,
// relative to the screen.
func (t *Component) CursorAtScreen() term.Coordinates {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.scroll.Offset().Y != 0 {
		return term.Coordinates{}
	}
	return t.parserHandler.sync.buf.CursorAtScreen()
}

// CursorAtScroll returns the current coordinates of the cursor,
// relative to the underlying scroll.
func (t *Component) CursorAtScroll() term.Coordinates {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.cursorAtScroll()
}

// CursorStyle returns the term.CursorStyle that should be rendered
// with this Component, if IsCursorVisible returns true.
func (t *Component) CursorStyle() term.CursorStyle {
	t.mu.Lock()
	defer t.mu.Unlock()

	if !t.parserHandler.modeBlinkingCursor {
		return t.parserHandler.cursorStyle
	}

	switch t.parserHandler.cursorStyle {
	case term.CursorStyleSteadyUnderline:
		return term.CursorStyleBlinkingUnderline
	case term.CursorStyleSteadyBar:
		return term.CursorStyleBlinkingBar
	default:
		return term.CursorStyleBlinkingBlock
	}
}

// Height returns the height of the underlying terminal buffer in lines.
func (t *Component) Height() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.parserHandler.sync.buf.Rows()
}

// MaxWidth returns the maximum width of the underlying terminal buffer in columns.
func (t *Component) MaxWidth() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	// NOTE: using a cell.Buffer's MaxColumns is not exact
	// when there are multi-width characters. As long as
	// MaxWidth is used for hinting, it should be ok.
	if t.parserHandler.useAlt {
		return t.parserHandler.sync.altBuf.MaxColumns()
	}
	return t.parserHandler.sync.primBuf.MaxColumns()
}

// ScrollDown scrolls down content of this terminal emulator.
func (t *Component) ScrollDown(count int) (ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.parserHandler.useAlt {
		return
	}
	offset := t.scroll.Offset().Y
	// do not rely on primary buffer max offset, as it uses cursor
	// or scroll max offset, as it doesn't allow for rows-1 full scroll,
	// only rows-height-1 scroll.
	maxOffset := t.scroll.MaxOffset().Y + t.height - 1
	target := min(maxOffset, offset+count)

	for i := 0; i < target-offset && t.scroll.SeekDown(); i++ {
		ok = true
	}
	return
}

// ScrollUp scrolls up the content of this terminal emulator.
func (t *Component) ScrollUp(count int) (ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.parserHandler.useAlt {
		return
	}
	for i := 0; i < count && t.scroll.SeekUp(); i++ {
		ok = true
	}
	return
}

// ScrollOffset returns the current vertical scroll offset.
func (t *Component) ScrollOffset() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.parserHandler.useAlt {
		return 0
	}

	return t.scroll.MaxOffset().Y - t.scroll.Offset().Y
}

// MaxScrollOffset returns the current vertical scroll offset.
func (t *Component) MaxScrollOffset() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.parserHandler.useAlt {
		return 0
	}

	return t.scroll.MaxOffset().Y
}

// ScrollBottom scrolls down the content of this terminal emulator to the bottom.
func (t *Component) ScrollBottom() (ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.parserHandler.useAlt {
		return
	}

	return t.scroll.SeekEndFile()
}

// SetDefaultAttributes updates the default attributes of this terminal emulator.
func (t *Component) SetDefaultAttributes(attrs term.Attributes) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.parserHandler.sync.primBuf.SetDefaultAttributes(attrs)
	t.parserHandler.sync.altBuf.SetDefaultAttributes(attrs)
	t.scroll.Attributes = attrs
}

// IsApplicationCursorKeysMode returns whether cursor keys mode is enabled.
// https://vt100.net/docs/vt510-rm/chapter2.html#S2.8.11
func (t *Component) IsApplicationCursorKeysMode() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.parserHandler.modeCursorKeys
}

// IsAltBuffer returns true if underlying buffer utilizes is the alternate buffer.
func (t *Component) IsAltBuffer() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.parserHandler.useAlt
}

// IsNewLineMode returns whether new line mode is enabled.
// https://vt100.net/docs/vt510-rm/chapter2.html#S2.5.13
func (t *Component) IsNewLineMode() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.parserHandler.modeLineFeedNewLine
}

// Draw satisfies tui.Component.
func (t *Component) Draw(w term.Writer) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.parserHandler.useAlt {
		t.parserHandler.sync.altBuf.Draw(w)
	} else if t.scroll.Offset().Y == 0 {
		t.parserHandler.sync.primBuf.Draw(w)
	} else {
		t.scroll.Draw(w)
	}

	t.drawSelection(w)
}

// IsComplete returnes whether this terminal has stopped processing
// data from the pty file.
func (t *Component) IsComplete() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.complete
}

// Unselect clears this Component's selection.
func (t *Component) Unselect() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.parserHandler.useAlt {
		t.parserHandler.sync.altBuf.Unselect()
	} else {
		t.parserHandler.sync.primBuf.Unselect()
	}
}

// Select anchors the current cursor position as the start and end of a text selection.
func (t *Component) Select(pos term.Coordinates) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.parserHandler.useAlt {
		t.parserHandler.sync.altBuf.Select(pos)
	} else if t.scroll.Offset().Y == 0 {
		t.parserHandler.sync.primBuf.Select(pos)
	} else {
		pos.Y -= t.scroll.Offset().Y
		t.parserHandler.sync.primBuf.Select(pos)
	}
}

// SelectEnd anchors the current cursor position as the end of a text selection.
func (t *Component) SelectEnd(pos term.Coordinates) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.parserHandler.useAlt {
		t.parserHandler.sync.altBuf.SelectEnd(pos)
	} else if t.scroll.Offset().Y == 0 {
		t.parserHandler.sync.primBuf.SelectEnd(pos)
	} else {
		pos.Y -= t.scroll.Offset().Y
		t.parserHandler.sync.primBuf.SelectEnd(pos)
	}
}

// SelectWordAt selects the word under the current cursor position.
func (t *Component) SelectWordAt(pos term.Coordinates) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.parserHandler.useAlt {
		t.parserHandler.sync.altBuf.SelectWordAt(pos)
	} else if t.scroll.Offset().Y == 0 {
		t.parserHandler.sync.primBuf.SelectWordAt(pos)
	} else {
		pos.Y -= t.scroll.Offset().Y
		t.parserHandler.sync.primBuf.SelectWordAt(pos)
	}
}

// SelectLine anchors the current cursor position as the start and end line of
// the text selection.
func (t *Component) SelectLine(pos term.Coordinates) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.parserHandler.useAlt {
		t.parserHandler.sync.altBuf.SelectLine(pos)
	} else if t.scroll.Offset().Y == 0 {
		t.parserHandler.sync.primBuf.SelectLine(pos)
	} else {
		pos.Y -= t.scroll.Offset().Y
		t.parserHandler.sync.primBuf.SelectLine(pos)
	}
}

// Selection returns the current selection or false if no
// text is currently selected.
func (t *Component) Selection() (data string, ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var cells [][]term.Cell
	cells, ok = t.selection()
	if !ok {
		return
	}
	data = term.CellsToString(cells)
	data = strings.ReplaceAll(data, "\x00", "")
	return data, ok
}

// OnFocusChange allows clients to report whether this vte.Component is on focus or not.
func (t *Component) OnFocusChange(inFocus bool) error {
	t.mu.Lock()
	cmd, ok := t.parserHandler.onFocusChange(inFocus)
	t.mu.Unlock()
	if !ok {
		return nil
	}
	err := t.WriteToPty(fmt.Appendf(nil, "\x1b[%s", cmd))
	if err != nil {
		return fmt.Errorf("write to pty: %w", err)
	}
	return nil
}

// PrimaryScroll returns the primary buffer's underlying component.Scroll.
func (t *Component) PrimaryScroll() *component.Scroll {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.parserHandler.sync.primBuf.Scroll()
}

// Snapshot returns a durable snapshot of the terminal's primary rendered
// buffer. It captures primary history plus visible screen, but it does not
// attempt to persist the live pty process or alternate-screen program state.
func (t *Component) Snapshot() (Snapshot, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return Snapshot{
		Version:      terminalSnapshotVersion,
		Title:        t.parserHandler.title,
		Width:        t.width,
		Height:       t.height,
		ScrollOffset: t.scroll.Offset(),
		Primary: ScreenSnapshot{
			Cells:  term.CloneCells(t.parserHandler.sync.primBuf.Cells.RawCells()),
			Cursor: t.parserHandler.sync.primBuf.CursorAtScroll(),
		},
	}, nil
}

// RestoreFromSnapshot restores a previously captured terminal snapshot
// into this live terminal emulator's primary buffer. It restores rendered
// buffer contents, cursor position, title and scroll offset while leaving the
// currently running pty process intact.
//
// The snapshot's Width/Height are used as the authoritative layout
// dimensions for the restored cells, since those cells were laid out at
// that geometry. After loading the cells, this method routes through the
// regular Resize path so the pty winsize, parser-handler buffers and
// scroll component all converge on the snapshot dimensions through a
// single, well-tested code path. Without this, a follow-up Resize with
// dimensions that happen to match the component's stale live width/height
// would short-circuit at Resize's "same size" early return, leaving the
// pty winsize at its kernel default; the shell would then write narrow
// output into primBuf (the user-visible "main\n?\n)\nblue" cascade) until
// the user manually resized the tile.
func (t *Component) RestoreFromSnapshot(snapshot Snapshot) (cursor term.Coordinates, err error) {
	width, height := snapshot.Width, snapshot.Height
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	// Replace primBuf contents and reset live dimensions under t.mu.
	// Live dimensions are cleared so the follow-up Resize below cannot
	// short-circuit at its "same size" early-return; this is the
	// invariant that previously broke when t.width was preset by the
	// reservoir / Facility.Resize path and matched the tile size.
	t.mu.Lock()
	t.parserHandler.sync.primBuf.Restore(
		snapshot.Primary.Cells, snapshot.Primary.Cursor, width, height)
	t.parserHandler.useAlt = false
	t.parserHandler.sync.buf = t.parserHandler.sync.primBuf
	if snapshot.Title != "" {
		t.parserHandler.title = snapshot.Title
	}
	t.scroll.InitPerformance(&t.parserHandler.sync.primBuf.Cells)
	t.scroll.InvertOffset = true
	t.scroll.SetTabspaces(1)
	t.width = 0
	t.height = 0
	t.mu.Unlock()

	// Drive the snapshot dimensions through the regular Resize path so
	// SetPtySize, parserHandler.Resize and scroll.Resize all run via the
	// same code as a normal window-manager Resize. Resize takes t.mu
	// (transitively via parserHandler.Resize), so it must run with the
	// lock released.
	if err = t.Resize(width, height); err != nil {
		return cursor, err
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	offset := snapshot.ScrollOffset
	maxOffset := t.scroll.MaxOffset()
	offset.X = max(0, min(offset.X, maxOffset.X))
	offset.Y = max(0, min(offset.Y, maxOffset.Y))
	t.scroll.SetOffset(offset)
	cursor = t.parserHandler.sync.primBuf.CursorAtScroll()
	return cursor, nil
}

// Locker returns the underlying sync.Locker used by this Component
// to synchronize access to the internal state.
func (t *Component) Locker() sync.Locker {
	return &t.mu
}

// UsedAlternateBuffer returns whether the alternate buffer was used
// at some point by the underlying program driving the vte.
func (t *Component) UsedAlternateBuffer() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.parserHandler.usedAlternate()
}

// ClearPrimaryBuffer clears the primary buffer of this Component.
// If this Component is using the alternate buffer,
// this method returns false.
func (t *Component) ClearPrimaryBuffer() (ok bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.parserHandler.useAlt {
		return
	}

	t.parserHandler.sync.primBuf.Reset()
	_ = t.WriteToPty([]byte("\n"))
	return true
}

// Close assumes lock has been acquired by caller
func (t *Component) Close() (ret error) {
	defer t.cancelCtx()

	if err := t.pty.Slave.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := t.pty.Master.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

//nolint:unused
func (t *Component) log(level log.Level, line string, params ...any) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "vte.Component").
		Logf(level, line, params...)
}

func (t *Component) createPty() error {
	pty, err := t.terminal.NewPty(t.ctx)
	if err != nil {
		return fmt.Errorf("new pty: %v", err)
	}

	t.uri, err = workspaceapi.CurrentUserHostURI(pty.Slave.Name())
	if err != nil {
		return fmt.Errorf("pty URI: %v", err)
	}

	cmdAndArgs := t.cmdAndArgs
	cmdAndArgsStr := strings.Join(cmdAndArgs, " ")
	// NOTE: this uses os.Getenv, but it should use the workspace's
	// Getenv mechanism, which should be implemented at some point.
	cmdAndArgs, err = shell.Fields(cmdAndArgsStr, os.Getenv)
	if err != nil {
		return fmt.Errorf("expand shell arguments: %w", err)
	}
	t.log(log.DebugLevel, "creating pty with cmdAndArgs: %#v", cmdAndArgs)
	cmd := workspaceapi.Cmd{
		SysProcAttr: &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
		},
		Watcher: t.watcher,
	}
	cmd.Env = appendDefaultTerminalEnv(cmd.Env)
	// When the user hasn't configured a CommandAndArgs, leave
	// cmd.Path empty: that's the protocol contract for "use the
	// user's login shell on the executor's host",
	if len(cmdAndArgs) > 0 {
		cmd.Path = cmdAndArgs[0]
		if len(cmdAndArgs) > 1 {
			cmd.Args = cmdAndArgs[1:]
		}
	}

	cmd.Stdout = pty.Slave
	cmd.Stderr = pty.Slave
	cmd.Stdin = pty.Slave

	_, retErr := t.executor.StartCommand(t.ctx, cmd)
	if retErr != nil {
		retErr = fmt.Errorf("start command: %w", retErr)
		if err := pty.Master.Close(); err != nil {
			err = fmt.Errorf("close pty: %w", err)
			retErr = multierr.Append(retErr, err)
		}
	}
	if retErr != nil {
		return retErr
	}
	t.pty = pty
	return nil
}

// appendDefaultTerminalEnv pins TERM=xterm-256color and unsets
// COLORTERM for the program spawned in the vte. Existing TERM /
// COLORTERM entries in env are preserved so a caller can override.
//
// The COLORTERM clear is intentional: we want programs running in
// the vte to render with the 256-color ANSI palette so the editor
// theme remains the single source of truth for colors. The
// executor's host env typically carries COLORTERM=truecolor (the
// local rune process sets it via setEnvForGUI); without an explicit
// empty override, fileScheme would append it after our entries when
// it merges stdcmd.Environ() with cmd.Env, and the child would
// advertise truecolor and bypass the theme palette.
func appendDefaultTerminalEnv(env []string) []string {
	// We pick plain "xterm" (16-color ANSI) rather than
	// "xterm-256color" so programs emit SGR 30-37/40-47/90-97/100-107
	// and the editor's theme palette is the single source of truth
	// for colors. xterm terminfo still carries Ss/Se, so the cursor
	// shape contract (block in vim normal mode) is preserved.
	const defaultTerm = "xterm"
	hasTerm, hasColorTerm := false, false
	for _, e := range env {
		switch {
		case strings.HasPrefix(e, "TERM="):
			hasTerm = true
		case strings.HasPrefix(e, "COLORTERM="):
			hasColorTerm = true
		}
	}
	if !hasTerm {
		env = append(env, "TERM="+defaultTerm)
	}
	if !hasColorTerm {
		env = append(env, "COLORTERM=")
	}
	return env
}

func (t *Component) selection() (cells [][]term.Cell, ok bool) {
	if t.parserHandler.useAlt {
		cells, ok = t.parserHandler.sync.altBuf.Selection()
	} else {
		cells, ok = t.parserHandler.sync.primBuf.Selection()
	}
	return
}

func (t *Component) drawSelection(w term.Writer) {
	var from, to, offset term.Coordinates
	var mode text.SelectMode
	var ok bool
	if t.parserHandler.useAlt {
		mode, from, to, ok = t.parserHandler.sync.altBuf.SelectionCoordinatesAtScroll()
	} else {
		mode, from, to, ok = t.parserHandler.sync.primBuf.SelectionCoordinatesAtScroll()
		offset = term.Coordinates{Y: t.scroll.Buffer().Rows() - t.height - t.scroll.Offset().Y}
	}
	if !ok {
		return
	}
	for y := from.Y; y <= to.Y; y++ {
		xStart, xEnd := 0, t.parserHandler.sync.buf.Columns(y)
		if y == from.Y && mode == text.StandardSelection {
			xStart = from.X
		}
		if y == to.Y && mode == text.StandardSelection {
			xEnd = to.X
		}
		for x := xStart; x < xEnd; x++ {
			pos := term.Coordinates{X: x, Y: y}
			c := t.parserHandler.sync.buf.CellAt(pos)
			if c == nil {
				continue
			}
			posAtScreen := term.CoordinatesDiff(pos, offset)
			if posAtScreen.Y < 0 || posAtScreen.Y >= t.height ||
				posAtScreen.X < 0 || posAtScreen.X >= t.width {
				continue
			}
			w.SetCell(posAtScreen, term.Cell{
				Ch:         c.Ch,
				Attributes: t.selectionAttr,
				Width:      c.Width,
				Combining:  c.Combining,
			})
		}
	}
}
func (t *Component) cursorAtScroll() term.Coordinates {
	return t.parserHandler.sync.buf.CursorAtScroll()
}

func (t *Component) scheduleBellCallback(callback func()) (ok bool) {
	return t.waitParserHandler.scheduleBellCallback(callback)
}

func (t *Component) systemCanDispatchBell(callback func(error)) {
	const systemCanDispatchBellTimeout = 3 * time.Second
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, systemCanDispatchBellTimeout)

	var called atomic.Bool
	t.waitParserHandler.scheduleBellCallback(func() {
		if called.CompareAndSwap(false, true) {
			cancel()
			t.cfg.ScheduleNextTick(func() {
				callback(nil)
			})
		}
	})

	go debug.CapturePanicReport(func() {
		defer cancel()
		<-ctx.Done()
		if called.CompareAndSwap(false, true) {
			t.cfg.ScheduleNextTick(func() {
				callback(fmt.Errorf("timeout waiting for bell: %w", ctx.Err()))
			})
		}
	})
}

func (t *Component) pendingCallbacks() int {
	return t.waitParserHandler.pendingCallbacks()
}

func (t *Component) run(updateChan chan struct{}) error {
	t.mu.Lock()
	complete := t.complete
	t.mu.Unlock()
	if complete {
		panic("called Run twice on vte.Component")
	}

	defer func() {
		t.mu.Lock()
		t.complete = true
		t.mu.Unlock()
	}()

	buf := make([]byte, os.Getpagesize())
	for {
		n, err := t.pty.Master.Read(buf[:])
		if err != nil {
			return err
		}
		for i := range n {
			t.parser.Advance(buf[i])
		}
		select {
		// Close was called, just return error
		case <-t.ctx.Done():
			return t.ctx.Err()
		// no interrupts in the last maxInterruptPeriod
		case updateChan <- struct{}{}:
		// an interrupt was requested in the last maxInterruptPeriod
		// don't request any further interrupts for now
		default:
		}
	}
}
