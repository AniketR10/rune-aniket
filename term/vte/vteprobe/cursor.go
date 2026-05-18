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
// on-disk file content.
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
//   - the URI of the file currently displayed by the editor.
//
// Output: the inferred (line, col) into the file content together with
// a confidence score.
package vteprobe

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/logging"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
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
}

// ErrUnknown is returned by Cursor.Infer when the package cannot infer
// a position with at least the configured minimum confidence — for
// example because the cursor lies on a chrome row, or because the rendered
// content does not align convincingly with the file.
var ErrUnknown = errors.New("vteprobe: cursor position unknown")

// Cursor maps a terminal cursor position to a file-content (line, col).
// Cursors are safe for concurrent use; the only mutable state is a
// small mtime-keyed content cache protected by a mutex.
type Cursor struct {
	fs            workspaceapi.FileSystem
	tabstopHints  []int
	minConfidence float64
	maxFileBytes  int64

	mu    sync.Mutex
	cache map[string]fileCacheEntry
}

type fileCacheEntry struct {
	mtimeUnixNano int64
	size          int64
	lines         []string
}

// New constructs a Cursor.
//
// All parameters are required; supplying a wrong value (for example a
// candidate tabstop list that does not include the value the editor is
// using) silently yields wrong Results. Callers must pass real values.
//
//   - fs:            the file system used to read on-disk file content.
//   - tabstopHints:  candidate tabstops to try when aligning content
//     with rendered rows. The first hint is the tie-breaker when
//     two candidates score equally.
//   - minConfidence: threshold in [0, 1] below which Infer returns
//     ErrUnknown instead of a Result.
//   - maxFileBytes:  upper bound for file content read from fs. Files
//     larger than this cause Infer to return ErrUnknown.
func New(
	fs workspaceapi.FileSystem,
	tabstopHints []int,
	minConfidence float64,
	maxFileBytes int64,
) *Cursor {
	if fs == nil {
		panic("vteprobe: nil FileSystem")
	}
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
		fs:            fs,
		tabstopHints:  cleaned,
		minConfidence: minConfidence,
		maxFileBytes:  maxFileBytes,
		cache:         make(map[string]fileCacheEntry),
	}
}

// Infer maps cur (a screen-relative cursor position inside the rendered
// cell grid cells) to a position in the file identified by uri.
// Returns ErrUnknown when the resulting confidence falls below the
// threshold configured in New.
//
// cells is the live rendered grid — typically Scroll.Buffer().RawCells()
// on the active vte.Component buffer (or the grid returned by
// vte.Replay). No copy is made, so callers must not mutate cells (or
// the underlying buffer) for the duration of the call.
func (i *Cursor) Infer(
	ctx context.Context,
	uri workspaceapi.URI,
	cells [][]term.Cell,
	cur term.Coordinates,
) (res Result, err error) {
	defer func() {
		if !log.IsLevelEnabled(log.DebugLevel) {
			return
		}
		log.WithField(logging.KeyClass, "vteprobe.Cursor").
			Debugf("Infer uri=%q cur=%+v -> res=%+v err=%v",
				uri.String(), cur, res, err)
	}()

	lines, err := i.readFileLines(uri)
	if err != nil {
		return Result{}, err
	}

	rows := extractRows(cells)
	if len(rows) == 0 {
		return Result{}, ErrUnknown
	}

	// 1. Chrome detection: trim status-line bands.
	top, bot := detectChrome(rows, lines)
	if cur.Y < top || cur.Y > bot {
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
		align = alignByGutter(rows, top, bot, gut, lines, i.tabstopHints)
	}
	gutterAligned := align.ok
	if !align.ok {
		gutterWidth := 0
		if gut.present {
			gutterWidth = gut.width
		}
		align = alignByContentWithGutter(rows, top, bot, gutterWidth, lines, i.tabstopHints)
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
	wrap := detectWrap(rows, top, bot, gut.width, align, lines)

	// 5. Cursor mapping.
	fileLine, ok := wrap.fileLineAtRow(cur.Y)
	if !ok {
		log.WithField(logging.KeyClass, "vteprobe.Cursor").
			Debugf("Infer cursor row has no file line: "+
				"cur.Y=%d band=[%d,%d] align.topFileLine=%d "+
				"coverage=%.3f tabstop=%d",
				cur.Y, top, bot, align.topFileLine,
				align.coverage, align.tabstop)
		return Result{}, ErrUnknown
	}
	folded := wrap.foldedAtRow(cur.Y)
	row := rows[cur.Y]
	runeOffset := row.runeColAt(cur.X) - gut.width + wrap.wrapOffsetAtRow(cur.Y)
	if runeOffset < 0 {
		// Cursor is inside the gutter; treat as column 1 on the line.
		runeOffset = 0
	}
	fileLineText := ""
	if fileLine >= 1 && fileLine <= len(lines) {
		fileLineText = lines[fileLine-1]
	}
	fileCol := visualToRawCol(fileLineText, runeOffset, align.tabstop)

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
	}
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithField(logging.KeyClass, "vteprobe.Cursor").
			Debugf("Infer aligned: band=[%d,%d] gutter=%+v "+
				"gutterAligned=%t coverage=%.3f tabstop=%d "+
				"runeOffset=%d wrapOffset=%d folded=%t",
				top, bot, gut, gutterAligned, align.coverage,
				align.tabstop, runeOffset,
				wrap.wrapOffsetAtRow(cur.Y), folded)
	}
	if res.Confidence < i.minConfidence {
		log.WithField(logging.KeyClass, "vteprobe.Cursor").
			Debugf("Infer below minConfidence=%.3f: %+v",
				i.minConfidence, res)
		return Result{}, ErrUnknown
	}
	return res, nil
}

// readFileLines reads the file at uri and returns its content split into
// lines (without trailing newline). Results are cached keyed by URI plus
// mtime so repeated calls during a session are cheap, while edits to the
// file invalidate the cache.
func (i *Cursor) readFileLines(uri workspaceapi.URI) ([]string, error) {
	path := uri.Path()
	if path == "" {
		return nil, fmt.Errorf("vteprobe: empty path for URI %q", uri.String())
	}

	info, err := i.fs.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("vteprobe: stat %q: %w", path, err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("vteprobe: %q is a directory", path)
	}
	if info.Size() > i.maxFileBytes {
		return nil, fmt.Errorf(
			"vteprobe: file %q size %d exceeds limit %d",
			path, info.Size(), i.maxFileBytes)
	}

	key := uri.String()
	mtime := info.ModTime().UnixNano()
	size := info.Size()

	i.mu.Lock()
	hit, ok := i.cache[key]
	i.mu.Unlock()
	if ok && hit.mtimeUnixNano == mtime && hit.size == size {
		return hit.lines, nil
	}

	f, err := i.fs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("vteprobe: open %q: %w", path, err)
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, i.maxFileBytes+1))
	if err != nil {
		return nil, fmt.Errorf("vteprobe: read %q: %w", path, err)
	}
	if int64(len(data)) > i.maxFileBytes {
		return nil, fmt.Errorf(
			"vteprobe: file %q size exceeds limit %d", path, i.maxFileBytes)
	}

	lines := splitLines(data)
	i.mu.Lock()
	i.cache[key] = fileCacheEntry{
		mtimeUnixNano: mtime,
		size:          size,
		lines:         lines,
	}
	i.mu.Unlock()
	return lines, nil
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
