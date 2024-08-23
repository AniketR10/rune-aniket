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

package dialogue

import (
	"fmt"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

func newMouseDelegate(
	l *component.ResponsiveList,
) *mouseDelegate {
	return &mouseDelegate{list: l}
}

var _ text.MouseDelegate = (*mouseDelegate)(nil)

// satisfies text.MouseDelegate
type mouseDelegate struct {
	list          *component.ResponsiveList
	selection     component.WithAttributes
	selectionAttr term.Attributes
	selectionText string
}

func (d *mouseDelegate) OnAction(ev term.Event, pos term.Coordinates, action text.MouseAction) bool {
	return false
}

func (d *mouseDelegate) ScrollUp(n int) (ok bool) {
	return d.list.SeekUp()
}

func (d *mouseDelegate) ScrollDown(n int) (ok bool) {
	return d.list.SeekDown()
}

func (d *mouseDelegate) SetSelectionStart(pos term.Coordinates) {
	if d.selection != nil {
		d.ClearSelection()
	}
}

func (d *mouseDelegate) SetSelectionEnd(pos term.Coordinates) {
	node, ok := d.list.ElementAt(pos)
	if !ok {
		return
	}
	if d.selection != nil {
		d.ClearSelection()
	}
	content := node.Value().(*component.Span).Content().(*component.AttrSetter)
	d.selectionAttr = content.SetAttr(term.Attributes{Attrs: tcell.AttrReverse})
	d.selection = content
	d.selectionText = content.Content().(fmt.Stringer).String()
}

func (d *mouseDelegate) Selection() (string, bool) {
	return d.selectionText, d.selection != nil
}

func (d *mouseDelegate) ClearSelection() {
	if d.selection == nil {
		return
	}
	d.selection.SetAttr(d.selectionAttr)
	d.selection = nil
}

func (d *mouseDelegate) SelectWordAt(pos term.Coordinates) {
	d.SetSelectionEnd(pos)
}

func (d *mouseDelegate) SelectLine(y int) {
	pos := term.Coordinates{Y: y}
	d.SetSelectionEnd(pos)
}

func (d *mouseDelegate) Width() int {
	return d.list.SizeWidth()
}

func (d *mouseDelegate) Height() int {
	return d.list.SizeHeight()
}
