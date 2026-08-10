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

package vte

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/unstablebuild/pty"
	"golang.org/x/sys/unix"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/term/vte/vteparser"
)

// BenchmarkPtyWriterBlocked reproduces what vtebench actually measures:
// how long a program writing to the pty blocks, not how long the
// terminal takes to finish parsing. The gather ring holds only
// gatherBatchCount*gatherBatchSize bytes, so a payload larger than that
// makes the writer wait on the parse stage and the number tracks Rune's
// drain rate end to end, including gather and per-batch wakeups.
func BenchmarkPtyWriterBlocked(b *testing.B) {
	const height = 24

	cases := []struct {
		name  string
		width int
		setup func(width, height int) []byte
		make  func(width, height int) []byte
	}{
		{"scroll_short", 80, nil, func(int, int) []byte {
			return buildLinefeedPayload(scrollWorkloadLines)
		}},
		{"scroll_short", 240, nil, func(int, int) []byte {
			return buildLinefeedPayload(scrollWorkloadLines)
		}},
		{"scroll_alt_region", 240, func(_, height int) []byte {
			return fmt.Appendf(nil, "\x1b[?1049h\x1b[1;%dr", height-1)
		}, func(int, int) []byte {
			return buildLinefeedPayload(scrollWorkloadLines)
		}},
		{"dense_cells", 80, nil, buildDenseCellsPayload},
		{"sync_cells", 80, nil, func(w, h int) []byte {
			return buildSyncFramesPayload(w, h, true)
		}},
		{"unicode", 80, nil, buildWideUnicodePayload},
	}

	for _, tc := range cases {
		b.Run(fmt.Sprintf("%s/w%d", tc.name, tc.width), func(b *testing.B) {
			payload := tc.make(tc.width, height)
			var setup []byte
			if tc.setup != nil {
				setup = tc.setup(tc.width, height)
			}

			master, tty, err := pty.Open()
			if err != nil {
				b.Skipf("pty unavailable: %v", err)
			}
			defer master.Close()
			defer tty.Close()
			_ = unix.IoctlSetWinsize(int(master.Fd()), unix.TIOCSWINSZ,
				&unix.Winsize{Row: uint16(height), Col: uint16(tc.width)})

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			g, ok := newPtyGather(ctx, master)
			if !ok {
				b.Skip("gather stage unavailable")
			}

			ph := newBenchParserHandler(tc.width, height)
			parser := vteparser.NewParser(ph, new(vteparser.StdTimeout))
			done := make(chan struct{})
			go debug.CapturePanicReport(func() {
				defer close(done)
				for batch := range g.ready {
					parser.AdvanceBytes(batch)
					g.release(batch)
				}
			})

			if len(setup) > 0 {
				_, _ = tty.Write(setup)
			}
			// Warm up so the scrollback and row pool reach steady state.
			_, _ = tty.Write(payload)
			time.Sleep(50 * time.Millisecond)

			b.SetBytes(int64(len(payload)))
			b.ResetTimer()
			for range b.N {
				_, _ = tty.Write(payload)
			}
			b.StopTimer()

			cancel()
			<-done
		})
	}
}
