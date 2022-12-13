package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	termpb "unstable.build/go-tui/term/rpc"
)

func TestBufferEditRequest(t *testing.T) {
	tsuite := []struct {
		in  string
		out EditRequest
	}{
		{
			in: "a",
			out: EditRequest{
				ResourceName: &URI{Uri: ""},
				Buffer: []*termpb.CellRow{
					&termpb.CellRow{Cells: []*termpb.Cell{&termpb.Cell{Character: 'a'}}},
				},
			},
		},
		{
			in: "a\nbb\nccc",
			out: EditRequest{
				ResourceName: &URI{Uri: ""},
				Buffer: []*termpb.CellRow{
					&termpb.CellRow{Cells: []*termpb.Cell{&termpb.Cell{Character: 'a'}}},
					&termpb.CellRow{Cells: []*termpb.Cell{&termpb.Cell{Character: 'b'}, &termpb.Cell{Character: 'b'}}},
					&termpb.CellRow{Cells: []*termpb.Cell{&termpb.Cell{Character: 'c'}, &termpb.Cell{Character: 'c'}, &termpb.Cell{Character: 'c'}}},
				},
			},
		},
	}

	for _, tcase := range tsuite {
		buf := cell.NewBuffer()
		buf.WriteString(tcase.in)
		out := NewEditRequest(workspaceapi.URI{}, buf)
		assert.Equal(t, tcase.out, out)

		outbuf := EditRequestToBuffer(&out)
		assert.Equal(t, tcase.in, outbuf.String())
	}
}

func benchmarkEditRequest(b *testing.B, width, height int) {
	var str string
	for i := 0; i < width; i++ {
		str += "fjkelwjflk\njflw\njfklewfkjlkew\n"
	}

	buf := cell.NewBuffer()
	buf.WriteString(str)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewEditRequest(workspaceapi.URI{}, buf)
	}
}

func BenchmarkEditRequestTiny(b *testing.B) {
	benchmarkEditRequest(b, 5, 5)
}
func BenchmarkEditRequestSmall(b *testing.B) {
	benchmarkEditRequest(b, 50, 50)
}
func BenchmarkEditRequestMedium(b *testing.B) {
	benchmarkEditRequest(b, 500, 500)
}
func BenchmarkEditRequestBig(b *testing.B) {
	benchmarkEditRequest(b, 5000, 5000)
}
