// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package ideshell

import (
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
	tterm "unstable.build/go-tui/term"
)

func newMouseDelegate(grid *tterm.SelectionWriter, scroll func(ev term.Event)) *mouseDelegate {
	return &mouseDelegate{grid: grid, scroll: scroll}
}

var _ mouse.Delegate = (*mouseDelegate)(nil)

// mouseDelegate implements mouse.Delegate over the captured output
// grid. Selection coordinates are screen-relative: the repl output
// band starts at row 0, so no offset translation is needed. Wheel
// scroll is forwarded to the inner repl via scroll.
type mouseDelegate struct {
	grid   *tterm.SelectionWriter
	scroll func(ev term.Event)
	sel    tterm.SelRange
	anchor term.Coordinates // raw start position (before sorting)
	// outH bounds the output band; clicks at or below it belong to
	// the input band and are ignored here.
	outH int
}

func (d *mouseDelegate) OnAction(ev term.Event, pos term.Coordinates, action mouse.Action) bool {
	return false
}

func (d *mouseDelegate) ScrollUp(n int) (ok bool) {
	for range n {
		d.scroll(term.Event{Type: term.EventMouse, Key: term.MouseWheelUp})
		ok = true
	}
	return
}

func (d *mouseDelegate) ScrollDown(n int) (ok bool) {
	for range n {
		d.scroll(term.Event{Type: term.EventMouse, Key: term.MouseWheelDown})
		ok = true
	}
	return
}

func (d *mouseDelegate) SetSelectionStart(pos term.Coordinates) {
	d.sel = tterm.SelRange{}
	d.anchor = d.clamp(pos)
}

func (d *mouseDelegate) SetSelectionEnd(pos term.Coordinates) {
	start, end := term.CoordinatesSort(d.anchor, d.clamp(pos))
	d.sel = tterm.SelRange{Start: start, End: end, Active: true}
}

func (d *mouseDelegate) ClearSelection() {
	d.sel = tterm.SelRange{}
}

func (d *mouseDelegate) SelectWordAt(pos term.Coordinates) {
	if d.outH > 0 && pos.Y >= d.outH {
		d.sel = tterm.SelRange{}
		return
	}
	start, end, ok := d.grid.WordBoundsAt(pos)
	if !ok {
		d.sel = tterm.SelRange{}
		return
	}
	d.sel = tterm.SelRange{Start: start, End: end, Active: true}
}

func (d *mouseDelegate) SelectLine(y int) {
	if d.outH > 0 && y >= d.outH {
		d.sel = tterm.SelRange{}
		return
	}
	start, end, ok := d.grid.LineBounds(y)
	if !ok {
		d.sel = tterm.SelRange{}
		return
	}
	d.sel = tterm.SelRange{Start: start, End: end, Active: true}
}

// clamp restricts pos to the output band: X to [0, width-1] and Y to
// [0, outH-1]. It keeps a drag that wanders past the grid edges or into
// the input band from selecting cells outside the shell output.
func (d *mouseDelegate) clamp(pos term.Coordinates) term.Coordinates {
	maxY := d.grid.Height() - 1
	if d.outH > 0 {
		maxY = min(maxY, d.outH-1)
	}
	maxY = max(maxY, 0)
	maxX := max(d.grid.Width()-1, 0)
	pos.X = min(max(pos.X, 0), maxX)
	pos.Y = min(max(pos.Y, 0), maxY)
	return pos
}

func (d *mouseDelegate) Selection() (string, bool) {
	if !d.sel.Active {
		return "", false
	}
	text := d.grid.TextBetween(d.sel.Start, d.sel.End)
	return text, text != ""
}

func (d *mouseDelegate) Width() int {
	return d.grid.Width()
}

func (d *mouseDelegate) Height() int {
	return d.grid.Height()
}
