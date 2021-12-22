package editor

import (
	"context"
	"errors"
	"fmt"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/term"
	log "github.com/sirupsen/logrus"
)

var (
	// ErrInvalidSave is returned when trying to save a buffer that it's not a file
	// in the file system.
	ErrInvalidSave = errors.New("Cannot save this buffer")

	// ErrInvalidSplit is returned when attempting to split over a floating window.
	ErrInvalidSplit = errors.New("Cannot split this window")
)

// Component is an implementation of browser.Browser for file editing.
// It also satisfies tui.Component, and editor.Editor.
type Component struct {
	comp           browser.Component
	ed             Editor
	config         Config
	keymap         map[term.Event]term.Event
	edSubscribers  map[EventType][]EventHandler
	cmdSubscribers map[string]CommandHandler
}

// used to intercept calls to Close and Flush to dispatch
// corresponding events to subscribers.
type editorFlusherCloser struct {
	parent    *Component
	fc        FlusherCloser
	h         Handler
	name      string
	buf       *cell.Buffer
	lastFlush string
}

func (c editorFlusherCloser) OnWillInsert(at term.Coordinates, str string) {
}

func (c editorFlusherCloser) OnDidInsert(from, to term.Coordinates) {
	c.parent.setTabAttr(c.name, c.buf, c.lastFlush)
}

func (c editorFlusherCloser) OnWillDelete(from, to term.Coordinates) {
}

func (c editorFlusherCloser) OnDidDelete(start, end term.Coordinates, str string) {
	c.parent.setTabAttr(c.name, c.buf, c.lastFlush)
}

func (e *editorFlusherCloser) Flush() error {
	content, err := e.parent.dispatchFlush(e.name, e.h)
	if err != nil {
		return err
	}
	e.lastFlush = content
	return e.fc.Flush()
}

func (e *editorFlusherCloser) Close() error {
	ev := Event{
		Type:         EventTypeClose,
		ResourceName: e.name,
		Resource:     e.h,
	}
	e.parent.dispatchEvent(ev)
	return e.fc.Close()
}

// NewComponent allocates storage for a new Component and initializes it.
func NewComponent(ed Editor, config Config) (c *Component, err error) {
	c = new(Component)
	err = c.Init(ed, config)
	if err != nil {
		return
	}
	return
}

func (c *Component) tryLog(level log.Level, msg string, args ...interface{}) {
	if c.config.Logger != nil {
		c.config.Logger.Logf(level, msg, args...)
	}
}

func (c *Component) newCellBuffer() *cell.Buffer {
	buf := cell.NewBuffer()
	buf.InitWithTabspaces(c.config.Tabspaces)
	if c.config.Logger != nil {
		// NOTE: only enable when trying to debug low level buffer bugs
		// as it degrades performance quite a bit.
		// buf.WithLogger(c.config.Logger)
	}
	return buf
}

func (c *Component) resetTabProperties(id string) {
	c.comp.SetTabAttr(id, term.Attributes{})
	c.comp.SetTabName(id, c.getTabName(id))
}

func (c *Component) setTabAttr(id string, buf *cell.Buffer, lastFlush string) {
	content := buf.String()
	if content == lastFlush {
		c.resetTabProperties(id)
		return
	}
	c.comp.SetTabAttr(id, c.config.DirtyTabAttr)
	tabname := fmt.Sprintf("%s*", c.getTabName(id))
	c.comp.SetTabName(id, tabname)
}

func (c *Component) newFileBuffer(
	filename, recSwapFile string, buf *cell.Buffer, readOnly bool,
) (ret *editorFlusherCloser, err error) {
	var fc FlusherCloser
	if recSwapFile != "" {
		fc, err = c.config.RecoverFileFn(filename, recSwapFile, buf)
	} else {
		fc, err = c.config.OpenFileFn(filename, buf, c.config.SwapDir, readOnly)
	}

	if err != nil {
		return nil, err
	}

	efc := &editorFlusherCloser{
		parent:    c,
		fc:        fc,
		name:      filename,
		buf:       buf,
		lastFlush: buf.String(),
	}

	buf.Subscribe(efc)

	return efc, nil
}

// Init initializes this Component with the given editor and Options.
// It returns an error if an initial filepath was given through WithFilePath option
// and the file failed to be opened.
func (c *Component) Init(ed Editor, config Config) error {
	c.config = config

	c.comp.Init(c.config.Config)

	c.ed = ed
	c.edSubscribers = make(map[EventType][]EventHandler)
	c.cmdSubscribers = make(map[string]CommandHandler)

	var first browser.Handler

	if c.config.RecoveryFilepath != "" {
		if len(c.config.Filepaths) != 1 {
			return errors.New("only one file expected if recovery file is passed")
		}
		h, err := c.OpenFileTab(c.config.Filepaths[0], c.config.RecoveryFilepath, false)
		if err != nil {
			return err
		}
		first = h
	}

	for _, filename := range c.config.Filepaths {
		h, err := c.Open(filename)
		if err == ErrFileAlreadyOpen {
			// handled via user Prompt
			err = nil
		}
		if err != nil {
			return err
		}
		if first == nil {
			first = h
		}
	}

	if first != nil {
		return c.comp.Focus().SetContent(first)
	}

	return nil
}

func (c *Component) setFocusToTab(t *browser.Tab) (browser.Handler, error) {
	err := c.comp.Focus().SetContent(t)
	if err != nil {
		if err != browser.ErrTabNotFree {
			return nil, err
		}
	}
	return t, nil
}

func (c *Component) getTabName(filename string) string {
	return filepath.Base(filename)
}

func getFileID(filename string) (string, error) {
	usr, _ := user.Current()
	dir := usr.HomeDir
	if filename == "~" {
		filename = dir
	} else if strings.HasPrefix(filename, "~/") {
		filename = filepath.Join(dir, filename[2:])
	}
	// needed as tab ID
	return filepath.Abs(filename)
}

type compTabSubscriber Component

func (s *compTabSubscriber) OnFocus(t *browser.Tab) {
	res, ok := t.Handler().(Handler)
	if !ok {
		return
	}
	(*Component)(s).dispatchEvent(Event{
		Type:         EventTypeFocus,
		ResourceName: t.ID(),
		Resource:     res,
	})
}

func (s compTabSubscriber) OnFree(t *browser.Tab) {
	// not used for now
}

// OpenFileTab opens the file at filename path, with an optional recovery file,
// as a new browser tab. It's up to the caller to use the returned
// browser.Handler and switch any of the active windows to use it.
//
// If recoveryFilename is not empty, then the file will be recovered from the
// contents of recoveryFilename.
func (c *Component) OpenFileTab(
	filename, recoveryFilename string, readOnly bool,
) (browser.Handler, error) {
	filename, err := getFileID(filename)
	if err != nil {
		return nil, fmt.Errorf("could not evaluate file path '%s': %v", filename, err)
	}

	t, ok := c.comp.Tab(filename)
	if ok {
		return c.setFocusToTab(t)
	}

	buf := c.newCellBuffer()
	fc, err := c.newFileBuffer(filename, recoveryFilename, buf, readOnly)
	if err != nil {
		return nil, err
	}

	editor, err := c.ed.Edit(filename, buf)
	if err != nil {
		return nil, err
	}
	fc.h = editor

	tabname := c.getTabName(filename)
	c.tryLog(log.DebugLevel, "Open(%s): opening tab with tabname='%s'", filename, tabname)

	t = c.comp.NewTab(filename, tabname, editor, fc)
	t.Subscribe((*compTabSubscriber)(c))
	return t, nil
}

func (c *Component) openRecoveryPrompt(file string) {
	const (
		recoverOpt  = "Recover"
		readOnlyOpt = "Open Read-Only"
		skipOpt     = "Skip"
	)

	msg := fmt.Sprintf(`File %s is already
open by another process or
an edit session for this file crashed.`, file)

	c.comp.Prompt(msg, []string{recoverOpt, readOnlyOpt, skipOpt},
		[]term.Event{{Ch: 'R'}, {Ch: 'O'}, {Ch: 'S'}},
		func(i int, opt string) {

			var h browser.Handler
			var err error

			switch opt {
			case recoverOpt:
				file, _ = getFileID(file) // to get right swap file name
				_, swapFileName := swapFileName(c.config.SwapDir, file)
				h, err = c.OpenFileTab(file, swapFileName, false)
			case readOnlyOpt:
				h, err = c.OpenFileTab(file, "", true)
			case skipOpt:
			}
			if h != nil {
				err = c.comp.Focus().SetContent(h)
			}
			if err != nil {
				c.tryLog(log.ErrorLevel, "recovery prompt: %v", err)
				c.SetMessage("%v", err)
				return
			}
		})
}

// Open opens the given file in a new browser tab. If file is already
// open by another session or the last edit session crashed, it
// will create a prompt for the user to decide what to do.
func (c *Component) Open(file string) (browser.Handler, error) {
	h, err := c.OpenFileTab(file, "", false)
	if err != nil && err == ErrFileAlreadyOpen {
		c.openRecoveryPrompt(file)
	}
	return h, err
}

// Editor satisfies Editor interface.
func (c *Component) Editor(name string) (Handler, error) {
	filename, err := getFileID(name)
	if err != nil {
		return nil, fmt.Errorf("could not evaluate file path '%s': %v", filename, err)
	}

	for _, tab := range c.comp.Tabs() {
		if tab.ID() == filename {
			h, ok := tab.Handler().(Handler)
			if !ok {
				continue
			}
			return h, nil
		}
	}
	return nil, errors.New("handler not found")
}

// KeyMapping returns a key mapping for ev and true
// or the original ev and false if there's
// no mapping. It also returns any command that was mapped
// to the return mapping (or the original event, if there's no event mapping).
// Mappings are created via MergeKeyMap.
func (c *Component) KeyMapping(ev term.Event) (term.Event, string, bool) {
	mev, ok := c.keymap[ev]
	if !ok {
		mev = ev
	}
	cmd, _ := c.config.CommandKeyBindings[mev]
	c.tryLog(log.TraceLevel, "KeyMapping(%#v): %#v, %s", ev, mev, cmd)
	return mev, cmd, ok
}

// MergeKeyMap takes the given keymap and merges it with the Browser's keymap
// to override the current event key mappings.
func (c *Component) MergeKeyMap(keymap map[term.Event]term.Event) error {
	if c.keymap == nil {
		c.keymap = make(map[term.Event]term.Event)
	}
	for k, v := range keymap {
		if k.Type != term.EventKey || v.Type != term.EventKey {
			return errors.New("invalid mapping of non-key event")
		}
		c.keymap[k] = v
	}
	return nil
}

// DispatchCommand dispatches a EventTypeCommand with cmd to subscribers
// subscribed via SubscribeEditorEvents.
func (c *Component) DispatchCommand(cmd Command) (handled bool) {
	commander, handled := c.cmdSubscribers[cmd.Name]
	if !handled {
		return false
	}
	exit := commander.HandleCommand(cmd)
	if exit {
		delete(c.cmdSubscribers, cmd.Name)
	}
	return true
}

func (c *Component) dispatchFlush(id string, h Handler) (string, error) {
	content, err := c.getContent(h)
	if err != nil {
		return "", err
	}

	ev := Event{
		Type:         EventTypeFlush,
		ResourceName: id,
		Resource:     h,
		Content:      content,
	}
	// clear dirty/flushed attributes
	c.resetTabProperties(id)
	c.dispatchEvent(ev)
	return content, nil
}

// dispatchEvent either flush or close events
func (c *Component) dispatchEvent(ev Event) (handled bool) {
	subs, ok := c.edSubscribers[ev.Type]
	if !ok {
		return
	}

	remain := make([]EventHandler, 0, len(subs))
	for _, h := range subs {
		exit := h.Handle(ev)
		if !exit {
			remain = append(remain, h)
		}
		handled = true
	}
	c.edSubscribers[ev.Type] = remain
	return
}

// SetMessage formats the given msg and args and displays it on next Draw.
func (c *Component) SetMessage(msg string, args ...interface{}) error {
	c.comp.SetMessage(msg, args...)
	return nil
}

// Split satisfies browser.WindowManager.
func (c *Component) Split(o browser.Orientation, h browser.Handler) (browser.Window, error) {
	w, ok := c.comp.Split(o, h)
	if !ok {
		return nil, ErrInvalidSplit
	}
	return w, nil
}

// Bar creates a new status bar with h's component and delegates handling of mouse events to h.
func (c *Component) Bar(o browser.Orientation, h tui.Handler) error {
	c.comp.Bar(o, h)
	return nil
}

// PublishInterrupt interrupts the main event loop to redraw the terminal.
func (c *Component) PublishInterrupt() error {
	c.config.InterruptDraw()
	return nil
}

// Focus returns the current window in focus. It satisfies browser.Browser.
func (c *Component) Focus() (browser.Window, error) {
	return c.comp.Focus(), nil
}

// Edit edits the resource with name and buffer with the underlying Editor
// in a new browser buffer.
func (c *Component) Edit(name string, buf *cell.Buffer) (Handler, error) {
	editor, err := c.ed.Edit(name, buf)
	if err != nil {
		return nil, err
	}

	t := c.comp.NewTab(name, name, editor, nil)
	t.Subscribe((*compTabSubscriber)(c))

	return editor, nil
}

// SetLocationList satisfies editor.Editor.
func (c *Component) SetLocationList(h Handler, ID string, loc LocationList) error {
	return c.ed.SetLocationList(h, ID, loc)
}

// MoveToNextLocation satisfies editor.Editor.
func (c *Component) MoveToNextLocation(h Handler, ID string) error {
	return c.ed.MoveToNextLocation(h, ID)
}

// MoveToPrevLocation satisfies editor.Editor.
func (c *Component) MoveToPrevLocation(h Handler, ID string) error {
	return c.ed.MoveToPrevLocation(h, ID)
}

// Reader satisfies editor.Editor.
func (c *Component) Reader(h Handler) Reader {
	return c.ed.Reader(h)
}

// Writer satisfies editor.Editor.
func (c *Component) Writer(h Handler) Writer {
	return c.ed.Writer(h)
}

// Flush flushes the contents of the buffer at win, if this buffer
// was created with a FlusherCloser. See browser.NewBuffer.
func (c *Component) Flush(win browser.Window) error {
	content, err := win.Content()
	if err != nil {
		return fmt.Errorf("editor.Component.Flush: win.Content: %v", err)
	}
	t, ok := content.(*browser.Tab)
	if !ok {
		return ErrInvalidSave
	}

	fc := t.Closer().(FlusherCloser)
	err = fc.Flush()
	if err != nil {
		return fmt.Errorf("editor.Component.Flush: %v", err)
	}
	return nil
}

func (c *Component) getContent(h Handler) (string, error) {
	cells, err := c.ed.Reader(h).RawCells()
	if err != nil {
		return "", fmt.Errorf("Error dispatching event content: RawCells: %v", err)
	}
	return cell.CellsToString(cells), nil
}

// SubscribeEditorEvents subscribes h to editor events of type ev.
// If ev is of type EventTypeOpen, an event will be dispatched for
// every Tab currently open.
func (c *Component) SubscribeEditorEvents(ev EventType, h EventHandler) error {
	switch ev {
	case EventTypeOpen:
		for _, tab := range c.comp.Tabs() {
			resHandler, ok := tab.Handler().(Handler)
			if !ok {
				// tab handler does not implement Handler
				continue
			}
			str, err := c.getContent(resHandler)
			if err != nil {
				return err
			}
			exit := h.Handle(Event{
				Type:         EventTypeOpen,
				ResourceName: tab.ID(),
				Resource:     resHandler,
				Content:      str,
			})
			if exit {
				return nil
			}
		}
		fallthrough
	// delegate open/insert/delete event dispatching to underlying editor.
	case EventTypeDelete, EventTypeInsert, EventTypeScroll, EventTypeCursor:
		return c.ed.SubscribeEditorEvents(ev, h)
	case EventTypeFocus:
		t, ok := c.comp.FocusTab()
		if ok {
			resHandler, ok := t.Handler().(Handler)
			if ok {
				ev := Event{
					Type:         EventTypeClose,
					ResourceName: t.ID(),
					Resource:     resHandler,
				}
				h.Handle(ev)
			}
		}
	}

	if _, ok := c.edSubscribers[ev]; !ok {
		c.edSubscribers[ev] = make([]EventHandler, 0, 1)
	}

	c.edSubscribers[ev] = append(c.edSubscribers[ev], h)
	return nil
}

// Commands returns a list of commands registered via SubscribeCommand.
func (c *Component) Commands() (ret []string) {
	ret = make([]string, len(c.cmdSubscribers))
	var i int
	for cmd := range c.cmdSubscribers {
		ret[i] = cmd
		i++
	}
	return ret
}

// SubscribeCommand installs cm as a command handler of cmd or returns
// an error if there's already a CommandHandler installed for this cmd.
func (c *Component) SubscribeCommand(cmd string, cm CommandHandler) error {
	if _, ok := c.cmdSubscribers[cmd]; ok {
		return errors.New("command already registered")
	}

	c.cmdSubscribers[cmd] = cm
	return nil
}

// Browser returns this Component's underlying browser.Component.
func (c *Component) Browser() *browser.Component {
	return &c.comp
}

// Resize satisfies tui.Component.
func (c *Component) Resize(width, height int) {
	c.comp.Resize(width, height)
}

// Draw satisfies tui.Component.
func (c *Component) Draw(w term.Writer) {
	c.comp.Draw(w)
}

// Create satisfies browser.Storage
func (c *Component) Create(
	ctx context.Context, ID string, doc interface{},
) error {
	return c.config.Storage.Create(ctx, ID, doc)
}

// Set satisfies browser.Storage
func (c *Component) Set(
	ctx context.Context, ID string, doc interface{},
) error {
	return c.config.Storage.Set(ctx, ID, doc)
}

// Update satisfies browser.Storage
func (c *Component) Update(
	ctx context.Context, ID string, updates []document.Update,
) error {
	return c.config.Storage.Update(ctx, ID, updates)
}

// Get satisfies browser.Storage
func (c *Component) Get(
	ctx context.Context, ID string, doc interface{},
) error {
	return c.config.Storage.Get(ctx, ID, doc)
}

// Delete satisfies browser.Storage
func (c *Component) Delete(
	ctx context.Context, ID string,
) error {
	return c.config.Storage.Delete(ctx, ID)
}

// List satisfies browser.Storage
func (c *Component) List(
	ctx context.Context, filters []document.Filter,
) (document.Iterator, error) {
	return c.config.Storage.List(ctx, filters)
}

// SetCursor satisfies editor.Editor
func (c *Component) SetCursor(h Handler, pos term.Coordinates) error {
	return c.ed.SetCursor(h, pos)
}

// Cursor satisfies editor.Editor
func (c *Component) Cursor(h Handler) (term.Coordinates, error) {
	return c.ed.Cursor(h)
}

// Floating satisfies browser.WindowManager.
func (c *Component) Floating(
	h browser.Handler, at term.Coordinates, width, height int,
) (browser.Window, error) {
	if at.Y < 0 || at.X < 0 {
		return nil, fmt.Errorf("invalid floating window coordinates: %v", at)
	}
	return c.comp.Floating(h, at, width, height), nil
}

// Close closes all resources associated with this Component.
func (c *Component) Close() error {
	// avoid dispatching close events on flusherCloser callbacks
	c.edSubscribers = make(map[EventType][]EventHandler)

	err1 := c.comp.Close()
	err2 := c.config.Storage.Close()
	if err2 != nil {
		c.tryLog(log.ErrorLevel, "config.Storage.Close error: %v", err2)
	}
	if err1 != nil {
		c.tryLog(log.ErrorLevel, "browser.Component.Close error: %v", err1)
		return err1
	}
	return err2
}
