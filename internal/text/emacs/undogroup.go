// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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

type undoSequenceState int

const (
	undoSequenceNone undoSequenceState = iota
	undoSequenceUndo
	undoSequenceRedoNext
	undoSequenceRedo
)

func isUndoEvent(ev term.Event) bool {
	return ev.Type == term.EventKey && ev.Mod == term.ModCtrl &&
		(ev.Ch == '/' || ev.Ch == '_')
}

// dispatchesUndo reports whether ev dispatches an undo or undo-redo
// command. Undo and redo rewrite the undo timeline themselves, so their
// dispatch must never run inside a MarkStartUndo/GroupUndo pair: grouping
// the entries a redo pushes back would fold distinct changes into one.
func dispatchesUndo(ev term.Event) bool {
	if ev.Type != term.EventKey {
		return false
	}
	switch ev.Mod {
	case term.ModCtrl:
		return ev.Ch == '/' || ev.Ch == '_' || ev.Ch == '?'
	case term.ModCtrlAlt:
		return ev.Ch == '/' || ev.Ch == '_'
	}
	return false
}

func (h *emacsHandler) breakUndoSequence() {
	switch h.undoSequence {
	case undoSequenceUndo:
		h.undoSequence = undoSequenceRedoNext
	case undoSequenceRedo:
		h.undoSequence = undoSequenceNone
	}
}

func (h *emacsHandler) undo() bool {
	switch h.undoSequence {
	case undoSequenceRedoNext:
		if h.cursor.Redo() {
			h.undoSequence = undoSequenceRedo
			return true
		}
	case undoSequenceRedo:
		if h.cursor.Redo() {
			return true
		}
		// GNU rolls an unbroken sequence past the redos into undoing
		// earlier changes again instead of going inert.
	}
	if !h.cursor.Undo() {
		return false
	}
	h.undoSequence = undoSequenceUndo
	return true
}

func (h *emacsHandler) undoFromPrefix() bool {
	if h.undoPrefix != nil {
		h.undoSequence = *h.undoPrefix
		h.undoPrefix = nil
	}
	return h.undo()
}

func (h *emacsHandler) undoRedo() bool {
	h.undoSequence = undoSequenceNone
	return h.cursor.Redo()
}

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
	if dispatchesUndo(ev) {
		h.closeUndoRun()
		return false
	}
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
