package editor

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/cell"
	"github.com/ernestrc/fractal/component"
	"github.com/ernestrc/fractal/handler"
	"github.com/ernestrc/fractal/term"
)

var ErrLastWindow = errors.New("Cannot close last window")
var ErrLastBuffer = errors.New("No free buffers left")

type mode int8

const (
	proxyMode mode = iota
	commandMode
)

type openFileFunc func(filePath string,
	buf *cell.Buffer, swapDir string) (*FileBuffer, error)

type recoverFileFunc func(filePath,
	swapFilePath string, buf *cell.Buffer) (*FileBuffer, error)

// NOTE: for now this behaves like vi's ex-command. Once we
// have a better understanding on how we can abstract the "command" we should
// refactor it: (i.e. we should force Editor to provide a command Handler?)
type editorHandler struct {
	openFileFn    openFileFunc
	recoverFileFn recoverFileFunc

	ed     Editor
	config editorConfig

	commandBuf    *cell.Buffer
	commandSpan   *component.Span
	logBuf        *cell.Buffer
	logSpan       *component.Span
	wm            *handler.WindowManager
	buffers       []*editorBuffer
	mode          mode
	width, height int
}

func (e *editorHandler) emptyBuffer() *editorBuffer {
	buf := cell.NewBuffer()
	editor := e.ed.Edit(buf)
	handler := editorBuffer{
		filename: "",
		editor:   editor,
		node:     nil,
		fileBuf:  nil,
	}
	return &handler
}

func newLogSpan(buf *cell.Buffer, bgAttr term.Attributes) *component.Span {
	scroll := component.NewScroll()
	scroll.InitWithBuffer(buf)
	scroll.Attributes = bgAttr
	// FIXME panics scroll.Wrap = true

	background := term.Cell{Bg: bgAttr.Bg, Fg: bgAttr.Fg}
	content := component.WithBackground(scroll, background)

	span := component.NewSpan(content)
	span.ContentAlignment = component.SpanAlignmentBottom
	span.Padding.Vertical = -1

	// add margin bottom
	span = component.NewSpan(span)
	span.ContentAlignment = component.SpanAlignmentTop |
		component.SpanAlignmentHorizontallyCentered
	span.Padding.Vertical = 1
	span.Padding.Horizontal = 2

	return span
}

func newOsEditor() *editorHandler {
	ret := new(editorHandler)
	ret.openFileFn = NewFileBuffer
	ret.recoverFileFn = RecoverFileBuffer
	return ret
}

// New returns a Handler based on the given
func New(ed Editor, opts ...Option) (h fractal.Handler, err error) {
	e := newOsEditor()
	err = e.Init(ed, opts...)
	if err != nil {
		return
	}
	h = e
	return
}

func (e *editorHandler) Init(ed Editor, opts ...Option) (err error) {
	e.config = defaultEditorConfig

	for _, o := range opts {
		o(&e.config)
	}

	e.ed = ed
	e.mode = proxyMode

	e.commandBuf = cell.NewBuffer()
	e.commandSpan = newLogSpan(e.commandBuf,
		term.Attributes{Bg: term.ColorWhite, Fg: term.ColorBlack})

	e.logBuf = cell.NewBuffer()
	e.logSpan = newLogSpan(e.logBuf,
		term.Attributes{Bg: term.ColorRed, Fg: term.ColorWhite})

	var initBuffer *editorBuffer
	if e.config.Filepath != "" {
		initBuffer, err = e.newBufferWithFile(e.config.Filepath, e.config.RecoveryFilepath)
		if err != nil {
			return
		}
	} else {
		initBuffer = e.emptyBuffer()
		e.buffers = append(e.buffers, initBuffer)
	}
	e.wm = handler.NewWindowManager(initBuffer, e.config.WindowBorder)
	initBuffer.setNode(e.wm.Focus())

	return
}

func (e *editorHandler) newBuffer() *cell.Buffer {
	buf := cell.NewBuffer()
	buf.InitWithTabspaces(e.config.Tabspaces)
	if e.config.Logger != nil {
		buf = buf.WithLogger(e.config.Logger)
	}
	return buf
}

func (e *editorHandler) newFileBuffer(filename, recSwapFile string, buf *cell.Buffer) (
	fileBuf *FileBuffer, err error,
) {
	if recSwapFile != "" {
		fileBuf, err = e.recoverFileFn(filename, recSwapFile, buf)
	} else {
		fileBuf, err = e.openFileFn(filename, buf, e.config.SwapDir)
	}
	return
}

func (e *editorHandler) newBufferWithFile(
	filename, recoveryFilename string,
) (*editorBuffer, error) {
	buf := e.newBuffer()
	fileBuf, err := e.newFileBuffer(filename, recoveryFilename, buf)
	if err != nil {
		return nil, err
	}

	editor := e.ed.Edit(buf)

	buffer := &editorBuffer{
		filename: filename,
		fileBuf:  fileBuf,
		editor:   editor,
	}

	e.buffers = append(e.buffers, buffer)

	return buffer, nil
}

func (e *editorHandler) findBufferIdx(buf *editorBuffer) int {
	idx := -1
	for i, f := range e.buffers {
		if f == buf {
			idx = i
			break
		}
	}
	if idx == -1 {
		panic("corrupted list of file buffers: could not find file")
	}
	return idx
}

func (e *editorHandler) switchPrevBuffer() {
	idx := e.findBufferIdx(e.focus())
	if idx == 0 {
		idx = len(e.buffers) - 1
	} else {
		idx--
	}
	e.switchFocusContent(e.buffers[idx])
}

func (e *editorHandler) switchNextBuffer() {
	idx := e.findBufferIdx(e.focus())
	idx++
	if idx == len(e.buffers) {
		idx = 0
	}
	e.switchFocusContent(e.buffers[idx])
}

func (e *editorHandler) switchFocusContent(newBuf *editorBuffer) *editorBuffer {
	oldBufIfc := e.wm.SetFocusContent(newBuf)
	oldBuf := oldBufIfc.(*editorBuffer)
	newBuf.setNode(oldBuf.setNode(nil))
	return oldBuf
}

func (e *editorHandler) focusNewBufferWithFile(filename string) error {
	buffer, err := e.newBufferWithFile(filename, "")
	if err != nil {
		return err
	}

	oldFocus := e.switchFocusContent(buffer)
	if oldFocus.filename == "" && oldFocus.fileBuf == nil {
		e.removeBuffer(oldFocus)
	}

	return nil
}

func (e *editorHandler) focus() *editorBuffer {
	return e.wm.FocusContent().(*editorBuffer)
}

func (e *editorHandler) closeWindow() error {
	focus := e.focus()
	ok := e.wm.ShiftFocus()
	if !ok {
		return ErrLastWindow
	}
	focus.node.Close()

	return nil
}

func (e *editorHandler) freeBuffers() []int {
	freeBufs := make([]int, 0)
	for i, b := range e.buffers {
		if b.node == nil {
			freeBufs = append(freeBufs, i)
		}
	}
	return freeBufs
}

func (e *editorHandler) removeBuffer(buf *editorBuffer) {
	if buf.node != nil {
		panic("trying to remove buffer that is still attached to a window")
	}
	if buf.fileBuf != nil {
		buf.Close()
	}
	idx := e.findBufferIdx(buf)
	e.buffers = append(e.buffers[:idx], e.buffers[idx+1:]...)
}

func (e *editorHandler) closeFocusBuffer() error {
	freeBufs := e.freeBuffers()
	if len(freeBufs) == 0 {
		return ErrLastBuffer
	}

	oldBuf := e.focus()
	newBuf := e.buffers[freeBufs[0]]
	e.switchFocusContent(newBuf)

	oldBuf.Close()
	e.removeBuffer(oldBuf)

	return nil
}

func (e *editorHandler) runSingleCommand(cmd string) (quit bool, err error) {
	switch cmd {
	case "wq", "wq!":
		quit = true
		fallthrough
	case "close":
		err = e.closeWindow()
		if err != ErrLastWindow {
			return
		}
		err = e.closeFocusBuffer()
	case "w", "w!":
		if e.focus().fileBuf == nil {
			err = fmt.Errorf("Cannot save this buffer")
		} else {
			err = e.focus().fileBuf.Flush()
		}
	case "q!", "q":
		quit = true
	default:
		err = fmt.Errorf("Unknown command: %s", cmd)
	}
	return
}

func (e *editorHandler) runCommand() (quit bool, err error) {
	cmd := e.commandBuf.String()
	cmds := strings.Split(cmd, " ")
	if len(cmds) == 1 {
		return e.runSingleCommand(cmds[0])
	}

	switch cmds[0] {
	case "e":
		err = e.focusNewBufferWithFile(cmds[1])
	}
	return
}

func (e *editorHandler) setProxyMode() {
	e.commandBuf.Reset()
	e.mode = proxyMode
}

func (e *editorHandler) setCommandMode() {
	e.mode = commandMode
}

func (e *editorHandler) setMessage(msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	if e.config.Logger != nil {
		e.config.Logger.Infof("Message: %s", msg)
	}
	e.logBuf.WriteString(msg)
}

func (e *editorHandler) setError(err error) {
	e.setMessage("Error: %s", err)
}

func (e *editorHandler) handleCommand(ev term.Event) (quit bool) {
	switch ev.Key {
	case term.KeyEnter:
		var err error
		quit, err = e.runCommand()
		e.setProxyMode()
		if err != nil {
			e.setError(err)
		}
	case term.KeyEsc:
		e.setProxyMode()
	case term.KeyBackspace, term.KeyBackspace2:
		cols := e.commandBuf.Columns(0)
		if cols == 0 {
			e.setProxyMode()
			return
		}
		e.commandBuf.DeleteCell(term.Coordinates{X: cols - 1})
	case term.KeySpace:
		ev.Ch = ' '
		fallthrough
	default:
		if ev.Type == term.EventKey && ev.Ch != 0 {
			e.commandBuf.WriteString(string(ev.Ch))
		}
	}
	return
}

func (e *editorHandler) handleProxy(ev term.Event) bool {
	switch ev.Key {
	case term.KeyCtrlL:
		e.switchNextBuffer()
		return false
	case term.KeyCtrlH:
		e.switchPrevBuffer()
		return false
	}
	switch ev.Ch {
	case ':':
		e.setCommandMode()
		return false
	default:
		return e.wm.Handle(ev)
	}
}

func (e *editorHandler) Handle(ev term.Event) bool {
	switch e.mode {
	case proxyMode:
		return e.handleProxy(ev)
	case commandMode:
		return e.handleCommand(ev)
	default:
		panic(fmt.Sprintf("unknown mode: %+v", e.mode))
	}
}

func (e *editorHandler) Cursor() (pos term.Coordinates, show bool) {
	if e.mode == commandMode {
		pos := e.commandSpan.ContentOffset()
		pos.X += len(e.commandBuf.String())
		return pos, true
	}
	return e.wm.Cursor()
}

func (e *editorHandler) Man() fractal.Manual {
	panic("TODO")
}

func (e *editorHandler) Resize(width, height int) {
	e.width, e.height = width, height
	e.wm.Resize(width, height)
	e.commandSpan.Resize(width, height)
	e.logSpan.Resize(width, height)
}

func (e *editorHandler) Draw(w fractal.Writer) {
	e.wm.Draw(w)

	if e.logBuf.Columns(0) != 0 {
		e.logSpan.Draw(w)

		// only draw once
		e.logBuf.Reset()
	} else if e.mode == commandMode {
		e.commandSpan.Draw(w)
	}
}

// Close closes the resources associated with this editor.
func (e *editorHandler) Close() error {
	for _, f := range e.buffers {
		f.Close()
	}
	e.buffers = e.buffers[:0]
	return nil
}
