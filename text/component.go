package text

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/logging"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/workspace"
)

// used for command and event handlers
const defaultTimeout = 1 * time.Second

var _ tui.Component = (*Component)(nil)
var _ browser.Browser = (*Component)(nil)
var _ Editor = (*Component)(nil)

// Component is an implementation of browser.Browser for file editing.
// It also satisfies tui.Component, and text.Editor.
type Component struct {
	comp           browser.Component
	workspace      workspace.Loader
	ed             Editor
	config         Config
	edSubscribers  map[textapi.EventType][]EventHandler
	cmdSubscribers map[string]CommandHandler
}

// used to intercept calls to Close and Flush to dispatch
// corresponding events to subscribers.
type editorFlusherCloser struct {
	parent    *Component
	fc        workspace.FlusherCloser
	h         Handler
	uri       workspaceapi.URI
	buf       *cell.Buffer
	lastFlush string
}

func (c editorFlusherCloser) OnWillEdit(start, end term.Coordinates, str string) {
}

func (c editorFlusherCloser) OnDidEdit(from, to term.Coordinates, old string) {
	c.parent.setTabAttr(c.uri, c.buf, c.lastFlush)
}

func (e *editorFlusherCloser) Flush() error {
	content, err := e.parent.dispatchFlush(e.uri, e.h)
	if err != nil {
		return err
	}
	e.lastFlush = content
	return e.fc.Flush()
}

func (e *editorFlusherCloser) Close() error {
	ev := textapi.Event{
		Type:     textapi.EventTypeClose,
		URI:      e.uri,
		Resource: e.h,
	}
	e.parent.dispatchEvent(ev)
	return e.fc.Close()
}

// NewComponent allocates storage for a new Component and initializes it.
func NewComponent(ed Editor, w workspace.Loader, config Config) (
	c *Component, err error,
) {
	c = new(Component)
	err = c.Init(ed, w, config)
	if err != nil {
		return
	}
	return
}

func (c *Component) log(level log.Level, msg string, args ...interface{}) {
	log.
		WithField(logging.KeyClass, "text.Component").Logf(level, msg, args...)
}

func (c *Component) newCellBuffer() *cell.Buffer {
	buf := cell.NewBuffer()
	buf.InitWithTabspaces(c.config.Tabspaces)
	return buf
}

func (c *Component) resetTabProperties(file workspaceapi.URI) {
	c.comp.SetTabAttr(file, term.Attributes{})
	c.comp.SetTabName(file, file.Name())
}

// TODO this is a very inefficient way of checking if a file was changed.
// We should instead collect edits and check for undos by comparing arguments
// and return values.
func (c *Component) setTabAttr(file workspaceapi.URI, buf *cell.Buffer, lastFlush string) {
	content := buf.String()
	if content == lastFlush {
		c.resetTabProperties(file)
		return
	}
	c.comp.SetTabAttr(file, c.config.DirtyTabAttr)
	tabname := fmt.Sprintf("%s*", file.Name())
	c.comp.SetTabName(file, tabname)
}

func (c *Component) getSwapDir(file workspaceapi.URI) (workspaceapi.URI, error) {
	return workspace.DefaultSwapDirectory(file)
}

func (c *Component) newFileBuffer(
	file, recSwapFile workspaceapi.URI, buf *cell.Buffer,
	readOnly, forceRecover bool,
) (ret *editorFlusherCloser, err error) {
	var fc workspace.FlusherCloser
	if recSwapFile != (workspaceapi.URI{}) {
		fc, err = c.workspace.Recover(file, recSwapFile, buf, forceRecover)
	} else {
		var swapDir workspaceapi.URI
		swapDir, err = c.getSwapDir(file)
		if err == nil {
			fc, err = c.workspace.Load(file, buf, swapDir, readOnly)
		}
	}

	if err != nil {
		return nil, err
	}

	efc := &editorFlusherCloser{
		parent:    c,
		fc:        fc,
		uri:       file,
		buf:       buf,
		lastFlush: buf.String(),
	}

	// no need to unsubscribe upon Close since the assumption
	// is that a Component always outlives a cell.Buffer
	buf.Subscribe(efc)

	return efc, nil
}

// Init initializes this Component with the given editor and Options.
// It returns an error if an initial filepath was given through WithFilePath option
// and the file failed to be opened.
func (c *Component) Init(ed Editor, w workspace.Loader, config Config) error {
	c.config = config

	c.comp.Init(c.config.Config)
	c.comp.Subscribe((*windowFocusSubscriber)(c))

	c.ed = ed
	c.workspace = w
	c.edSubscribers = make(map[textapi.EventType][]EventHandler)
	c.cmdSubscribers = make(map[string]CommandHandler)

	var first browser.Handler

	if c.config.RecoveryFilepath != (workspaceapi.URI{}) {
		if len(c.config.Filepaths) != 1 {
			return errors.New("only one file expected if recovery file is passed")
		}
		h, err := c.RecoverFileTab(c.config.Filepaths[0],
			c.config.RecoveryFilepath, false)
		if err != nil {
			return err
		}
		first = h
	}

	for _, filename := range c.config.Filepaths {
		h, err := c.Open(filename)
		if err == workspaceapi.ErrFileAlreadyOpen {
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

	// validate that config aliases are not recursive
	seenAliased := map[string]struct{}{}
	for k := range c.config.CommandAliases {
		seenAliased[k] = struct{}{}
	}
	for _, v := range c.config.CommandAliases {
		for _, v := range v {
			if _, ok := seenAliased[v]; ok {
				return fmt.Errorf("a command alias cannot reference other command aliases")
			}
		}
	}

	return nil
}

type windowFocusSubscriber Component

func (c *windowFocusSubscriber) OnFocus(old, focus handler.Window) {
	c.tryDispatchEvent(old, textapi.EventTypeUnfocus)
	c.tryDispatchEvent(focus, textapi.EventTypeFocus)
}

func (c *windowFocusSubscriber) tryDispatchEvent(win handler.Window, evType textapi.EventType) {
	content := win.Content()
	t, ok := content.(*browser.Tab)
	if !ok {
		return
	}
	res, ok := t.Handler().(Handler)
	if !ok {
		return
	}
	(*Component)(c).dispatchEvent(textapi.Event{
		Type:     evType,
		URI:      t.URI(),
		Resource: res,
	})
}

func (c *Component) setFocusToTab(t *browser.Tab) (browser.Handler, error) {
	err := c.comp.Focus().SetContent(t)
	if err != nil {
		if err != browserapi.ErrTabNotFree {
			return nil, err
		}
	}
	return t, nil
}

type compTabSubscriber struct {
	parent *Component
	window browser.Window
}

func (s *compTabSubscriber) OnFocus(t *browser.Tab) {
	res, ok := t.Handler().(Handler)
	if !ok {
		return
	}
	w, ok := t.Window()
	if !ok {
		// this should never happen, given that we just received OnFocus
		return
	}
	s.window = w
	if ok, err := w.Focus(); err != nil || !ok {
		// tab has been assigned to window but window
		// is not main window
		return
	}
	s.parent.dispatchEvent(textapi.Event{
		Type:     textapi.EventTypeFocus,
		URI:      t.URI(),
		Resource: res,
	})
}

func (s *compTabSubscriber) OnFree(t *browser.Tab) {
	res, ok := t.Handler().(Handler)
	if !ok {
		return
	}
	if s.window == nil {
		// should not happen, if OnFree is called
		// OnFocus should have been called before
		return
	}
	if ok, err := s.window.Focus(); err != nil || !ok {
		return
	}
	s.parent.dispatchEvent(textapi.Event{
		Type:     textapi.EventTypeUnfocus,
		URI:      t.URI(),
		Resource: res,
	})
}

// OpenFileTab opens the file at filename path, as a new browser tab.
// It's up to the caller to use the returned browser.Handler and switch
// any of the active windows to use it.
//
// If recoveryFilename is not empty, then the file will be recovered from the
// contents of recoveryFilename.
func (c *Component) OpenFileTab(file workspaceapi.URI, readOnly bool) (
	browser.Handler, error,
) {
	if file == (workspaceapi.URI{}) {
		return nil, errors.New("empty URI")
	}
	return c.openFileTab(file, workspaceapi.URI{}, readOnly, false)
}

// RecoverFileTab recovers the file at filename by using the file at recoverFilename
// and opens a tab it like OpenFileTab. See OpenFileTab for more details.
func (c *Component) RecoverFileTab(
	file workspaceapi.URI, recoveryFilename workspaceapi.URI, readOnly bool,
) (browser.Handler, error) {
	if file == (workspaceapi.URI{}) || recoveryFilename == (workspaceapi.URI{}) {
		return nil, errors.New("empty URI")
	}
	return c.openFileTab(file, recoveryFilename, readOnly, false)
}

func (c *Component) openFileTab(
	file workspaceapi.URI, recoveryFilename workspaceapi.URI,
	readOnly, forceRecover bool,
) (browser.Handler, error) {
	t, ok := c.comp.Tab(file)
	if ok {
		return c.setFocusToTab(t)
	}

	buf := c.newCellBuffer()
	fc, err := c.newFileBuffer(file, recoveryFilename, buf, readOnly, forceRecover)
	if err != nil {
		return nil, err
	}

	editor, err := c.ed.Edit(file, buf)
	if err != nil {
		return nil, err
	}
	fc.h = editor

	t = c.newTab(file, file.Name(), editor, fc)
	return t, nil
}

func (c *Component) openAreYouSurePrompt(file workspaceapi.URI) {
	const (
		yesOpt = "Yes"
		noOpt  = "No"
	)

	msg := fmt.Sprintf(`File %s
has been updated since the last back up was created.
Are you sure you want to recover it
and lose all the new updates?`, file)

	c.comp.Prompt(msg, []string{yesOpt, noOpt},
		[]term.KeyComb{{Ch: 'Y'}, {Ch: 'N'}},
		func(i int, opt string) {

			var h browser.Handler
			var err error

			switch opt {
			case yesOpt:
				var swapDir, swapFile workspaceapi.URI
				swapDir, err = c.getSwapDir(file)
				if err == nil {
					swapFile, err = workspace.DefaultSwapFile(swapDir, file)
					if err == nil {
						h, err = c.openFileTab(file, swapFile, false, true)
					}
				}
			case noOpt:
			}
			if h != nil {
				err = c.comp.Focus().SetContent(h)
			}
			if err != nil {
				c.log(log.ErrorLevel, "recovery prompt: %v", err)
				c.SetMessage("%v", err)
				return
			}
		})
}

func (c *Component) openRecoveryPrompt(file workspaceapi.URI) {
	const (
		recoverOpt  = "Recover"
		readOnlyOpt = "Open Read-Only"
		skipOpt     = "Skip"
	)

	msg := fmt.Sprintf(`File %s is already
open by another process or
an edit session for this file crashed.`, file)

	c.comp.Prompt(msg, []string{recoverOpt, readOnlyOpt, skipOpt},
		[]term.KeyComb{{Ch: 'R'}, {Ch: 'O'}, {Ch: 'S'}},
		func(i int, opt string) {

			var h browser.Handler
			var err error

			switch opt {
			case recoverOpt:
				var swapDir, swapFile workspaceapi.URI
				swapDir, err = c.getSwapDir(file)
				if err == nil {
					swapFile, err = workspace.DefaultSwapFile(swapDir, file)
					if err == nil {
						h, err = c.RecoverFileTab(file, swapFile, false)
						if err == workspaceapi.ErrStaleData {
							c.openAreYouSurePrompt(file)
							return
						}
					}
				}
			case readOnlyOpt:
				h, err = c.OpenFileTab(file, true)
			case skipOpt:
			}
			if h != nil {
				err = c.comp.Focus().SetContent(h)
			}
			if err != nil {
				c.log(log.ErrorLevel, "recovery prompt: %v", err)
				c.SetMessage("%v", err)
				return
			}
		})
}

// Open opens the given file in a new browser tab. If file is already
// open by another session or the last edit session crashed, it
// will create a prompt for the user to decide what to do.
func (c *Component) Open(file workspaceapi.URI) (browser.Handler, error) {
	h, err := c.OpenFileTab(file, false)
	if err != nil && err == workspaceapi.ErrFileAlreadyOpen {
		c.openRecoveryPrompt(file)
	}
	return h, err
}

// Editor satisfies Editor interface.
func (c *Component) Editor(file workspaceapi.URI) (Handler, error) {
	for _, tab := range c.comp.Tabs() {
		if tab.URI().String() == file.String() {
			h, ok := tab.Handler().(Handler)
			if !ok {
				continue
			}
			return h, nil
		}
	}
	return nil, errors.New("handler not found")
}

// KeyMapping returns a command that was mapped to the given key
// combination and true or an empty string and false if there was
// no command mapped to the given key.
func (c *Component) KeyMapping(key term.KeyComb) ([]string, bool) {
	cmd, ok := c.config.CommandKeyBindings[key]
	c.log(log.TraceLevel, "KeyMapping(%#v): %s", key, cmd)
	return cmd, ok
}

func (c *Component) CompleteCommand(ctx context.Context, cmd string, args ...string) (
	iterator.Iterator[string], error,
) {
	// aliases cannot be auto-completed
	if _, ok := c.config.CommandAliases[cmd]; ok {
		c.log(log.DebugLevel, "complete command %q: aliases cannot get completed", cmd)
		return iterator.FromSlice[string](nil), nil
	}

	commander, ok := c.cmdSubscribers[cmd]
	if !ok {
		c.log(log.DebugLevel, "complete command %q: no subscribers", cmd)
		return iterator.FromSlice[string](nil), nil
	}

	completer, ok := commander.(CommandCompleter)
	if !ok {
		c.log(log.DebugLevel, "complete command %q: completion not implemented", cmd)
		return iterator.FromSlice[string](nil), nil
	}

	return completer.Complete(ctx, args)
}

// DispatchCommand dispatches a EventTypeCommand with cmd to subscribers
// subscribed via SubscribeEditorEvents.
func (c *Component) DispatchCommand(cmd textapi.Command) (handled bool, err error) {
	if cmd.Window == nil {
		panic("invalid command: missing Window from which command was invoked")
	}
	targets, ok := c.config.CommandAliases[cmd.Name]
	if ok {
		c.log(log.InfoLevel, "Dispatching alias %s: %#v", cmd.Name, targets)
		for _, target := range targets {
			argv := strings.Split(target, " ")
			cmd.Name = argv[0]
			cmd.Args = argv[1:]
			targetHandled, targetErr := c.DispatchCommand(cmd)
			if targetErr != nil {
				return targetHandled, fmt.Errorf("%s: %s", target, targetErr)
			}
			handled = handled || targetHandled
		}
		return handled, nil
	}
	commander, ok := c.cmdSubscribers[cmd.Name]
	if !ok {
		c.log(log.DebugLevel, "Dispatching command %q: no subscribers", cmd.Name)
		return false, nil
	}
	c.log(log.InfoLevel, "Dispatching command %q with args %v", cmd.Name, cmd.Args)
	exit, err := commander.HandleCommand(context.Background(), cmd)
	if err != nil {
		return true, err
	}
	if exit {
		c.log(log.DebugLevel, "Removing command %q: returned exit=true: %#v", cmd.Name, commander)
		delete(c.cmdSubscribers, cmd.Name)
	}
	return true, nil
}

func (c *Component) dispatchFlush(file workspaceapi.URI, h Handler) (string, error) {
	content, err := c.getContent(h)
	if err != nil {
		return "", err
	}

	ev := textapi.Event{
		Type:     textapi.EventTypeFlush,
		URI:      file,
		Resource: h,
		Content:  content,
	}
	// clear dirty/flushed attributes
	c.resetTabProperties(file)
	c.dispatchEvent(ev)
	return content, nil
}

// dispatchEvent either flush or close events
func (c *Component) dispatchEvent(ev textapi.Event) (handled bool) {
	subs, ok := c.edSubscribers[ev.Type]
	if !ok {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	remain := make([]EventHandler, 0, len(subs))
	for _, h := range subs {
		exit := h.Handle(ctx, ev)
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
func (c *Component) Split(
	o browserapi.Orientation, win browser.Window, h browser.Handler,
) (browser.Window, error) {
	w, ok := c.comp.Split(o, win, h)
	if !ok {
		return nil, textapi.ErrInvalidSplit
	}
	return w, nil
}

// Bar creates a new status bar with h's component and delegates handling of mouse events to h.
func (c *Component) Bar(o browserapi.Orientation, h tui.Handler) error {
	c.comp.Bar(o, h)
	return nil
}

// Interrupt interrupts the main event loop to redraw the terminal.
func (c *Component) Interrupt() error {
	return c.config.Interrupter.Interrupt()
}

// PublishEventNone sends an EventNone to the main event loop which
// forces Handle to be called on the tui.Handler in focus.
func (c *Component) PublishEventNone() error {
	c.config.SendEventNone()
	return nil
}

// Focus returns the current window in focus. It satisfies browser.Browser.
func (c *Component) Focus() (browser.Window, error) {
	return c.comp.Focus(), nil
}

// SetFocus sets the window in focus and returns the previous window in focus.
// It satisfies browser.Browser.
func (c *Component) SetFocus(win browser.Window) (browser.Window, error) {
	return c.comp.SetFocus(win), nil
}

// Edit edits the resource with name and buffer with the underlying Editor
// in a new browser buffer.
func (c *Component) Edit(file workspaceapi.URI, buf *cell.Buffer) (Handler, error) {
	editor, err := c.ed.Edit(file, buf)
	if err != nil {
		return nil, err
	}

	c.newTab(file, file.Name(), editor, nil)
	return editor, nil
}

// SetLocationList satisfies text.Editor.
func (c *Component) SetLocationList(
	h Handler, pri textapi.LocationPriority, ID string, loc LocationList,
) error {
	return c.ed.SetLocationList(h, pri, ID, loc)
}

// MoveToNextLocation satisfies text.Editor.
func (c *Component) MoveToNextLocation(h Handler, ID string) error {
	return c.ed.MoveToNextLocation(h, ID)
}

// MoveToPrevLocation satisfies text.Editor.
func (c *Component) MoveToPrevLocation(h Handler, ID string) error {
	return c.ed.MoveToPrevLocation(h, ID)
}

// CellView satisfies text.Editor.
func (c *Component) CellView(h Handler) CellView {
	return c.ed.CellView(h)
}

// CellEditor satisfies text.Editor.
func (c *Component) CellEditor(h Handler) CellEditor {
	return c.ed.CellEditor(h)
}

// Flush flushes the contents of the buffer at win, if this buffer
// was created with a FlusherCloser. See browser.NewBuffer.
func (c *Component) Flush(win browser.Window) error {
	content, err := win.Content()
	if err != nil {
		return fmt.Errorf("editor.Component.Flush: win.Content: %v", err)
	}
	t, ok := content.(*browser.Tab)
	if !ok || t.Closer() == nil {
		return textapi.ErrInvalidSave
	}

	fc := t.Closer().(workspace.FlusherCloser)
	err = fc.Flush()
	if err != nil {
		return fmt.Errorf("editor.Component.Flush: %v", err)
	}
	return nil
}

func (c *Component) getContent(h Handler) (string, error) {
	cells, err := c.ed.CellView(h).RawCells()
	if err != nil {
		return "", fmt.Errorf("CellView.RawCells: %v", err)
	}
	return cell.CellsToString(cells), nil
}

func (c *Component) dispatchOpenTabs(h EventHandler) (error, bool) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var ret error
	for _, tab := range c.comp.Tabs() {
		resHandler, ok := tab.Handler().(Handler)
		if !ok {
			// tab handler does not implement Handler
			continue
		}
		str, err := c.getContent(resHandler)
		if err != nil {
			ret = multierr.Append(ret, err)
			continue
		}
		exit := h.Handle(ctx, textapi.Event{
			Type:     textapi.EventTypeOpen,
			URI:      tab.URI(),
			Resource: resHandler,
			Content:  str,
		})
		if exit {
			return ret, true
		}
	}
	return ret, false
}

func (c *Component) dispatchFocusTab(h EventHandler) bool {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	t, ok := c.comp.FocusTab()
	if ok {
		resHandler, ok := t.Handler().(Handler)
		if ok {
			ev := textapi.Event{
				Type:     textapi.EventTypeFocus,
				URI:      t.URI(),
				Resource: resHandler,
			}
			return h.Handle(ctx, ev)
		}
	}
	return false
}

// SubscribeEditorEvents subscribes h to editor events of type ev.
// If ev is of type EventTypeOpen, an event will be dispatched for
// every Tab currently open.
func (c *Component) SubscribeEditorEvents(evs []textapi.EventType, h EventHandler) error {
	// iterate to dispatch immediate events
	for _, ev := range evs {
		switch ev {
		case textapi.EventTypeOpen:
			err, exit := c.dispatchOpenTabs(h)
			if err != nil {
				return err
			}
			if exit {
				return nil
			}
		case textapi.EventTypeFocus:
			exit := c.dispatchFocusTab(h)
			if exit {
				return nil
			}
		}
	}

	// iterate again so if handler exited for any event, we have returned
	// and we do not subscribe it
	var delegated []textapi.EventType
	for _, ev := range evs {
		switch ev {
		// delegate certain event dispatching to underlying editor.
		case textapi.EventTypeOpen, textapi.EventTypeEdit,
			textapi.EventTypeScroll, textapi.EventTypeCursor:
			delegated = append(delegated, ev)
		default:
			if _, ok := c.edSubscribers[ev]; !ok {
				c.edSubscribers[ev] = make([]EventHandler, 0, 1)
			}
			c.edSubscribers[ev] = append(c.edSubscribers[ev], h)
		}
	}

	return c.ed.SubscribeEditorEvents(delegated, h)
}

// Commands returns a list of commands registered via SubscribeCommand
// or via Config.CommandAliases.
func (c *Component) Commands() (ret []string) {
	ret = make([]string, len(c.cmdSubscribers))
	var i int
	for cmd := range c.cmdSubscribers {
		ret[i] = cmd
		i++
	}
	for k := range c.config.CommandAliases {
		ret = append(ret, k)
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

// SetDim sets whether next call to draw should use
// non-focus window diming feature.
func (c *Component) SetDim(to bool) bool {
	return c.comp.SetDim(to)
}

// WindowManagerPosition returns the offset from the top left corner
// where the underlying window manager starts.
func (c *Component) WindowManagerPosition() term.Coordinates {
	return c.comp.WindowManagerPosition()
}

// SetCursor satisfies text.Editor
func (c *Component) SetCursor(h Handler, pos term.Coordinates) error {
	return c.ed.SetCursor(h, pos)
}

// Cursor satisfies text.Editor
func (c *Component) Cursor(h Handler) (term.Coordinates, error) {
	return c.ed.Cursor(h)
}

// Floating satisfies browser.WindowManager.
func (c *Component) Floating(
	h browser.Floating, cfg component.FloatingConfig,
) (browser.Window, error) {
	return c.comp.Floating(h, cfg), nil
}

func (c *Component) newTab(
	resource workspaceapi.URI, name string, h browser.Handler, closer io.Closer,
) *browser.Tab {
	t := c.comp.NewTab(resource, name, h, closer)
	t.Subscribe(&compTabSubscriber{parent: c})
	return t
}

// Tab satisfies browser.WindowManager.
func (c *Component) Tab(resource workspaceapi.URI, name string, h browser.Handler) (
	browser.Handler, error,
) {
	t, ok := c.comp.Tab(resource)
	if ok {
		t, err := c.setFocusToTab(t)
		return t, err
	}

	t = c.newTab(resource, name, h, nil)
	return t, nil
}

// Resource returns an open resource or false if resource with uri is not open.
func (c *Component) Resource(uri workspaceapi.URI) (browser.Handler, bool) {
	return c.comp.Tab(uri)
}

func (c *Component) Window(id uint64) (browser.Window, bool) {
	return c.comp.Window(id)
}

// SetDefaultAttributes satisfies text.Editor.
func (c *Component) SetDefaultAttributes(h Handler, attr term.Attributes) error {
	return c.ed.SetDefaultAttributes(h, attr)
}

// Close closes all resources associated with this Component.
func (c *Component) Close() error {
	// avoid dispatching close events on flusherCloser callbacks
	c.edSubscribers = make(map[textapi.EventType][]EventHandler)

	err := c.comp.Close()
	if err != nil {
		c.log(log.ErrorLevel, "browser.Component.Close error: %v", err)
		return err
	}
	return nil
}
