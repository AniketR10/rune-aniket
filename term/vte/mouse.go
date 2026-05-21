// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package vte

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
)

type mouseDriver struct {
	t              *Component
	selectionStart term.Coordinates
	hookRawBytes   []byte
	clipboard      clipboard.Register
	lastButton     rune // last pressed button (0=left, 1=middle, 2=right)
}

func (e *mouseDriver) OnAction(
	ev term.Event, pos term.Coordinates, action mouse.Action,
) bool {
	if e.t.MouseModeReportMouseClicks() || e.t.MouseModeReportCellMouseMotion() {
		return e.reportAction(ev, pos, action)
	}
	switch action {
	case mouse.MiddleClick:
		paste, _ := e.clipboard.Paste(clipboard.DefaultRegisterID)
		e.hookRawBytes = []byte(paste.Text)
		return true
	default:
		return false
	}
}

func (e *mouseDriver) reportAction(
	ev term.Event, pos term.Coordinates, action mouse.Action,
) bool {
	tx, ty := pos.X, pos.Y
	var button rune
	switch action {
	case mouse.WheelUp:
		e.hookRawBytes = ev.Raw
		return true
	case mouse.WheelDown:
		e.hookRawBytes = ev.Raw
		return true
	case mouse.LeftClick:
		button = 0
		e.lastButton = 0
	case mouse.MiddleClick:
		button = 1
		e.lastButton = 1
	case mouse.RightClick:
		button = 2
		e.lastButton = 2
	case mouse.Release:
		if e.t.MouseModeSgrMouse() {
			button = e.lastButton
		} else {
			button = 3
		}
	default:
		return false
	}

	if e.t.MouseModeSgrMouse() {
		final := 'M'
		if action == mouse.Release {
			final = 'm'
		}
		e.hookRawBytes = fmt.Appendf(nil, "\x1b[<%d;%d;%d%c", button, tx+1, ty+1, final)
	} else {
		e.hookRawBytes = fmt.Appendf(nil, "\x1b[M%c%c%c", button+32, tx+33, ty+33)
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
	// Select/SelectEnd assume reading order: from is the top-left and
	// to is one past the bottom-right. A leftward drag would otherwise
	// produce from > to, and the buffer's internal sort then drops the
	// press cell and the drag-end cell from the selection.
	start, end := e.selectionStart, pos
	if end.Y < start.Y || (end.Y == start.Y && end.X < start.X) {
		start, end = end, start
	}
	e.t.Select(start)
	e.t.SelectEnd(end)
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
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "vte.mouseDriver").
		Logf(level, msg, args...)
}
