package editor

import (
	"fmt"
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

// Ex satisfies browser.Browser and tui.Handler by wrapping a browser.Component
// to provide an ex editor interface.
type Ex struct {
	Component
	commandBuf *cell.Buffer
	cmdVirt    handler.Virtual
	mode       mode
}

// NewEx allocates storage for a new Ex and initializes it.
func NewEx(ed Editor, opts ...browser.Option) (e *Ex, err error) {
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
func (e *Ex) Init(ed Editor, opts ...browser.Option) (err error) {
	e.Component.Init(ed, opts...)

	e.commandBuf = cell.NewBuffer()
	e.cmdVirt = browser.NewMessageSpan(e.commandBuf, commandBarAttr)
	e.mode = modeDefault

	return
}

func (e *Ex) runSingleCommand(cmd string) (quit bool, err error) {
	browser := e.Component.Browser()
	switch cmd {
	case "bprev":
		browser.UpdateWindowTabPrev(browser.Focus())
	case "bnext":
		browser.UpdateWindowTabNext(browser.Focus())
	case "bclose":
		browser.RemoveWindowContent(browser.Focus())
	case "bcloseAll":
		browser.RemoveAllTabs()
	case "close":
		err = browser.Focus().Close()
	case "wq", "wq!":
		quit = true
		fallthrough
	case "w", "w!":
		err = e.Component.Flush(browser.Focus())
	case "q!", "q":
		quit = true
	default:
		err = fmt.Errorf("Unknown command: %s", cmd)
	}
	return
}

func (e *Ex) runCommand() (quit bool, err error) {
	cmd := e.commandBuf.String()
	cmds := strings.Split(cmd, " ")
	if len(cmds) == 1 {
		return e.runSingleCommand(cmds[0])
	}

	switch cmds[0] {
	case "e":
		var h browser.Handler
		h, err = e.Component.OpenFileTab(cmds[1], "")
		if err != nil {
			return
		}
		e.Component.Browser().Focus().SetContent(h)
	default:
		err = fmt.Errorf("Unknown command: %s", cmd)
	}
	return
}

func (e *Ex) setError(err error) {
	e.Component.Browser().SetMessage("Error: %s", err)
}

func (e *Ex) handleCommand(ev term.Event) (quit, handled bool) {
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

	prev := ev
	ev, _ = e.Component.KeyMapping(ev)

	// client subscriptions take precedence over ex key mappings
	handled = e.Component.Publish(ev)
	if handled {
		return
	}

	browser := e.Component.Browser()
	switch ev.Key {
	case term.KeyCtrlA:
		browser.RemoveAllTabs()
	case term.KeyCtrlW:
		browser.RemoveWindowContent(browser.Focus())
	case term.KeyCtrlL:
		browser.UpdateWindowTabNext(browser.Focus())
	case term.KeyCtrlH:
		browser.UpdateWindowTabPrev(browser.Focus())
	default:
		// do not map for children
		ev = prev
		exit, handled = browser.Handle(ev)
		if handled {
			return
		}
		// If ex is configured with character
		// command mode trigger event (i.e. ':')
		// then we assume that the underlying editor is
		// a modal editor, and so does not handle
		// the command trigger event in its "initial" mode
		// (in vi terms, this would be normal mode).
		handled = e.handleCommandEvent(ev)
		return
	}

	return false, true
}

func (e *Ex) setNormalMode() {
	e.commandBuf.Reset()
	e.mode = modeDefault
}

func (e *Ex) setCommandMode() {
	e.mode = modeCommand
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

// Cursor satisfies tui.Handler.
func (e *Ex) Cursor() (pos term.Coordinates, show bool) {
	if e.mode == modeCommand {
		pos := e.cmdVirt.Position()
		pos.X += len(e.commandBuf.String())
		return pos, true
	}
	return e.Component.Browser().Cursor()
}

// Man satisfies tui.Handler.
func (e *Ex) Man() tui.Manual {
	panic("TODO")
}

// Resize satisfies tui.Component
func (e *Ex) Resize(width, height int) {
	browser.ResizeMessageSpan(&e.cmdVirt, width, height)
	e.Component.Resize(width, height)
}

// Draw satisfies tui.Component
func (e *Ex) Draw(w term.Writer) {
	e.Component.Draw(w)

	if e.mode == modeCommand {
		e.cmdVirt.Draw(w)
	}
}

// Close closes the resources associated with this browser.
func (e *Ex) Close() error {
	return e.Component.Close()
}
