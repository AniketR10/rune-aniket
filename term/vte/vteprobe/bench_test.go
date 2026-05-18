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
	"os"
	"path/filepath"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
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
	if _, err := c.Infer(context.Background(), uri, buf.RawCells(), cur); err != nil {
		b.Fatalf("warmup infer: %v", err)
	}

	ctx := context.Background()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := c.Infer(ctx, uri, buf.RawCells(), cur); err != nil {
			b.Fatalf("infer: %v", err)
		}
	}
}
