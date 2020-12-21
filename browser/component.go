package browser

import (
	"errors"
	"fmt"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
)

var (
	// ErrInvalidSave is returned when trying to save a buffer that it's not a file
	// in the file system.
	ErrInvalidSave = errors.New("Cannot save this buffer")
	// ErrNoFreeBuffers is returned when trying to delete the Browser's last buffer.
	ErrNoFreeBuffers = errors.New("No free buffers left")

	focusFileAttr    = term.Attributes{Fg: term.ColorDefault}
	nonFocusFileAttr = term.Attributes{Fg: 243}
	scrollAttr       = term.Attributes{Fg: term.ColorWhite}
	frameFileAttr    = term.Attributes{Fg: 243}
	logBarAttr       = term.Attributes{Bg: term.ColorRed, Fg: term.ColorWhite}
	wmFocusAttr      = frameFileAttr
	wmDefaultAttr    = frameFileAttr
	logBufDrawTimes  = 2
)

// Component renders a browser-like tui.Compontent and exposes an API
// to open new windows, add new buffers, and switch between buffers.
//
// All tui.Handlers installed other than via NewBuffer are considered ephemeral,
// and will be destroyed either when windows close or when they return exit=true
// to a call to Handle. Conversely, tui.Handlers installed via NewBuffer
// will remain as a tab and can be managed independently from windows.
type Component struct {
	logBuf     *cell.Buffer
	logBufDraw int
	logVirt    handler.Virtual
	tabs       *handler.Tabs
	tabsVirt   handler.Virtual
	wm         *handler.WindowManager
	wmVirt     handler.Virtual
	frames     *component.FrameUnion

	config         Config
	startHandler   browserWindowContent
	buffers        []*buffer
	windows        []browserWindow
	fileListHeight int
}

type browserWindow struct {
	parent *Component
	win    handler.Window
}

type browserWindowContent struct {
	win browserWindow
	tui.Handler
}

// browserWindow is passed by value, so we store whether
// it has been closed or not in Handler.
func (w browserWindow) Close() error {
	if w.parent == nil {
		return nil
	}

	err := w.parent.closeWindow(w)
	w.parent = nil

	return err
}

func (e browserWindowContent) Handle(ev term.Event) (exit, handled bool) {
	exit, handled = e.Handler.Handle(ev)
	if exit {
		// for completeness, although this should never happen
		if buf, ok := e.Handler.(*buffer); ok {
			e.win.parent.doRemoveBuffer(e.win.parent.findBufferID(buf))
		}
	}
	return
}

func (c *Component) addWindow(win browserWindow) {
	c.windows = append(c.windows, win)
}

func (c *Component) newWindow(win handler.Window) browserWindow {
	browserWin := browserWindow{
		parent: c,
		win:    win,
	}
	c.addWindow(browserWin)
	return browserWin
}

// closeWindow closes win or returns an error if win is the last Window.
func (c *Component) closeWindow(win Window) error {
	bWin := win.(browserWindow)
	id, _ := c.findWindow(bWin)
	if id == -1 {
		return nil
	}

	err := bWin.win.Close()
	if err != nil {
		return err
	}

	buf, ok := c.browserBufferAtWindow(win.(browserWindow))
	if ok {
		buf.setFree()
	}

	c.windows = append(c.windows[:id], c.windows[id+1:]...)

	return nil
}

func (c *Component) findWindow(win browserWindow) (int, browserWindow) {
	for i, w := range c.windows {
		if w == win {
			return i, w
		}
	}
	return -1, browserWindow{}
}

// NewComponent allocates storage for a new Component and initializes it.
func NewComponent(config Config) *Component {
	ret := new(Component)
	ret.Init(config)
	return ret
}

// Init initializes this Component with config.
func (c *Component) Init(config Config) {
	c.config = config

	c.logBuf = cell.NewBuffer()
	c.logVirt = NewMessageSpan(c.logBuf, logBarAttr)

	c.tabs = handler.NewTabs()
	c.tabs.OnClick = func(id int) {
		buf := c.buffers[id]
		if buf.free {
			c.updateWindowContent(c.focus(), buf)
		}
	}
	c.tabs.SetAttr(focusFileAttr, nonFocusFileAttr, frameFileAttr, scrollAttr)
	n := handler.Nop(component.String(c.config.StartText))
	c.startHandler = newBrowserWindowContent(n, browserWindow{})

	c.wm = handler.NewWindowManager(c.startHandler, c.config.WindowManagerConfig)
	c.wm.SetAttr(wmDefaultAttr, wmFocusAttr)

	win := c.newWindow(c.wm.Focus()) // init handler with initial window
	c.startHandler.win = win

	c.wmVirt = handler.Virtual{Virtual: component.Virtual{C: c.wm}}
	c.tabsVirt = handler.Virtual{Virtual: component.Virtual{C: c.tabs}}
	c.frames = component.NewFrameUnion(&c.tabsVirt.Virtual, &c.wmVirt.Virtual)
	c.frames.MiddleLeft.Bg = frameFileAttr.Bg
	c.frames.MiddleLeft.Fg = frameFileAttr.Fg
	c.frames.MiddleRight.Bg = frameFileAttr.Bg
	c.frames.MiddleRight.Fg = frameFileAttr.Fg

	return
}

func (c *Component) newBuffer(name string, h tui.Handler, f FlusherCloser) (*buffer, int) {
	b := newBuffer(name, h, f)
	c.buffers = append(c.buffers, b)
	return b, c.tabs.Add(b.name)
}

// NewBuffer adds a new buffer to this Component and sets it as the buffer
// of the current Window on focus except if the handler of the window on focus
// is an external handler (installed via Split methods). In that case, the
// buffer is added as the buffer of the main window.
func (c *Component) NewBuffer(name string, h tui.Handler, f FlusherCloser) tui.Handler {
	buf, _ := c.newBuffer(name, h, f)

	// TODO this is a terrible API. Callers should create new buffer to
	// add new buffer to tabs, and then SplitHorizontal, or Focus().SetContent
	// to use it, instead of this component running all this logic under the hood.
	// First we need to add SetContent method to browser.Window which requires
	// a refactor to how we handle rpc windows internally.

	// Do not allow file editing on windows controlled externally.
	// This also happens to be a workaround around
	// plugins exiting upon trying to open a file,
	// expecting that the plugin window is going to close
	// but not closing because OpenFile swaps the plugin
	// handler before the rpc Handler processes the exit
	// return from a HandleResponse (see handler/rpc_breaker.go).
	// focus := c.Focus()
	// focusWin := focus.(browserWindow)
	//if _, ok := focusWin.win.Content().(*buffer); ok {
	// _ = c.updateWindowContent(focusWin, buf)
	// } else {
	// 	// find another win to update the content
	// 	// NOTE: this is not a very robust approach.
	// 	// Shiftable could return a window controlled externally
	// 	// in certain scenarios.
	// TODO this is a hack to keep tests passing until we refactor API
	w, ok := c.Shiftable()
	if ok {
		c.updateWindowContent(w.(browserWindow), buf)
	} else {
		focus := c.Focus()
		focusWin := focus.(browserWindow)
		_ = c.updateWindowContent(focusWin, buf)
	}
	// }

	// remove initial empty buffer
	// if oldBuf, ok := oldFocus.(*buffer); ok &&
	// 	oldBuf.handler == c.startHandler {
	// 	c.doRemoveBuffer(0)
	// }

	return buf
}

func (c *Component) doRemoveBuffer(id int) {
	if id >= len(c.buffers) {
		panic(fmt.Sprintf("invalid buffer at index: %d", id))
	}

	buf := c.buffers[id]
	if !buf.free {
		panic("trying to remove buffer that is still attached to a window")
	}
	defer buf.Close()

	c.buffers = append(c.buffers[:id], c.buffers[id+1:]...)
	ok := c.tabs.Remove(id)
	if !ok {
		panic(fmt.Sprintf("corrupted tabs: could not find tab with id %v", id))
	}
}

func (c *Component) findBufferID(buf *buffer) int {
	for i, f := range c.buffers {
		if f == buf {
			return i
		}
	}
	panic("could not find buffer")
}

func (c *Component) browserBufferID(win browserWindow) (
	*buffer, int,
) {
	buf, ok := c.browserBufferAtWindow(win)
	if !ok {
		return nil, 0
	}
	return buf, c.findBufferID(buf)
}

func (c *Component) updateWindowBuffer(win browserWindow, bufferID int) bool {
	if bufferID >= len(c.buffers) {
		panic(fmt.Sprintf("invalid buffer at index: %d", bufferID))
	}
	buf := c.buffers[bufferID]
	if buf.free {
		c.updateWindowContent(win, buf)
		return true
	}
	return false
}

// UpdateWindowBufferNextFree updates win with the next available buffer.
func (c *Component) UpdateWindowBufferNextFree(win Window) bool {
	freeBufs := c.freeBuffers()
	if len(freeBufs) != 0 {
		c.updateWindowContent(win.(browserWindow), c.buffers[freeBufs[0]])
		return true
	}

	return false
}

// UpdateWindowBufferPrev updates win with the buffer before the current buffer.
func (c *Component) UpdateWindowBufferPrev(win Window) {
	bWin := win.(browserWindow)
	buf, id := c.browserBufferID(bWin)
	if buf == nil {
		c.UpdateWindowBufferNextFree(win)
		return
	}
	for i := 0; i < len(c.buffers); i++ {
		if id == 0 {
			id = len(c.buffers) - 1
		} else {
			id--
		}
		if c.updateWindowBuffer(bWin, id) {
			return
		}
	}
}

// UpdateWindowBufferNext updates win with the buffer after the current buffer.
func (c *Component) UpdateWindowBufferNext(win Window) {
	bWin := win.(browserWindow)
	buf, id := c.browserBufferID(bWin)
	if buf == nil {
		c.UpdateWindowBufferNextFree(win)
		return
	}
	for i := 0; i < len(c.buffers); i++ {
		id++
		if id == len(c.buffers) {
			id = 0
		}
		if c.updateWindowBuffer(bWin, id) {
			return
		}
	}
}

func (c *Component) updateWindowContent(
	win browserWindow, content tui.Handler,
) tui.Handler {
	newBuf, ok := content.(*buffer)
	if ok {
		id := c.findBufferID(newBuf)
		c.tabs.SetFocus(id)
		newBuf.setWindow(win)
	}
	oldComponent := win.win.SetContent(content)
	if oldBuf, ok := oldComponent.(*buffer); ok {
		oldBuf.setFree()
	}
	return oldComponent
}

func (c *Component) browserBufferAtWindow(win browserWindow) (*buffer, bool) {
	buf, ok := win.win.Content().(*buffer)
	return buf, ok
}

// RemoveAllBuffers removes all buffers but the last one.
func (c *Component) RemoveAllBuffers() {
	for {
		err := c.RemoveWindowBuffer(c.Focus())
		if err != nil {
			if err != ErrNoFreeBuffers {
				c.setError(err)
			}
			break
		}
	}
}

func (c *Component) freeBuffers() []int {
	freeBufs := make([]int, 0)
	for i, b := range c.buffers {
		if b.free {
			freeBufs = append(freeBufs, i)
		}
	}
	return freeBufs
}

// RemoveWindowBuffer removes the buffer at win or returns ErrNoFreeBuffers
// if buffer is the last one.
func (c *Component) RemoveWindowBuffer(win Window) error {
	freeBufs := c.freeBuffers()
	if len(freeBufs) == 0 {
		return ErrNoFreeBuffers
	}

	id := freeBufs[0]
	oldComponent := c.updateWindowContent(win.(browserWindow), c.buffers[id])
	oldBuf, ok := oldComponent.(*buffer)
	if ok {
		oldBuf.Close()
		c.doRemoveBuffer(c.findBufferID(oldBuf))
	}
	return nil
}

// FlushBuffer flushes the contents of the buffer at win, if this buffer
// was created with a FlusherCloser. See NewBuffer.
func (c *Component) FlushBuffer(win Window) error {
	buf, ok := c.browserBufferAtWindow(win.(browserWindow))
	if !ok {
		return ErrInvalidSave
	}
	if buf.flusherCloser == nil {
		return ErrInvalidSave
	}

	return buf.flusherCloser.Flush()
}

func (c *Component) splitInverted(
	split func(*handler.WindowManager, tui.Handler) handler.Window,
	h tui.Handler,
) browserWindow {
	bWin, focusContent := c.focus(), c.wm.Focus().Content()
	id, w := c.findWindow(bWin)
	if id == -1 {
		panic("Handler: corrupted window list")
	}

	newWindow := split(c.wm, focusContent)
	c.newWindow(newWindow)

	w.win = bWin.win
	bWin.win.SetContent(newBrowserWindowContent(h, w))
	return w
}

func newBrowserWindowContent(h tui.Handler, w browserWindow) browserWindowContent {
	return browserWindowContent{Handler: h, win: w}
}

func (c *Component) split(
	split func(*handler.WindowManager, tui.Handler) handler.Window,
	h tui.Handler,
) browserWindow {
	// force split a new window tile
	win := split(c.wm, h)

	browserWin := c.newWindow(win)
	// update content with browserWindowContent
	// so we can have a browserWindow with the correct win
	win.SetContent(newBrowserWindowContent(h, browserWin))

	c.wm.SetFocus(win)
	return browserWin
}

// SplitVerticalRight opens a new window tile to the right of the
// current window in focus and initializes it with h.
// Note that if h is not a handler created with NewBuffer
// the handler is cleaned as soon as the window's content is swapped.
func (c *Component) SplitVerticalRight(h tui.Handler) Window {
	return c.split((*handler.WindowManager).SplitVertical, h)
}

// SplitVerticalLeft opens a new window tile to the left of the
// current window in focus and initializes it with h.
// Note that if h is not a handler created with NewBuffer
// the handler is cleaned as soon as the window's content is swapped.
func (c *Component) SplitVerticalLeft(h tui.Handler) Window {
	return c.splitInverted((*handler.WindowManager).SplitVertical, h)
}

// SplitHorizontalBelow opens a new window tile below the current window in focus
// and initializes it with h. Note that if h is not a handler created with NewBuffer
// the handler is cleaned as soon as the window's content is swapped.
func (c *Component) SplitHorizontalBelow(h tui.Handler) Window {
	return c.split((*handler.WindowManager).SplitHorizontal, h)
}

// SplitHorizontalAbove opens a new window tile above the current window in focus
// and initializes it with h. Note that if h is not a handler created with NewBuffer
// the handler is cleaned as soon as the window's content is swapped.
func (c *Component) SplitHorizontalAbove(h tui.Handler) Window {
	return c.splitInverted((*handler.WindowManager).SplitHorizontal, h)
}

func (c *Component) setError(err error) {
	c.SetMessage("Error: %s", err)
}

// SetMessage formats the given msg and args and displays it on next Draw.
func (c *Component) SetMessage(msg string, args ...interface{}) {
	msg = fmt.Sprintf(msg, args...)
	if c.config.Logger != nil {
		c.config.Logger.Infof("Message: %s", msg)
	}
	c.logBuf.WriteString(msg)
	c.logBufDraw = logBufDrawTimes
}

// Resize satisfies tui.Component
func (c *Component) Resize(width, height int) {
	ResizeMessageSpan(&c.logVirt, width, height)

	c.fileListHeight = 3
	if height < 3 {
		c.fileListHeight = 0
	}
	c.tabsVirt.Resize(width, c.fileListHeight)
	c.frames.Resize(width, height)
}

// Draw satisfies tui.Component
func (c *Component) Draw(w term.Writer) {
	c.tabs.ResetFocus()
	for id, buf := range c.buffers {
		if !buf.free {
			c.tabs.SetFocus(id)
		}
	}
	c.frames.Draw(w)

	// only draw logBufDraw times
	if c.logBufDraw > 0 {
		c.logVirt.Draw(w)
		c.logBufDraw--
	} else {
		c.logBuf.Reset()
	}
}

// ShiftFocus calls the underlying WindowManager.ShiftFocus.
func (c *Component) ShiftFocus() bool {
	return c.wm.ShiftFocus()
}

// Focus calls the underlying WindowManager.Focus.
func (c *Component) Focus() Window {
	return c.focus()
}

func (c *Component) focus() browserWindow {
	return browserWindow{parent: c, win: c.wm.Focus()}
}

// Shiftable calls the underlying WindowManager.Shiftable.
func (c *Component) Shiftable() (Window, bool) {
	win, ok := c.wm.Shiftable()
	if !ok {
		return nil, false
	}

	return browserWindow{parent: c, win: win}, true
}

// FocusDown calls the underlying WindowManager.FocusDown.
func (c *Component) FocusDown() bool {
	return c.wm.FocusDown()
}

// FocusLeft calls the underlying WindowManager.FocusLeft.
func (c *Component) FocusLeft() bool {
	return c.wm.FocusLeft()
}

// FocusRight calls the underlying WindowManager.FocusRight.
func (c *Component) FocusRight() bool {
	return c.wm.FocusRight()
}

// FocusUp calls the underlying WindowManager.FocusUp.
func (c *Component) FocusUp() bool {
	return c.wm.FocusUp()
}

// Close closes the resources associated with this browser.
func (c *Component) Close() (ret error) {
	for _, f := range c.buffers {
		err := f.Close()
		if err != nil {
			ret = err
		}
	}
	c.buffers = c.buffers[:0]
	return ret
}

// Handle proxies events to either the underlying Tabs or WindowManager.
func (c *Component) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type == term.EventMouse && ev.MouseY < c.fileListHeight {
		_, handled = c.tabs.Handle(ev)
		return
	}

	_, handled = c.wmVirt.Handle(ev)
	return
}

// Cursor calls the underlying WindowManager.Cursor.
func (c *Component) Cursor() (pos term.Coordinates, show bool) {
	return c.wmVirt.Cursor()
}
