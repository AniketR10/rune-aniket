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

package vtescreen

import (
	"math"
	"strings"

	"unstable.build/go-tui/term"
)

// PrimaryBuffer wraps a Buffer to provide scroll-back for a primary vte screen buffer.
type PrimaryBuffer struct {
	AltBuffer
}

// NewPrimaryBuffer allocates storage for a new PrimaryBuffer and initializes it.
func NewPrimaryBuffer() *PrimaryBuffer {
	ret := new(PrimaryBuffer)
	ret.Init()
	return ret
}

// Init initializes this PrimaryBuffer.
func (b *PrimaryBuffer) Init() {
	b.AltBuffer.Init()
}

// Resize resizes this Buffer and resets the vertical margins.
func (b *PrimaryBuffer) Resize(width, height int) {
	b.AltBuffer.width = width
	b.AltBuffer.height = height
	b.scroll.Resize(width, height)

	// prevents empty rows that were filled up by previous extendRowsToHeight
	// in the event of height < previous height.
	b.truncateBottomEmptyLines()
	// ensures that we don't break alternate buffer
	// invariant of having at least n=height rows
	b.extendRowsToHeight()
	b.Cells.ResetCapacity(width)
	b.ResetOffset()
}

func (b *PrimaryBuffer) truncateBottomEmptyLines() {
	cells := b.Cells.RawCells()
lines:
	for y := b.Cells.Rows() - 1; y > 0 && y >= b.AltBuffer.height-1; y-- {
		for x := 0; x < b.Cells.Columns(y); x++ {
			cell := cells[y][x]
			if cell.Ch != DefaultChar && cell.Ch != ' ' {
				break lines
			}
		}
		from := term.Coordinates{Y: y}
		to := term.Coordinates{Y: y}
		b.Cells.DeleteLineContext(b.AltBuffer.ctx, from, to)
	}
}

func (b *PrimaryBuffer) extendRowsToHeight() {
	if b.AltBuffer.width > 0 && b.AltBuffer.height > 0 && b.Cells.Rows() < b.AltBuffer.height {
		pos := term.Coordinates{Y: b.AltBuffer.height - 1, X: b.AltBuffer.width - 1}
		b.Cells.InsertContext(b.AltBuffer.ctx, pos, b.AltBuffer.defaultChar)
	}
}

// Dimensions returns the dimensions of this buffer.
func (b *PrimaryBuffer) Dimensions() (width, height int) {
	width = b.width
	height = b.height
	return
}

// ScrollUp panics. Use MoveToOffset or SetOffset.
func (b *PrimaryBuffer) ScrollUp(rows int) {
	// this prevents calling AltBuffer's ScrollUp inadvertently
	// and forces thinking about MoveToOffset vs SetOffset.
	panic("ScrollUp not supported for primary buffer, use SetOffset or MoveToOffset instead")
}

// ScrollDown panics. Use MoveToOffset or SetOffset.
func (b *PrimaryBuffer) ScrollDown(rows int) {
	// this prevents calling AltBuffer's ScrollDown inadvertently
	// and forces thinking about MoveToOffset vs SetOffset.
	panic("ScrollDown not supported for primary buffer, use SetOffset or MoveToOffset instead")
}

// MoveToOffset moves to the new offset.
// It does not change the cursor content/scroll position, thus
// changes the screen cursor position.
func (b *PrimaryBuffer) MoveToOffset(offset term.Coordinates) {
	b.AltBuffer.scroll.SetOffset(offset)
}

// SetOffset sets the raw offset of the underlying scroll.
// It does not change the screen cursor position, thus
// changes the content/scroll position.
func (b *PrimaryBuffer) SetOffset(offset term.Coordinates) {
	orig := b.CursorAtScreen()
	b.AltBuffer.scroll.SetOffset(offset)
	b.SetCursorAtScreen(orig, false)
	// do not violate all screen must be always filled invariant
	// or else very nasty side-effects can occur, like
	// scrollDown not using correct coordinates
	if diff := b.Offset().Y - b.MaxOffset(); diff > 0 {
		y := b.Rows() + diff - 1
		x := b.Width() - 1
		b.Cells.InsertContext(b.ctx,
			term.Coordinates{Y: y, X: x}, b.defaultChar)
	}
}

// Offset returns the scroll offset of this PrimaryBuffer.
func (b *PrimaryBuffer) Offset() term.Coordinates {
	return b.AltBuffer.scroll.Offset()
}

// TopScrollableRegion is always 0 for a PrimaryBuffer,
// scrollable regions are not supported.
func (b *PrimaryBuffer) TopScrollableRegion() int {
	return 0
}

// BottomScrollableRegion is always height for a PrimaryBuffer,
// scrollable regions are not supported.
func (b *PrimaryBuffer) BottomScrollableRegion() int {
	return b.height
}

// InsertLines inserts blank lines on the cursor's position.
func (b *PrimaryBuffer) InsertLines(count int) {
	var builder strings.Builder
	for range count {
		_ = builder.WriteByte('\n')
	}
	pos := b.cursor.position
	pos.X = b.Cells.Columns(pos.Y)
	b.Cells.Edit(b.ctx, pos, pos, builder.String())
}

// DeleteLinesCursor deletes lines on the cursor's position.
func (b *PrimaryBuffer) DeleteLinesCursor(count int) {
	from := b.cursor.position
	to := from
	to.Y += count
	b.Cells.DeleteLineContext(b.ctx, from, to)
}

// DeleteLines deletes the lines from start to end, inclusively.
func (b *PrimaryBuffer) DeleteLines(start, end int) {
	if start >= end {
		return
	}
	rowcount := b.Cells.Rows()
	if end > rowcount {
		end = rowcount
	}
	orig := b.CursorAtScreen()

	from := term.Coordinates{Y: start}
	to := term.Coordinates{Y: end, X: b.Cells.Columns(end)}
	b.Cells.DeleteLineContext(b.ctx, from, to)

	orig.Y -= end - start + 1
	if orig.Y > 0 {
		orig.Y = 0
	}
	b.SetCursorAtScreen(orig, false)
}

// Reset clears the screen and removes history, effectively
// leaving the content as blank and the cursor position at the top.
func (b *PrimaryBuffer) Reset() {
	b.resetLinesTrim(0, b.height, true, b.defaultChar)
	b.scroll.SetOffset(term.Coordinates{})
	b.SetCursorAtScroll(term.Coordinates{}, false)
}

// SetCursorAtScroll sets the cursor at the content/scroll position c.
// The relative argument is ignored for PrimaryBuffer.
func (b *PrimaryBuffer) SetCursorAtScroll(c term.Coordinates, relative bool) {
	b.cursor.position.X = int(math.Max(float64(c.X), 0))
	b.cursor.position.Y = int(math.Max(float64(c.Y), 0))
}

// SetCursorAtScreen sets the cursor at the screen position c.
// The relative argument is ignored for PrimaryBuffer.
func (b *PrimaryBuffer) SetCursorAtScreen(c term.Coordinates, relative bool) {
	b.SetCursorAtScroll(term.CoordinatesSum(c, b.scroll.Offset()), false)
}

// CursorAtScroll returns the cursor position in relation to the underlying
// content scroll.
func (b *PrimaryBuffer) CursorAtScroll() term.Coordinates {
	return b.cursor.position
}

// CursorAtScreen returns the current cursor position in relation to the
// screen coordinates.
func (b *PrimaryBuffer) CursorAtScreen() term.Coordinates {
	return term.CoordinatesDiff(b.cursor.position, b.scroll.Offset())
}

// ResetOffset resets the offset to MaxOffset.
func (b *PrimaryBuffer) ResetOffset() {
	b.MoveToOffset(term.Coordinates{Y: b.MaxOffset()})
}

// MaxOffset returns the max offset such that the last line is at its bottommost position.
// If there's enough content to fill up the height of the screen, then that is height - 1,
// otherwise it ensures that the view content start at position 0.
func (b *PrimaryBuffer) MaxOffset() int {
	return int(math.Max(float64(b.Rows()-b.height), 0))
}

// ResetLines erases all the lines from start to end.
// The start to end range is left inclusive, right exclusive.
// As oppposed to AltBuffer's ResetLines, the `end` argument is not capped.
func (b *PrimaryBuffer) ResetLines(start, end int) {
	b.AltBuffer.ResetLinesWith(start, end, b.defaultChar)
}
