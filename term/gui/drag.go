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

package gui

import (
	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// DragEventKind classifies a stage of a file drag over the GUI window.
type DragEventKind int

const (
	// DragHover reports the current position of files dragged over the
	// window. It repeats whenever the position changes.
	DragHover DragEventKind = iota
	// DragLeave reports that the drag left the window or was released.
	DragLeave
	// DragDrop reports files released on the window.
	DragDrop
)

// DragEvent describes a file drag over the GUI window. Pos is in cell
// coordinates; Paths is set for DragDrop only.
type DragEvent struct {
	Kind  DragEventKind
	Pos   term.Coordinates
	Paths []string
}

// dragPoller turns ebiten's per-frame drag state into drag transitions.
type dragPoller struct {
	mouse *mouse
	// observer is never nil; hosts install theirs with WithDragObserver.
	observer func(DragEvent)
	// position and paths are the ebiten accessors, replaced in tests.
	position func() (int, int, bool)
	paths    func() []string

	hovering bool
	last     term.Coordinates
	// tracked records that the host reported a position for the drag in
	// progress. Hosts that do not report drag positions leave it false,
	// and the drop falls back to the mouse cursor.
	tracked bool
}

func newDragPoller(m *mouse) *dragPoller {
	return &dragPoller{
		mouse:    m,
		observer: func(DragEvent) {},
		position: ebiten.DraggingPosition,
		paths:    ebiten.DroppedFilePaths,
	}
}

// poll reports the drag transitions observed since the previous frame and
// returns whether anything changed, so the caller can force a repaint.
func (d *dragPoller) poll() bool {
	var changed bool
	x, y, dragging := d.position()
	pos := d.mouse.clamp(d.mouse.cellAt(float64(x), float64(y)))
	switch {
	case dragging:
		d.tracked = true
		if !d.hovering || pos != d.last {
			d.hovering = true
			d.last = pos
			d.observer(DragEvent{Kind: DragHover, Pos: pos})
			changed = true
		}
	case d.hovering:
		d.hovering = false
		d.observer(DragEvent{Kind: DragLeave, Pos: d.last})
		changed = true
	}

	// The host reports the drop after ending the drag, so the veil is
	// already gone by the time the paths arrive. The reported position
	// still describes where the files were released, which is where the
	// drop belongs: the mouse cursor is stale during a drag, so using it
	// would send every drop to the window clicked last.
	if paths := d.paths(); len(paths) > 0 {
		drop := pos
		if !d.tracked {
			drop = d.mouse.clampedCoordinates()
		}
		d.hovering = false
		d.tracked = false
		d.observer(DragEvent{
			Kind:  DragDrop,
			Pos:   drop,
			Paths: paths,
		})
		changed = true
	}
	return changed
}
