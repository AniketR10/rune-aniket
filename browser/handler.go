package browser

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
)

// ErrLastWindow is returned when trying to close Browser's last window.
var ErrLastWindow = errors.New("Cannot close last window")

// ErrLastBuffer is returned when trying to delete the Browser's last buffer.
var ErrLastBuffer = errors.New("No free buffers left")

// ErrInvalidSave is returned when trying to save a buffer that it's not a file
// in the file system.
var ErrInvalidSave = errors.New("Cannot save this buffer")

const logBufDrawTimes = 2

type mode int8

const (
	proxyMode mode = iota
	commandMode
)

// used to abstract editor.FileBufer
type fileBuffer interface {
	Flush() error
	Close() error
}

type openFileFunc func(filePath string,
	buf *cell.Buffer, swapDir string) (fileBuffer, error)

type recoverFileFunc func(filePath,
	swapFilePath string, buf *cell.Buffer) (fileBuffer, error)

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

// Handler adds tab and window management to a editor.Editor.
type Handler struct {
	openFileFn    openFileFunc
	recoverFileFn recoverFileFunc

	ed     editor.Editor
	config editorConfig
	keymap map[term.Event]term.Event

	commandBuf *cell.Buffer
	cmdVirt    handler.Virtual
	logBuf     *cell.Buffer
	logBufDraw int
	logVirt    handler.Virtual

	tabs     *handler.Tabs
	tabsVirt handler.Virtual
	wm       *handler.WindowManager
	wmVirt   handler.Virtual
	frames   *component.FrameUnion

	buffers        []*browserBuffer
	mode           mode
	fileListHeight int
}

func (e *Handler) doEdit(buf *cell.Buffer) tui.Handler {
	editor := e.ed.Edit(buf)
	return editor
}

func (e *Handler) emptyBuffer() *browserBuffer {
	buf := cell.NewBuffer()
	editor := e.doEdit(buf)
	handler := browserBuffer{
		filename: "",
		handler:  editor,
		node:     nil,
		fileBuf:  nil,
	}
	return &handler
}

func newOsHandler() *Handler {
	ret := new(Handler)
	ret.openFileFn = func(filePath string,
		buf *cell.Buffer, swapDir string) (fileBuffer, error) {
		return editor.NewFileBuffer(filePath, buf, swapDir)
	}

	ret.recoverFileFn = func(filePath,
		swapFilePath string, buf *cell.Buffer) (fileBuffer, error) {
		return editor.RecoverFileBuffer(filePath, swapFilePath, buf)
	}
	return ret
}

// New allocates storage for a new Handler and initializes it.
func New(ed editor.Editor, opts ...Option) (e *Handler, err error) {
	e = newOsHandler()
	err = e.Init(ed, opts...)
	if err != nil {
		return
	}
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

func (e *Handler) addBuffer(buf *browserBuffer) {
	e.buffers = append(e.buffers, buf)
	e.tabs.Add(buf.filename)
}

func (e *Handler) removeBuffer(buf *browserBuffer) {
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

// Init initializes this Handler with the given editor and Options.
// It returns an error if an initial filepath was given through WithFilePath option
// and the file failed to be opened.
func (e *Handler) Init(ed editor.Editor, opts ...Option) (err error) {
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

	var initBuffer *browserBuffer
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

func (e *Handler) newBuffer() *cell.Buffer {
	buf := cell.NewBuffer()
	buf.InitWithTabspaces(e.config.Tabspaces)
	if e.config.Logger != nil {
		// NOTE: only enable when trying to debug low level buffer bugs
		// as it degrades performance quite a bit.
		// buf = buf.WithLogger(e.config.Logger)
	}
	return buf
}

func (e *Handler) newFileBuffer(filename, recSwapFile string, buf *cell.Buffer) (
	fileBuf fileBuffer, err error,
) {
	if recSwapFile != "" {
		fileBuf, err = e.recoverFileFn(filename, recSwapFile, buf)
	} else {
		fileBuf, err = e.openFileFn(filename, buf, e.config.SwapDir)
	}
	return
}

func (e *Handler) newBufferWithFile(
	filename, recoveryFilename string,
) (*browserBuffer, error) {
	buf := e.newBuffer()
	fileBuf, err := e.newFileBuffer(filename, recoveryFilename, buf)
	if err != nil {
		return nil, err
	}

	editor := e.doEdit(buf)

	buffer := &browserBuffer{
		filename: filepath.Base(filename),
		fileBuf:  fileBuf,
		handler:  editor,
	}

	e.addBuffer(buffer)

	return buffer, nil
}

func (e *Handler) findBufferIdx(buf *browserBuffer) int {
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

func (e *Handler) switchBuffer(node *component.TileNode) (*browserBuffer, int) {
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

func (e *Handler) switchPrevBuffer(node *component.TileNode) {
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

func (e *Handler) switchNextBuffer(node *component.TileNode) {
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

func (e *Handler) updateNodeContent(
	node *component.TileNode, content tui.Handler,
) tui.Handler {
	oldHandler := e.wm.SetContent(node, content)
	if oldBuf, ok := oldHandler.(*browserBuffer); ok {
		oldBuf.setNode(nil)
	}
	return oldHandler
}

func (e *Handler) updateNodeBuffer(
	node *component.TileNode, idx int, newBuf *browserBuffer,
) tui.Handler {
	oldHandler := e.updateNodeContent(node, newBuf)
	newBuf.setNode(node)
	e.tabs.SetFocus(idx)
	return oldHandler
}

func (e *Handler) nodeBuffer(node *component.TileNode) (*browserBuffer, bool) {
	buf, ok := e.wm.FocusContent().(*browserBuffer)
	return buf, ok
}

func (e *Handler) closeNode(node *component.TileNode) error {
	ok := e.wm.ShiftFocus()
	if !ok {
		return ErrLastWindow
	}
	node.Close()

	return nil
}

func (e *Handler) freeBuffers() []int {
	freeBufs := make([]int, 0)
	for i, b := range e.buffers {
		if b.node == nil {
			freeBufs = append(freeBufs, i)
		}
	}
	return freeBufs
}

func (e *Handler) closeNodeBuffer(node *component.TileNode) error {
	freeBufs := e.freeBuffers()
	if len(freeBufs) == 0 {
		return ErrLastBuffer
	}

	idx := freeBufs[0]
	oldHandler := e.updateNodeBuffer(node, idx, e.buffers[idx])
	oldBuf, ok := oldHandler.(*browserBuffer)
	if ok {
		oldBuf.Close()
		e.removeBuffer(oldBuf)
	}
	return nil
}

func (e *Handler) saveFocusBuffer() error {
	buf, ok := e.nodeBuffer(e.wm.Focus())
	if !ok {
		return ErrInvalidSave
	}
	if buf.fileBuf == nil {
		return ErrInvalidSave
	}

	return buf.fileBuf.Flush()
}

func (e *Handler) runSingleCommand(cmd string) (quit bool, err error) {
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

func (e *Handler) runCommand() (quit bool, err error) {
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

func (e *Handler) setProxyMode() {
	e.commandBuf.Reset()
	e.mode = proxyMode
}

func (e *Handler) setCommandMode() {
	e.mode = commandMode
}

func (e *Handler) setError(err error) {
	e.SetMessage("Error: %s", err)
}

func (e *Handler) handleCommand(ev term.Event) (quit, handled bool) {
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

func (e *Handler) closeAllBuffers() {
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

func (e *Handler) mapEvent(ev term.Event) term.Event {
	// map event if applicable
	mev, ok := e.keymap[ev]
	if !ok {
		mev = ev
	}
	return mev
}

func (e *Handler) handleProxy(ev term.Event) (bool, bool) {
	if ev == e.config.CommandEvent {
		e.setCommandMode()
		return false, true
	}

	prev := ev
	ev = e.mapEvent(ev)

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
		// do not map for children
		ev = prev
		if ev.Type == term.EventMouse && ev.MouseY < e.fileListHeight {
			return e.tabs.Handle(ev)
		}

		return e.wmVirt.Handle(ev)
	}

	return false, true
}

// Handle satisfies tui.Handler.
func (e *Handler) Handle(ev term.Event) (bool, bool) {
	switch e.mode {
	case proxyMode:
		return e.handleProxy(ev)
	case commandMode:
		return e.handleCommand(ev)
	default:
		panic(fmt.Sprintf("unknown mode: %+v", e.mode))
	}
}

// Cursor satisfies tui.Handler.
func (e *Handler) Cursor() (pos term.Coordinates, show bool) {
	if e.mode == commandMode {
		pos := e.cmdVirt.Position()
		pos.X += len(e.commandBuf.String())
		return pos, true
	}
	return e.wmVirt.Cursor()
}

// Man satisfies tui.Handler.
func (e *Handler) Man() tui.Manual {
	panic("TODO")
}

// Resize satisfies tui.Component
func (e *Handler) Resize(width, height int) {
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
	if height < 3 {
		e.fileListHeight = 0
	}
	e.tabsVirt.Resize(width, e.fileListHeight)
	e.frames.Resize(width, height)
}

// Draw satisfies tui.Component
func (e *Handler) Draw(w tui.Writer) {
	e.frames.Draw(w)

	// only draw logBufDraw times
	if e.logBufDraw > 0 {
		e.logVirt.Draw(w)
		e.logBufDraw--
	} else {
		e.logBuf.Reset()
	}

	if e.mode == commandMode {
		e.cmdVirt.Draw(w)
	}
}

// Close closes the resources associated with this browser.
func (e *Handler) Close() error {
	for _, f := range e.buffers {
		f.Close()
	}
	e.buffers = e.buffers[:0]
	return nil
}

// OpenFile opens the given file in a new browser tab.
func (e *Handler) OpenFile(filename string) error {
	buffer, err := e.newBufferWithFile(filename, "")
	if err != nil {
		return err
	}

	oldFocus := e.updateNodeBuffer(e.wm.Focus(), len(e.buffers)-1, buffer)

	// remove initial empty buffer
	if oldBuf, ok := oldFocus.(*browserBuffer); ok &&
		oldBuf.filename == "" && oldBuf.fileBuf == nil {
		e.removeBuffer(oldBuf)
	}

	return nil
}

// SetMessage formats the given msg and args and displays it on next Draw.
func (e *Handler) SetMessage(msg string, args ...interface{}) error {
	msg = fmt.Sprintf(msg, args...)
	if e.config.Logger != nil {
		e.config.Logger.Infof("Message: %s", msg)
	}
	e.logBuf.WriteString(msg)
	e.logBufDraw = logBufDrawTimes
	return nil
}

// MergeKeyMap takes the given keymap and merges it with the Browser's keymap
// to override the current event key mappings.
func (e *Handler) MergeKeyMap(keymap map[term.Event]term.Event) error {
	if e.keymap == nil {
		e.keymap = make(map[term.Event]term.Event)
	}
	for k, v := range keymap {
		e.keymap[k] = v
	}
	return nil
}

func (e *Handler) splitInverted(
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

// SplitVerticalRight opens a new window tile to the right of the
// current tile in focus and initializes it with h.
func (e *Handler) SplitVerticalRight(h tui.Handler) error {
	e.wm.SplitVertical(h)
	return nil
}

// SplitVerticalLeft opens a new window tile to the left of the
// current tile in focus and initializes it with h.
func (e *Handler) SplitVerticalLeft(h tui.Handler) error {
	e.splitInverted((*handler.WindowManager).SplitVertical, h)
	return nil
}

// SplitHorizontalBelow opens a new window tile below the current tile in focus
// and initializes it with h.
func (e *Handler) SplitHorizontalBelow(h tui.Handler) error {
	e.wm.SplitHorizontal(h)
	return nil
}

// SplitHorizontalAbove opens a new window tile above the current tile in focus
// and initializes it with h.
func (e *Handler) SplitHorizontalAbove(h tui.Handler) error {
	e.splitInverted((*handler.WindowManager).SplitHorizontal, h)
	return nil
}
