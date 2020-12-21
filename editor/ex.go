package editor

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/term"
)

var (
	commandBarAttr = term.Attributes{Bg: term.ColorWhite, Fg: term.ColorBlack}
)

type mode int8

const (
	modeDefault mode = iota
	modeCommand
)

type openFileFunc func(filePath string,
	buf *cell.Buffer, swapDir string) (browser.FlusherCloser, error)

type recoverFileFunc func(filePath,
	swapFilePath string, buf *cell.Buffer) (browser.FlusherCloser, error)

type Handler struct {
	openFileFn    openFileFunc
	recoverFileFn recoverFileFunc
	interruptDraw func()
	comp          browser.Component
	commandBuf    *cell.Buffer
	cmdVirt       handler.Virtual
	ed            Editor
	config        browser.Config
	keymap        map[term.Event]term.Event
	subscribers   map[term.Event]browser.EventHandler
	mode          mode
}

func newOsHandler() *Handler {
	ret := new(Handler)
	return ret
}

// New allocates storage for a new Handler and initializes it.
func New(ed Editor, opts ...browser.Option) (e *Handler, err error) {
	e = new(Handler)
	err = e.Init(ed, opts...)
	if err != nil {
		return
	}
	return
}

func (e *Handler) initConstructors() {
	if e.openFileFn == nil {
		e.openFileFn = func(filePath string,
			buf *cell.Buffer, swapDir string) (browser.FlusherCloser, error) {
			return NewFileBuffer(filePath, buf, swapDir)
		}
	}

	if e.recoverFileFn == nil {
		e.recoverFileFn = func(filePath,
			swapFilePath string, buf *cell.Buffer) (browser.FlusherCloser, error) {
			return RecoverFileBuffer(filePath, swapFilePath, buf)
		}
	}
	if e.interruptDraw == nil {
		e.interruptDraw = term.Interrupt
	}
}

func (e *Handler) tryLog(msg string, args ...interface{}) {
	if e.config.Logger != nil {
		e.config.Logger.Debugf(msg, args...)
	}
}

// Init initializes this Handler with the given editor and Options.
// It returns an error if an initial filepath was given through WithFilePath option
// and the file failed to be opened.
func (e *Handler) Init(ed Editor, opts ...browser.Option) (err error) {
	e.initConstructors()
	e.config = browser.DefaultConfig()

	for _, o := range opts {
		o(&e.config)
	}

	e.comp.Init(e.config)

	e.ed = ed
	e.commandBuf = cell.NewBuffer()
	e.cmdVirt = browser.NewMessageSpan(e.commandBuf, commandBarAttr)
	e.mode = modeDefault
	e.subscribers = make(map[term.Event]browser.EventHandler)

	if e.config.Filepath != "" {
		err = e.newBufferWithFile(e.config.Filepath,
			e.config.RecoveryFilepath)
		if err != nil {
			return
		}
	}

	return
}

func (e *Handler) newCellBuffer() *cell.Buffer {
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
	fileBuf browser.FlusherCloser, err error,
) {
	if recSwapFile != "" {
		fileBuf, err = e.recoverFileFn(filename, recSwapFile, buf)
	} else {
		fileBuf, err = e.openFileFn(filename, buf, e.config.SwapDir)
	}
	return
}

func emptyHandler(ed Editor) tui.Handler {
	buf := cell.NewBuffer()
	return ed.Edit(buf)
}

func (e *Handler) newBufferWithFile(
	filename, recoveryFilename string,
) error {
	buf := e.newCellBuffer()
	fileBuf, err := e.newFileBuffer(filename, recoveryFilename, buf)
	if err != nil {
		e.tryLog("error opening new file buffer: %v", err)
		e.setError(err)
		return err
	}

	editor := e.ed.Edit(buf)
	e.comp.NewBuffer(filepath.Base(filename), editor, fileBuf)
	return nil
}

func (e *Handler) runSingleCommand(cmd string) (quit bool, err error) {
	switch cmd {
	case "bprev":
		e.comp.UpdateWindowBufferPrev(e.comp.Focus())
	case "bnext":
		e.comp.UpdateWindowBufferNext(e.comp.Focus())
	case "bclose":
		err = e.comp.RemoveWindowBuffer(e.comp.Focus())
	case "bcloseAll":
		e.comp.RemoveAllBuffers()
	case "close":
		err = e.comp.Focus().Close()
	case "wq", "wq!":
		quit = true
		fallthrough
	case "w", "w!":
		err = e.comp.FlushBuffer(e.comp.Focus())
	case "q!", "q":
		quit = true
	default:
		err = fmt.Errorf("Unknown command: %s", cmd)
	}
	if err != nil {
		e.tryLog("failed to run command '%s': %v", cmd, err)
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

func (e *Handler) setError(err error) {
	e.comp.SetMessage("Error: %s", err)
}

func (e *Handler) handleCommand(ev term.Event) (quit, handled bool) {
	handled = true

	switch ev.Key {
	case term.KeyEnter:
		var err error
		quit, err = e.runCommand()
		e.setNormalMode()
		if err != nil {
			e.setError(err)
		}
	case term.KeyEsc:
		e.setNormalMode()
	case term.KeyBackspace, term.KeyBackspace2:
		cols := e.commandBuf.Columns(0)
		if cols == 0 {
			e.setNormalMode()
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

func (e *Handler) removeAllBuffers() {
	for {
		err := e.comp.RemoveWindowBuffer(e.comp.Focus())
		if err != nil {
			if err != browser.ErrNoFreeBuffers {
				e.tryLog("unable to remove window buffer: %v", err)
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

func (e *Handler) handleProxy(ev term.Event) (
	exit, handled bool,
) {
	if ev == e.config.CommandEvent {
		e.setCommandMode()
		handled = true
		return
	}

	prev := ev
	ev = e.mapEvent(ev)

	if subscriber, ok := e.subscribers[ev]; ok {
		subscriber.Handle(ev)
		handled = true
		return
	}

	switch ev.Key {
	case term.KeyCtrlA:
		e.removeAllBuffers()
	case term.KeyCtrlW:
		err := e.comp.RemoveWindowBuffer(e.comp.Focus())
		if err != nil {
			e.tryLog("unable to remove window buffer: %v", err)
			e.setError(err)
		}
	case term.KeyCtrlL:
		e.comp.UpdateWindowBufferNext(e.comp.Focus())
	case term.KeyCtrlH:
		e.comp.UpdateWindowBufferPrev(e.comp.Focus())
	default:
		// do not map for children
		ev = prev
		exit, handled = e.comp.Handle(ev)
		return
	}

	return false, true
}

// Handle satisfies tui.Handler.
func (e *Handler) Handle(ev term.Event) (bool, bool) {
	switch e.mode {
	case modeDefault:
		return e.handleProxy(ev)
	case modeCommand:
		return e.handleCommand(ev)
	default:
		panic(fmt.Sprintf("unknown mode: %+v", e.mode))
	}
}

// Cursor satisfies tui.Handler.
func (e *Handler) Cursor() (pos term.Coordinates, show bool) {
	if e.mode == modeCommand {
		pos := e.cmdVirt.Position()
		pos.X += len(e.commandBuf.String())
		return pos, true
	}
	return e.comp.Cursor()
}

// Man satisfies tui.Handler.
func (e *Handler) Man() tui.Manual {
	panic("TODO")
}

// Resize satisfies tui.Component
func (e *Handler) Resize(width, height int) {
	browser.ResizeMessageSpan(&e.cmdVirt, width, height)
	e.comp.Resize(width, height)
}

// Draw satisfies tui.Component
func (e *Handler) Draw(w term.Writer) {
	e.comp.Draw(w)

	if e.mode == modeCommand {
		e.cmdVirt.Draw(w)
	}
}

// Close closes the resources associated with this browser.
func (e *Handler) Close() error {
	err := e.comp.Close()
	if err != nil {
		e.tryLog("browser.Component.Close error: %v", err)
	}
	return err
}

// OpenFile opens the given file in a new browser tab.
func (e *Handler) OpenFile(filename string) error {
	return e.newBufferWithFile(filename, "")
}

// SetMessage formats the given msg and args and displays it on next Draw.
func (e *Handler) SetMessage(msg string, args ...interface{}) error {
	e.comp.SetMessage(msg, args...)
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

func (e *Handler) setNormalMode() {
	e.commandBuf.Reset()
	e.mode = modeDefault
}

func (e *Handler) setCommandMode() {
	e.mode = modeCommand
}

// SplitVerticalRight opens a new window tile to the right of the
// current window in focus and initializes it with h.
func (e *Handler) SplitVerticalRight(h tui.Handler) (browser.Window, error) {
	return e.comp.SplitVerticalRight(h), nil
}

// SplitVerticalLeft opens a new window tile to the left of the
// current window in focus and initializes it with h.
func (e *Handler) SplitVerticalLeft(h tui.Handler) (browser.Window, error) {
	return e.comp.SplitVerticalLeft(h), nil
}

// SplitHorizontalBelow opens a new window tile below the current window in focus
// and initializes it with h.
func (e *Handler) SplitHorizontalBelow(h tui.Handler) (browser.Window, error) {
	return e.comp.SplitHorizontalBelow(h), nil
}

// SplitHorizontalAbove opens a new window tile above the current window in focus
// and initializes it with h.
func (e *Handler) SplitHorizontalAbove(h tui.Handler) (browser.Window, error) {
	return e.comp.SplitHorizontalAbove(h), nil
}

func (e *Handler) unsubscribe(ev term.Event) {
	delete(e.subscribers, ev)
}

// Subscribe subscribers h EventHandler to term.Event ev.
func (e *Handler) Subscribe(ev term.Event, h browser.EventHandler) error {
	if _, ok := e.subscribers[ev]; ok {
		return fmt.Errorf("there's already a subscriber subscribed to: %#v", ev)
	}
	e.subscribers[ev] = browserEventHandler{browser: e, h: h}
	return nil
}

// PublishInterrupt interrupts the main event loop to redraw the terminal.
func (e *Handler) PublishInterrupt() error {
	// prevent deadlock if PublishInterrupt is called during a Draw call.
	go e.interruptDraw()
	return nil
}
