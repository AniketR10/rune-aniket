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
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/ernestrc/logd-go/logging"
	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/term"
)

// PrimaryBuffer wraps a Buffer to provide scroll-back for a primary vte screen buffer.
type PrimaryBuffer struct {
	AltBuffer
	wraps     int
	wrapLines map[int]int
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
	b.wrapLines = make(map[int]int)
}

// Resize resizes this Buffer and resets the vertical margins.
func (b *PrimaryBuffer) Resize(width, height int) {
	isMaxOffset := b.scroll.Offset().Y >= b.maxOffset(b.height)
	var wraps int
	if width != 0 && width > b.width {
		wraps = b.growColumns(width, height)
	} else if width != 0 && width < b.width {
		wraps = b.shrinkColumns(width)
	}
	if height != 0 && (height > b.height || wraps < b.wraps) {
		b.growLines(isMaxOffset, width, height)
	}
	if height != 0 && (height < b.height || wraps > b.wraps) {
		b.shrinkLines(isMaxOffset, height)
	}
	b.wraps = wraps
	b.width = width
	b.height = height
	b.scroll.Resize(width, height)
	b.Cells.ResetCapacity(width)
}

func (b *PrimaryBuffer) growLines(isMaxOffset bool, width, height int) {
	if b.Cells.Rows() < height {
		pos := term.Coordinates{
			Y: max(0, height-1),
			X: max(0, width-1),
		}
		b.Cells.InsertContext(b.AltBuffer.ctx, pos, b.AltBuffer.defaultChar)
	}
	// do not do update offset to new max offset if not in max offset
	if !isMaxOffset {
		return
	}
	newMaxOffset := b.maxOffset(height)
	b.MoveToOffset(term.Coordinates{Y: newMaxOffset})
}

func (b *PrimaryBuffer) shrinkLines(isMaxOffset bool, height int) {
	for y := b.Cells.Rows() - 1; y > 0 && b.Cells.Rows() > height; y-- {
		if y == b.cursor.position.Y {
			break
		}
		from := term.Coordinates{Y: y}
		to := term.Coordinates{Y: y}
		b.Cells.DeleteLineContext(b.AltBuffer.ctx, from, to)
	}
	// do not do update offset to new max offset if not in max offset
	if !isMaxOffset {
		return
	}
	newMaxOffset := b.maxOffset(height)
	b.MoveToOffset(term.Coordinates{Y: newMaxOffset})
}

func (b *PrimaryBuffer) growColumns(width, height int) (wraps int) {
	if width == 0 {
		return
	}
	var y int
	for y = max(0, b.Cells.Rows()-1); y > 0; y-- {
		if y == b.cursor.position.Y {
			wraps = b.wrapTopLines(y, width)
			break
		}
		if b.Cells.Columns(y) < width {
			at := term.Coordinates{Y: y, X: width - 1}
			b.Cells.InsertContext(b.AltBuffer.ctx, at, b.AltBuffer.defaultChar)
		}
	}

	b.cursor.position.Y = max(0, b.cursor.position.Y+wraps)
	offset := b.AltBuffer.scroll.Offset()
	offset.Y = max(0, offset.Y+wraps)
	b.MoveToOffset(offset)
	b.savedCursor.position.Y = max(0, b.savedCursor.position.Y+wraps)
	b.savedCursor.position.X = min(b.savedCursor.position.X, width)
	return
}

func (b *PrimaryBuffer) shrinkColumns(width int) (wraps int) {
	var y int
	for y = max(0, b.Cells.Rows()-1); y > 0; y-- {
		if y == b.cursor.position.Y {
			wraps = b.wrapTopLines(y, width)
			break
		}
		if cols := b.Cells.Columns(y); cols > width {
			from := term.Coordinates{Y: y, X: width}
			to := term.Coordinates{Y: y, X: cols}
			b.Cells.DeleteContext(b.AltBuffer.ctx, from, to)
		}
	}

	for ; y > 0; y-- {
		if cols := b.Cells.Columns(y); cols > width {
			from := term.Coordinates{Y: y, X: width}
			to := term.Coordinates{Y: y, X: cols}
			b.Cells.DeleteContext(b.AltBuffer.ctx, from, to)
		}
	}

	b.cursor.position.Y = max(0, b.cursor.position.Y+wraps)
	offset := b.AltBuffer.scroll.Offset()
	offset.Y = max(0, offset.Y+wraps)
	b.MoveToOffset(offset)
	b.savedCursor.position.Y = max(0, b.savedCursor.position.Y+wraps)
	b.savedCursor.position.X = min(b.savedCursor.position.X, width)
	return
}

func (b *PrimaryBuffer) wrapTopLines(at, width int) (n int) {
	if width == 0 {
		return
	}
	// unwrap previous wraps
	var lines []int
	for y := range b.wrapLines {
		lines = append(lines, y)
	}
	sort.Ints(lines)
	for _, y := range lines {
		count := b.wrapLines[y]
		for range count {
			if _, ok := b.Cells.ConflateRowContext(b.AltBuffer.ctx, y); ok {
				at--
				n--
			}
		}
	}
	b.log(log.TraceLevel, "un-wrapped %d lines: %#v", -n, b.wrapLines)
	clear(b.wrapLines)
	if at < 0 {
		return
	}
	// wraps represents the wrapped lines and so
	// it's always a positive number, whereas n represent the total
	// wraps variation, so it can be negative if we unwrapped more
	// lines that we wrapped.
	var wraps int
	for y := 0; y < at && y < b.Cells.Rows(); y++ {
		var x int
		for x = b.Cells.Columns(y) - 1; x > 0; x-- {
			cell, _ := b.Cells.Cell(term.Coordinates{Y: y, X: x})
			if cell.Ch != b.defaultChar && cell.Ch != ' ' {
				break
			}
		}
		cols := b.Cells.Columns(y)
		switch {
		case x < width && width <= cols:
			from := term.Coordinates{Y: y, X: width}
			to := term.Coordinates{Y: y, X: cols}
			b.Cells.DeleteContext(b.AltBuffer.ctx, from, to)
		case x >= width && width <= cols:
			from := term.Coordinates{Y: y, X: x + 1}
			to := term.Coordinates{Y: y, X: cols}
			b.Cells.DeleteContext(b.AltBuffer.ctx, from, to)
			// use the new number of columns
			cols = b.Cells.Columns(y)
			times := cols / width
			remainder := cols % width
			if remainder == 0 {
				times--
			}
			// y represents current working row, like screen coordinates,
			// whereas yi represents the original, pre-wrap row, like scroll coordinates.
			yi := y - wraps
			for i := times; i > 0 && b.Cells.WrapRowContext(b.AltBuffer.ctx, y, width*i); i-- {
				b.wrapLines[yi] = b.wrapLines[yi] + 1
				n++
				at++
				wraps++
			}
			b.log(log.TraceLevel, "wrapped a new line line (%d): %#v", yi, b.wrapLines)
		case x < width && width > cols:
			at := term.Coordinates{Y: y, X: width - 1}
			b.Cells.InsertContext(b.AltBuffer.ctx, at, b.AltBuffer.defaultChar)
		default:
			panic("pack it up boys")
		}
	}

	// grow history lines that fall short or were wrapped
	for y := range at {
		if b.Cells.Columns(y) < width {
			at := term.Coordinates{Y: y, X: width - 1}
			b.Cells.InsertContext(b.AltBuffer.ctx, at, b.AltBuffer.defaultChar)
		}
	}
	return
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
// It also ensures that bottom lines grow if there's
// not enough lines from offset to bottom of the screen.
func (b *PrimaryBuffer) SetOffset(offset term.Coordinates) {
	prev := b.AltBuffer.scroll.Offset()
	orig := b.CursorAtScreen()
	if !b.AltBuffer.scroll.SetOffset(offset) {
		return
	}
	if offset.Y > prev.Y {
		b.growLines(false /* no offset change */, b.width, offset.Y+b.height)
	} else {
		b.shrinkLines(false /* no offset change */, offset.Y+b.height)
	}
	b.SetCursorAtScreen(orig, false)
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
	end = min(end, b.Cells.Rows()-1)
	orig := b.CursorAtScreen()

	from := term.Coordinates{Y: start}
	to := term.Coordinates{Y: end}
	b.Cells.DeleteLineContext(b.ctx, from, to)

	diff := end - start
	orig.Y = max(0, orig.Y-diff)
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
	b.log(log.TraceLevel, "set cursor at screen %#v (relative: %t), prev: %#v",
		c, relative, b.CursorAtScreen())
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

// MaxOffset returns the max offset such that the last line is at its bottommost position.
// If there's enough content to fill up the height of the screen, then that is height - 1,
// otherwise it ensures that the view content start at position 0.
func (b *PrimaryBuffer) MaxOffset() int {
	return b.maxOffset(b.height)
}

// ResetLines erases all the lines from start to end.
// The start to end range is left inclusive, right exclusive.
// As oppposed to AltBuffer's ResetLines, the `end` argument is not capped.
func (b *PrimaryBuffer) ResetLines(start, end int) {
	b.AltBuffer.ResetLinesWith(start, end, b.defaultChar)
}

func (b *PrimaryBuffer) maxOffset(height int) int {
	return int(math.Max(float64(b.cursor.position.Y+1-height), 0))
}

func (t *PrimaryBuffer) log(level log.Level, line string, params ...interface{}) {
	if !log.IsLevelEnabled(level) {
		return
	}
	log.WithField(logging.KeyClass, "vtescreen.PrimaryBuffer").
		WithField("instance", fmt.Sprintf("%p", t)).
		Logf(level, line, params...)
}
