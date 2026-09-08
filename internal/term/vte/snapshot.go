// Copyright (C) 2017-2026 Unstable Build, LLC
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

package vte

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

const terminalSnapshotVersion = 1

// Snapshot is a durable representation of a terminal's rendered state.
//
// It intentionally captures terminal output and screen state, not the live
// process attached to the pty. This mirrors tmux's distinction between an
// in-memory pane grid/history and process respawn: saved snapshots can restore
// output/history, but cannot continue the original process after Rune exits.
//
// Exactly one of Primary or Alternate is populated, matching the buffer
// that was active at snapshot time. Reading both cells and cursor under
// one mutex acquire — instead of through separate component accessors —
// is what callers like vteprobe rely on to keep the two consistent
// across parser callbacks.
type Snapshot struct {
	// Schema is the on-disk snapshot format version, bumped only when
	// the persisted layout changes. Version is the live grid revision
	// from the source Component, used by callers to detect whether the
	// rendered grid changed between snapshots; it is not part of the
	// durable format's compatibility contract.
	Schema  int
	Version uint64
	Title   string
	Width   int
	Height  int

	ScrollOffset term.Coordinates
	Primary      ScreenSnapshot
	Alternate    ScreenSnapshot
}

// ActiveCells returns the cells of the buffer that was active when the
// snapshot was taken (Alternate when populated, Primary otherwise).
func (s Snapshot) ActiveCells() [][]term.Cell {
	return term.CloneCells(s.Active().Cells)
}

// Active returns the ScreenSnapshot of the buffer that was active when
// the snapshot was taken: Alternate when its Cells are populated,
// otherwise Primary.
func (s Snapshot) Active() ScreenSnapshot {
	if len(s.Alternate.Cells) != 0 {
		return s.Alternate
	}
	return s.Primary
}

// ScreenSnapshot is the rendered state of one VTE screen buffer.
type ScreenSnapshot struct {
	Cells  [][]term.Cell
	Cursor term.Coordinates
}
