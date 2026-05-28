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

package streamload

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// fsReader is a minimal walkdir.Reader rooted at a real directory on
// disk, sufficient for unit-testing the streamload Handler.
type fsReader struct {
	root  string
	opens atomic.Int32
}

func newFSReader(root string) *fsReader { return &fsReader{root: root} }

func (r *fsReader) URI(p string) (workspaceapi.URI, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	return workspaceapi.ParseURI("file://" + p)
}

func (r *fsReader) OpenFile(p string, flag int, perm os.FileMode) (workspaceapi.File, error) {
	r.opens.Add(1)
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	return os.OpenFile(p, flag, perm)
}

func (r *fsReader) Stat(p string) (os.FileInfo, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	return os.Stat(p)
}

func (r *fsReader) ReadDir(p string) ([]os.DirEntry, error) {
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.root, p)
	}
	return os.ReadDir(p)
}

// writeLines writes nLines lines of the form "lineN\n" into path and
// returns the URI.
func writeLines(tb testing.TB, dir string, name string, nLines int) workspaceapi.URI {
	tb.Helper()
	full := filepath.Join(dir, name)
	var b strings.Builder
	for i := range nLines {
		fmt.Fprintf(&b, "line%d\n", i)
	}
	require.NoError(tb, os.WriteFile(full, []byte(b.String()), 0o644))
	u, err := workspaceapi.ParseURI("file://" + full)
	require.NoError(tb, err)
	return u
}

func TestHandlerCloseIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	uri := writeLines(t, dir, "a.txt", 3)
	r := newFSReader(dir)

	h, err := New(r, uri, Config{})
	require.NoError(t, err)

	require.NoError(t, h.Close())
	require.NoError(t, h.Close(), "Close must be idempotent")
	assert.True(t, h.atEOF(), "Close should leave handler at EOF")
}

func TestHandlerOpenMissingFileReturnsError(t *testing.T) {
	dir := t.TempDir()
	r := newFSReader(dir)
	uri, err := workspaceapi.ParseURI("file://" + filepath.Join(dir, "missing.txt"))
	require.NoError(t, err)

	_, err = New(r, uri, Config{})
	require.Error(t, err)
}

func TestHandlerNilReader(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///tmp/nope")
	require.NoError(t, err)
	assert.PanicsWithValue(t,
		"streamload: nil walkdir.Reader",
		func() { _, _ = New(nil, uri, Config{}) },
		"passing a nil reader is a programmer error and must panic")
}

// TestHandlerLifecycle drives the streaming handler through its
// public surface — initial render, lazy paging on scroll, EOF — and
// asserts the full rendered framebuffer at each step. The sequences
// share a single 10-line file so every golden frame is unambiguous.
func TestHandlerLifecycle(t *testing.T) {
	const (
		width  = 10
		height = 4
	)
	// The streaming handler is a read-only viewer, so Cursor()
	// returns false in normal mode — no ▐ overlay appears in any
	// of these frames. Only entering / search mode would surface
	// a cursor, in the command bar.

	tests := []struct {
		name  string
		lines int
		cfg   Config
		cases []handlertest.SequenceTestCase
	}{
		{
			name:  "initial frame shows top of file",
			lines: 10,
			// PageRows=2, InitialPages=2 ⇒ 4 rows read up front.
			// Viewport is 4 rows tall; the bottom row is reserved
			// for the (empty) less command bar, leaving 3 content
			// rows so the top of the file is shown.
			cfg: Config{InitialPages: 2, Overscan: 1, PageRows: 2},
			cases: []handlertest.SequenceTestCase{{
				InputSequence: "",
				Expected: "" +
					"line0     \n" +
					"line1     \n" +
					"line2     \n" +
					"          ",
			}},
		},
		{
			name:  "jjj scrolls down and triggers lazy page reads",
			lines: 10,
			cfg:   Config{InitialPages: 2, Overscan: 1, PageRows: 2},
			cases: []handlertest.SequenceTestCase{{
				// Each j is one SeekDown. By the third j the
				// viewport has moved through enough rows to trigger
				// overscan page reads. The content area is 3 rows
				// tall (bottom row is the command bar) so rows 3,4,5
				// are visible.
				InputSequence: "jjj",
				Expected: "" +
					"line3     \n" +
					"line4     \n" +
					"line5     \n" +
					"          ",
			}},
		},
		{
			name:  "k scrolls back up after scrolling down",
			lines: 10,
			cfg:   Config{InitialPages: 2, Overscan: 1, PageRows: 2},
			cases: []handlertest.SequenceTestCase{{
				InputSequence: "jjjk",
				Expected: "" +
					"line2     \n" +
					"line3     \n" +
					"line4     \n" +
					"          ",
			}},
		},
		{
			name:  "G seeks to end of loaded buffer and reads ahead one page",
			lines: 10,
			cfg:   Config{InitialPages: 2, Overscan: 1, PageRows: 2},
			cases: []handlertest.SequenceTestCase{{
				// G is a single SeekEndFile against the currently
				// loaded buffer (4 rows) followed by one overscan
				// page read. The viewport (3 content rows) shows
				// rows 2,3,4 (the buffer's trailing-newline empty
				// row pushes the max scroll offset to 2). Repeated
				// Gs keep walking forward.
				InputSequence: "G",
				Expected: "" +
					"line2     \n" +
					"line3     \n" +
					"line4     \n" +
					"          ",
			}},
		},
		{
			name:  "repeated G walks lazily all the way to true EOF",
			lines: 10,
			cfg:   Config{InitialPages: 2, Overscan: 1, PageRows: 2},
			cases: []handlertest.SequenceTestCase{{
				// Enough Gs to consume the whole file: each one
				// only seeks within the loaded buffer and pulls
				// one more page. By the time we run out of pages
				// the viewport sits at the bottom of the file —
				// the trailing-newline empty row leaves only two
				// content lines visible above the command bar.
				InputSequence: "GGGGGGGGGG",
				Expected: "" +
					"line8     \n" +
					"line9     \n" +
					"          \n" +
					"          ",
			}},
		},
		{
			name:  "g returns to start after walking forward",
			lines: 10,
			cfg:   Config{InitialPages: 2, Overscan: 1, PageRows: 2},
			cases: []handlertest.SequenceTestCase{{
				InputSequence: "GGGGGGGGGGg",
				Expected: "" +
					"line0     \n" +
					"line1     \n" +
					"line2     \n" +
					"          ",
			}},
		},
		{
			name:  "file shorter than initial pages renders padded",
			lines: 2,
			cfg:   Config{InitialPages: 2, Overscan: 1, PageRows: 2},
			cases: []handlertest.SequenceTestCase{{
				InputSequence: "",
				Expected: "" +
					"line0     \n" +
					"line1     \n" +
					"          \n" +
					"          ",
			}},
		},
		{
			name:  "entering search mode shows cursor in the command bar",
			lines: 10,
			cfg:   Config{InitialPages: 2, Overscan: 1, PageRows: 2},
			cases: []handlertest.SequenceTestCase{{
				// `/` enters search mode; the search input bar is
				// superimposed at the bottom row with a `/`
				// prompt and the cursor right after it. This is
				// the only flow in which the streaming handler
				// surfaces a cursor.
				InputSequence: "/",
				Expected: "" +
					"line0     \n" +
					"line1     \n" +
					"line2     \n" +
					"/▐        ",
			}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			uri := writeLines(t, dir, "f.txt", tc.lines)
			r := newFSReader(dir)
			h, err := New(r, uri, tc.cfg)
			require.NoError(t, err)
			t.Cleanup(func() { _ = h.Close() })

			handlertest.RunHandlerSequence(t, h, width, height, tc.cases)
		})
	}
}

// TestHandlerLifecyclePagingAcrossLazyReads is a single test case
// that exercises a sequence of paging events on the same handler
// instance. It documents that the handler keeps growing its buffer
// as the user scrolls, reads stop at EOF, and a subsequent k after
// hitting EOF reveals the previously-loaded last rows.
func TestHandlerLifecyclePagingAcrossLazyReads(t *testing.T) {
	const (
		width  = 10
		height = 4
	)
	dir := t.TempDir()
	uri := writeLines(t, dir, "f.txt", 10)
	r := newFSReader(dir)
	h, err := New(r, uri, Config{InitialPages: 2, Overscan: 1, PageRows: 2})
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })

	// Before any input, OpenFile has been called exactly once by New.
	require.Equal(t, int32(1), r.opens.Load())

	handlertest.RunHandlerSequence(t, h, width, height, []handlertest.SequenceTestCase{
		{
			InputSequence: "j",
			Expected: "" +
				"line1     \n" +
				"line2     \n" +
				"line3     \n" +
				"          ",
		},
		{
			InputSequence: "jjj",
			Expected: "" +
				"line4     \n" +
				"line5     \n" +
				"line6     \n" +
				"          ",
		},
		{
			// Walk forward to true EOF; each G only steps within
			// the loaded buffer and reads one more page, so we
			// need several to consume the remaining rows.
			InputSequence: "GGGGG",
			Expected: "" +
				"line8     \n" +
				"line9     \n" +
				"          \n" +
				"          ",
		},
	})

	assert.True(t, h.atEOF(), "handler should be at EOF after paging to end")
}

// TestHandlerSingleGoroutineContract documents the invariant that
// Handle and Close must run on the same goroutine. The test itself
// does just that, so it passes (no race) and exists to catch a
// future refactor that, e.g., starts driving Handle from one
// goroutine and Close from another. Such a change would race on
// pageReader's eof/file/scan fields and would be caught by the
// race detector here only when the change is paired with an
// honest concurrent-access reproduction; the comment is the
// primary record. See pageReader's doc comment for the rationale.
func TestHandlerSingleGoroutineContract(t *testing.T) {
	dir := t.TempDir()
	uri := writeLines(t, dir, "a.txt", 8)
	r := newFSReader(dir)

	h, err := New(r, uri, Config{})
	require.NoError(t, err)

	for range 100 {
		_, _ = h.Handle(term.Event{})
	}
	require.NoError(t, h.Close())
}

