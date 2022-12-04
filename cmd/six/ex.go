package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/iterator"
	"github.com/ernestrc/blue/logging"
	"github.com/ernestrc/blue/retry"
	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/handler/command"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

const (
	commandHistoryDocumentID  = "ex-command-history"
	reissuePadding            = 10 * time.Millisecond
	cmdEdit                   = "edit"
	cmdChangeSplitOrientation = "changeSplitOrientation"
)

var (
	commandBarAttr        = term.Attributes{Bg: term.ColorWhite, Fg: term.ColorBlack}
	errInvalidSetCursor   = errors.New("Cannot set cursor on this buffer")
	windowControlCommands = map[string]struct{}{
		"changeSplitOrientation": {},
		"splitWindow":            {},
		"newWindow":              {},
		"focusNextWindow":        {},
		"focusPrevWindow":        {},
		"focusAboveWindow":       {},
		"focusBelowWindow":       {},
		cmdSwitchToWorkspace:     {}, // workspace_handler
	}
	exCommands = map[string]func(*ex, ...string) error{
		"bufferPrev":              (*ex).previousBuffer,
		"bufferNext":              (*ex).nextBuffer,
		"bufferClose":             (*ex).closeBuffer,
		"bufferCloseAll":          (*ex).closeAllBuffers,
		"closeWindow":             (*ex).closeFocusWindow,
		"writeQuit":               (*ex).flushCloseIgnoreNonFlushed,
		"writeForceQuit!":         (*ex).flushCloseIgnoreNonFlushed,
		"write":                   (*ex).forceFlush,
		"forceWrite!":             (*ex).forceFlush,
		"forceQuit!":              (*ex).forceQuit,
		"quit":                    (*ex).forceQuit,
		cmdEdit:                   (*ex).editFiles,
		"reload":                  (*ex).reloadFile,
		cmdChangeSplitOrientation: (*ex).splitDirectionChange,
		"splitWindow":             (*ex).newWindow,
		"newWindow":               (*ex).newWindow,
		"focusNextWindow":         (*ex).focusNextWindow,
		"focusPrevWindow":         (*ex).focusPrevWindow,
		"focusAboveWindow":        (*ex).focusAboveWindow,
		"focusBelowWindow":        (*ex).focusBelowWindow,
		"panic":                   (*ex).panic,
	}
	exDefaultBindings = map[term.KeyComb]string{
		{Key: term.KeyCtrlW}: "bufferClose",
		{Key: term.KeyCtrlL}: "bufferNext",
		{Key: term.KeyCtrlH}: "bufferPrev",
	}
	exDefaultSequences = map[handler.Sequence][]string{
		{First: term.KeyComb{Key: term.KeyCtrlX},
			Last: term.KeyComb{Key: term.KeyEnter}}: {"newWindow"},
		{First: term.KeyComb{Key: term.KeyCtrlX},
			Last: term.KeyComb{Key: term.KeyCtrlH}}: {"focusPrevWindow"},
		{First: term.KeyComb{Key: term.KeyCtrlX},
			Last: term.KeyComb{Key: term.KeyCtrlL}}: {"focusNextWindow"},
		{First: term.KeyComb{Key: term.KeyCtrlX},
			Last: term.KeyComb{Key: term.KeyCtrlJ}}: {"focusBelowWindow"},
		{First: term.KeyComb{Key: term.KeyCtrlX},
			Last: term.KeyComb{Key: term.KeyCtrlK}}: {"focusAboveWindow"},
		{First: term.KeyComb{Key: term.KeyCtrlX},
			Last: term.KeyComb{Ch: 'h'}}: {"changeSplitOrientation", "h"},
		{First: term.KeyComb{Key: term.KeyCtrlX},
			Last: term.KeyComb{Ch: 'v'}}: {"changeSplitOrientation", "v"},
		{First: term.KeyComb{Key: term.KeyCtrlX},
			Last: term.KeyComb{Key: term.KeyCtrlW}}: {"closeWindow"},
	}
	errEventStreamNotReady = errors.New("event stream not ready to publish")
	forcePublishRetry      = retry.SequentialStrategy(5 * time.Millisecond)
)

type workspaceLoader interface {
	workspace.Loader
	workspace.WorkspaceDirectory
}

// ex implements a tui.Handler by wrapping an editor.Component and
// providing an ex editor type of interface.
type ex struct {
	config               text.Config
	comp                 text.Component
	ed                   text.Editor
	storage              document.Service
	workspace            workspaceLoader
	sequencer            handler.Sequencer
	publishEvent         func(term.Event) bool
	cancelPartialReissue func()
	ctxPartialReissue    context.Context
	reissueEvent         term.Event
	cmd                  *command.Prompt
	cmdWin               browser.Window
	noResetFocus         bool
	quit                 bool
	height               int
}

func newEx(
	ed text.Editor, m workspaceLoader,
	storage document.Service,
	publishEvent func(term.Event) bool,
	opts ...text.Option,
) (e *ex, err error) {
	e = new(ex)
	err = e.init(ed, m, storage, publishEvent, opts...)
	if err != nil {
		return
	}
	return
}

func forcePublishEvent(publishEvent func(term.Event) bool) func(term.Event) {
	return func(ev term.Event) {
		retry.Retry(context.Background(), forcePublishRetry, func(context.Context) (bool, error) {
			ok := publishEvent(ev)
			return !ok, errEventStreamNotReady
		})
	}
}

// init initializes this ex with the given editor and Options.
// It returns an error if an initial filepath was given through WithFilePath option
// and the file failed to be opened.
func (e *ex) init(
	ed text.Editor, m workspaceLoader,
	storage document.Service,
	publishEvent func(term.Event) bool,
	opts ...text.Option,
) (err error) {
	err = e.doInit(ed, m, storage, publishEvent, opts...)
	if err != nil {
		return
	}
	err = e.comp.Init(ed, m, e.config)
	if err != nil {
		return
	}
	return err
}

func (e *ex) subscribeCommands() error {
	var ret error
	for cmd, fn := range exCommands {
		cmd := cmd
		fn := fn
		err := e.comp.SubscribeCommand(cmd, text.FuncCommandCompleter(
			func(ctx context.Context, cmd text.Command) (bool, error) {
				return false, fn(e, cmd.Args...)
			}, func(ctx context.Context, args []string) (iterator.Iterator[string], error) {
				return e.completeCommand(ctx, cmd, args)
			}))
		if err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return ret
}

// FIXME ~/ doesn't really work because command handler doesn't update
// the path used to fuzzy search so it fuzzy searches ~/ against /home/user
// TODO update editFiles completer to expand and somehow writeback to command handler
func (e *ex) completeEdit(
	ctx context.Context, args []string,
) (iterator.Iterator[string], error) {
	if len(args) == 0 {
		return workspace.ListFiles(ctx, e.workspace, ".")
	}

	last := args[len(args)-1]
	if last == "" {
		return workspace.ListFiles(ctx, e.workspace, ".")
	}

	uri, err := e.parseURIOrWorkspaceURI(last)
	if err != nil {
		return nil, err
	}

	return workspace.ListFiles(ctx, e.workspace, uri.Path())
}

func (e *ex) log(level log.Level, msg string, args ...interface{}) {
	log.WithField(logging.KeyClass, "ex").Logf(level, msg, args...)
}

func (e *ex) completeCommand(
	ctx context.Context, cmd string, args []string,
) (iterator.Iterator[string], error) {
	e.log(log.DebugLevel, "complete command: %s %v", cmd, args)
	switch cmd {
	case cmdEdit:
		return e.completeEdit(ctx, args)
	case cmdChangeSplitOrientation:
		return iterator.FromSlice([]string{"horizontal", "vertical"}), nil
	default:
		return iterator.FromSlice[string](nil), nil
	}
}

func (e *ex) Interrupt() error {
	if !e.publishEvent(term.Event{Type: term.EventInterrupt}) {
		return errors.New("event stream not ready")
	}
	return nil
}

func (e *ex) doInit(
	ed text.Editor, m workspaceLoader,
	storage document.Service,
	publishEvent func(term.Event) bool,
	opts ...text.Option,
) (err error) {
	e.workspace = m
	e.publishEvent = publishEvent
	e.storage = storage

	e.config = text.DefaultConfig()

	for ev, cmd := range exDefaultBindings {
		opts = append(opts, text.WithCommandKeyBinding(ev, []string{cmd}))
	}
	// write default sequences
	for seq, cmdAndArgs := range exDefaultSequences {
		e.config.CommandSequenceBindings[seq] = cmdAndArgs
	}

	for _, o := range opts {
		o(&e.config)
	}

	seqInterests := make([]handler.Sequence, 0,
		len(e.config.CommandSequenceBindings))
	for seq := range e.config.CommandSequenceBindings {
		seqInterests = append(seqInterests, seq)
	}
	e.sequencer.Init(seqInterests, e.config.SequencerTimeout)

	e.ed = ed
	e.cleanPartialReissueState()
	return
}

func (e *ex) Wait() {
	if e.cmd != nil {
		e.cmd.Wait()
	}
}

// Complete satisfies command.Completer for command.Handler.
func (e *ex) Complete(ctx context.Context, cmd string, args ...string) iterator.Iterator[string] {
	it, err := e.comp.CompleteCommand(ctx, cmd, args...)
	if err != nil {
		e.setError(fmt.Errorf("complete command: %v", err))
		return iterator.FromSlice[string](nil)
	}
	return it
}

// Dispatch satisfies command.Dispatcher for command.Handler.
func (e *ex) Dispatch(command string, args ...string) bool {
	quit, err := e.runCommand(command, args)
	if err != nil {
		e.setError(err)
	}
	return quit
}

func (e *ex) invokeWindow() browser.Window {
	if e.cmd != nil {
		return e.cmdWin
	}
	ret, _ := e.comp.Focus()
	return ret
}

func (e *ex) handlerInFocus() (workspace.URI, text.Handler, bool) {
	content, _ := e.invokeWindow().Content()
	t, ok := content.(*browser.Tab)
	if !ok {
		return workspace.URI{}, nil, false
	}
	ret, ok := t.Handler().(text.Handler)
	if !ok {
		return workspace.URI{}, nil, false
	}
	return t.URI(), ret, true
}

func (e *ex) moveFocusCursor(line int) error {
	_, h, ok := e.handlerInFocus()
	if !ok {
		return errInvalidSetCursor
	}
	// silently correct invalid line numbers
	// NOTE: this won't work if we implement +/- relative
	// line go to i.e. :+1, :-20
	if line < 0 {
		line = 0
	}
	return e.comp.SetCursor(h, term.Coordinates{Y: line})
}

func (e *ex) previousBuffer(args ...string) error {
	b := e.comp.Browser()
	b.PreviousTab(e.invokeWindow())
	return nil
}

func (e *ex) nextBuffer(args ...string) error {
	b := e.comp.Browser()
	b.NextTab(e.invokeWindow())
	return nil
}

func (e *ex) closeBuffer(args ...string) error {
	b := e.comp.Browser()
	b.RemoveWindowContent(e.invokeWindow())
	return nil
}

func (e *ex) closeAllBuffers(args ...string) error {
	b := e.comp.Browser()
	b.RemoveAllTabs()
	return nil
}

func (e *ex) closeFocusWindow(args ...string) error {
	if e.cmd != nil {
		e.noResetFocus = true
	}
	return e.invokeWindow().Close()
}

func (e *ex) flushCloseIgnoreNonFlushed(args ...string) error {
	e.quit = true
	return e.comp.Flush(e.invokeWindow())
}

func (e *ex) forceFlush(args ...string) error {
	return e.comp.Flush(e.invokeWindow())
}

func (e *ex) forceQuit(args ...string) error {
	e.quit = true
	return nil
}

func (e *ex) dispatchCommand(cmd string, args ...string) (err error) {
	uri, h, ok := e.handlerInFocus()
	scmd := text.Command{
		Name:     cmd,
		Args:     args,
		Resource: h,
		URI:      uri,
	}
	if ok {
		scmd.Cursor.Content, _ = e.ed.Cursor(h)
		scmd.Cursor.Window, _ = h.Cursor()
	}
	var handled bool
	handled, err = e.comp.DispatchCommand(scmd)
	if err != nil {
		return
	}
	if !handled {
		err = fmt.Errorf("Unknown command %q or alias targets", cmd)
	}
	return
}

func (e *ex) editFileURI(uri workspace.URI) error {
	e.log(log.DebugLevel, "edit: %s", uri.String())

	h, err := e.comp.Open(uri)
	if err != nil {
		return err
	}
	err = e.invokeWindow().SetContent(h)
	if err == browser.ErrTabNotFree {
		err = nil
	}
	return err
}

func (e *ex) parseURIOrWorkspaceURI(path string) (workspace.URI, error) {
	uri, err := workspace.ParseURI(path)
	e.log(log.TraceLevel, "parse uri (%s): %s, %v", path, uri.String(), err)
	if err != nil {
		uri, err = e.workspace.URI(path)
		e.log(log.TraceLevel, "URI (%s): %s, %v", path, uri.String(), err)
	}
	return uri, err
}

func (e *ex) editFiles(args ...string) error {
	if len(args) == 0 {
		return errors.New("expected at least one file name")
	}

	for _, arg := range args {
		// attempt to parse URI otherwise expect local file path
		uri, err := e.parseURIOrWorkspaceURI(arg)
		if err != nil {
			return err
		}
		err = e.editFileURI(uri)
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *ex) reloadFile(args ...string) error {
	b := e.comp.Browser()
	focus := e.invokeWindow()
	uri, _, ok := e.handlerInFocus()
	if !ok {
		return errors.New("not a file")
	}
	b.RemoveWindowContent(focus)
	return e.editFileURI(uri)
}

func (e *ex) splitDirectionChange(args ...string) error {
	if len(args) == 0 {
		return errors.New("expecting argument 'horizontal', 'h', 'vertical', 'v'")
	}

	b := e.comp.Browser()
	switch args[0] {
	case "horizontal", "h":
		b.SetDefaultSplit(browser.OrientationBottom)
		b.SetMessage("changed split direction to horizontal")
	case "vertical", "v":
		b.SetDefaultSplit(browser.OrientationRight)
		b.SetMessage("changed split direction to vertical")
	}
	return nil
}

func (e *ex) newWindowHandler(h browser.Handler) {
	eb := e.comp.Browser()
	eb.SetFocus(e.invokeWindow())
	eb.Split(browser.OrientationDefault, h)
}

func (e *ex) prepareFocusShift() {
	eb := e.comp.Browser()
	if e.cmd != nil {
		eb.Focus().Close()
		e.noResetFocus = true
	}
	eb.SetFocus(e.invokeWindow())
}

func (e *ex) focusNextWindow(args ...string) error {
	e.prepareFocusShift()
	e.comp.Browser().FocusRight()
	return nil
}

func (e *ex) focusPrevWindow(args ...string) error {
	e.prepareFocusShift()
	e.comp.Browser().FocusLeft()
	return nil
}

func (e *ex) focusAboveWindow(args ...string) error {
	e.prepareFocusShift()
	e.comp.Browser().FocusUp()
	return nil
}

func (e *ex) focusBelowWindow(args ...string) error {
	e.prepareFocusShift()
	e.comp.Browser().FocusDown()
	return nil
}

func (e *ex) panic(args ...string) error {
	panic("this could be a panic")
}

func (e *ex) newWindow(args ...string) error {
	e.newWindowHandler(nil)
	if e.cmd != nil {
		e.noResetFocus = true
	}
	return nil
}

func (e *ex) runCommand(cmd string, args []string) (quit bool, err error) {
	if len(args) == 0 {
		line, cerr := strconv.Atoi(cmd)
		if cerr == nil {
			err = e.moveFocusCursor(line - 1)
			return
		}
		return e.quit, e.dispatchCommand(cmd)
	}

	return e.quit, e.dispatchCommand(cmd, args...)
}

func (e *ex) setError(err error) {
	e.comp.Browser().SetMessage("%s", err)
}

func (e *ex) handleCommandEvent(ev term.Event) bool {
	if ev.KeyComb() == e.config.CommandEvent {
		e.openCommandPrompt()
		return true
	}
	return false
}

func isWindowControlCommand(cmdAndArgs []string) bool {
	cmd := cmdAndArgs[0]
	_, ok := windowControlCommands[cmd]
	return ok
}

func (e *ex) handleEvent(ev term.Event) (
	exit, handled bool,
) {
	if ev.Type == term.EventMouse {
		_, handled = e.comp.Browser().Handle(ev)
		return
	}

	// If ex is configured with non character
	// command mode trigger event, then this takes
	// precedence over any other event
	if e.config.CommandEvent.Ch == 0 {
		handled = e.handleCommandEvent(ev)
		if handled {
			return
		}
	}

	var seq handler.Sequence
	var match handler.SequenceMatchResult
	// err nil indicates that match is still valid as timer hasn't expired
	// and it was not canceled yet or simply it hasn't even started and
	// this is first event in sequence.
	if e.ctxPartialReissue.Err() == nil {
		seq, match = e.sequencer.Sequence(ev.KeyComb())
	}

	var cmdAndArgs []string
	var ok bool
	switch match {
	case handler.SequenceMatch:
		cmdAndArgs, ok = e.config.CommandSequenceBindings[seq]
		if !ok {
			panic("key sequencer matched but no command configured")
		}
		if e.cancelPartialReissue != nil {
			e.cancelPartialReissue()
			e.cleanPartialReissueState()
		}
	case handler.SequencePartialMatch:
		if e.cancelPartialReissue == nil {
			// this is not a re-issue of a partial command, so set
			// timer to re-issue if user doesn't complete sequence,
			// and if timer expires
			ctx := context.Background()
			e.ctxPartialReissue, e.cancelPartialReissue = context.WithTimeout(ctx,
				e.config.SequencerTimeout+reissuePadding)
			e.reissueEvent = ev
			go func(ctx context.Context) {
				<-ctx.Done()
				if ctx.Err() == context.DeadlineExceeded {
					// timer expired, reissue event because
					// user didn't send a matching key combination.
					forcePublishEvent(e.publishEvent)(e.reissueEvent)
				}
			}(e.ctxPartialReissue)
			return
		}
		// this is a re-issue so continue processing
	default:
		if e.cancelPartialReissue != nil {
			err := e.ctxPartialReissue.Err()
			e.cancelPartialReissue()
			e.cleanPartialReissueState()
			if err == nil {
				// issue previous event right before this next one
				// since we know now it's not a match.
				_, _ = e.comp.Browser().Handle(e.reissueEvent)
			}
		}
		cmdAndArgs, _ = e.comp.KeyMapping(ev.KeyComb())
	}

	// first dispatch window control commands
	if len(cmdAndArgs) != 0 && isWindowControlCommand(cmdAndArgs) {
		// make sure that the command is applied to the right window
		// and it needs to be set here to differentiate between
		// runCommand being called from command prompt
		// or from a key mapping event
		quit, err := e.runCommand(cmdAndArgs[0], cmdAndArgs[1:])
		if err != nil {
			e.setError(err)
		}
		return quit, true
	}

	if match != handler.SequenceMatch {
		// then the focus handler takes precedence
		b := e.comp.Browser()
		_, handled = b.Handle(ev)
		if handled {
			return
		}
	}

	// finally dispatch user event command or sequence command
	if len(cmdAndArgs) != 0 {
		quit, err := e.runCommand(cmdAndArgs[0], cmdAndArgs[1:])
		if err != nil {
			e.setError(err)
		}
		return quit, true
	}

	// If ex is configured with character
	// command mode trigger event (i.e. ':')
	// then we assume that the underlying editor is
	// a modal editor, and so does not handle
	// the command trigger event in its "initial" mode
	// (in vi terms, this would be normal mode).
	handled = e.handleCommandEvent(ev)
	if handled {
		return
	}
	return
}

func (e *ex) resetCommandList(cmd *command.Prompt) {
	// commands can be registered dynamicall via Editor.Register:
	// compile a new list every time we switch to command mode
	var commands []string
	for _, cmd := range e.comp.Commands() {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)
	cmd.Reset(commands)
}

func (e *ex) closeCommandPrompt() error {
	err := e.cmd.Close()
	// reverse focus to window before prompt, except
	// if command was a focus shifting command
	if e.noResetFocus {
		e.noResetFocus = false
	} else {
		e.comp.SetFocus(e.cmdWin)
	}
	e.cmd = nil
	return err
}

func (e *ex) openCommandPrompt() {
	commandCfg := command.Config{
		MaxHistory:       e.config.CommandMaxHistory,
		HistoryKey:       e.config.CommandEvent,
		MatchedTextAttr:  e.config.CommandOverlay.MatchedTextAttr,
		FocusElementAttr: e.config.CommandOverlay.FocusElementAttr,
		ElementAttr:      e.config.CommandOverlay.ElementAttr,
		DocumentID:       commandHistoryDocumentID,
	}
	cmd := command.NewPrompt(e.storage, e, e, e, []string{}, commandCfg)

	var commandHandler browser.Floating
	if e.config.CommandOverlay.Frame {
		frame := handler.NewFrame(cmd)
		frame.FrameCharSet = e.config.CommandOverlay.FrameCharSet
		frame.Attributes = e.config.CommandOverlay.FrameAttributes
		commandHandler = browser.FuncFloatingHandler(frame, e.closeCommandPrompt)
	} else {
		commandHandler = cmd
	}

	commandHandler = browser.FuncFloating(
		browser.FuncHandler(
			handler.WithComponent(commandHandler,
				component.WithBackground(
					commandHandler, term.Cell{Bg: e.config.CommandOverlay.ElementAttr.Bg},
				),
			), e.closeCommandPrompt),
		commandHandler.Dimensions,
	)
	e.cmdWin, _ = e.comp.Focus()
	e.resetCommandList(cmd)
	_, err := e.comp.Floating(commandHandler,
		component.FloatingConfig{
			Offset:    term.Coordinates{Y: e.height / 4},
			Alignment: component.SpanAlignmentHorizontallyCentered,
		})
	if err != nil {
		e.setError(err)
		return
	}
	e.cmd = cmd
	// reset in case last command was run via key mapping
	e.noResetFocus = false
}

// Handle satisfies tui.Handler.
func (e *ex) Handle(ev term.Event) (exit, handled bool) {
	if e.cmd != nil {
		_, handled = e.comp.Browser().Handle(ev)
	} else {
		_, handled = e.handleEvent(ev)
	}
	return e.quit, handled
}

// Cursor satisfies tui.Handler.
func (e *ex) Cursor() (pos term.Coordinates, show bool) {
	return e.comp.Browser().Cursor()
}

// Man satisfies tui.Handler.
func (e *ex) Man() tui.Manual {
	panic("TODO")
}

// Resize satisfies tui.Component
func (e *ex) Resize(width, height int) {
	e.height = height
	e.comp.Resize(width, height)
}

// Draw satisfies tui.Component
func (e *ex) Draw(w term.Writer) {
	e.comp.Draw(w)
}

// Editor returns the underlying Editor implementation.
func (e *ex) Editor() text.Editor {
	return &e.comp
}

// Browser returns the underlying browser.Browser implementaiton.
func (e *ex) Browser() browser.Browser {
	return &e.comp
}

func (e *ex) cleanPartialReissueState() {
	e.cancelPartialReissue = nil
	e.ctxPartialReissue = context.Background()
}

// Close closes the resources associated with this browser.
func (e *ex) Close() (ret error) {
	e.sequencer.Reset()
	if err := e.comp.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if e.cancelPartialReissue != nil {
		e.cancelPartialReissue()
		e.cleanPartialReissueState()
	}
	return
}
