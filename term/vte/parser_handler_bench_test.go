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
	"sync"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/go-tui/workspace/workspacetest"
)

// BenchmarkParserHandlerStream simulates a shell streaming printable
// characters into the primary VTE buffer the same way the parser
// driver does at runtime: a tight loop of Input(rune) calls with
// CarriageReturn/Linefeed at end-of-line boundaries. This is what
// `cat large_file`, build logs, or any chatty command produces.
//
// Sub-benchmarks vary terminal width and line width so we can tell
// apart three regimes:
//   - narrow lines on a wide terminal (typical log output: lots of
//     blank tail columns per row)
//   - lines that fit the terminal exactly (no wrap, dense rows)
//   - lines wider than the terminal (line wrapping triggers
//     scrollUp -> PrimaryBuffer.InsertLines which materializes a
//     width-wide blank row)
//
// totalChars is held constant across sub-benchmarks so b.N iterations
// produce comparable B/op numbers.
func BenchmarkParserHandlerStream(b *testing.B) {
	const (
		// Total printable characters written per iteration. Sized so
		// that with the smallest line length (40) we overflow the
		// default 10_000-row scrollback at least once, exercising the
		// trim path in PrimaryBuffer.InsertLines.
		totalChars = 600_000
		height     = 24
	)

	cases := []struct {
		name    string
		width   int
		lineLen int
	}{
		// Logs / shell output: lines much shorter than the terminal,
		// so each row carries lots of trailing capacity that is never
		// written. This is the regime that today's defColumnCap=64
		// (clamped up by the primary-buffer width on Resize) over-
		// allocates the most.
		{name: "logs_w80_l40", width: 80, lineLen: 40},
		{name: "logs_w200_l60", width: 200, lineLen: 60},

		// Output that exactly fills the terminal width: rows are
		// fully written before Linefeed, so columnCap savings come
		// only from the empty trailing row, not the body.
		{name: "fit_w80", width: 80, lineLen: 80},
		{name: "fit_w200", width: 200, lineLen: 200},

		// Wide payloads that wrap. Each wrap inserts a width-wide
		// blank row via PrimaryBuffer.InsertLines, which is the
		// dominant allocator in the production heap snapshot.
		{name: "wrap_w80_l160", width: 80, lineLen: 160},
		{name: "wrap_w200_l400", width: 200, lineLen: 400},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			payload := buildParserStreamPayload(totalChars, tc.lineLen)

			b.ReportAllocs()
			b.ResetTimer()

			for range b.N {
				b.StopTimer()
				ph := newBenchParserHandler(tc.width, height)
				b.StartTimer()

				writeStreamToParserHandler(ph, payload)
			}
		})
	}
}

// buildParserStreamPayload returns a deterministic byte payload that,
// when written one rune at a time, totals `totalChars` printable
// characters interspersed with '\n' every `lineLen` characters.
// '\n' bytes are not counted toward totalChars (they translate to
// CarriageReturn+Linefeed, not Input).
func buildParserStreamPayload(totalChars, lineLen int) string {
	if lineLen <= 0 {
		lineLen = 1
	}
	buf := make([]byte, 0, totalChars+totalChars/lineLen+1)
	// Use a small repeating alphabet so each rune is single-byte
	// width=1, which matches the dominant case in real shell output.
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789 -_/."
	col := 0
	for written := range totalChars {
		buf = append(buf, alphabet[written%len(alphabet)])
		col++
		if col == lineLen {
			buf = append(buf, '\n')
			col = 0
		}
	}
	return string(buf)
}

func writeStreamToParserHandler(ph *parserHandler, payload string) {
	for _, c := range payload {
		if c == '\n' {
			ph.CarriageReturn()
			ph.Linefeed()
			continue
		}
		ph.Input(c)
	}
}

func newBenchParserHandler(width, height int) *parserHandler {
	uri, err := workspaceapi.ParseURI("memory:///bench")
	if err != nil {
		panic(err)
	}
	mockPty := &workspacetest.File{}
	tm := &mockTabManager{}
	pty := workspaceapi.Pty{Master: mockPty, Slave: mockPty}
	cfg := DefaultConfig()
	ph := newParserHandler(
		new(sync.Mutex), pty, tm,
		clipboard.NewInMemory(), tm.bell, uri,
		cfg.NeedsAttentionAttributes, false, cfg.MaxLines, cfg.MinWidth)
	ph.sync.primBuf.SetDefaultChar(' ')
	ph.sync.altBuf.SetDefaultChar(' ')
	ph.Resize(width, height)
	return ph
}
