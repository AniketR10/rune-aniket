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

package vteprobe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/term/vte"
)

// BenchmarkCursorInfer measures Cursor.Infer over a real editor capture
// of the go-complex sample (an 83-line Go file). The buffer and
// fixture are decoded once, outside the timing loop, so the benchmark
// focuses on the inference path itself (chrome detection, gutter
// detection, alignment, wrap detection, cursor mapping, confidence) plus
// the cached file read.
func BenchmarkCursorInfer(b *testing.B) {
	const (
		sampleDir = "testdata/samples/go-complex"
		editorDir = sampleDir + "/hx"
	)

	sampleBytes, err := os.ReadFile(filepath.Join(sampleDir, "sample.txt"))
	if err != nil {
		b.Fatalf("read sample: %v", err)
	}
	fx, err := loadEditorFixture(editorDir)
	if err != nil {
		b.Fatalf("load fixture: %v", err)
	}
	screen, err := os.ReadFile(filepath.Join(editorDir, "screen.ansi"))
	if err != nil {
		b.Fatalf("read screen: %v", err)
	}
	cur, err := readCursor(filepath.Join(editorDir, "cursor.txt"))
	if err != nil {
		b.Fatalf("read cursor: %v", err)
	}

	data := captureToReplayBytes(screen, fx.Width, fx.Height, cur)
	buf, _, err := vte.Replay(fx.Width, fx.Height, data)
	if err != nil {
		b.Fatalf("replay: %v", err)
	}

	lines := splitLines(sampleBytes)
	fileCells := cellLines(lines)
	c := New([]int{4, 2, 8}, fx.MinConfidence, 8<<20)
	// Warm the path once so the steady-state cost is what we measure.
	if _, err := c.Infer(buf.RawCells(), cur, fileCells, nil); err != nil {
		b.Fatalf("warmup infer: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := c.Infer(buf.RawCells(), cur, fileCells, nil); err != nil {
			b.Fatalf("infer: %v", err)
		}
	}
}

// BenchmarkCursorInferWrapBraceTop measures Cursor.Infer on a
// soft-wrapped view whose first visible row is a lone "}" (the
// go-bot-wrap-202-1 fixture: a 105x51 nvim capture scrolled near
// end-of-file). This exercises the wrap-alignment vote and the
// past-EOF anchor rejection, which the simpler captures do not.
func BenchmarkCursorInferWrapBraceTop(b *testing.B) {
	const (
		sampleDir = "testdata/samples/go-bot-wrap-202-1"
	)

	sampleBytes, err := os.ReadFile(filepath.Join(sampleDir, "sample.txt"))
	if err != nil {
		b.Fatalf("read sample: %v", err)
	}
	ed := editorCase{
		name: "nvim",
		dir:  sampleDir + "/nvim",
	}
	b.Run("fresh", func(b *testing.B) {
		benchInferFixture(b, sampleBytes, ed)
	})
	b.Run("slab", func(b *testing.B) {
		benchInferFixtureWithSlab(b, sampleBytes, ed)
	})
}

// BenchmarkCursorInferWrapMidFile measures Cursor.Infer on a soft-wrapped
// view scrolled into the middle of a large file (the go-bot-wrap-191-18
// fixture: a 105x51 nvim/vim capture). Unlike the brace-top fixture this
// lands the cursor deep in the file, so the wrap-alignment vote scans the
// most candidate top lines; the fresh/slab split shows the per-call
// allocation a caller saves by reusing scratch on every cursor move.
func BenchmarkCursorInferWrapMidFile(b *testing.B) {
	const (
		sampleDir = "testdata/samples/go-bot-wrap-191-18"
	)

	sampleBytes, err := os.ReadFile(filepath.Join(sampleDir, "sample.txt"))
	if err != nil {
		b.Fatalf("read sample: %v", err)
	}
	for _, editor := range []string{"nvim", "vim"} {
		ed := editorCase{name: editor, dir: sampleDir + "/" + editor}
		b.Run(editor+"/fresh", func(b *testing.B) {
			benchInferFixture(b, sampleBytes, ed)
		})
		b.Run(editor+"/slab", func(b *testing.B) {
			benchInferFixtureWithSlab(b, sampleBytes, ed)
		})
	}
}

// BenchmarkCursorInferLargeFile measures how Cursor.Infer scales with
// file size. The content alignment vote tries every file line as a
// candidate top-of-band, so cost grows with line count; this benchmark
// renders the tail of a synthetic Go-like file (the worst case, where
// the matching offset is only found after scanning every earlier line)
// at a few sizes so regressions in the per-candidate cost are visible.
func BenchmarkCursorInferLargeFile(b *testing.B) {
	const (
		width  = 100
		height = 50
	)

	for _, nlines := range []int{500, 2000, 10000} {
		content := syntheticGoFile(nlines)
		lines := splitLines(content)
		fileCells := cellLines(lines)

		// Render the bottom `height-1` file lines plus one status row so
		// the visible band is the tail of the file.
		start := len(lines) - (height - 1)
		rowStrs := make([]string, 0, height)
		for i := start; i < len(lines); i++ {
			rowStrs = append(rowStrs, expandTabs(lines[i], 8))
		}
		rowStrs = append(rowStrs, "big.go") // status-ish row
		buf := makeBuffer(rowStrs, width)
		cells := buf.RawCells()
		cur := term.Coordinates{X: 0, Y: 0} // top visible row

		c := New([]int{4, 2, 8}, 0.6, 64<<20)
		if _, err := c.Infer(cells, cur, fileCells, nil); err != nil {
			b.Fatalf("warmup infer: %v", err)
		}

		// Fresh allocation every call (no slab reuse).
		b.Run(fmt.Sprintf("lines=%d/fresh", nlines), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := c.Infer(cells, cur, fileCells, nil); err != nil {
					b.Fatalf("infer: %v", err)
				}
			}
		})

		// Reused scratch arena across calls (the steady-state cost for a
		// caller that probes on every cursor move).
		b.Run(fmt.Sprintf("lines=%d/slab", nlines), func(b *testing.B) {
			slab := NewSlab()
			if _, err := c.Infer(cells, cur, fileCells, slab); err != nil {
				b.Fatalf("warmup slab infer: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := c.Infer(cells, cur, fileCells, slab); err != nil {
					b.Fatalf("infer: %v", err)
				}
			}
		})
	}
}

// syntheticGoFile builds a deterministic Go-like source file with
// nlines lines: tab-indented statements interspersed with lone "}"
// lines so the content alignment has many ambiguous anchors.
func syntheticGoFile(nlines int) []byte {
	var sb strings.Builder
	for i := 0; i < nlines; i++ {
		switch i % 5 {
		case 0:
			fmt.Fprintf(&sb, "func fn%d() {\n", i)
		case 4:
			sb.WriteString("}\n")
		default:
			fmt.Fprintf(&sb, "\tx%d := compute(%d) + offset\n", i, i)
		}
	}
	return []byte(sb.String())
}

// BenchmarkCursorInferMatrix runs Cursor.Infer against every
// (sample, editor) fixture under testdata/samples so the inference
// cost can be compared across editor chromes (vim, nvim, emacs, hx,
// nano), across content shapes (go-basic, go-complex, go-cli with a
// non-zero scroll offset, go-pem / go-pem-eof whose long PEM blocks
// stress wrap detection), and across cursor positions captured in the
// fixture. Each sub-benchmark prepares its buffer and warms the
// file-content cache outside the timing loop so the steady-state
// inference path is what we measure.
func BenchmarkCursorInferMatrix(b *testing.B) {
	samples, err := discoverSamples("testdata/samples")
	if err != nil {
		b.Fatalf("discover samples: %v", err)
	}
	if len(samples) == 0 {
		b.Skip("no samples under testdata/samples")
	}

	for _, s := range samples {
		if len(s.editors) == 0 {
			continue
		}
		sampleBytes, err := os.ReadFile(s.sampleFile)
		if err != nil {
			b.Fatalf("read sample %q: %v", s.name, err)
		}
		for _, ed := range s.editors {
			b.Run(s.name+"/"+ed.name, func(b *testing.B) {
				benchInferFixture(b, sampleBytes, ed)
			})
		}
	}
}

func benchInferFixture(
	b *testing.B, sampleBytes []byte, ed editorCase,
) {
	fileCells := cellLines(splitLines(sampleBytes))
	benchInferFixtureWithRunner(b, ed,
		func(c *Cursor, cells [][]term.Cell, cur term.Coordinates) error {
			_, err := c.Infer(cells, cur, fileCells, nil)
			return err
		})
}

func benchInferFixtureWithSlab(
	b *testing.B, sampleBytes []byte, ed editorCase,
) {
	fileCells := cellLines(splitLines(sampleBytes))
	slab := NewSlab()
	benchInferFixtureWithRunner(b, ed,
		func(c *Cursor, cells [][]term.Cell, cur term.Coordinates) error {
			_, err := c.Infer(cells, cur, fileCells, slab)
			return err
		})
}

func benchInferFixtureWithRunner(
	b *testing.B,
	ed editorCase,
	run func(*Cursor, [][]term.Cell, term.Coordinates) error,
) {
	b.Helper()

	fx, err := loadEditorFixture(ed.dir)
	if err != nil {
		b.Fatalf("load fixture: %v", err)
	}
	screen, err := os.ReadFile(filepath.Join(ed.dir, "screen.ansi"))
	if err != nil {
		b.Fatalf("read screen: %v", err)
	}
	cur, err := readCursor(filepath.Join(ed.dir, "cursor.txt"))
	if err != nil {
		b.Fatalf("read cursor: %v", err)
	}

	data := captureToReplayBytes(screen, fx.Width, fx.Height, cur)
	buf, _, err := vte.Replay(fx.Width, fx.Height, data)
	if err != nil {
		b.Fatalf("replay: %v", err)
	}

	c := New([]int{4, 2, 8}, fx.MinConfidence, 8<<20)
	cells := buf.RawCells()
	if err := run(c, cells, cur); err != nil {
		b.Fatalf("warmup infer: %v", err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := run(c, cells, cur); err != nil {
			b.Fatalf("infer: %v", err)
		}
	}
}
