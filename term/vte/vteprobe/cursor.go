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

// Package vteprobe maps a VTE-rendered terminal-cursor position back to
// a file-content (line, col) by aligning the cell grid against the
// file content supplied by the caller.
//
// The package has zero knowledge of which editor produced the buffer.
// Chrome (status lines), gutters (line numbers), wraps, and fold
// placeholders are detected structurally — through attributes and
// regex matches against the rendered text — never by editor name.
//
// Inputs:
//   - the rendered cell grid (a *cell.Buffer returned by, for example,
//     vte.Replay or vte.Component.Snapshot);
//   - the cursor position in screen coordinates (X column, Y row);
//   - the file content the editor is displaying, as pre-split cell rows.
//
// Output: the inferred (line, col) into the file content together with
// a confidence score.
package vteprobe

import (
	"errors"
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Result is the outcome of Cursor.Infer when the cursor is mapped to a
// position in the file.
type Result struct {
	// CursorAtScroll is the 0-based file position of the cursor:
	// X is the rune column (tab expansion unwound via Tabstop) and
	// Y is the line index. Both are zero-based to match the term
	// coordinate convention used elsewhere in the codebase.
	CursorAtScroll term.Coordinates
	// Scroll is the 0-based file position of the top-left visible
	// cell of the editor's content band: Y is the file line displayed
	// on the first content row (after chrome), X is the horizontal
	// offset into that line — for editors that allow side-scrolling a
	// long line off-screen. The current implementation always reports
	// X=0 because horizontal-scroll detection is not yet implemented;
	// the field is reserved so callers can rely on the shape.
	Scroll term.Coordinates
	// Tabstop is the inferred tabstop the editor uses when expanding
	// tab characters. One of the candidates passed to New.
	Tabstop int
	// Folded is true when the cursor sits on a row that the editor is
	// displaying as a fold placeholder. FileLine then points at the
	// first folded line.
	Folded bool
	// Confidence is a value in [0, 1] estimating how likely the result
	// is correct. Callers that received a non-error Result may still
	// want to threshold it depending on their tolerance for mistakes.
	Confidence float64
	// Bands describes the editor's chrome and gutter layout: the
	// inclusive [Top, Bottom] terminal-row range that holds file
	// content, and GutterWidth the number of visual columns consumed
	// by the line-number gutter (0 when no gutter is present). Right
	// chrome (e.g. a sign column on the right edge) is not detected;
	// the right edge is the grid width.
	Bands Bands
	// Rows projects each terminal row of the rendered grid back to a
	// position in the file. Len(Rows) equals the number of rows in
	// the input cell grid. Rows outside the content band have
	// FileLine == 0 and Folded == false. The leading content row
	// reports WrapOffset == 0; soft-wrap continuation rows carry a
	// positive WrapOffset (rune-cell offset into the file line).
	Rows []RowMapping
	// FileLines is the file content as pre-split cell rows, as supplied
	// to Infer. Callers that need to translate raw file columns into the
	// visual columns used by Rows/Bands can read this slice. Indices are
	// 0-based; Len matches the number of source lines. The slice is the
	// same one passed to Infer — callers must not mutate it.
	FileLines [][]term.Cell
}

// Bands describes the chrome and gutter layout the probe inferred.
type Bands struct {
	// Top is the inclusive terminal-row index of the first content
	// row (the first row after any top chrome).
	Top int
	// Bottom is the inclusive terminal-row index of the last content
	// row (the last row before any bottom chrome).
	Bottom int
	// GutterWidth is the visual column width consumed by the
	// line-number gutter at the left of the content band. Zero when
	// no gutter is present.
	GutterWidth int
	// GridWidth is the total visual width of the terminal grid in
	// cells. Right-edge chrome (e.g. a sign column) is not detected;
	// the body width is GridWidth - GutterWidth.
	GridWidth int
}

// RowMapping describes how a single terminal row projects back to a
// position in the file.
type RowMapping struct {
	// FileLine is the 1-based file line displayed on this row, or 0
	// when the row is not anchored to a file line (chrome row,
	// out-of-band, or empty trailing row past EOF).
	FileLine int
	// WrapOffset is the rune-cell offset (after gutter strip and tab
	// expansion) at which the row starts into its FileLine. Always 0
	// for the leading segment of a non-wrapped line and for the first
	// segment of a soft-wrapped line.
	WrapOffset int
	// Folded is true when the row is a fold placeholder. FileLine
	// then points at the first folded line.
	Folded bool
}

// ErrUnknown is returned by Cursor.Infer when the package cannot infer
// a position with at least the configured minimum confidence — for
// example because the cursor lies on a chrome row, or because the rendered
// content does not align convincingly with the file.
var ErrUnknown = errors.New("vteprobe: cursor position unknown")

// Cursor maps a terminal cursor position to a file-content (line, col).
// Cursors are safe for concurrent use; they hold no mutable state.
type Cursor struct {
	tabstopHints  []int
	minConfidence float64
	maxFileBytes  int64
}

// New constructs a Cursor.
//
// All parameters are required; supplying a wrong value (for example a
// candidate tabstop list that does not include the value the editor is
// using) silently yields wrong Results. Callers must pass real values.
//
//   - tabstopHints:  candidate tabstops to try when aligning content
//     with rendered rows. The first hint is the tie-breaker when
//     two candidates score equally.
//   - minConfidence: threshold in [0, 1] below which Infer returns
//     ErrUnknown instead of a Result.
//   - maxFileBytes:  upper bound for the supplied file content. Content
//     larger than this causes Infer to return ErrUnknown.
func New(
	tabstopHints []int,
	minConfidence float64,
	maxFileBytes int64,
) *Cursor {
	if len(tabstopHints) == 0 {
		panic("vteprobe: tabstopHints must not be empty")
	}
	if minConfidence < 0 || minConfidence > 1 {
		panic(fmt.Sprintf(
			"vteprobe: minConfidence out of range: %v", minConfidence))
	}
	if maxFileBytes <= 0 {
		panic(fmt.Sprintf(
			"vteprobe: maxFileBytes must be > 0: %v", maxFileBytes))
	}

	cleaned := make([]int, 0, len(tabstopHints))
	for _, t := range tabstopHints {
		if t > 0 {
			cleaned = append(cleaned, t)
		}
	}
	if len(cleaned) == 0 {
		panic("vteprobe: tabstopHints must contain at least one positive value")
	}

	return &Cursor{
		tabstopHints:  cleaned,
		minConfidence: minConfidence,
		maxFileBytes:  maxFileBytes,
	}
}

// Infer maps cur (a screen-relative cursor position inside the rendered
// cell grid cells) to a position in the file content given by fileLines.
// Returns ErrUnknown when the resulting confidence falls below the
// threshold configured in New.
//
// cells is the live rendered grid — typically Scroll.Buffer().RawCells()
// on the active vte.Component buffer (or the grid returned by
// vte.Replay). No copy is made, so callers must not mutate cells (or
// the underlying buffer) for the duration of the call.
//
// fileLines is the file content the editor is displaying, as pre-split
// cell rows without a trailing synthetic empty row. It is the caller's
// authoritative copy of the buffer, never re-read from disk here. The
// slice is not copied and surfaces unchanged as Result.FileLines, so
// callers must not mutate it for the duration of the call.
//
// slab is a caller-provided scratch arena. Passing the same non-nil
// *Slab across successive calls recycles the per-call working memory
// (tab-expanded lines and alignment scratch), which matters for callers
// that probe on every cursor move over a large file. The slab carries
// no results between calls, so Infer is still a pure function of (fileLines,
// cells, cur); slab only affects allocation. A nil slab allocates
// fresh. The slab must not be shared across concurrent calls.
func (i *Cursor) Infer(
	cells [][]term.Cell,
	cur term.Coordinates,
	fileLines [][]term.Cell,
	slab *Slab,
) (res Result, err error) {
	defer func() {
		if !log.IsLevelEnabled(log.DebugLevel) {
			return
		}
		log.WithField(logging.KeyClass, "vteprobe.Cursor").
			Debugf("Infer lines=%d cur=%+v -> res=%+v err=%v",
				len(fileLines), cur, res, err)
	}()

	if slab == nil {
		slab = NewSlab()
	}
	slab.reset()

	if i.tooLarge(fileLines) {
		return Result{}, ErrUnknown
	}

	rows := extractRowsWithSlab(cells, slab)
	if len(rows) == 0 {
		return Result{}, ErrUnknown
	}

	// 1. Chrome detection: trim status-line bands.
	top, bot := detectChrome(rows, fileLines)
	// Editors such as nano let the cursor rest on the blank virtual line
	// immediately past the last file line (between the last content row
	// and the bottom chrome). Treat that one row as part of the band so
	// the cursor maps to the line just past EOF instead of being rejected
	// outright, which would discard the otherwise-valid row mapping.
	cursorPastEOF := cur.Y == bot+1 && bot >= top && isBlankRow(rows, cur.Y)
	if cur.Y < top || (cur.Y > bot && !cursorPastEOF) {
		log.WithField(logging.KeyClass, "vteprobe.Cursor").
			Debugf("Infer cursor outside content band: "+
				"cur.Y=%d band=[%d,%d]", cur.Y, top, bot)
		return Result{}, ErrUnknown
	}

	// 2. Gutter detection on the content band.
	gut := detectGutter(rows, top, bot)

	// 3. Alignment: gutter fast path, fall back to content alignment
	// (still stripping the gutter so cursor offsets are correct).
	// Falling back is what handles relative line numbering: the gutter
	// is structurally there but its numbers are not absolute, so
	// content alignment recovers the file lines without any
	// editor-specific logic.
	var align alignment
	if gut.present {
		align = alignByGutter(rows, top, bot, gut, fileLines, i.tabstopHints, slab)
	}
	gutterAligned := align.ok
	if !align.ok {
		gutterWidth := 0
		if gut.present {
			gutterWidth = gut.width
		}
		align = alignByContentWithGutter(rows, top, bot, gutterWidth, fileLines, i.tabstopHints, slab)
		if !align.ok {
			log.WithField(logging.KeyClass, "vteprobe.Cursor").
				Debugf("Infer alignment failed: "+
					"band=[%d,%d] gutter=%+v "+
					"tabstopHints=%v",
					top, bot, gut, i.tabstopHints)
			return Result{}, ErrUnknown
		}
	}

	// 4. Wrap / fold detection on the aligned band. Wrap detection may
	// reject the alignment if it cannot fit visible rows into the file
	// line range.
	wrap := detectWrap(rows, top, gut.width, align, fileLines, slab)

	// 5. Cursor mapping.
	// When the cursor sits past EOF, anchor it to the row above (the last
	// content row) and report the file line one past it at column 0.
	cursorRow := cur.Y
	if cursorPastEOF {
		cursorRow = bot
	}
	fileLine, ok := wrap.fileLineAtRow(cursorRow)
	if !ok {
		log.WithField(logging.KeyClass, "vteprobe.Cursor").
			Debugf("Infer cursor row has no file line: "+
				"cur.Y=%d band=[%d,%d] align.topFileLine=%d "+
				"coverage=%.3f tabstop=%d",
				cur.Y, top, bot, align.topFileLine,
				align.coverage, align.tabstop)
		return Result{}, ErrUnknown
	}
	folded := wrap.foldedAtRow(cursorRow)
	var (
		fileCol    int
		runeOffset int
	)
	if cursorPastEOF {
		// One file line past the last content row, at column 0.
		fileLine++
		fileCol = 1
		folded = false
	} else {
		row := rows[cur.Y]
		runeOffset = row.runeColAt(cur.X) - gut.width + wrap.wrapOffsetAtRow(cur.Y)
		if runeOffset < 0 {
			// Cursor is inside the gutter; treat as column 1 on the line.
			runeOffset = 0
		}
		var fileLineCells []term.Cell
		if fileLine >= 1 && fileLine <= len(fileLines) {
			fileLineCells = fileLines[fileLine-1]
		}
		fileCol = visualToRawColCells(fileLineCells, runeOffset, align.tabstop)
	}

	res = Result{
		CursorAtScroll: term.Coordinates{
			X: fileCol - 1,
			Y: fileLine - 1,
		},
		Scroll: term.Coordinates{
			X: 0, // horizontal-scroll detection not implemented.
			Y: align.topFileLine - 1,
		},
		Tabstop:    align.tabstop,
		Folded:     folded,
		Confidence: computeConfidence(align, wrap, rows, cur),
		Bands: Bands{
			Top:         top,
			Bottom:      bot,
			GutterWidth: gut.width,
			GridWidth:   gridWidth(rows),
		},
		Rows:      buildRowMappings(len(rows), wrap),
		FileLines: fileLines,
	}
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField(logging.KeyClass, "vteprobe.Cursor").
			Debugf("Infer aligned: band=[%d,%d] gutter=%+v "+
				"gutterAligned=%t coverage=%.3f tabstop=%d "+
				"runeOffset=%d wrapOffset=%d folded=%t",
				top, bot, gut, gutterAligned, align.coverage,
				align.tabstop, runeOffset,
				wrap.wrapOffsetAtRow(cursorRow), folded)
	}
	if res.Confidence < i.minConfidence {
		log.WithField(logging.KeyClass, "vteprobe.Cursor").
			Debugf("Infer below minConfidence=%.3f: %+v",
				i.minConfidence, res)
		return Result{}, ErrUnknown
	}
	return res, nil
}

// tooLarge reports whether the supplied lines exceed the maxFileBytes
// guard configured in New, counting one byte per rune plus a newline
// per line. The bound only needs to be approximate: it keeps Infer from
// aligning against pathologically large buffers.
func (i *Cursor) tooLarge(lines [][]term.Cell) bool {
	var n int64
	for _, l := range lines {
		n += int64(lineRuneLen(l)) + 1
		if n > i.maxFileBytes {
			return true
		}
	}
	return false
}

// isBlankRow reports whether the terminal row at y holds no visible
// content (only blanks). Used to recognise the empty virtual line some
// editors leave past the last file line.
func isBlankRow(rows []extractedRow, y int) bool {
	if y < 0 || y >= len(rows) {
		return false
	}
	return len(trimRightSpace(rows[y].runes)) == 0
}

// splitLines splits data on \n, stripping a single trailing \r per line.
// Unlike bufio.Scanner, this preserves a trailing empty line when data
// ends with a newline.
func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	lines := make([]string, 0, 64)
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] != '\n' {
			continue
		}
		end := i
		if end > start && data[end-1] == '\r' {
			end--
		}
		lines = append(lines, string(data[start:end]))
		start = i + 1
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}
