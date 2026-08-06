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

package vi

import (
	"github.com/unstablebuild/rune-go-sdk/mouse"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/text"
)

// wraps text.CursorDelegate to set vi states
type mouseDelegate struct {
	mouse.Delegate
	vi *viHandlerImpl
}

func newDelegate(vi *viHandlerImpl) mouse.Delegate {
	return mouseDelegate{Delegate: text.CursorMouseDelegate(&vi.cursor), vi: vi}
}

func (d mouseDelegate) SetSelectionStart(pos term.Coordinates) {
	if d.vi.mode() == insertMode {
		// vim's mouse=a: reposition and stay in insert. The anchor
		// must follow because insert-mode arrows snap back to it.
		d.vi.cursor.MoveToScroll(d.vi.cursor.ScrollCoordinates(pos))
		d.vi.anchor = d.vi.cursorAtScroll()
		return
	}
	d.Delegate.SetSelectionStart(pos)
	d.vi.setVisualMode()
	d.vi.anchor = d.vi.cursorAtScroll()
	d.vi.markMatchingBrace()
}

func (d mouseDelegate) SetSelectionEnd(pos term.Coordinates) {
	if !isSelectMode(d.vi.mode()) {
		// Drag from insert: the cursor still sits on the pressed
		// cell, so setVisualMode anchors the selection there.
		d.vi.setVisualMode()
		d.vi.anchor = d.vi.cursorAtScroll()
		d.vi.markMatchingBrace()
	}
	d.Delegate.SetSelectionEnd(pos)
}

func (d mouseDelegate) ClearSelection() {
	if isSelectMode(d.vi.mode()) {
		d.vi.setNormalMode()
	}
	d.Delegate.ClearSelection()
}
