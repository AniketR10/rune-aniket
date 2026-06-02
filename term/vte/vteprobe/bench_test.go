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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/vte"
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
		filePath  = "/sample.txt"
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

	fs := newFakeFS(map[string][]byte{filePath: sampleBytes})
	uri, err := workspaceapi.ParseURI("file://" + filePath)
	if err != nil {
		b.Fatalf("parse uri: %v", err)
	}

	c := New(fs, []int{4, 2, 8}, fx.MinConfidence, 8<<20)
	// Warm the file-content cache so the steady-state path is what we
	// actually measure; a separate Stat hit on the first call would
	// otherwise distort the first iteration.
	if _, err := c.Infer(context.Background(), uri, buf.RawCells(), cur, nil); err != nil {
		b.Fatalf("warmup infer: %v", err)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := c.Infer(ctx, uri, buf.RawCells(), cur, nil); err != nil {
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
		filePath  = "/sample.txt"
	)

	sampleBytes, err := os.ReadFile(filepath.Join(sampleDir, "sample.txt"))
	if err != nil {
		b.Fatalf("read sample: %v", err)
	}
	uri, err := workspaceapi.ParseURI("file://" + filePath)
	if err != nil {
		b.Fatalf("parse uri: %v", err)
	}
	ed := editorCase{
		name: "nvim",
		dir:  sampleDir + "/nvim",
	}
	b.Run("fresh", func(b *testing.B) {
		benchInferFixture(b, sampleBytes, uri, ed)
	})
	b.Run("slab", func(b *testing.B) {
		benchInferFixtureWithSlab(b, sampleBytes, uri, ed)
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
		filePath  = "/sample.txt"
	)

	sampleBytes, err := os.ReadFile(filepath.Join(sampleDir, "sample.txt"))
	if err != nil {
		b.Fatalf("read sample: %v", err)
	}
	uri, err := workspaceapi.ParseURI("file://" + filePath)
	if err != nil {
		b.Fatalf("parse uri: %v", err)
	}
	for _, editor := range []string{"nvim", "vim"} {
		ed := editorCase{name: editor, dir: sampleDir + "/" + editor}
		b.Run(editor+"/fresh", func(b *testing.B) {
			benchInferFixture(b, sampleBytes, uri, ed)
		})
		b.Run(editor+"/slab", func(b *testing.B) {
			benchInferFixtureWithSlab(b, sampleBytes, uri, ed)
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
		filePath = "/big.go"
		width    = 100
		height   = 50
	)
	uri, err := workspaceapi.ParseURI("file://" + filePath)
	if err != nil {
		b.Fatalf("parse uri: %v", err)
	}

	for _, nlines := range []int{500, 2000, 10000} {
		content := syntheticGoFile(nlines)
		lines := splitLines(content)

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

		fs := newFakeFS(map[string][]byte{filePath: content})
		c := New(fs, []int{4, 2, 8}, 0.6, 64<<20)
		ctx := context.Background()
		if _, err := c.Infer(ctx, uri, cells, cur, nil); err != nil {
			b.Fatalf("warmup infer: %v", err)
		}

		// Fresh allocation every call (no slab reuse).
		b.Run(fmt.Sprintf("lines=%d/fresh", nlines), func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := c.Infer(ctx, uri, cells, cur, nil); err != nil {
					b.Fatalf("infer: %v", err)
				}
			}
		})

		// Reused scratch arena across calls (the steady-state cost for a
		// caller that probes on every cursor move).
		b.Run(fmt.Sprintf("lines=%d/slab", nlines), func(b *testing.B) {
			slab := NewSlab()
			if _, err := c.Infer(ctx, uri, cells, cur, slab); err != nil {
				b.Fatalf("warmup slab infer: %v", err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if _, err := c.Infer(ctx, uri, cells, cur, slab); err != nil {
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

	const filePath = "/sample.txt"
	uri, err := workspaceapi.ParseURI("file://" + filePath)
	if err != nil {
		b.Fatalf("parse uri: %v", err)
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
				benchInferFixture(b, sampleBytes, uri, ed)
			})
		}
	}
}

func benchInferFixture(
	b *testing.B, sampleBytes []byte, uri workspaceapi.URI, ed editorCase,
) {
	benchInferFixtureWithRunner(b, sampleBytes, ed,
		func(ctx context.Context, c *Cursor, cells [][]term.Cell, cur term.Coordinates) error {
			_, err := c.Infer(ctx, uri, cells, cur, nil)
			return err
		})
}

func benchInferFixtureWithSlab(
	b *testing.B, sampleBytes []byte, uri workspaceapi.URI, ed editorCase,
) {
	slab := NewSlab()
	benchInferFixtureWithRunner(b, sampleBytes, ed,
		func(ctx context.Context, c *Cursor, cells [][]term.Cell, cur term.Coordinates) error {
			_, err := c.Infer(ctx, uri, cells, cur, slab)
			return err
		})
}

func benchInferFixtureWithRunner(
	b *testing.B,
	sampleBytes []byte,
	ed editorCase,
	run func(context.Context, *Cursor, [][]term.Cell, term.Coordinates) error,
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

	fs := newFakeFS(map[string][]byte{"/sample.txt": sampleBytes})
	c := New(fs, []int{4, 2, 8}, fx.MinConfidence, 8<<20)
	cells := buf.RawCells()
	if err := run(context.Background(), c, cells, cur); err != nil {
		b.Fatalf("warmup infer: %v", err)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := run(ctx, c, cells, cur); err != nil {
			b.Fatalf("infer: %v", err)
		}
	}
}
