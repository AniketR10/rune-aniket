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
var ErrInvalidSave = errors.New("Cannot save this buffer")

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
	keymap map[term.Event]term.Event

	commandBuf *cell.Buffer
	cmdVirt    handler.Virtual
	logBuf     *cell.Buffer
	logVirt    handler.Virtual

	tabs     *handler.Tabs
	tabsVirt handler.Virtual
	wm       *handler.WindowManager
	wmVirt   handler.Virtual
	frames   *component.FrameUnion

	buffers        []*editorBuffer
	mode           mode
	fileListHeight int
}

func (e *editorHandler) doEdit(buf *cell.Buffer) tui.Handler {
	editor := e.ed.Edit(buf)
	if e.keymap != nil {
		editor = handler.WithMapping(editor, e.keymap)
	}
	return editor
}

func (e *editorHandler) emptyBuffer() *editorBuffer {
	buf := cell.NewBuffer()
	editor := e.doEdit(buf)
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
		e.updateNodeBuffer(e.wm.Focus(), idx, e.buffers[idx])
	}
	e.tabs.SetAttr(focusFileAttr, nonFocusFileAttr, frameFileAttr, scrollAttr)

	var initBuffer *editorBuffer
	if e.config.Filepath != "" {
		initBuffer, err = e.newBufferWithFile(e.config.Filepath,
			e.config.RecoveryFilepath)
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
	e.tabsVirt = handler.Virtual{Virtual: component.Virtual{C: e.tabs}}
	e.frames = component.NewFrameUnion(&e.tabsVirt.Virtual, &e.wmVirt.Virtual)
	e.frames.MiddleLeft.Bg = frameFileAttr.Bg
	e.frames.MiddleLeft.Fg = frameFileAttr.Fg
	e.frames.MiddleRight.Bg = frameFileAttr.Bg
	e.frames.MiddleRight.Fg = frameFileAttr.Fg

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

	editor := e.doEdit(buf)

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

func (e *editorHandler) switchBuffer(node *component.TileNode) (*editorBuffer, int) {
	buf, ok := e.nodeBuffer(node)
	if !ok {
		freeBufs := e.freeBuffers()
		if len(freeBufs) != 0 {
			e.updateNodeContent(node, e.buffers[freeBufs[0]])
		}
		return nil, 0
	}
	return buf, e.findBufferIdx(buf)
}

func (e *editorHandler) switchPrevBuffer(node *component.TileNode) {
	buf, idx := e.switchBuffer(node)
	if buf == nil {
		return
	}
	if idx == 0 {
		idx = len(e.buffers) - 1
	} else {
		idx--
	}
	e.updateNodeBuffer(node, idx, e.buffers[idx])
}

func (e *editorHandler) switchNextBuffer(node *component.TileNode) {
	buf, idx := e.switchBuffer(node)
	if buf == nil {
		return
	}
	idx++
	if idx == len(e.buffers) {
		idx = 0
	}
	e.updateNodeBuffer(node, idx, e.buffers[idx])
}

func (e *editorHandler) updateNodeContent(
	node *component.TileNode, content tui.Handler,
) tui.Handler {
	oldHandler := e.wm.SetContent(node, content)
	if oldBuf, ok := oldHandler.(*editorBuffer); ok {
		oldBuf.setNode(nil)
	}
	return oldHandler
}

func (e *editorHandler) updateNodeBuffer(
	node *component.TileNode, idx int, newBuf *editorBuffer,
) tui.Handler {
	oldHandler := e.updateNodeContent(node, newBuf)
	newBuf.setNode(node)
	e.tabs.SetFocus(idx)
	return oldHandler
}

func (e *editorHandler) nodeBuffer(node *component.TileNode) (*editorBuffer, bool) {
	buf, ok := e.wm.FocusContent().(*editorBuffer)
	return buf, ok
}

func (e *editorHandler) closeNode(node *component.TileNode) error {
	ok := e.wm.ShiftFocus()
	if !ok {
		return ErrLastWindow
	}
	node.Close()

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

func (e *editorHandler) closeNodeBuffer(node *component.TileNode) error {
	freeBufs := e.freeBuffers()
	if len(freeBufs) == 0 {
		return ErrLastBuffer
	}

	idx := freeBufs[0]
	oldHandler := e.updateNodeBuffer(node, idx, e.buffers[idx])
	oldBuf, ok := oldHandler.(*editorBuffer)
	if ok {
		oldBuf.Close()
		e.removeBuffer(oldBuf)
	}
	return nil
}

func (e *editorHandler) saveFocusBuffer() error {
	buf, ok := e.nodeBuffer(e.wm.Focus())
	if !ok {
		return ErrInvalidSave
	}
	if buf.fileBuf == nil {
		return ErrInvalidSave
	}

	return buf.fileBuf.Flush()
}

func (e *editorHandler) runSingleCommand(cmd string) (quit bool, err error) {
	switch cmd {
	case "bclose":
		err = e.closeNodeBuffer(e.wm.Focus())
	case "bcloseAll":
		e.closeAllBuffers()
	case "close":
		err = e.closeNode(e.wm.Focus())
	case "wq", "wq!":
		quit = true
		fallthrough
	case "w", "w!":
		err = e.saveFocusBuffer()
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
		err = e.OpenFile(cmds[1])
	default:
		err = fmt.Errorf("Unknown command: %s", cmd)
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

func (e *editorHandler) setError(err error) {
	e.SetMessage("Error: %s", err)
}

func (e *editorHandler) handleCommand(ev term.Event) (quit, handled bool) {
	handled = true

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
		} else {
			handled = false
		}
	}
	return
}

func (e *editorHandler) closeAllBuffers() {
	for {
		err := e.closeNodeBuffer(e.wm.Focus())
		if err != nil {
			if err != ErrLastBuffer {
				e.setError(err)
			}
			break
		}
	}
}

func (e *editorHandler) handleProxy(ev term.Event) (bool, bool) {
	switch ev.Ch {
	case ':':
		e.setCommandMode()
		return false, true
	}

	switch ev.Key {
	case term.KeyCtrlA:
		e.closeAllBuffers()
	case term.KeyCtrlW:
		err := e.closeNodeBuffer(e.wm.Focus())
		if err != nil {
			e.setError(err)
		}
	case term.KeyCtrlL:
		e.switchNextBuffer(e.wm.Focus())
	case term.KeyCtrlH:
		e.switchPrevBuffer(e.wm.Focus())
	default:
		if ev.Type == term.EventMouse && ev.MouseY < e.fileListHeight {
			return e.tabs.Handle(ev)
		}

		return e.wmVirt.Handle(ev)
	}

	return false, true
}

func (e *editorHandler) Handle(ev term.Event) (bool, bool) {
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
	if height < 4 {
		e.fileListHeight = 0
	}
	e.tabsVirt.Resize(width, e.fileListHeight)
	e.frames.Resize(width, height)
}

func (e *editorHandler) Draw(w tui.Writer) {
	e.frames.Draw(w)

	if e.logBuf.Columns(0) != 0 {
		e.logVirt.Draw(w)

		// only draw once
		e.logBuf.Reset()
	} else if e.mode == commandMode {
		e.cmdVirt.Draw(w)
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

// Conform to plugin.Editor interface
func (e *editorHandler) OpenFile(filename string) error {
	buffer, err := e.newBufferWithFile(filename, "")
	if err != nil {
		return err
	}

	oldFocus := e.updateNodeBuffer(e.wm.Focus(), len(e.buffers)-1, buffer)

	// remove initial empty buffer
	if oldBuf, ok := oldFocus.(*editorBuffer); ok &&
		oldBuf.filename == "" && oldBuf.fileBuf == nil {
		e.removeBuffer(oldBuf)
	}

	return nil
}

func (e *editorHandler) SetMessage(msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	if e.config.Logger != nil {
		e.config.Logger.Infof("Message: %s", msg)
	}
	e.logBuf.WriteString(msg)
}

func (e *editorHandler) MergeKeyMap(keymap map[term.Event]term.Event) {
	if e.keymap == nil {
		e.keymap = make(map[term.Event]term.Event)
	}
	for k, v := range keymap {
		e.keymap[k] = v
	}
}

func (e *editorHandler) splitInverted(
	split func(*handler.WindowManager, tui.Handler) *component.TileNode, h tui.Handler,
) {
	nodeInFocus := e.wm.Focus()
	buf, ok := e.nodeBuffer(nodeInFocus)
	newNode := split(e.wm, e.wm.FocusContent())
	if ok {
		buf.setNode(newNode)
	}
	e.updateNodeContent(nodeInFocus, h)
}

func (e *editorHandler) SplitVerticalRight(h tui.Handler) {
	e.wm.SplitVertical(h)
}

func (e *editorHandler) SplitVerticalLeft(h tui.Handler) {
	e.splitInverted((*handler.WindowManager).SplitVertical, h)
}

func (e *editorHandler) SplitHorizontalBelow(h tui.Handler) {
	e.wm.SplitHorizontal(h)
}

func (e *editorHandler) SplitHorizontalAbove(h tui.Handler) {
	e.splitInverted((*handler.WindowManager).SplitHorizontal, h)
}
