package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/handler/search"
	"github.com/ernestrc/go-tui/term"
	"github.com/ernestrc/go-tui/text"
	"github.com/ernestrc/go-tui/workspace"
)

var (
	commandBarAttr      = term.Attributes{Bg: term.ColorWhite, Fg: term.ColorBlack}
	errInvalidSetCursor = errors.New("Cannot set cursor on this buffer")
	exCommands          = map[string]func(*Ex, ...string) (bool, error){
		"bufferPrev":      (*Ex).previousBuffer,
		"bufferNext":      (*Ex).nextBuffer,
		"bufferClose":     (*Ex).closeBuffer,
		"bufferCloseAll":  (*Ex).closeAllBuffers,
		"close":           (*Ex).closeFocusWindow,
		"writeQuit":       (*Ex).flushCloseIgnoreNonFlushed,
		"writeForceQuit!": (*Ex).flushCloseIgnoreNonFlushed,
		"write":           (*Ex).forceFlush,
		"forceWrite!":     (*Ex).forceFlush,
		"forceQuit!":      (*Ex).forceQuit,
		"quit":            (*Ex).forceQuit,
		"edit":            (*Ex).editFile,
	}
)

type mode int8

const (
	modeDefault mode = iota
	modeCommand
)

// Ex implements a tui.Handler by wrapping an editor.Component and
// providing an ex editor type of interface.
type Ex struct {
	config  text.Config
	comp    text.Component
	ed      text.Editor
	command struct {
		// argsStartIdx is the position of the first space
		// that separates the 'command' from its args
		argsStartIdx int

		component.Responsive
		component.Virtual
		cell.Buffer

		search.List
		component.Frame
		component.Overlay
	}
	sequencer handler.Sequencer
	mode      mode
}

// NewEx allocates storage for a new Ex and initializes it.
func NewEx(ed text.Editor, opts ...text.Option) (e *Ex, err error) {
	e = new(Ex)
	err = e.Init(ed, opts...)
	if err != nil {
		return
	}
	return
}

// Init initializes this Ex with the given editor and Options.
// It returns an error if an initial filepath was given through WithFilePath option
// and the file failed to be opened.
func (e *Ex) Init(ed text.Editor, opts ...text.Option) (err error) {
	e.mode = modeDefault

	e.config = text.DefaultConfig()
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

	// overlay buffer over the search list so we can
	// stop the search for multiple argument commands
	// but we can display arguments
	e.command.Buffer.Init()
	responsive := component.BufferResponsive(&e.command.Buffer,
		component.StringConfig{})
	e.command.Responsive = responsive
	e.command.Virtual.C = responsive

	var commandOverlay tui.Component
	if e.config.CommandOverlay.Frame {
		commandOverlay = &e.command.Frame
		e.command.Frame.Init(&e.command.List)
	} else {
		commandOverlay = &e.command.List
	}

	cfg := search.ListConfig{
		Algo:             search.FuzzyMatch,
		Interrupt:        term.Interrupt,
		CaseSensitive:    false,
		MatchedTextAttr:  &e.config.CommandOverlay.MatchedTextAttr,
		CountAttr:        &e.config.CommandOverlay.CountAttr,
		FocusElementAttr: &e.config.CommandOverlay.FocusElementAttr,
		ElementAttr:      &e.config.CommandOverlay.ElementAttr,
	}

	e.command.List.Init(cfg)
	e.command.Overlay.Init(&e.comp, commandOverlay,
		e.config.CommandOverlay.ElementAttr,
		component.SpanConfig{
			PadVertical:      -e.config.CommandOverlay.Height,
			PadHorizontal:    -e.config.CommandOverlay.Width,
			ContentAlignment: component.SpanAlignmentCentered,
		})
	e.ed = ed
	err = e.comp.Init(ed, e.config)
	return
}

func (e *Ex) handlerInFocus() (workspace.URI, text.Handler, bool) {
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

func (e *Ex) moveFocusCursor(line int) error {
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

func (e *Ex) previousBuffer(args ...string) (bool, error) {
	b := e.comp.Browser()
	b.EditWindowTabPrev(b.Focus())
	return false, nil
}

func (e *Ex) nextBuffer(args ...string) (bool, error) {
	b := e.comp.Browser()
	b.EditWindowTabNext(b.Focus())
	return false, nil
}

func (e *Ex) closeBuffer(args ...string) (bool, error) {
	b := e.comp.Browser()
	b.RemoveWindowContent(b.Focus())
	return false, nil
}

func (e *Ex) closeAllBuffers(args ...string) (bool, error) {
	b := e.comp.Browser()
	b.RemoveAllTabs()
	return false, nil
}

func (e *Ex) closeFocusWindow(args ...string) (bool, error) {
	b := e.comp.Browser()
	return false, b.Focus().Close()
}

func (e *Ex) flushCloseIgnoreNonFlushed(args ...string) (bool, error) {
	b := e.comp.Browser()
	return true, e.comp.Flush(b.Focus())
}

func (e *Ex) forceFlush(args ...string) (bool, error) {
	b := e.comp.Browser()
	return false, e.comp.Flush(b.Focus())
}

func (e *Ex) forceQuit(args ...string) (bool, error) {
	return true, nil
}

func (e *Ex) dispatchCommand(cmd string, args ...string) (err error) {
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
	handled := e.comp.DispatchCommand(scmd)
	if !handled {
		err = fmt.Errorf("Unknown command: '%s'", cmd)
	}
	return
}

func (e *Ex) runSingleCommand(cmd string) (quit bool, err error) {
	fnCmd, ok := exCommands[cmd]
	if ok {
		return fnCmd(e)
	}

	line, cerr := strconv.Atoi(cmd)
	if cerr == nil {
		err = e.moveFocusCursor(line - 1)
		return
	}
	return false, e.dispatchCommand(cmd)
}

func (e *Ex) editFile(args ...string) (bool, error) {
	if len(args) == 0 {
		return false, errors.New("Expected file name")
	}
	uri, err := workspace.LocalURI(args[0])
	if err != nil {
		return false, err
	}
	h, err := e.comp.Open(uri)
	if err != nil {
		return false, err
	}
	err = e.comp.Browser().Focus().SetContent(h)
	if err == browser.ErrTabNotFree {
		return false, nil
	}
	return false, err
}

func (e *Ex) runCommand(cmd string, cmdAndArgs string) (quit bool, err error) {
	parts := strings.Split(cmdAndArgs, " ")
	if len(parts) == 1 {
		if len(cmd) == 0 {
			cmd = cmdAndArgs
		}
		return e.runSingleCommand(cmd)
	}

	fnCmd, ok := exCommands[cmd]
	if ok {
		return fnCmd(e, parts[1:]...)
	}

	return false, e.dispatchCommand(cmd, parts[1:]...)
}

func (e *Ex) setError(err error) {
	e.comp.Browser().SetMessage("Error: %s", err)
}

func (e *Ex) handleCommand(ev term.Event) (quit, handled bool) {
	handled = true

	switch ev.Key {
	case term.KeyEnter:
		// before selecting command wait for previous search to finish
		e.command.List.Wait()

		command, _ := e.command.List.Focus()
		var err error
		quit, err = e.runCommand(string(command), e.command.Buffer.String())
		e.setNormalMode()
		if err != nil {
			e.setError(err)
		}
	case term.KeyEsc:
		e.setNormalMode()
	case term.KeyArrowDown:
		e.command.List.FocusDown()
	case term.KeyArrowUp:
		e.command.List.FocusUp()
	case term.KeySpace:
		ev.Ch = ' '
		handled = false
	case term.KeyBackspace, term.KeyBackspace2:
		cols := e.command.Buffer.Columns(0)
		if cols == 0 {
			e.setNormalMode()
			return
		}
		e.command.Buffer.DeleteCell(term.Coordinates{X: cols - 1})
		if e.command.argsStartIdx == e.command.Buffer.Size() ||
			e.command.argsStartIdx == 0 {
			e.command.List.Buffer().Reset()
			e.command.List.Buffer().WriteString(e.command.Buffer.String())
			e.command.argsStartIdx = 0
		}
	default:
		handled = false
	}

	if handled {
		return
	}

	if ev.Ch == 0 {
		return
	}

	handled = true
	if ev.Ch == ' ' && e.command.argsStartIdx == 0 {
		e.command.argsStartIdx = e.command.Buffer.Size()
	}
	e.command.Buffer.WriteString(string([]rune{ev.Ch}))

	if e.command.argsStartIdx == 0 {
		e.command.List.Buffer().WriteString(string(ev.Ch))
	}
	return
}

func (e *Ex) handleCommandEvent(ev term.Event) bool {
	if ev == e.config.CommandEvent {
		e.setCommandMode()
		return true
	}
	return false
}

func (e *Ex) handleProxy(ev term.Event) (
	exit, handled bool,
) {
	// If Ex is configured with non character
	// command mode trigger event, then this takes
	// precedence over any other event
	if e.config.CommandEvent.Ch == 0 {
		handled = e.handleCommandEvent(ev)
		if handled {
			return
		}
	}

	b := e.comp.Browser()

	// first map event, and map to potential command
	mev, cmd, ok := e.comp.KeyMapping(ev)

	// if event sequence has a match though
	// then priority is to dispatch sequence command
	seq, match := e.sequencer.Handle(mev)
	if match {
		cmd, ok = e.config.CommandSequenceBindings[seq]
		if !ok {
			panic("key sequencer matched but no command configured")
		}
	}

	// dispatch either sequence or event command and
	// and dispatch event as well
	if cmd != "" {
		quit, err := e.runCommand(cmd, cmd)
		if err != nil {
			e.setError(err)
		}
		_, _ = b.Handle(mev)
		return quit, true
	}

	switch mev.Key {
	case term.KeyCtrlA:
		b.RemoveAllTabs()
	case term.KeyCtrlW:
		b.RemoveWindowContent(b.Focus())
	case term.KeyCtrlL:
		b.EditWindowTabNext(b.Focus())
	case term.KeyCtrlH:
		b.EditWindowTabPrev(b.Focus())
	default:
		_, handled = b.Handle(mev)
		if handled {
			return
		}
		// If ex is configured with character
		// command mode trigger event (i.e. ':')
		// then we assume that the underlying editor is
		// a modal editor, and so does not handle
		// the command trigger event in its "initial" mode
		// (in vi terms, this would be normal mode).
		handled = e.handleCommandEvent(mev)
		return
	}

	return false, true
}

func (e *Ex) setNormalMode() {
	e.command.Buffer.Reset()
	e.command.List.Buffer().Reset()
	e.command.List.Wait()
	e.command.argsStartIdx = 0
	e.mode = modeDefault
}

func (e *Ex) setCommandMode() {
	e.mode = modeCommand

	// commands can be registered dynamicall via Editor.Register:
	// compile a new list every time we switch to command mode
	e.command.List.DataReset()
	for cmd := range exCommands {
		e.command.List.PushSync([]byte(cmd))
	}
	for _, cmd := range e.comp.Commands() {
		e.command.List.PushSync([]byte(cmd))
	}
}

// Handle satisfies tui.Handler.
func (e *Ex) Handle(ev term.Event) (bool, bool) {
	switch e.mode {
	case modeDefault:
		return e.handleProxy(ev)
	case modeCommand:
		return e.handleCommand(ev)
	default:
		panic(fmt.Sprintf("unknown mode: %+v", e.mode))
	}
}

func (e *Ex) overlayPosition() (pos term.Coordinates) {
	pos = e.command.Overlay.ContentOffset()
	if e.config.CommandOverlay.Frame {
		pos.Y++
		pos.X++
	}
	return
}

func (e *Ex) commandOverlayDimensions() (width, height int) {
	width, height = e.config.CommandOverlay.Width, e.config.CommandOverlay.Height
	if e.config.CommandOverlay.Frame && width > 2 && height > 2 {
		width -= 2
		height -= 2
	}
	return
}

// Cursor satisfies tui.Handler.
func (e *Ex) Cursor() (pos term.Coordinates, show bool) {
	if e.mode == modeCommand {
		pos := e.command.Virtual.Position()
		cmdWidth, cmdHeight := e.commandOverlayDimensions()
		x := len(e.command.Buffer.String()) % cmdWidth
		y := len(e.command.Buffer.String()) / cmdWidth
		if y >= cmdHeight {
			pos.X += cmdWidth - 1
			pos.Y += cmdHeight - 1
		} else {
			pos.X += x
			pos.Y += y
		}
		return pos, true
	}
	return e.comp.Browser().Cursor()
}

// Man satisfies tui.Handler.
func (e *Ex) Man() tui.Manual {
	panic("TODO")
}

// Resize satisfies tui.Component
func (e *Ex) Resize(width, height int) {
	// internally resizes e.comp
	e.command.Overlay.Resize(width, height)

	pos := e.overlayPosition()
	e.command.Virtual.Move(pos)
}

func (e *Ex) resizeCommandOverlay() {
	// propagate local cmd+args buffer height to
	// search list, which only has cmd, in case args alone span
	// multiple lines
	cmdWidth, cmdHeight := e.commandOverlayDimensions()
	height := e.command.Responsive.Height(cmdWidth)
	// set to min 1, as it's being used as input field
	// and max to the height of the overlayed component
	height = int(math.Min(math.Max(1, float64(height)), float64(cmdHeight)))
	// the list is very short so waiting is not a significant
	// perf penalty and it makes tests easier to make deterministic
	e.command.List.Wait()
	e.command.List.SetMinInputHeight(height)
	e.command.Virtual.Resize(cmdWidth, height)
}

// Draw satisfies tui.Component
func (e *Ex) Draw(w term.Writer) {
	if e.mode == modeCommand {
		// resize on every draw because search.List uses a responsive
		// input so local buffer changes must consider potential resize
		// of search.List
		e.resizeCommandOverlay()
		e.command.Overlay.Draw(w)
		e.command.Virtual.Draw(w)
		return
	}
	e.comp.Draw(w)
}

// Editor returns the underlying Editor implementation.
func (e *Ex) Editor() text.Editor {
	return &e.comp
}

// Browser returns the underlying browser.Browser implementaiton.
func (e *Ex) Browser() browser.Browser {
	return &e.comp
}

// Close closes the resources associated with this browser.
func (e *Ex) Close() error {
	e.sequencer.Reset()
	err1 := e.command.List.Close()
	err2 := e.comp.Close()
	if err2 != nil {
		return err2
	}
	return err1
}
