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

package emacs

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

// undoRunKind classifies the amalgamating commands: GNU Emacs merges runs
// of consecutive self-inserted characters — and, separately, runs of
// same-direction character deletes — into a single undo entry.
type undoRunKind int

const (
	undoRunNone undoRunKind = iota
	undoRunInsert
	undoRunDeleteBack
	undoRunDeleteFwd
)

// undoRunMax mirrors GNU amalgamating undo: at most 20 consecutive
// characters merge into one undo group before a new one starts.
const undoRunMax = 20

// amalgamationKind classifies ev before dispatch. Only plain character
// insertion (self-insert) and single-character deletes amalgamate; every
// other key is its own undo group.
func amalgamationKind(ev term.Event) undoRunKind {
	switch ev.Mod {
	case 0:
		switch ev.Key {
		case term.KeySpace:
			return undoRunInsert
		case term.KeyBackspace:
			return undoRunDeleteBack
		case term.KeyDelete:
			return undoRunDeleteFwd
		case 0:
			if ev.Ch != 0 {
				return undoRunInsert
			}
		}
	case term.ModCtrl:
		switch ev.Ch {
		case 'd':
			return undoRunDeleteFwd
		case 'h':
			return undoRunDeleteBack
		}
	}
	return undoRunNone
}

// closeUndoRun ends any open amalgamation run, merging its edits into a
// single undo group.
func (h *emacsHandler) closeUndoRun() {
	if h.undoRun == undoRunNone {
		return
	}
	h.buf.GroupUndo()
	h.undoRun = undoRunNone
	h.undoRunLen = 0
}

// beginEventUndo opens the undo scope for one normal-keymap event. An
// amalgamating key joins (or starts) a run whose group stays open across
// events; any other key closes the run and gets a group of its own. It
// returns true when the caller must close that group after dispatch.
func (h *emacsHandler) beginEventUndo(ev term.Event) bool {
	kind := amalgamationKind(ev)
	if kind == undoRunNone {
		h.closeUndoRun()
		h.buf.MarkStartUndo()
		return true
	}
	if kind != h.undoRun || h.undoRunLen >= undoRunMax {
		h.closeUndoRun()
	}
	if h.undoRun == undoRunNone {
		h.buf.MarkStartUndo()
		h.undoRun = kind
	}
	h.undoRunLen++
	return false
}
