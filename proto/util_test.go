package proto

import (
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/component"
	"github.com/stretchr/testify/assert"
)

func TestBufferEditRequest(t *testing.T) {
	tsuite := []struct {
		in  string
		out EditRequest
	}{
		{
			in: "a",
			out: EditRequest{
				Buffer: []*CellRow{
					&CellRow{Cells: []*Cell{&Cell{Character: 'a'}}},
				},
			},
		},
		{
			in: "a\nbb\nccc",
			out: EditRequest{
				Buffer: []*CellRow{
					&CellRow{Cells: []*Cell{&Cell{Character: 'a'}}},
					&CellRow{Cells: []*Cell{&Cell{Character: 'b'}, &Cell{Character: 'b'}}},
					&CellRow{Cells: []*Cell{&Cell{Character: 'c'}, &Cell{Character: 'c'}, &Cell{Character: 'c'}}},
				},
			},
		},
	}

	for _, tcase := range tsuite {
		buf := cell.NewBuffer()
		buf.WriteString(tcase.in)
		out := BufferToEditRequest(buf)
		assert.Equal(t, tcase.out, out)

		outbuf := EditRequestToBuffer(&out)
		assert.Equal(t, tcase.in, outbuf.String())
	}
}

func TestNewDrawResponse(t *testing.T) {
	tcase := []struct {
		in  string
		out *DrawResponse
	}{
		{
			in: "a",
			out: &DrawResponse{
				Rows: []*CellRow{
					&CellRow{Cells: []*Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					&CellRow{Cells: []*Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					&CellRow{Cells: []*Cell{
						&zeroCell,
						&zeroCell,
						&Cell{Character: 'a'},
						&zeroCell,
						&zeroCell,
					}},
					&CellRow{Cells: []*Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					&CellRow{Cells: []*Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
				},
				Cursor: &DrawResponse_Cursor{Position: &Coordinates{}},
			},
		},
		{
			in: "aaaaaa\naaaaaa\naaaaaa",
			out: &DrawResponse{
				Rows: []*CellRow{
					&CellRow{Cells: []*Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					&CellRow{Cells: []*Cell{
						&Cell{Character: 'a'}, &Cell{Character: 'a'}, &Cell{Character: 'a'},
						&Cell{Character: 'a'}, &Cell{Character: 'a'},
					}},
					&CellRow{Cells: []*Cell{
						&Cell{Character: 'a'}, &Cell{Character: 'a'}, &Cell{Character: 'a'},
						&Cell{Character: 'a'}, &Cell{Character: 'a'},
					}},
					&CellRow{Cells: []*Cell{
						&Cell{Character: 'a'}, &Cell{Character: 'a'}, &Cell{Character: 'a'},
						&Cell{Character: 'a'}, &Cell{Character: 'a'},
					}},
					&CellRow{Cells: []*Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
				},
				Cursor: &DrawResponse_Cursor{Position: &Coordinates{}},
			},
		},
	}

	for _, tcase := range tcase {
		comp := component.StringCentered(tcase.in)
		comp.Resize(5, 5)
		res := NewDrawResponse(comp, 5, 5)
		assert.Equal(t, tcase.out, res)
	}
}

func benchmarkDrawResponse(b *testing.B, width, height int) {
	var str string
	for i := 0; i < width; i++ {
		str += "fjkelwjflk\njflw\njfklewfkjlkew\n"
	}
	comp := component.StringCentered(str)
	comp.Resize(width, height)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewDrawResponse(comp, width, height)
	}
}

func BenchmarkDrawResponseTiny(b *testing.B) {
	benchmarkDrawResponse(b, 5, 5)
}
func BenchmarkDrawResponseSmall(b *testing.B) {
	benchmarkDrawResponse(b, 50, 50)
}
func BenchmarkDrawResponseMedium(b *testing.B) {
	benchmarkDrawResponse(b, 500, 500)
}
func BenchmarkDrawResponseBig(b *testing.B) {
	benchmarkDrawResponse(b, 5000, 5000)
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
		_ = BufferToEditRequest(buf)
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
