package vte

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/clipboard"
)

type mouseDriver struct {
	t              *Component
	selectionStart term.Coordinates
	hookRawBytes   []byte
	clipboard      clipboard.Register
}

func (e *mouseDriver) OnAction(
	ev term.Event, pos term.Coordinates, action text.MouseAction,
) bool {
	if e.t.MouseModeReportMouseClicks() || e.t.MouseModeReportCellMouseMotion() {
		return e.reportAction(ev, pos, action)
	}
	switch action {
	case text.MouseMiddleClick:
		paste, _ := e.clipboard.Paste(clipboard.DefaultRegisterID)
		e.hookRawBytes = []byte(paste.Text)
		return true
	default:
		return false
	}
}

func (e *mouseDriver) reportAction(
	ev term.Event, pos term.Coordinates, action text.MouseAction,
) bool {
	tx, ty := pos.X, pos.Y
	var button rune
	switch action {
	case text.MouseWheelUp:
		// TODO use alternate scroll mode
		// manage it here so we can delegate to
		// underlying program if terminal
		// could not handle scroll (i.e. alternate buffer)
		e.hookRawBytes = ev.Raw
		return true
	case text.MouseWheelDown:
		e.hookRawBytes = ev.Raw
		return true
	case text.MouseLeftClick:
		button = 0
	case text.MouseMiddleClick:
		button = 1
	case text.MouseRightClick:
		button = 2
	case text.MouseRelease:
		if !e.t.MouseModeSgrMouse() {
			button = 3
		}
	default:
		return false
	}

	if /* moved && */ e.t.MouseModeReportCellMouseMotion() {
		button |= 32
	}

	if e.t.MouseModeSgrMouse() {
		final := 'M'
		if action == text.MouseRelease {
			final = 'm'
		}
		e.hookRawBytes = []byte(fmt.Sprintf("\x1b[<%d;%d;%d%c", button, tx, ty, final))
	} else {
		e.hookRawBytes = []byte(fmt.Sprintf("\x1b[M%c%c%c", button+32, tx+32, ty+32))
	}
	return true
}

func (e *mouseDriver) ScrollUp(n int) (ok bool) {
	e.t.ScrollUp(n)
	return
}

func (e *mouseDriver) ScrollDown(n int) (ok bool) {
	e.t.ScrollDown(n)
	return
}

func (e *mouseDriver) ClearSelection() {
	e.t.Unselect()
}

func (e *mouseDriver) SetSelectionStart(pos term.Coordinates) {
	e.selectionStart = pos
}

func (e *mouseDriver) SetSelectionEnd(pos term.Coordinates) {
	e.t.Select(e.selectionStart)
	e.t.SelectEnd(pos)
	e.copySelectionToClipboard()
}

func (e *mouseDriver) SelectWordAt(pos term.Coordinates) {
	e.t.SelectWordAt(pos)
	e.copySelectionToClipboard()
}

func (e *mouseDriver) SelectLine(y int) {
	pos := term.Coordinates{Y: y}
	e.t.SelectLine(pos)
	e.copySelectionToClipboard()
}

func (e *mouseDriver) Width() int {
	return e.t.MaxWidth()
}

func (e *mouseDriver) Height() int {
	return e.t.Height()
}

func (e *mouseDriver) copySelectionToClipboard() {
	data, _ := e.t.Selection()
	clipdata := clipboard.Data{Text: data}
	err := e.clipboard.Copy(clipboard.DefaultRegisterID, clipdata)
	if err != nil {
		e.log(log.ErrorLevel, "copy clipboard data to register: %v", err)
	}
}

func (e *mouseDriver) log(level log.Level, msg string, args ...any) {
	log.WithField(logging.KeyClass, "vte.mouseDriver").
		Logf(level, msg, args...)
}
