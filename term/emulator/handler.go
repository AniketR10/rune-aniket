package emulator

import (
	"fmt"
	"math"
	"time"

	multierr "github.com/ernestrc/go-multierror"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	termutil "unstable.build/go-tui/term/emulator/util"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/clipboard"
	sysclip "unstable.build/go-tui/text/clipboard/system"
)

var _ tui.Handler = (*Handler)(nil)

// Handler is a tui.Handler that implements a terminal emulator.
type Handler struct {
	browser browser.Browser
	api     schemeapi.Terminal

	mouse             *text.Mouse
	mouseDriver       *mouseDriver
	windowManipulator *windowManipulator
	terminal          *termutil.Terminal
	theme             *termutil.Theme
	defAttr           term.Attributes
	selectAttr        term.Attributes

	closed        bool
	width, height int
	resizeErr     error
	updateCh      chan struct{}
	sema          chan struct{}
}

// New allocates storage for a new Handler and initializes it. See Handler.Init
// for more details.
func New(
	browser browser.Browser, api schemeapi.Terminal,
	config Config, initialCmd string,
) (*Handler, error) {
	ret := new(Handler)
	err := ret.Init(browser, api, config, initialCmd)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Init initializes this handler with the given browser API,
// shell, initialCmd, default attributes and selection attributes.
func (e *Handler) Init(
	browser browser.Browser, api schemeapi.Terminal,
	config Config, initialCmd string,
) error {
	e.browser = browser
	e.api = api
	e.defAttr = config.Attributes
	e.selectAttr = config.SelectionAttributes

	e.theme = &termutil.Theme{Default: config.Attributes}
	e.windowManipulator = newWindowManipulator(e.browser)
	opts := []termutil.Option{
		termutil.WithTheme(e.theme),
		termutil.WithWindowManipulator(e.windowManipulator),
	}
	if config.Shell != "" {
		opts = append(opts, termutil.WithShell(config.Shell))
	}
	if initialCmd != "" {
		opts = append(opts, termutil.WithInitialCommand(initialCmd))
	}
	e.terminal = termutil.New(api, opts...)
	_, err := e.terminal.CreatePty()
	if err != nil {
		return err
	}
	e.windowManipulator.SetTitle(e.terminal.Pty().Slave)
	clip, err := sysclip.NewRegister()
	if err != nil {
		log.Warnf("system clipboard unsupported: %v", err)
		clip = clipboard.NewInMemory()
	}
	e.mouseDriver = &mouseDriver{t: e.terminal, clipboard: clip}
	e.mouse = text.NewMouse(e.mouseDriver)

	e.updateCh = make(chan struct{}, 1)
	e.sema = make(chan struct{})
	go func() {
		logErr := e.terminal.Run(e.updateCh)
		if err := e.Close(); err != nil {
			logErr = multierr.Append(logErr, err)
		}
		if err := e.browser.PublishEventNone(); err != nil {
			err = fmt.Errorf("publish event: %s", err)
			logErr = multierr.Append(logErr, err)
		}
		if logErr != nil {
			log.Errorf("(%p): %s", e, logErr)
		} else {
			log.Infof("(%p) terminal.Cmd.Wait: OK", e)
		}
	}()

	go func() {
		for {
			select {
			case <-e.sema:
				e.sema <- struct{}{}
			case _, ok := <-e.updateCh:
				if !ok {
					return
				}
			}
			err = e.browser.Interrupt()
			if err != nil {
				log.Errorf("Interrupt: %s", err)
			}
		}
	}()

	return nil
}

// Resize satisfies tui.Component.
func (e *Handler) Resize(width, height int) {
	e.terminal.Lock()
	defer e.terminal.Unlock()
	// avoid SetSize error
	if e.closed {
		return
	}

	e.windowManipulator.ResizeInChars(height, width)
	e.width, e.height = width, height
	err := e.terminal.SetSize(uint16(height), uint16(width))
	if err != nil {
		log.Errorf("terminal.SetSize: %s", err)
	}
}

func (e *Handler) Draw(w term.Writer) {
	if e.resizeErr != nil {
		errStr := fmt.Sprintf("Error setting win size: %s", e.resizeErr)
		log.Errorf("(%p).emulator.Draw: resize err: %s", e, e.resizeErr)
		e.drawStr(errStr, w)
		return
	}

	e.terminal.Lock()
	defer e.terminal.Unlock()

	e.drawContent(w)
	e.drawSelection(w)
}

// Handle satisfies tui.Handler.
func (e *Handler) Handle(ev term.Event) (exit, handled bool) {
	exit, handled, raw := e.handleInput(ev)
	if exit || handled || len(raw) == 0 {
		return
	}

	e.sema <- struct{}{}
	defer func() { <-e.sema }()

	err := e.terminal.WriteToPty(raw)
	if err != nil {
		log.Errorf("(%p).emulator.Handle: %s", e, err)
		return
	}

	timer := time.NewTimer(50 * time.Millisecond)
	defer timer.Stop()

	select {
	case <-timer.C:
	case _, ok := <-e.updateCh:
		if !ok {
			exit = true
		}
		handled = true
	}
	return
}

// Cursor satisfies tui.Handler.
func (e *Handler) Cursor() (term.Coordinates, bool) {
	e.terminal.Lock()
	defer e.terminal.Unlock()

	termbuf := e.terminal.GetActiveBuffer()

	if !termbuf.IsCursorVisible() {
		return term.Coordinates{}, false
	}

	return term.Coordinates{
		X: int(math.Min(float64(termbuf.CursorColumn()), float64(e.width-1))),
		Y: int(math.Min(float64(termbuf.CursorLine()), float64(e.height-1))),
	}, true
}

// Man satisfies tui.Handler.
func (e *Handler) Man() tui.Manual {
	panic("TODO")
}

// URI returns the uri of the emulated terminal.
func (e *Handler) URI() (workspaceapi.URI, error) {
	var name string
	func() {
		e.terminal.Lock()
		defer e.terminal.Unlock()
		name = e.terminal.GetTitle()
	}()

	return workspaceapi.CurrentUserHostURI(name)
}

// Title returns the title of the emulated terminal.
func (e *Handler) Title() string {
	e.terminal.Lock()
	defer e.terminal.Unlock()

	return e.terminal.GetTitle()
}

// Close closes this terminal emulator and all the resources
// associated with it.
func (e *Handler) Close() error {
	log.Tracef("(%p).emulator.Close", e)

	e.terminal.Lock()
	defer e.terminal.Unlock()

	if e.closed || e.terminal == nil {
		return nil
	}

	// undo circular dependency
	e.mouseDriver.clipboard = nil
	e.mouseDriver = nil
	e.closed = true

	var ret error
	if err := e.terminal.Close(); err != nil {
		ret = multierr.Append(ret, err)
		log.Errorf("(%p).emulator.Close(Pty): %s", e, err)
	}
	// we can't remove /dev/pts files so leave it up to the system
	return ret
}

func (e *Handler) drawStr(str string, w term.Writer) {
	c := component.NewString(str)
	c.Resize(e.width, e.height)
	c.Draw(w)
}

func (e *Handler) drawRow(
	w term.Writer, termbuf *termutil.Buffer,
	viewY int, defattr term.Attributes,
) {
	maxX := uint16(math.Min(float64(e.width), float64(termbuf.ViewWidth())))
	for viewX := uint16(0); viewX < maxX; viewX++ {
		cell := termbuf.GetCell(viewX, uint16(viewY))
		pos := term.Coordinates{X: int(viewX), Y: viewY}
		if cell == nil || cell.Ch == 0 {
			w.SetCell(pos, term.Cell{Bg: defattr.Bg, Fg: defattr.Fg})
		} else {
			tcell := *cell
			if tcell.Fg == term.ColorDefault {
				tcell.Fg = defattr.Fg
			}
			if tcell.Bg == term.ColorDefault {
				tcell.Bg = defattr.Bg
			}
			w.SetCell(pos, tcell)
		}
	}
}

func (e *Handler) drawContent(w term.Writer) {
	termbuf := e.terminal.GetActiveBuffer()
	viewY := int(math.Min(float64(e.height), float64(termbuf.ViewHeight()))) - 1
	for ; viewY >= 0; viewY-- {
		e.drawRow(w, termbuf, viewY, e.defAttr)
	}
}

func (e *Handler) drawSelection(w term.Writer) {
	termbuf := e.terminal.GetActiveBuffer()
	_, selection := termbuf.GetSelection()
	if selection == nil {
		return
	}

	bg, fg := e.selectAttr.Bg, e.selectAttr.Fg

	for y := selection.Start.Line; y <= selection.End.Line; y++ {
		xStart, xEnd := 0, int(termbuf.ViewWidth())
		if y == selection.Start.Line {
			xStart = int(selection.Start.Col)
		}
		if y == selection.End.Line {
			xEnd = int(selection.End.Col)
		}
		for x := xStart; x <= xEnd; x++ {
			cell := termbuf.GetCell(uint16(x), uint16(y))
			if cell == nil {
				continue
			}
			ch := cell.Ch
			pos := term.Coordinates{X: x, Y: int(y)}
			w.SetCell(pos, term.Cell{Ch: ch, Fg: fg, Bg: bg})
		}
	}
}

func (e *Handler) handleInput(ev term.Event) (exit, handled bool, raw []byte) {
	e.terminal.Lock()
	defer e.terminal.Unlock()

	exit = e.closed
	if exit {
		log.Debugf("(%p).emulator.Handle: closed", e)
		return
	}

	if ev.Type == term.EventMouse {
		e.mouseDriver.hookRawBytes = nil
		exit, handled = e.mouse.Handle(ev)
		raw = e.mouseDriver.hookRawBytes
		// raw bytes should be sent directly only
		// if we didn't handle mouse event
		if len(raw) != 0 {
			handled = false
		}
		return
	}

	switch ev.Key {
	case term.KeyCtrlL:
		e.mouseDriver.ClearSelection()
	}

	// we cannot simply send raw bytes coming from termbox.
	// The running program sends escape sequences to e.terminal via stdout which
	// configure the program's I/O mode and so this might not might not necessarily
	// match termbox's configuration.
	raw, ok := mapKeyToEscapeSequence(e.terminal.GetActiveBuffer(), ev)
	if !ok {
		raw = ev.Raw
	}
	return
}
