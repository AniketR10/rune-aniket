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
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term/vte/vteparser"
)

// TestAdvanceBytesEquivalence pins that the batched AdvanceBytes path
// (printable runs delivered via InputRun) produces exactly the same
// terminal state as the per-byte Advance path for the same stream,
// regardless of how the stream is chunked.
func TestAdvanceBytesEquivalence(t *testing.T) {
	t.Parallel()
	streams := []struct {
		name   string
		stream string
	}{
		{"plain", "hello world"},
		{"crlf lines", "line one\r\nline two\r\nline three\r\n"},
		{"wrap", strings.Repeat("abcdefgh", 12)},
		{"sgr colors", "\x1b[31mred\x1b[42;1mgreen bg\x1b[0mplain tail after reset"},
		{"utf8 mixed", "héllo wörld … 漢字 tail ascii run 12345"},
		{"linedraw charset", "\x1b(0lqqqk\x1b(Bascii after"},
		{"insert mode", "before\x1b[4hINSERTED\x1b[4lappend"},
		{"rep after run", "abc\x1b[5b tail"},
		{"rep after escape", "abc\x1b[31m\x1b[2b tail"},
		{"osc title then text", "\x1b]2;mytitle\x1b\\after title"},
		{"cursor movement", "12345\x1b[2Gmid\x1b[Hhome"},
		{"alt buffer", "prim\x1b[?1049hALT SCREEN\x1b[?1049lback"},
		{"clear and rewrite", "first\x1b[2Jsecond"},
		{"del mixed", "abc\x7fdef"},
		{"tabs", "a\tb\tc"},
		{"conceal", "plain\x1b[8mhidden\x1b[28mvisible"},
		{"sync update", "\x1b[?2026habc\x1b[?2026ldef"},
		{"long stream", strings.Repeat("the quick brown fox jumps over the lazy dog\r\n", 40)},
	}
	chunkings := []int{1, 7, 1 << 20}

	for _, tc := range streams {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			want := parserStateDump(t, tc.stream, func(p *vteparser.Parser, b []byte) {
				for _, ch := range b {
					p.Advance(ch)
				}
			}, 1<<20)

			for _, chunk := range chunkings {
				got := parserStateDump(t, tc.stream, (*vteparser.Parser).AdvanceBytes, chunk)
				require.Equal(t, want, got, "AdvanceBytes chunk=%d diverged from Advance", chunk)
			}
		})
	}
}

func parserStateDump(
	t *testing.T, stream string,
	feed func(*vteparser.Parser, []byte), chunk int,
) string {
	t.Helper()
	ph := newInputParserHandler(t, false)
	ph.Resize(24, 8)
	parser := vteparser.NewParser(ph, new(vteparser.StdTimeout))

	data := []byte(stream)
	for len(data) > 0 {
		n := min(chunk, len(data))
		feed(parser, data[:n])
		data = data[n:]
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "prim:%+v\n", ph.sync.primBuf.Cells.RawCells())
	fmt.Fprintf(&sb, "alt:%+v\n", ph.sync.altBuf.Cells.RawCells())
	fmt.Fprintf(&sb, "cursor:%+v shouldWrap:%t useAlt:%t title:%q attrs:%+v",
		ph.sync.buf.CursorAtScroll(), ph.shouldWrap, ph.useAlt,
		ph.title, ph.sync.buf.CursorAttributes())
	return sb.String()
}
