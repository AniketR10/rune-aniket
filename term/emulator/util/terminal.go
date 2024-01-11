package termutil

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"

	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
)

const (
	MainBuffer     uint8 = 0
	AltBuffer      uint8 = 1
	InternalBuffer uint8 = 2
)

// Terminal represents the implementation of a terminal emulator.
type Terminal struct {
	terminal          schemeapi.Terminal
	executor          schemeapi.Executor
	mu                sync.Mutex
	pty               workspaceapi.Pty
	windowManipulator WindowManipulator
	reader            *bufio.Reader
	updateChan        chan struct{}
	buffers           []*Buffer
	activeBuffer      *Buffer
	mouseMode         MouseMode
	mouseExtMode      MouseExtMode
	logFile           *os.File
	theme             *Theme
	closed            bool
	complete          bool
	shell             string
	watcher           workspaceapi.Watcher
	ctx               context.Context
	cancelCtx         func()
}

// New allocates storage for a new Terminal and initializes it.
func New(t schemeapi.Terminal, e schemeapi.Executor, options ...Option) *Terminal {
	term := &Terminal{
		theme: &Theme{},
	}
	for _, opt := range options {
		opt(term)
	}
	attr := term.theme.Default
	term.buffers = []*Buffer{
		NewBuffer(1, 1, 0xffff, attr),
		NewBuffer(1, 1, 0xffff, attr),
		NewBuffer(1, 1, 0xffff, attr),
	}
	term.activeBuffer = term.buffers[0]
	term.terminal = t
	term.executor = e
	term.ctx, term.cancelCtx = context.WithCancel(context.Background())

	return term
}

// CreatePty creates a new pty and sets it as this terminal's pty.
// Calling this method twice panics.
func (t *Terminal) CreatePty() (workspaceapi.Pty, error) {
	if t.pty != (workspaceapi.Pty{}) {
		panic("called Terminal.CreatePty twice on the same instance")
	}
	pty, err := t.terminal.NewPty(t.ctx)
	if err != nil {
		return pty, fmt.Errorf("new pty: %v", err)
	}
	shell := t.shell
	if shell == "" {
		shell = os.Getenv("SHELL")
	}
	if shell == "" {
		shell = "sh"
	}
	cmdAndArgs := strings.Split(shell, " ")
	// setup command
	cmd := workspaceapi.Cmd{
		Path: cmdAndArgs[0],
		Args: cmdAndArgs[1:],
		SysProcAttr: &syscall.SysProcAttr{
			Setsid:  true,
			Setctty: true,
		},
		Watcher: t.watcher,
	}

	cmd.Stdout = pty.Slave
	cmd.Stderr = pty.Slave
	cmd.Stdin = pty.Slave

	_, retErr := t.executor.StartCommand(t.ctx, cmd)
	if retErr != nil {
		retErr = fmt.Errorf("start command: %v", retErr)
		if err := pty.Master.Close(); err != nil {
			err = fmt.Errorf("close pty: %v", err)
			retErr = multierr.Append(retErr, err)
		}
	}
	if retErr != nil {
		return workspaceapi.Pty{}, retErr
	}
	t.pty = pty
	return pty, nil
}

func (t *Terminal) SetWindowManipulator(m WindowManipulator) {
	t.windowManipulator = m
}

func (t *Terminal) log(line string, params ...interface{}) {
	log.WithField(logging.KeyClass, "util.Terminal").
		Tracef(line, params...)
}

func (t *Terminal) reset() {
	attr := t.theme.Default
	t.buffers = []*Buffer{
		NewBuffer(1, 1, 0xffff, attr),
		NewBuffer(1, 1, 0xffff, attr),
		NewBuffer(1, 1, 0xffff, attr),
	}
	t.useMainBuffer()
}

// Pty exposes the underlying terminal pty, if it exists
func (t *Terminal) Pty() workspaceapi.Pty {
	return t.pty
}

func (t *Terminal) WriteToPty(data []byte) error {
	_, err := t.pty.Master.Write(data)
	return err
}

func (t *Terminal) GetTitle() string {
	return t.windowManipulator.GetTitle()
}

func (t *Terminal) SetSize(rows, cols uint16) error {
	if t.pty.Master == nil {
		return fmt.Errorf("terminal is not running")
	}

	t.log("Terminal.SetSize: %d, %d\n", cols, rows)

	t.activeBuffer.resizeView(cols, rows)

	err := t.terminal.SetPtySize(t.pty, int(cols), int(rows))
	if err != nil {
		return err
	}

	return nil
}

// Run starts the terminal/shell proxying process
func (t *Terminal) Run(updateChan chan struct{}) error {
	defer func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		close(updateChan)
		t.complete = true
	}()

	t.mu.Lock()
	t.updateChan = updateChan
	t.reader = bufio.NewReaderSize(t.pty.Master, 1024*1024)
	t.mu.Unlock()

	for {
		r, _, err := t.reader.ReadRune()
		if err != nil && err != io.EOF {
			t.mu.Lock()
			closed := t.closed
			t.mu.Unlock()
			if closed {
				return nil
			}
			return err
		}

		width := 1 // terminal doesn't support wide characters yet
		render, exit := t.processSequence(MeasuredRune{Rune: r, Width: width})
		if exit || err == io.EOF {
			return nil
		}
		if render {
			t.requestRender()
		}
	}
}

func (t *Terminal) requestRender() {
	select {
	case t.updateChan <- struct{}{}:
	default:
	}
}

func (t *Terminal) processSequence(mr MeasuredRune) (render, exit bool) {
	if mr.Rune == 0x1b {
		return t.handleANSI()
	}
	return t.processRunes(mr)
}

func (t *Terminal) processRunes(runes ...MeasuredRune) (renderRequired, exit bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, r := range runes {

		// t.log("Terminal.processRunes: %c 0x%X", r.Rune, r.Rune)

		switch r.Rune {
		case 0x0, 0x1, 0x2, 0x3, 0x4, 0x6, 0x10,
			0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19:
			continue
		case 0x05: //enq
			continue
		case 0x07: //bell
			//DING DING DING
			continue
		case 0x8: //backspace
			t.activeBuffer.backspace()
			renderRequired = true
		case 0x9: //tab
			t.activeBuffer.tab()
			renderRequired = true
		case 0xa, 0xc: //newLine/form feed
			t.activeBuffer.newLine()
			renderRequired = true
		case 0xb: //vertical tab
			t.activeBuffer.verticalTab()
			renderRequired = true
		case 0xd: //carriageReturn
			t.activeBuffer.carriageReturn()
			renderRequired = true
		case 0xe: //shiftOut
			t.activeBuffer.currentCharset = 1
		case 0xf: //shiftIn
			t.activeBuffer.currentCharset = 0
		default:
			t.activeBuffer.write(t.translateRune(r))
			renderRequired = true
		}
	}

	// it doesn't matter whether we shortcircuit processing of runes
	// upon close so lower "is-closed" checks on a critical path
	return renderRequired, t.closed
}

func (t *Terminal) translateRune(b MeasuredRune) MeasuredRune {
	table := t.activeBuffer.charsets[t.activeBuffer.currentCharset]
	if table == nil {
		return b
	}
	chr, ok := (*table)[b.Rune]
	if ok {
		return MeasuredRune{Rune: chr, Width: 1}
	}
	return b
}

func (t *Terminal) setTitle(title string) {
	t.windowManipulator.SetTitle(title)
}

func (t *Terminal) switchBuffer(index uint8) {
	var carrySize bool
	var w, h uint16
	if t.activeBuffer != nil {
		w, h = t.activeBuffer.viewWidth, t.activeBuffer.viewHeight
		carrySize = true
	}
	t.activeBuffer = t.buffers[index]
	if carrySize {
		t.activeBuffer.resizeView(w, h)
	}
}

func (t *Terminal) GetMouseMode() MouseMode {
	return t.mouseMode
}

func (t *Terminal) GetMouseExtMode() MouseExtMode {
	return t.mouseExtMode
}

func (t *Terminal) GetActiveBuffer() *Buffer {
	return t.activeBuffer
}

// Height returns the height of the underlying terminal buffer in lines.
func (t *Terminal) Height() int {
	return t.GetActiveBuffer().Height()
}

// MaxWidth returns the maximum width of the underlying terminal buffer in columns.
func (t *Terminal) MaxWidth() int {
	return t.GetActiveBuffer().MaxWidth()
}

func (t *Terminal) useMainBuffer() {
	t.switchBuffer(MainBuffer)
}

func (t *Terminal) useAltBuffer() {
	t.switchBuffer(AltBuffer)
}

// Lock acquires a mutex to this terminal's state.
func (t *Terminal) Lock() {
	t.mu.Lock()
}

// Unlock releases a mutex to this terminal's state.
func (t *Terminal) Unlock() {
	t.mu.Unlock()
}

// IsComplete returnes whether this terminal has stop processing
// data from the pty file.
func (t *Terminal) IsComplete() bool {
	return t.complete
}

// Close assumes lock has been acquired by caller
func (t *Terminal) Close() (ret error) {
	defer t.cancelCtx()

	if t.closed {
		return nil
	}
	t.closed = true
	if err := t.pty.Slave.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := t.pty.Master.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}
