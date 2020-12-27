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
	// ErrBufferNotFree is returned when a buffer is being used in call to
	// SetContent but it's already owned by another Window.
	ErrBufferNotFree = errors.New("Buffer already rendered in another Window")

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
	startHandler   Handler
	buffers        []*buffer
	windows        []*browserWindow
	fileListHeight int
}

// needed mutable to inverse a split
type browserWindow struct {
	parent *Component
	win    handler.Window
}

func (w *browserWindow) SetContent(h Handler) error {
	if b, ok := h.(*buffer); ok {
		if !b.free {
			return ErrBufferNotFree
		}
	}
	w.parent.updateWindowContent(w, h)
	return nil
}

// browserWindow is passed by value, so we store whether
// it has been closed or not in Handler.
func (w *browserWindow) Close() error {
	if w.parent == nil {
		return nil
	}

	err := w.parent.closeWindow(w)
	w.parent = nil

	return err
}

func (c *Component) addWindow(win *browserWindow) {
	c.windows = append(c.windows, win)
}

func (c *Component) newWindow(win handler.Window) *browserWindow {
	browserWin := &browserWindow{
		parent: c,
		win:    win,
	}
	c.addWindow(browserWin)
	return browserWin
}

// closeWindow closes win or returns an error if win is the last Window.
func (c *Component) closeWindow(win Window) error {
	bWin := win.(*browserWindow)
	id, _ := c.findWindow(bWin.win)
	if id == -1 {
		return nil
	}

	err := bWin.win.Close()
	if err != nil {
		return err
	}

	c.windows = append(c.windows[:id], c.windows[id+1:]...)

	c.onUnmount(bWin.win.Content().(Handler))
	return nil
}

func (c *Component) findWindow(win handler.Window) (int, *browserWindow) {
	for i, w := range c.windows {
		if w.win == win {
			return i, w
		}
	}
	return -1, nil
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
		err := c.Focus().SetContent(buf)
		if err != nil {
			c.setError(err)
		}
	}
	c.tabs.SetAttr(focusFileAttr, nonFocusFileAttr, frameFileAttr, scrollAttr)
	c.startHandler = CallbackHandler(handler.Nop(component.String(c.config.StartText)), func() {})

	c.wm = handler.NewWindowManager(c.startHandler, c.config.WindowManagerConfig)
	c.wm.SetAttr(wmDefaultAttr, wmFocusAttr)

	_ = c.newWindow(c.wm.Focus()) // init handler with initial window

	c.wmVirt = handler.Virtual{Virtual: component.Virtual{C: c.wm}}
	c.tabsVirt = handler.Virtual{Virtual: component.Virtual{C: c.tabs}}
	c.frames = component.NewFrameUnion(&c.tabsVirt.Virtual, &c.wmVirt.Virtual)
	c.frames.MiddleLeft.Bg = frameFileAttr.Bg
	c.frames.MiddleLeft.Fg = frameFileAttr.Fg
	c.frames.MiddleRight.Bg = frameFileAttr.Bg
	c.frames.MiddleRight.Fg = frameFileAttr.Fg

	return
}

// NewBuffer adds a new buffer to the list of buffers on this Component.
func (c *Component) NewBuffer(name string, h tui.Handler, f FlusherCloser) Handler {
	b := newBuffer(c, name, h, f)
	c.buffers = append(c.buffers, b)
	c.tabs.Add(b.name)
	return b
}

func (c *Component) closeBuffer(buf *buffer) error {
	err := buf.Close()
	if err != nil && c.config.Logger != nil {
		c.config.Logger.Warningf("buffer Close error: %v", err)
	}
	return err
}

func (c *Component) doRemoveBuffer(buf *buffer) {
	id := c.findBufferID(buf)
	if !buf.free {
		panic("trying to remove buffer that is still attached to a window")
	}
	defer c.closeBuffer(buf)

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

func (c *Component) browserBufferID(win *browserWindow) (
	*buffer, int,
) {
	buf, ok := browserBufferAtWindow(win)
	if !ok {
		return nil, 0
	}
	return buf, c.findBufferID(buf)
}

func (c *Component) updateWindowBuffer(win *browserWindow, bufferID int) bool {
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
		c.updateWindowContent(win.(*browserWindow), c.buffers[freeBufs[0]])
		return true
	}

	return false
}

// UpdateWindowBufferLastFree updates win with the last available buffer.
func (c *Component) UpdateWindowBufferLastFree(win Window) bool {
	freeBufs := c.freeBuffers()
	if len(freeBufs) != 0 {
		c.updateWindowContent(win.(*browserWindow), c.buffers[freeBufs[len(freeBufs)-1]])
		return true
	}

	return false
}

// UpdateWindowBufferPrev updates win with the buffer before the current buffer.
func (c *Component) UpdateWindowBufferPrev(win Window) bool {
	bWin := win.(*browserWindow)
	buf, id := c.browserBufferID(bWin)
	if buf == nil {
		return c.UpdateWindowBufferNextFree(win)
	}
	for i := 0; i < len(c.buffers); i++ {
		if id == 0 {
			id = len(c.buffers) - 1
		} else {
			id--
		}
		if c.updateWindowBuffer(bWin, id) {
			return true
		}
	}
	return false
}

// UpdateWindowBufferNext updates win with the buffer after the current buffer.
func (c *Component) UpdateWindowBufferNext(win Window) bool {
	bWin := win.(*browserWindow)
	buf, id := c.browserBufferID(bWin)
	if buf == nil {
		return c.UpdateWindowBufferNextFree(win)
	}
	for i := 0; i < len(c.buffers); i++ {
		id++
		if id == len(c.buffers) {
			id = 0
		}
		if c.updateWindowBuffer(bWin, id) {
			return true
		}
	}
	return false
}

func (c *Component) onUnmount(h Handler) {
	err := h.OnUnmount()
	if err != nil && c.config.Logger != nil {
		c.config.Logger.Warningf("OnUnmount error: %v", err)
	}
}

// UpdateWindowContent updates the content of win to content.
func (c *Component) UpdateWindowContent(win Window, content Handler) error {
	if b, ok := content.(*buffer); ok {
		if !b.free {
			return ErrBufferNotFree
		}
	}
	_ = c.updateWindowContent(win.(*browserWindow), content)
	return nil
}

func (c *Component) updateWindowContent(
	win *browserWindow, content Handler,
) Handler {
	newBuf, ok := content.(*buffer)
	if ok {
		id := c.findBufferID(newBuf)
		c.tabs.SetFocus(id)
		newBuf.setWindow(win)
	}
	oldComponent := win.win.SetContent(content).(Handler)
	c.onUnmount(oldComponent)
	return oldComponent
}

func browserBufferAtWindow(win *browserWindow) (*buffer, bool) {
	buf, ok := win.win.Content().(*buffer)
	return buf, ok
}

// RemoveAllBuffers removes all buffers but the last one.
func (c *Component) RemoveAllBuffers() {
	for _, buf := range c.buffers {
		c.closeBuffer(buf)
		if !buf.free {
			c.updateWindowContent(buf.win, c.startHandler)
		}
	}

	c.tabs.RemoveAll()
	c.buffers = c.buffers[:0]
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

// returns the next free buffer or an empty Handler
func (c *Component) getFreeBuffer() (Handler, bool) {
	freeBufs := c.freeBuffers()
	if len(freeBufs) == 0 {
		return c.startHandler, false
	}

	id := freeBufs[0]
	return c.buffers[id], true
}

// RemoveWindowBuffer removes the buffer at win. It returns false
// if the replacement is just an empty buffer because all buffers have been
// removed.
func (c *Component) RemoveWindowBuffer(win Window) bool {
	buf, isNotEmptyBuffer := c.getFreeBuffer()
	oldComponent := c.updateWindowContent(win.(*browserWindow), buf)
	oldBuf, ok := oldComponent.(*buffer)
	if ok {
		c.doRemoveBuffer(oldBuf)
	}
	return isNotEmptyBuffer
}

// FlushBuffer flushes the contents of the buffer at win, if this buffer
// was created with a FlusherCloser. See NewBuffer.
func (c *Component) FlushBuffer(win Window) error {
	buf, ok := browserBufferAtWindow(win.(*browserWindow))
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
	h Handler,
) *browserWindow {
	focusBrowserWin := c.focus()
	focusContent := focusBrowserWin.win.Content()

	// set content
	newWin := split(c.wm, focusContent)
	focusBrowserWin.win.SetContent(h)

	// invert handler.Window
	newBrowserWin := c.newWindow(focusBrowserWin.win)
	focusBrowserWin.win = newWin

	if buf, ok := focusContent.(*buffer); ok {
		buf.setWindow(focusBrowserWin)
	}

	if buf, ok := h.(*buffer); ok {
		buf.setWindow(newBrowserWin)
	}

	// return new instance of browser window
	// pointing to old instance of focus window
	return newBrowserWin
}

func (c *Component) split(
	split func(*handler.WindowManager, tui.Handler) handler.Window,
	h Handler,
) *browserWindow {
	// force split a new window tile
	win := split(c.wm, h)

	browserWin := c.newWindow(win)
	if buf, ok := h.(*buffer); ok {
		buf.setWindow(browserWin)
	}
	win.SetContent(h)

	c.wm.SetFocus(win)

	return browserWin
}

// SplitVerticalRight opens a new window tile to the right of the
// current window in focus and initializes it with h.
// Note that if h is not a handler created with NewBuffer
// the handler is cleaned as soon as the window's content is swapped.
func (c *Component) SplitVerticalRight(h Handler) Window {
	return c.split((*handler.WindowManager).SplitVertical, h)
}

// SplitVerticalLeft opens a new window tile to the left of the
// current window in focus and initializes it with h.
// Note that if h is not a handler created with NewBuffer
// the handler is cleaned as soon as the window's content is swapped.
func (c *Component) SplitVerticalLeft(h Handler) Window {
	return c.splitInverted((*handler.WindowManager).SplitVertical, h)
}

// SplitHorizontalBelow opens a new window tile below the current window in focus
// and initializes it with h. Note that if h is not a handler created with NewBuffer
// the handler is cleaned as soon as the window's content is swapped.
func (c *Component) SplitHorizontalBelow(h Handler) Window {
	return c.split((*handler.WindowManager).SplitHorizontal, h)
}

// SplitHorizontalAbove opens a new window tile above the current window in focus
// and initializes it with h. Note that if h is not a handler created with NewBuffer
// the handler is cleaned as soon as the window's content is swapped.
func (c *Component) SplitHorizontalAbove(h Handler) Window {
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

func (c *Component) focus() *browserWindow {
	_, win := c.findWindow(c.wm.Focus())
	return win
}

// Shiftable calls the underlying WindowManager.Shiftable.
func (c *Component) Shiftable() (Window, bool) {
	win, ok := c.wm.Shiftable()
	if !ok {
		return nil, false
	}

	_, bWin := c.findWindow(win)
	return bWin, true
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
		err := c.closeBuffer(f)
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
