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

package dialoguetui

import (
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func newMouseDelegate(
	grid *cellGrid,
	list *component.ResponsiveList,
) *mouseDelegate {
	return &mouseDelegate{grid: grid, list: list}
}

var _ mouse.Delegate = (*mouseDelegate)(nil)

type mouseDelegate struct {
	grid   *cellGrid
	list   *component.ResponsiveList
	offset term.Coordinates // messages-area offset within the grid, updated each Draw
	sel    selRange         // messages-relative coordinates
	anchor term.Coordinates // raw start position (before sorting)
}

func (d *mouseDelegate) OnAction(ev term.Event, pos term.Coordinates, action mouse.Action) bool {
	return false
}

func (d *mouseDelegate) ScrollUp(n int) (ok bool) {
	for range n {
		if d.list.SeekUp() {
			ok = true
		}
	}
	return
}

func (d *mouseDelegate) ScrollDown(n int) (ok bool) {
	for range n {
		if d.list.SeekDown() {
			ok = true
		}
	}
	return
}

func (d *mouseDelegate) SetSelectionStart(pos term.Coordinates) {
	d.sel = selRange{}
	d.anchor = pos
}

func (d *mouseDelegate) SetSelectionEnd(pos term.Coordinates) {
	start, end := term.CoordinatesSort(d.anchor, pos)
	d.sel = selRange{start: start, end: end, active: true}
}

func (d *mouseDelegate) ClearSelection() {
	d.sel = selRange{}
}

func (d *mouseDelegate) SelectWordAt(pos term.Coordinates) {
	gridPos := term.CoordinatesSum(pos, d.offset)
	start, end, ok := d.grid.WordBoundsAt(gridPos)
	if !ok {
		d.sel = selRange{}
		return
	}
	d.sel = selRange{
		start:  term.CoordinatesDiff(start, d.offset),
		end:    term.CoordinatesDiff(end, d.offset),
		active: true,
	}
}

func (d *mouseDelegate) SelectLine(y int) {
	gridY := y + d.offset.Y
	start, end, ok := d.grid.LineBounds(gridY)
	if !ok {
		d.sel = selRange{}
		return
	}
	d.sel = selRange{
		start:  term.CoordinatesDiff(start, d.offset),
		end:    term.CoordinatesDiff(end, d.offset),
		active: true,
	}
}

func (d *mouseDelegate) Selection() (string, bool) {
	if !d.sel.active {
		return "", false
	}
	start := term.CoordinatesSum(d.sel.start, d.offset)
	end := term.CoordinatesSum(d.sel.end, d.offset)
	text := d.grid.TextBetween(start, end)
	return text, text != ""
}

func (d *mouseDelegate) Width() int {
	return d.grid.width
}

func (d *mouseDelegate) Height() int {
	return d.grid.height
}
