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

package vteprobe

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
)

// drawRow renders a row of plain runes into term.Cells with the given
// per-cell attributes (or default when attrs is nil). Wide runes are
// not supported here; use drawWideRow for CJK content.
func drawRow(s string, attrs []term.Attributes) []term.Cell {
	out := make([]term.Cell, 0, len(s))
	i := 0
	for _, r := range s {
		var a term.Attributes
		if attrs != nil && i < len(attrs) {
			a = attrs[i]
		}
		out = append(out, term.Cell{Attributes: a, Ch: r, Width: 1})
		i++
	}
	return out
}

// padRow pads cells to width visible columns with spaces so the
// resulting RawCells row has a consistent visible width.
func padRow(cells []term.Cell, width int) []term.Cell {
	used := 0
	for _, c := range cells {
		w := int(c.Width)
		if w < 1 {
			w = 1
		}
		used += w
	}
	for used < width {
		cells = append(cells, term.Cell{Ch: ' ', Width: 1})
		used++
	}
	return cells
}

// makeBuffer assembles a *cell.Buffer from a list of row strings. Width
// determines the padded width of every row.
func makeBuffer(rows []string, width int) *cell.Buffer {
	cells := make([][]term.Cell, len(rows))
	for i, s := range rows {
		cells[i] = padRow(drawRow(s, nil), width)
	}
	return cell.CellsToBuffer(cells)
}

func makeBufferWithAttrs(rows []rowSpec, width int) *cell.Buffer {
	cells := make([][]term.Cell, len(rows))
	for i, r := range rows {
		cells[i] = padRow(drawRow(r.text, r.attrs), width)
	}
	return cell.CellsToBuffer(cells)
}

type rowSpec struct {
	text  string
	attrs []term.Attributes
}

// styledRow turns a string into a rowSpec whose every non-space rune
// carries the same attributes.
func styledRow(s string, a term.Attributes) rowSpec {
	attrs := make([]term.Attributes, len([]rune(s)))
	i := 0
	for _, r := range s {
		if r != ' ' {
			attrs[i] = a
		}
		i++
	}
	return rowSpec{text: s, attrs: attrs}
}

func plainRow(s string) rowSpec {
	return rowSpec{text: s}
}

// ----- fakeFS: minimal in-memory workspaceapi.FileSystem for tests -----

type fakeFS struct {
	files map[string]*fakeFileData
}

type fakeFileData struct {
	data    []byte
	modTime time.Time
}

func newFakeFS(files map[string][]byte) *fakeFS {
	fs := &fakeFS{files: map[string]*fakeFileData{}}
	now := time.Now()
	for path, data := range files {
		fs.files[path] = &fakeFileData{data: data, modTime: now}
	}
	return fs
}

func (f *fakeFS) bump(path string, data []byte) {
	if fd, ok := f.files[path]; ok {
		fd.data = data
		fd.modTime = fd.modTime.Add(time.Second)
		return
	}
	f.files[path] = &fakeFileData{data: data, modTime: time.Now()}
}

func (f *fakeFS) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + path)
}

func (f *fakeFS) OpenFile(path string, _ int, _ os.FileMode) (workspaceapi.File, error) {
	fd, ok := f.files[path]
	if !ok {
		return nil, errors.New("fakeFS: no such file: " + path)
	}
	return &fakeFile{data: bytes.NewReader(fd.data)}, nil
}

func (f *fakeFS) Stat(path string) (os.FileInfo, error) {
	fd, ok := f.files[path]
	if !ok {
		return nil, errors.New("fakeFS: no such file: " + path)
	}
	return fakeFileInfo{name: path, size: int64(len(fd.data)), modTime: fd.modTime}, nil
}

func (f *fakeFS) Remove(string) error { panic("fakeFS.Remove: not implemented") }
func (f *fakeFS) ReadDir(string) ([]os.DirEntry, error) {
	panic("fakeFS.ReadDir: not implemented")
}
func (f *fakeFS) MkdirAll(string, os.FileMode) error {
	panic("fakeFS.MkdirAll: not implemented")
}

type fakeFile struct {
	data *bytes.Reader
}

func (f *fakeFile) Name() string               { return "" }
func (f *fakeFile) Stat() (os.FileInfo, error) { return nil, io.EOF }
func (f *fakeFile) Sync() error                { return nil }
func (f *fakeFile) Truncate(int64) error       { return nil }
func (f *fakeFile) Fd() uintptr                { return 0 }
func (f *fakeFile) Seek(o int64, w int) (int64, error) {
	return f.data.Seek(o, w)
}
func (f *fakeFile) Read(p []byte) (int, error) { return f.data.Read(p) }
func (f *fakeFile) ReadAt(p []byte, off int64) (int, error) {
	return f.data.ReadAt(p, off)
}
func (f *fakeFile) Write([]byte) (int, error) { return 0, errors.New("read-only") }
func (f *fakeFile) Close() error              { return nil }

type fakeFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (i fakeFileInfo) Name() string       { return i.name }
func (i fakeFileInfo) Size() int64        { return i.size }
func (i fakeFileInfo) Mode() os.FileMode  { return 0 }
func (i fakeFileInfo) ModTime() time.Time { return i.modTime }
func (i fakeFileInfo) IsDir() bool        { return false }
func (i fakeFileInfo) Sys() any           { return nil }

// ----- synthetic test cases -----

func newCursor(t *testing.T, fs workspaceapi.FileSystem) *Cursor {
	t.Helper()
	return New(fs, []int{4, 2, 8}, 0.6, 1<<20)
}

// TestInferEmitsDebugLog verifies that every Infer call writes at least
// one DebugLevel entry, so production operators can correlate
// "the IDE statusbar showed line N" with "the probe inferred line N".
// The deferred log in Infer is the contract we lock in here; the
// internal alignment-detail entries are best-effort.
func TestInferEmitsDebugLog(t *testing.T) {
	const path = "/code.go"
	content := []byte("aaa\nbbb\nccc\n")
	buf := makeBuffer([]string{"aaa", "bbb", "ccc"}, 8)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	// Logrus is process-global; flip it on for this test and restore
	// the prior state afterwards so parallel tests are not affected.
	prevLevel := log.GetLevel()
	prevOut := log.StandardLogger().Out
	log.SetLevel(log.DebugLevel)
	var buffer bytes.Buffer
	log.SetOutput(&buffer)
	t.Cleanup(func() {
		log.SetLevel(prevLevel)
		log.SetOutput(prevOut)
	})

	inf := newCursor(t, fs)
	_, err = inf.Infer(context.Background(), uri, buf.RawCells(),
		term.Coordinates{X: 1, Y: 1})
	require.NoError(t, err)

	out := buffer.String()
	assert.Contains(t, out, "class=vteprobe.Cursor",
		"Infer must log under the vteprobe.Cursor class")
	assert.Contains(t, out, "Infer uri=",
		"deferred Infer log must include uri and result; got %q", out)
}

func TestInferGutterAlignsWithTabs(t *testing.T) {
	t.Parallel()

	const path = "/sample.go"
	const content = "package main\n" +
		"\n" +
		"import \"fmt\"\n" +
		"\n" +
		"func main() {\n" +
		"\tfmt.Println(\"hello\")\n" +
		"}\n"

	rows := []string{
		" 1 package main",
		" 2 ",
		" 3 import \"fmt\"",
		" 4 ",
		" 5 func main() {",
		" 6     fmt.Println(\"hello\")",
		" 7 }",
	}
	buf := makeBuffer(rows, 60)
	fs := newFakeFS(map[string][]byte{path: []byte(content)})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	// Cursor on the body of line 6 ("    fmt.Println..."), pointing at
	// 'f' in fmt. The gutter occupies 3 visual columns; visual column 7
	// is the 'f'.
	got, err := inf.Infer(context.Background(), uri, buf.RawCells(), term.Coordinates{X: 7, Y: 5})
	require.NoError(t, err)
	// Rendered visual column 7 -> body offset 4 (after gutter width 3),
	// which corresponds to the rune right after the tab on file line 6,
	// i.e. the 'f' in "\tfmt". Coordinates are 0-based.
	assert.Equal(t, term.Coordinates{X: 1, Y: 5}, got.CursorAtScroll)
	assert.Equal(t, term.Coordinates{X: 0, Y: 0}, got.Scroll)
	assert.Equal(t, 4, got.Tabstop)
}

func TestInferRelativeLineNumbers(t *testing.T) {
	t.Parallel()

	const path = "/code.go"
	content := []byte("aaa\nbbb\nccc\nddd\neee\n")

	// Relative numbering around line 3 (the current line). Editors
	// typically render the absolute number on the cursor row and
	// relative distances elsewhere.
	rows := []string{
		"2 aaa",
		"1 bbb",
		"3 ccc", // current line, shown absolute
		"1 ddd",
		"2 eee",
	}
	buf := makeBuffer(rows, 40)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	got, err := inf.Infer(context.Background(), uri, buf.RawCells(), term.Coordinates{X: 2, Y: 2})
	require.NoError(t, err)
	// The absolute number on the cursor row is 3, so the cursor maps
	// to file line 3 (Y=2), column 1 (X=0) — visual col 2 minus gutter
	// width 2 lands on the first body cell.
	assert.Equal(t, term.Coordinates{X: 0, Y: 2}, got.CursorAtScroll)
}

func TestInferCursorOnChromeRowIsUnknown(t *testing.T) {
	t.Parallel()

	reverse := term.Attributes{Attrs: term.AttrReverse}

	const path = "/code.go"
	content := []byte("foo\nbar\nbaz\n")
	rows := []rowSpec{
		plainRow("foo"),
		plainRow("bar"),
		plainRow("baz"),
		styledRow("-- INSERT --", reverse),
	}
	buf := makeBufferWithAttrs(rows, 40)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	_, err = inf.Infer(context.Background(), uri, buf.RawCells(), term.Coordinates{X: 5, Y: 3})
	assert.ErrorIs(t, err, ErrUnknown)
}

func TestInferFoldPlaceholder(t *testing.T) {
	t.Parallel()

	const path = "/code.go"
	content := []byte("alpha\nbeta\ngamma\ndelta\nepsilon\n")

	// Row 1 hides lines 2..4 inside a fold; gutter shows the first
	// folded line (2).
	rows := []string{
		" 1 alpha",
		" 2 +-- 3 lines: fold body ---",
		" 5 epsilon",
	}
	buf := makeBuffer(rows, 60)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	got, err := inf.Infer(context.Background(), uri, buf.RawCells(), term.Coordinates{X: 10, Y: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, got.CursorAtScroll.Y, "cursor on second file line (Y=1)")
	assert.True(t, got.Folded)
}

func TestInferStaleCacheInvalidation(t *testing.T) {
	t.Parallel()

	const path = "/code.go"
	original := []byte("foo\nbar\n")
	updated := []byte("foo\nBAR\nqux\n")

	fs := newFakeFS(map[string][]byte{path: original})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)

	buf1 := makeBuffer([]string{" 1 foo", " 2 bar"}, 30)
	res1, err := inf.Infer(context.Background(), uri, buf1.RawCells(), term.Coordinates{X: 4, Y: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, res1.CursorAtScroll.Y)

	// Bump the file: line 2 is now "BAR". The new buffer reflects the
	// edit; the inferrer must read the new content rather than trust
	// its cache.
	fs.bump(path, updated)
	buf2 := makeBuffer([]string{" 1 foo", " 2 BAR", " 3 qux"}, 30)
	res2, err := inf.Infer(context.Background(), uri, buf2.RawCells(), term.Coordinates{X: 5, Y: 1})
	require.NoError(t, err)
	// Body offset 5-3 = 2 → rune index 2 = 'R' on "BAR" (0-based).
	assert.Equal(t, term.Coordinates{X: 2, Y: 1}, res2.CursorAtScroll)
}

func TestInferLowConfidenceUnknown(t *testing.T) {
	t.Parallel()

	const path = "/code.go"
	content := []byte("alpha\nbeta\ngamma\n")

	// Render content totally unrelated to the file — no gutter, no
	// numeric markers, every line different from anything on disk.
	rows := []string{
		"zzz qqq",
		"yyy ppp",
	}
	buf := makeBuffer(rows, 20)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	_, err = inf.Infer(context.Background(), uri, buf.RawCells(), term.Coordinates{X: 0, Y: 0})
	assert.ErrorIs(t, err, ErrUnknown)
}

func TestInferFuzzyContentMatch(t *testing.T) {
	t.Parallel()

	const path = "/code.go"
	content := []byte("alpha foo\nbeta bar\ngamma baz\n")

	// The middle row swaps one char (b → B in "bar"); the alignment
	// must still succeed thanks to the fuzzy-prefix similarity ratio.
	rows := []string{
		"alpha foo",
		"beta Bar",
		"gamma baz",
	}
	buf := makeBuffer(rows, 20)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	got, err := inf.Infer(context.Background(), uri, buf.RawCells(), term.Coordinates{X: 0, Y: 1})
	require.NoError(t, err)
	assert.Equal(t, 1, got.CursorAtScroll.Y)
}

func TestInferWideChars(t *testing.T) {
	t.Parallel()

	const path = "/cjk.txt"
	// File content uses one wide rune (你, width 2) per line.
	content := []byte("你好 alpha\n你好 beta\n")

	// Build the rendered rows manually with explicit cell widths.
	wide := func(text string) []term.Cell {
		out := make([]term.Cell, 0, len(text))
		for _, r := range text {
			w := uint8(1)
			if r == '你' || r == '好' {
				w = 2
			}
			out = append(out, term.Cell{Ch: r, Width: w})
		}
		return out
	}
	rowCells := [][]term.Cell{
		padRow(wide("你好 alpha"), 20),
		padRow(wide("你好 beta"), 20),
	}
	buf := cell.CellsToBuffer(rowCells)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	// Cursor on the second visual cell of 你 (continuation cell) on
	// the second row. The mapping should resolve to file line 2 and
	// the rune containing 你 (file column 1).
	got, err := inf.Infer(context.Background(), uri, buf.RawCells(), term.Coordinates{X: 1, Y: 1})
	require.NoError(t, err)
	assert.Equal(t, term.Coordinates{X: 0, Y: 1}, got.CursorAtScroll)
}

func TestInferSoftWrap(t *testing.T) {
	t.Parallel()

	const path = "/long.txt"
	// One very long line that wraps across two terminal rows.
	long := "aaaaaaaaaa" + "bbbbbbbbbb" + "cccccccccc"
	content := []byte(long + "\nshort\n")

	// Terminal width 20 -> two wrap segments of 20+20 cells.
	rows := []string{
		long[:20],
		long[20:],
		"short",
	}
	buf := makeBuffer(rows, 20)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)

	// Cursor on the continuation row (Y=1) maps back to line 1.
	got, err := inf.Infer(context.Background(), uri, buf.RawCells(), term.Coordinates{X: 0, Y: 1})
	require.NoError(t, err)
	assert.Equal(t, 0, got.CursorAtScroll.Y)

	// Cursor on row 2 maps to line 2 ("short").
	got2, err := inf.Infer(context.Background(), uri, buf.RawCells(), term.Coordinates{X: 0, Y: 2})
	require.NoError(t, err)
	assert.Equal(t, 1, got2.CursorAtScroll.Y)
}

// TestInferExposesBandsAndRows asserts the public Bands/Rows/Tabstop
// fields agree with the rendered fixture so exo consumers can project
// file coordinates back to screen coordinates without reaching into
// vteprobe internals.
func TestInferExposesBandsAndRows(t *testing.T) {
	t.Parallel()

	const path = "/sample.go"
	content := []byte("aaa\nbbb\nccc\n")

	rows := []string{
		" 1 aaa",
		" 2 bbb",
		" 3 ccc",
	}
	buf := makeBuffer(rows, 20)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	got, err := inf.Infer(context.Background(), uri, buf.RawCells(),
		term.Coordinates{X: 3, Y: 1})
	require.NoError(t, err)

	assert.Equal(t, 0, got.Bands.Top, "content band starts at row 0")
	assert.Equal(t, 2, got.Bands.Bottom, "content band ends at last file row")
	assert.Equal(t, 3, got.Bands.GutterWidth, "leading ` N ` gutter width")
	require.Len(t, got.Rows, 3)
	assert.Equal(t, 1, got.Rows[0].FileLine)
	assert.Equal(t, 2, got.Rows[1].FileLine)
	assert.Equal(t, 3, got.Rows[2].FileLine)
	for _, r := range got.Rows {
		assert.Equal(t, 0, r.WrapOffset)
		assert.False(t, r.Folded)
	}
}

// TestInferExposesWrappedRows asserts Rows reflects the wrap layout
// detected for soft-wrapped long lines.
func TestInferExposesWrappedRows(t *testing.T) {
	t.Parallel()

	const path = "/long.txt"
	long := "aaaaaaaaaa" + "bbbbbbbbbb" + "cccccccccc"
	content := []byte(long + "\nshort\n")

	rows := []string{
		long[:20],
		long[20:],
		"short",
	}
	buf := makeBuffer(rows, 20)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	got, err := inf.Infer(context.Background(), uri, buf.RawCells(),
		term.Coordinates{X: 0, Y: 0})
	require.NoError(t, err)

	require.Len(t, got.Rows, 3)
	assert.Equal(t, 1, got.Rows[0].FileLine)
	assert.Equal(t, 0, got.Rows[0].WrapOffset)
	assert.Equal(t, 1, got.Rows[1].FileLine, "continuation row keeps file line")
	assert.Positive(t, got.Rows[1].WrapOffset,
		"continuation row must report a positive wrap offset")
	assert.Equal(t, 2, got.Rows[2].FileLine)
	assert.Equal(t, 0, got.Rows[2].WrapOffset)
}

// TestInferTabstopDefaultsToVim8 documents the exo-with-vim scenario:
// a Go file rendered by vim with the default tabstop=8, no gutter,
// and no chrome. The probe must pick tabstop 8 even though smaller
// tabstops also appear in the hint list — at ts=4 the rendered tab
// indentation no longer matches expandTabs(file).
func TestInferTabstopDefaultsToVim8(t *testing.T) {
	t.Parallel()

	const path = "/document_tracker.go"
	content := []byte(
		"type documentTracker struct {\n" +
			"\tdb              document.Service\n" +
			"\tlastIssueNumber map[string]int\n" +
			"}\n")

	rows := []string{
		"type documentTracker struct {",
		"        db              document.Service",
		"        lastIssueNumber map[string]int",
		"}",
	}
	buf := makeBuffer(rows, 80)
	fs := newFakeFS(map[string][]byte{path: content})
	uri, err := workspaceapi.ParseURI("file://" + path)
	require.NoError(t, err)

	inf := newCursor(t, fs)
	got, err := inf.Infer(context.Background(), uri, buf.RawCells(),
		term.Coordinates{X: 8, Y: 1})
	require.NoError(t, err)
	assert.Equal(t, 8, got.Tabstop, "vim default tabstop must be inferred")
	// The 'd' of db sits at visual column 8 (one tab) and is the
	// first character on file line 2 (Y=1).
	assert.Equal(t, term.Coordinates{X: 1, Y: 1}, got.CursorAtScroll)
}
