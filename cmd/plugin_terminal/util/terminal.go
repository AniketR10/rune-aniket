package termutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

const (
	MainBuffer     uint8 = 0
	AltBuffer      uint8 = 1
	InternalBuffer uint8 = 2
)

// Terminal communicates with the underlying terminal
type Terminal struct {
	mu                sync.Mutex
	windowManipulator WindowManipulator
	pty               *os.File
	tty               *os.File
	reader            *bufio.Reader
	updateChan        chan struct{}
	closeChan         chan struct{}
	buffers           []*Buffer
	activeBuffer      *Buffer
	mouseMode         MouseMode
	mouseExtMode      MouseExtMode
	logFile           *os.File
	theme             *Theme
	closed            bool
	shell             string
	initialCommand    string
	stdinFd           int
	stdinOldState     *term.State
}

// NewTerminal creates a new terminal instance
func New(options ...Option) *Terminal {
	term := &Terminal{
		closeChan: make(chan struct{}),
		theme:     &Theme{},
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

	os.Setenv("TERM", "xterm-256color")

	return term
}

func (t *Terminal) start(cmd *exec.Cmd) (ret error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid = true
	cmd.SysProcAttr.Setctty = true

	t.pty, t.tty, ret = pty.Open()
	if ret != nil {
		return ret
	}

	if cmd.Stdout == nil {
		cmd.Stdout = t.tty
	}
	if cmd.Stderr == nil {
		cmd.Stderr = t.tty
	}
	if cmd.Stdin == nil {
		cmd.Stdin = t.tty
	}

	if err := cmd.Start(); err != nil {
		ret = multierr.Append(ret, err)
		if err := t.pty.Close(); err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	if err := t.tty.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	return ret
}

func (t *Terminal) CreatePty() (*exec.Cmd, error) {
	if t.shell == "" {
		t.shell = os.Getenv("SHELL")
		if t.shell == "" {
			t.shell = "/bin/sh"
		}
	}

	// Create arbitrary command.
	c := exec.Command(t.shell)

	// Start the command with a pty.
	err := t.start(c)
	if err != nil {
		return nil, err
	}

	// Set stdin in raw mode.
	if fd := int(os.Stdin.Fd()); term.IsTerminal(fd) {
		oldState, err := term.MakeRaw(fd)
		if err != nil {
			t.windowManipulator.ReportError(err)
		}
		t.stdinFd = fd
		t.stdinOldState = oldState
	}

	if t.initialCommand != "" {
		if err := t.WriteToPty([]byte(t.initialCommand)); err != nil {
			return nil, err
		}
	}

	return c, nil
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
func (t *Terminal) Pty() *os.File {
	return t.pty
}

// Tty exposes the underlying terminal tty, if it exists
func (t *Terminal) Tty() *os.File {
	return t.tty
}

func (t *Terminal) WriteToPty(data []byte) error {
	_, err := t.pty.Write(data)
	return err
}

func (t *Terminal) GetTitle() string {
	return t.windowManipulator.GetTitle()
}

func (t *Terminal) Theme() *Theme {
	return t.theme
}

func (t *Terminal) SetSize(rows, cols uint16) error {
	if t.pty == nil {
		return fmt.Errorf("terminal is not running")
	}

	t.log("Terminal.SetSize: %d, %d\n", cols, rows)

	t.activeBuffer.resizeView(cols, rows)

	if err := pty.Setsize(t.pty, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	}); err != nil {
		return err
	}

	return nil
}

// Run starts the terminal/shell proxying process
func (t *Terminal) Run(updateChan chan struct{}) error {
	t.mu.Lock()
	t.updateChan = updateChan
	t.reader = bufio.NewReaderSize(t.pty, 1024*1024)
	t.mu.Unlock()

	defer func() { _ = term.Restore(t.stdinFd, t.stdinOldState) }() // Best effort.

	for {
		r, size, err := t.reader.ReadRune()
		if err == io.EOF {
			break
		}
		render, exit := t.processSequence(MeasuredRune{Rune: r, Width: size})
		if exit {
			break
		}
		if render {
			t.requestRender()
		}
	}
	close(t.closeChan)
	return nil
}

func (t *Terminal) requestRender() {
	select {
	case <-t.closeChan:
		close(t.updateChan)
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

func (t *Terminal) useMainBuffer() {
	t.switchBuffer(MainBuffer)
}

func (t *Terminal) useAltBuffer() {
	t.switchBuffer(AltBuffer)
}

func (t *Terminal) Lock() {
	t.mu.Lock()
}

func (t *Terminal) Unlock() {
	t.mu.Unlock()
}

// assumes lock has been acquired by caller
func (t *Terminal) Close() error {
	t.closed = true
	return t.pty.Close()
}
