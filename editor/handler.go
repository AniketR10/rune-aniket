package editor

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
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

var (
	focusFileAttr    = term.Attributes{Fg: term.ColorDefault}
	nonFocusFileAttr = term.Attributes{Fg: 243}
	scrollAttr       = term.Attributes{Fg: term.ColorWhite}
	frameFileAttr    = term.Attributes{Fg: 243}
	commandBarAttr   = term.Attributes{Bg: term.ColorWhite, Fg: term.ColorBlack}
	logBarAttr       = term.Attributes{Bg: term.ColorRed, Fg: term.ColorWhite}
	wmFocusAttr      = frameFileAttr
	wmDefaultAttr    = frameFileAttr
)

// NOTE: for now this behaves like vi's ex-command. Once we
// have a better understanding on how we can abstract the "command" we should
// refactor it: (i.e. we should force Editor to provide a command Handler?)
type editorHandler struct {
	openFileFn    openFileFunc
	recoverFileFn recoverFileFunc

	ed     Editor
	config editorConfig

	tabs *handler.Tabs

	commandBuf *cell.Buffer
	cmdVirt    handler.Virtual
	logBuf     *cell.Buffer
	logVirt    handler.Virtual
	wm         *handler.WindowManager
	wmVirt     handler.Virtual

	buffers        []*editorBuffer
	mode           mode
	fileListHeight int
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

func newOsEditor() *editorHandler {
	ret := new(editorHandler)
	ret.openFileFn = NewFileBuffer
	ret.recoverFileFn = RecoverFileBuffer
	return ret
}

// New returns a Handler based on the given
func New(ed Editor, opts ...Option) (h tui.Handler, err error) {
	e := newOsEditor()
	err = e.Init(ed, opts...)
	if err != nil {
		return
	}
	h = e
	return
}

func newLogSpan(buf *cell.Buffer, bgAttr term.Attributes) handler.Virtual {
	scroll := component.NewScroll()
	scroll.InitWithBuffer(buf)
	scroll.Attributes = bgAttr
	// FIXME panics scroll.Wrap = true

	background := term.Cell{Bg: bgAttr.Bg, Fg: bgAttr.Fg}
	content := component.WithBackground(scroll, background)
	return handler.Virtual{Virtual: component.Virtual{C: content}}
}

func (e *editorHandler) addBuffer(buf *editorBuffer) {
	e.buffers = append(e.buffers, buf)

	e.tabs.Add(buf.filename)
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

	e.tabs.Remove(idx)
}

func (e *editorHandler) Init(ed Editor, opts ...Option) (err error) {
	e.config = defaultEditorConfig

	for _, o := range opts {
		o(&e.config)
	}

	e.ed = ed
	e.mode = proxyMode

	e.commandBuf = cell.NewBuffer()
	e.cmdVirt = newLogSpan(e.commandBuf, commandBarAttr)

	e.logBuf = cell.NewBuffer()
	e.logVirt = newLogSpan(e.logBuf, logBarAttr)

	e.tabs = handler.NewTabs()
	e.tabs.OnClick = func(idx int) {
		e.switchFocusContent(idx, e.buffers[idx])
	}
	e.tabs.SetAttr(focusFileAttr, nonFocusFileAttr, frameFileAttr, scrollAttr)

	var initBuffer *editorBuffer
	if e.config.Filepath != "" {
		initBuffer, err = e.newBufferWithFile(e.config.Filepath, e.config.RecoveryFilepath)
		if err != nil {
			return
		}
	} else {
		initBuffer = e.emptyBuffer()
		e.addBuffer(initBuffer)
	}
	e.wm = handler.NewWindowManager(initBuffer, e.config.WindowBorder)
	e.wm.SetAttr(wmDefaultAttr, wmFocusAttr)
	initBuffer.setNode(e.wm.Focus())

	e.wmVirt = handler.Virtual{Virtual: component.Virtual{C: e.wm}}

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
		filename: filepath.Base(filename),
		fileBuf:  fileBuf,
		editor:   editor,
	}

	e.addBuffer(buffer)

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
	e.switchFocusContent(idx, e.buffers[idx])
}

func (e *editorHandler) switchNextBuffer() {
	idx := e.findBufferIdx(e.focus())
	idx++
	if idx == len(e.buffers) {
		idx = 0
	}
	e.switchFocusContent(idx, e.buffers[idx])
}

func (e *editorHandler) switchFocusContent(idx int, newBuf *editorBuffer) *editorBuffer {
	oldBufIfc := e.wm.SetFocusContent(newBuf)
	oldBuf := oldBufIfc.(*editorBuffer)
	newBuf.setNode(oldBuf.setNode(nil))

	e.tabs.SetFocus(idx)
	return oldBuf
}

func (e *editorHandler) focusNewBufferWithFile(filename string) error {
	buffer, err := e.newBufferWithFile(filename, "")
	if err != nil {
		return err
	}

	oldFocus := e.switchFocusContent(len(e.buffers)-1, buffer)
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

func (e *editorHandler) closeFocusBuffer() error {
	freeBufs := e.freeBuffers()
	if len(freeBufs) == 0 {
		return ErrLastBuffer
	}

	oldBuf := e.focus()

	idx := freeBufs[0]
	e.switchFocusContent(idx, e.buffers[idx])

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
	}

	if ev.Type == term.EventMouse && ev.MouseY < e.fileListHeight {
		return e.tabs.Handle(ev)
	}

	return e.wmVirt.Handle(ev)
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
		pos := e.cmdVirt.Position()
		pos.X += len(e.commandBuf.String())
		return pos, true
	}
	return e.wmVirt.Cursor()
}

func (e *editorHandler) Man() tui.Manual {
	panic("TODO")
}

func (e *editorHandler) Resize(width, height int) {
	if width > 1 && height > 0 {
		e.cmdVirt.Resize(width-2, 1)
		e.logVirt.Resize(width-2, 1)

		busPos := term.Coordinates{X: 1, Y: height - 2}
		e.cmdVirt.Move(busPos)
		e.logVirt.Move(busPos)
	} else {
		e.cmdVirt.Resize(0, 0)
		e.logVirt.Resize(0, 0)
	}

	e.fileListHeight = 3
	wmHeight := height - e.fileListHeight + 1
	wmY := height - wmHeight
	if wmHeight < 0 {
		wmHeight = height
		e.fileListHeight = 0
		wmY = 0
	}
	e.tabs.Resize(width, e.fileListHeight)
	e.wmVirt.Resize(width, wmHeight)
	e.wmVirt.Move(term.Coordinates{Y: wmY})
}

func (e *editorHandler) Draw(w tui.Writer) {
	e.wmVirt.Draw(w)

	if e.logBuf.Columns(0) != 0 {
		e.logVirt.Draw(w)

		// only draw once
		e.logBuf.Reset()
	} else if e.mode == commandMode {
		e.cmdVirt.Draw(w)
	}

	e.tabs.Draw(w)
}

// Close closes the resources associated with this editor.
func (e *editorHandler) Close() error {
	for _, f := range e.buffers {
		f.Close()
	}
	e.buffers = e.buffers[:0]
	return nil
}
