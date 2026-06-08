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

package exoeditor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/term/vte/vteprobe"
)

// bufferOf builds an editable *cell.Buffer (with an undoer so Version
// advances on edits) seeded with content.
func bufferOf(tb testing.TB, content string) *cell.Buffer {
	tb.Helper()
	buf := cell.NewBuffer()
	_, err := buf.ReadFrom(strings.NewReader(content))
	require.NoError(tb, err)
	return buf
}

// fakeComponent is a componentSnapshotter that returns a test-controlled
// rendered grid and cursor, so the real refreshProbe path (Snapshot
// decode, Active, Infer) runs without a pty-backed vte handler. err lets
// a test exercise the snapshot-failure branch.
type fakeComponent struct {
	cells  [][]term.Cell
	cursor term.Coordinates
	err    error
}

func (c *fakeComponent) Snapshot() (vte.Snapshot, error) {
	if c.err != nil {
		return vte.Snapshot{}, c.err
	}
	return vte.Snapshot{Primary: vte.ScreenSnapshot{Cells: c.cells, Cursor: c.cursor}}, nil
}

// SnapshotInto mirrors Component.SnapshotInto: it copies the fixed test
// grid into dst, reusing its capacity, so the reuse path refreshProbe
// takes in production is exercised here too.
func (c *fakeComponent) SnapshotInto(dst [][]term.Cell) (vte.Snapshot, error) {
	if c.err != nil {
		return vte.Snapshot{}, c.err
	}
	cells := term.CopyCells(dst, c.cells)
	return vte.Snapshot{Primary: vte.ScreenSnapshot{Cells: cells, Cursor: c.cursor}}, nil
}

// handlerForBufferTest builds an editorHandler around buf the way
// newHandler does for the parts relevant to line snapshots and probe
// refresh: it seeds the snapshot, subscribes to buffer edits, installs a
// real probe, and points the component at a fake snapshotter the caller
// drives. The full newHandler is not used because it spawns a PTY-backed
// vte handler a unit test cannot drive.
func handlerForBufferTest(
	tb testing.TB, buf *cell.Buffer,
) (*editorHandler, *fakeComponent) {
	tb.Helper()
	comp := &fakeComponent{}
	h := &editorHandler{
		buf:       buf,
		probe:     vteprobe.New([]int{8, 4, 2}, 0.6, 8<<20),
		component: comp,
	}
	h.snapshotBufferLines()
	h.bufSub = &bufLineWatcher{h: h}
	buf.Subscribe(h.bufSub)
	tb.Cleanup(func() { buf.Unsubscribe(h.bufSub) })
	return h, comp
}

// gutterGrid renders rows of the form " N text" padded to width, the
// shape an editor with a line-number gutter produces. It mirrors the
// fixtures vteprobe's own tests use so the probe aligns confidently.
func gutterGrid(lines []string, width int) [][]term.Cell {
	cells := make([][]term.Cell, len(lines))
	for i, s := range lines {
		row := []rune(s)
		out := make([]term.Cell, 0, width)
		for _, r := range row {
			out = append(out, term.Cell{Ch: r, Width: 1})
		}
		for len(out) < width {
			out = append(out, term.Cell{Ch: ' ', Width: 1})
		}
		cells[i] = out
	}
	return cells
}

// TestBufferLinesFollowsBufferNotDisk proves the lines the probe sees
// come from the in-memory cell.Buffer mirror, never from disk. Mutating
// the on-disk file without touching h.buf must not change the lines;
// editing h.buf must.
func TestBufferLinesFollowsBufferNotDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "code.go")
	// Disk starts out diverged from the buffer mirror on purpose.
	require.NoError(t, os.WriteFile(path, []byte("DISK\nDISK\nDISK\n"), 0o600))

	buf := bufferOf(t, "foo\nbar\nbaz\n")
	h, _ := handlerForBufferTest(t, buf)

	// Lines reflect the buffer, not the diverged disk content.
	assert.Equal(t, []string{"foo", "bar", "baz"}, h.bufferLines())

	// Mutate disk again without touching the buffer: lines are stable.
	require.NoError(t, os.WriteFile(path, []byte("OTHER\n"), 0o600))
	assert.Equal(t, []string{"foo", "bar", "baz"}, h.bufferLines())

	// Edit the buffer: the version advances and lines now follow it.
	beforeV := buf.Version()
	buf.Edit(context.Background(),
		term.Coordinates{X: 0, Y: 1}, term.Coordinates{X: 3, Y: 1}, "BAR")
	require.NotEqual(t, beforeV, buf.Version(),
		"editing the buffer must advance its Version")
	assert.Equal(t, []string{"foo", "BAR", "baz"}, h.bufferLines())
}

// TestBufferLinesSnapshotStableBetweenEdits verifies repeated reads
// return the same snapshot while the buffer is unchanged, and a fresh
// snapshot once it is edited.
func TestBufferLinesSnapshotStableBetweenEdits(t *testing.T) {
	t.Parallel()

	buf := bufferOf(t, "foo\nbar\n")
	h, _ := handlerForBufferTest(t, buf)

	first := h.bufferLines()
	again := h.bufferLines()
	// No edit -> same backing slice (no re-split, no re-store).
	assert.Equal(t, &first[0], &again[0],
		"unchanged buffer must reuse the cached snapshot")

	buf.Edit(context.Background(),
		term.Coordinates{X: 0, Y: 0}, term.Coordinates{X: 3, Y: 0}, "FOO")
	updated := h.bufferLines()
	assert.Equal(t, []string{"FOO", "bar"}, updated)
}

// TestRefreshProbeFollowsBufferAcrossDiskMutation drives the real
// refreshProbe path: a probe against a rendered grid maps the cursor
// using the buffer mirror, a disk mutation that leaves the buffer
// untouched does not change the cached result, and once the buffer is
// reloaded the cached probe follows the buffer without any extra
// interrupt. The last leg is the regression guard for the stale-screen
// race where saving did not refresh the overlay until the next keypress.
func TestRefreshProbeFollowsBufferAcrossDiskMutation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "code.go")
	require.NoError(t, os.WriteFile(path, []byte("DISK\nDISK\nDISK\n"), 0o600))

	buf := bufferOf(t, "foo\nbar\nbaz\n")
	h, comp := handlerForBufferTest(t, buf)

	// Rendered gutter view of the buffer; cursor on the body of line 2.
	comp.cells = gutterGrid([]string{" 1 foo", " 2 bar", " 3 baz"}, 30)
	comp.cursor = term.Coordinates{X: 3, Y: 1}

	h.refreshProbe()
	got := h.lastProbe.Load()
	require.NotNil(t, got)
	assert.Equal(t, term.Coordinates{X: 0, Y: 1}, got.CursorAtScroll)

	// Mutate disk out-of-band; the buffer mirror is untouched, so a
	// refresh against the same grid yields the same result.
	require.NoError(t, os.WriteFile(path, []byte("x\ny\n"), 0o600))
	h.refreshProbe()
	got2 := h.lastProbe.Load()
	require.NotNil(t, got2)
	assert.Equal(t, term.Coordinates{X: 0, Y: 1}, got2.CursorAtScroll,
		"disk mutation must not affect inference driven by the buffer")

	// Reload the buffer (the watcher's OnDidEdit) with a matching grid
	// update. The edit alone must refresh lastProbe — no extra
	// refreshProbe call — so the overlay follows the buffer.
	comp.cells = gutterGrid([]string{" 1 foo", " 2 BAR", " 3 baz"}, 30)
	comp.cursor = term.Coordinates{X: 5, Y: 1}
	buf.Edit(context.Background(),
		term.Coordinates{X: 0, Y: 1}, term.Coordinates{X: 3, Y: 1}, "BAR")
	got3 := h.lastProbe.Load()
	require.NotNil(t, got3)
	// Body offset 5-3 = 2 -> rune index 2 = 'R' on "BAR" (0-based).
	assert.Equal(t, term.Coordinates{X: 2, Y: 1}, got3.CursorAtScroll,
		"reload must refresh the cached probe without an extra interrupt")
}

// TestRefreshProbeKeepsLastResultOnSnapshotError verifies a failed
// component snapshot leaves the previously cached probe untouched, so a
// transient redraw mid-clear does not flicker the overlay off.
func TestRefreshProbeKeepsLastResultOnSnapshotError(t *testing.T) {
	t.Parallel()

	buf := bufferOf(t, "foo\nbar\nbaz\n")
	h, comp := handlerForBufferTest(t, buf)

	comp.cells = gutterGrid([]string{" 1 foo", " 2 bar", " 3 baz"}, 30)
	comp.cursor = term.Coordinates{X: 3, Y: 1}
	h.refreshProbe()
	good := h.lastProbe.Load()
	require.NotNil(t, good)

	comp.err = errors.New("snapshot failed")
	h.refreshProbe()
	assert.Same(t, good, h.lastProbe.Load(),
		"a snapshot error must keep the last good probe")
}

// BenchmarkRefreshProbe measures the steady-state cost of one
// refreshProbe on the exo handler: a component snapshot, the buffer-line
// load, and the vteprobe alignment over a realistic gutter-rendered
// screen. The fixture is built once outside the timing loop and the
// probe slab is reused across calls, mirroring how refreshProbe runs on
// every grid mutation.
func BenchmarkRefreshProbe(b *testing.B) {
	const lines = 48

	var content strings.Builder
	rows := make([]string, 0, lines)
	for i := range lines {
		body := fmt.Sprintf("x%d := compute(%d) + offset", i, i)
		fmt.Fprintf(&content, "%s\n", body)
		rows = append(rows, fmt.Sprintf(" %d %s", i+1, body))
	}

	buf := bufferOf(b, content.String())
	h, comp := handlerForBufferTest(b, buf)
	comp.cells = gutterGrid(rows, 80)
	comp.cursor = term.Coordinates{X: 6, Y: lines / 2}

	h.refreshProbe()
	if h.lastProbe.Load() == nil {
		b.Fatal("warmup refreshProbe did not produce a result")
	}

	b.ReportAllocs()
	for b.Loop() {
		h.refreshProbe()
	}
}
