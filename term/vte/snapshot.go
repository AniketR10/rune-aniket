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
	Version int
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
