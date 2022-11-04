package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ernestrc/blue/document"
	"github.com/ernestrc/blue/retry"
	multierr "github.com/ernestrc/go-multierror"
	"unstable.build/go-tui"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

const (
	commandHistoryDocumentID = "ex-command-history"
	reissuePadding           = 10 * time.Millisecond
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
		"switchToWorkspace":      {}, // workspace_handler
	}
	exCommands = map[string]func(*ex, ...string) error{
		"bufferPrev":             (*ex).previousBuffer,
		"bufferNext":             (*ex).nextBuffer,
		"bufferClose":            (*ex).closeBuffer,
		"bufferCloseAll":         (*ex).closeAllBuffers,
		"close":                  (*ex).closeFocusWindow,
		"writeQuit":              (*ex).flushCloseIgnoreNonFlushed,
		"writeForceQuit!":        (*ex).flushCloseIgnoreNonFlushed,
		"write":                  (*ex).forceFlush,
		"forceWrite!":            (*ex).forceFlush,
		"forceQuit!":             (*ex).forceQuit,
		"quit":                   (*ex).forceQuit,
		"edit":                   (*ex).editFile,
		"reload":                 (*ex).reloadFile,
		"changeSplitOrientation": (*ex).splitDirectionChange,
		"splitWindow":            (*ex).newWindow,
		"newWindow":              (*ex).newWindow,
		"focusNextWindow":        (*ex).focusNextWindow,
		"focusPrevWindow":        (*ex).focusPrevWindow,
		"focusAboveWindow":       (*ex).focusAboveWindow,
		"focusBelowWindow":       (*ex).focusBelowWindow,
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
			Last: term.KeyComb{Key: term.KeyCtrlW}}: {"close"},
	}
	errEventStreamNotReady = errors.New("event stream not ready to publish")
	forcePublishRetry      = retry.SequentialStrategy(5 * time.Millisecond)
)

type mode int8

const (
	modeDefault mode = iota
	modeCommand
)

// ex implements a tui.Handler by wrapping an editor.Component and
// providing an ex editor type of interface.
type ex struct {
	config               text.Config
	comp                 text.Component
	ed                   text.Editor
	workspace            workspace.Loader
	sequencer            handler.Sequencer
	publishEvent         func(term.Event) bool
	cmd                  commandListHandler
	overlay              component.Overlay
	mode                 mode
	cancelPartialReissue func()
	ctxPartialReissue    context.Context
	reissueEvent         term.Event
	quit                 bool
}

func newEx(
	ed text.Editor, m workspace.Loader,
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

func forcePublishEvent(publishEvent func(term.Event) bool) func(ev term.Event) {
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
	ed text.Editor, m workspace.Loader,
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
	e.resetCommandList()
	e.cmd.loadHistory()
	return err
}

func (e *ex) subscribeCommands() error {
	var ret error
	for cmd, _fn := range exCommands {
		fn := _fn
		err := e.comp.SubscribeCommand(cmd, text.FuncCommandHandler(
			func(ctx context.Context, cmd text.Command) (bool, error) {
				return false, fn(e, cmd.Args...)
			}))
		if err != nil {
			ret = multierr.Append(ret, err)
		}
	}
	return ret
}

func (e *ex) publishInterrupt() {
	e.publishEvent(term.Event{Type: term.EventInterrupt})
}

func (e *ex) doInit(
	ed text.Editor, m workspace.Loader,
	storage document.Service,
	publishEvent func(term.Event) bool,
	opts ...text.Option,
) (err error) {
	e.mode = modeDefault
	e.workspace = m
	e.publishEvent = publishEvent

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

	if e.config.CommandOverlay.Width <= 0 || e.config.CommandOverlay.Height <= 0 {
		msg := fmt.Sprintf("invalid CommandOverlay dimensions: %v",
			e.config.CommandOverlay)
		panic(msg)
	}

	e.cmd.init(storage, e.config.CommandMaxHistory,
		e.config.CommandOverlay, e.config.CommandEvent,
		func(command string, cmdAndArgs string) bool {
			parts := strings.Split(cmdAndArgs, " ")
			quit, err := e.runCommand(string(command), parts)
			e.setProxyMode()
			if err != nil {
				e.setError(err)
			}
			return quit
		}, e.publishInterrupt)

	var commandOverlay tui.Component
	if e.config.CommandOverlay.Frame {
		frame := component.NewFrame(&e.cmd)
		frame.FrameCharSet = e.config.CommandOverlay.FrameCharSet
		frame.Attributes = e.config.CommandOverlay.FrameAttributes
		commandOverlay = frame
	} else {
		commandOverlay = &e.cmd
	}

	e.overlay.Init(&e.comp, commandOverlay,
		e.config.CommandOverlay.ElementAttr,
		component.SpanConfig{
			PadVertical:      -e.config.CommandOverlay.Height,
			PadHorizontal:    -e.config.CommandOverlay.Width,
			ContentAlignment: component.SpanAlignmentCentered,
		})

	e.ed = ed
	e.cleanPartialReissueState()
	return
}

func (e *ex) handlerInFocus() (workspace.URI, text.Handler, bool) {
	focus, _ := e.comp.Focus()
	content, _ := focus.Content()
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
	b.PreviousTab(b.Focus())
	return nil
}

func (e *ex) nextBuffer(args ...string) error {
	b := e.comp.Browser()
	b.NextTab(b.Focus())
	return nil
}

func (e *ex) closeBuffer(args ...string) error {
	b := e.comp.Browser()
	b.RemoveWindowContent(b.Focus())
	return nil
}

func (e *ex) closeAllBuffers(args ...string) error {
	b := e.comp.Browser()
	b.RemoveAllTabs()
	return nil
}

func (e *ex) closeFocusWindow(args ...string) error {
	b := e.comp.Browser()
	return b.Focus().Close()
}

func (e *ex) flushCloseIgnoreNonFlushed(args ...string) error {
	b := e.comp.Browser()
	e.quit = true
	return e.comp.Flush(b.Focus())
}

func (e *ex) forceFlush(args ...string) error {
	b := e.comp.Browser()
	return e.comp.Flush(b.Focus())
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
	h, err := e.comp.Open(uri)
	if err != nil {
		return err
	}
	err = e.comp.Browser().Focus().SetContent(h)
	if err == browser.ErrTabNotFree {
		err = nil
	}
	return err
}

func (e *ex) editFile(args ...string) error {
	if len(args) == 0 {
		return errors.New("expected file name")
	}
	// attempt to parse URI otherwise expect local file path
	uri, err := workspace.ParseURI(args[0])
	if err != nil {
		uri, err = e.workspace.URI(args[0])
		if err != nil {
			return err
		}
	}
	return e.editFileURI(uri)
}

func (e *ex) reloadFile(args ...string) error {
	b := e.comp.Browser()
	focus := b.Focus()
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
	eb.Split(browser.OrientationDefault, h)
}

func (e *ex) focusNextWindow(args ...string) error {
	e.comp.Browser().FocusRight()
	return nil
}

func (e *ex) focusPrevWindow(args ...string) error {
	e.comp.Browser().FocusLeft()
	return nil
}

func (e *ex) focusAboveWindow(args ...string) error {
	e.comp.Browser().FocusUp()
	return nil
}

func (e *ex) focusBelowWindow(args ...string) error {
	e.comp.Browser().FocusDown()
	return nil
}

func (e *ex) newWindow(args ...string) error {
	e.newWindowHandler(nil)
	return nil
}

func (e *ex) runCommand(cmd string, parts []string) (quit bool, err error) {
	// check to workaround default :<number> command to go to line:
	// cmd is empty because it didn't match any command in the list
	// but default behaviour is to move cursor to line
	if cmd != "" {
		parts[0] = cmd // cmdAndArgs contains fuzzy completed command
	}

	if len(parts) == 1 {
		line, cerr := strconv.Atoi(parts[0])
		if cerr == nil {
			err = e.moveFocusCursor(line - 1)
			return
		}
		return e.quit, e.dispatchCommand(parts[0])
	}

	return e.quit, e.dispatchCommand(parts[0], parts[1:]...)
}

func (e *ex) setError(err error) {
	e.comp.Browser().SetMessage("%s", err)
}

func (e *ex) handleCommandEvent(ev term.Event) bool {
	if ev.KeyComb() == e.config.CommandEvent {
		e.setCommandMode()
		return true
	}
	return false
}

func isWindowControlCommand(cmdAndArgs []string) bool {
	cmd := cmdAndArgs[0]
	_, ok := windowControlCommands[cmd]
	return ok
}

func (e *ex) handleProxy(ev term.Event) (
	exit, handled bool,
) {
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
					forcePublishEvent(e.publishEvent)(ev)
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
		cmdAndArgs, ok = e.comp.KeyMapping(ev.KeyComb())
	}

	// first dispatch window control commands
	if len(cmdAndArgs) != 0 && isWindowControlCommand(cmdAndArgs) {
		quit, err := e.runCommand(cmdAndArgs[0], cmdAndArgs)
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
		quit, err := e.runCommand(cmdAndArgs[0], cmdAndArgs)
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

func (e *ex) setProxyMode() {
	e.cmd.reset()
	e.mode = modeDefault
}

func (e *ex) resetCommandList() {
	// commands can be registered dynamicall via Editor.Register:
	// compile a new list every time we switch to command mode
	var commands []string
	for _, cmd := range e.comp.Commands() {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)
	e.cmd.dataReset(commands)
}

func (e *ex) setCommandMode() {
	e.mode = modeCommand
	e.resetCommandList()
}

// Handle satisfies tui.Handler.
func (e *ex) Handle(ev term.Event) (bool, bool) {
	switch e.mode {
	case modeDefault:
		return e.handleProxy(ev)
	case modeCommand:
		quit, handled := e.cmd.Handle(ev)
		if quit {
			// hack to signal proc exit
			if !handled {
				return true, true
			}
			e.setProxyMode()
		}
		return false, handled
	default:
		panic(fmt.Sprintf("unknown mode: %+v", e.mode))
	}
}

func (e *ex) overlayPosition() (pos term.Coordinates) {
	pos = e.overlay.ContentOffset()
	if e.config.CommandOverlay.Frame {
		pos.Y++
		pos.X++
	}
	return
}

// Cursor satisfies tui.Handler.
func (e *ex) Cursor() (pos term.Coordinates, show bool) {
	if e.mode == modeCommand {
		overlay := e.overlayPosition()
		pos, show = e.cmd.Cursor()
		pos.X += overlay.X
		pos.Y += overlay.Y
		return
	}
	return e.comp.Browser().Cursor()
}

// Man satisfies tui.Handler.
func (e *ex) Man() tui.Manual {
	panic("TODO")
}

// Resize satisfies tui.Component
func (e *ex) Resize(width, height int) {
	e.overlay.Resize(width, height)
}

// Draw satisfies tui.Component
func (e *ex) Draw(w term.Writer) {
	if e.mode == modeCommand {
		e.overlay.Draw(w)
		return
	}
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
	if err := e.cmd.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if err := e.comp.Close(); err != nil {
		ret = multierr.Append(ret, err)
	}
	if e.cancelPartialReissue != nil {
		e.cancelPartialReissue()
		e.cleanPartialReissueState()
	}
	return
}
