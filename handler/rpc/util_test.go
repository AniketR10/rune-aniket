package rpc

import (
	"testing"

	"github.com/ernestrc/go-tui/component"
	termpb "github.com/ernestrc/go-tui/term/rpc"
	"github.com/stretchr/testify/assert"
)

func TestNewDrawResponse(t *testing.T) {
	tcase := []struct {
		in  string
		out *DrawResponse
	}{
		{
			in: "a",
			out: &DrawResponse{
				Rows: []*termpb.CellRow{
					&termpb.CellRow{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					&termpb.CellRow{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					&termpb.CellRow{Cells: []*termpb.Cell{
						&zeroCell,
						&zeroCell,
						&termpb.Cell{Character: 'a'},
						&zeroCell,
						&zeroCell,
					}},
					&termpb.CellRow{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					&termpb.CellRow{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
				},
				Cursor: &DrawResponse_Cursor{Position: &termpb.Coordinates{}},
			},
		},
		{
			in: "aaaaaa\naaaaaa\naaaaaa",
			out: &DrawResponse{
				Rows: []*termpb.CellRow{
					&termpb.CellRow{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
					&termpb.CellRow{Cells: []*termpb.Cell{
						&termpb.Cell{Character: 'a'}, &termpb.Cell{Character: 'a'}, &termpb.Cell{Character: 'a'},
						&termpb.Cell{Character: 'a'}, &termpb.Cell{Character: 'a'},
					}},
					&termpb.CellRow{Cells: []*termpb.Cell{
						&termpb.Cell{Character: 'a'}, &termpb.Cell{Character: 'a'}, &termpb.Cell{Character: 'a'},
						&termpb.Cell{Character: 'a'}, &termpb.Cell{Character: 'a'},
					}},
					&termpb.CellRow{Cells: []*termpb.Cell{
						&termpb.Cell{Character: 'a'}, &termpb.Cell{Character: 'a'}, &termpb.Cell{Character: 'a'},
						&termpb.Cell{Character: 'a'}, &termpb.Cell{Character: 'a'},
					}},
					&termpb.CellRow{Cells: []*termpb.Cell{&zeroCell, &zeroCell, &zeroCell, &zeroCell, &zeroCell}},
				},
				Cursor: &DrawResponse_Cursor{Position: &termpb.Coordinates{}},
			},
		},
	}

	for _, tcase := range tcase {
		comp := component.StringWithConfig(tcase.in,
			component.StringConfig{Alignment: component.SpanAlignmentCentered})
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
	comp := component.StringWithConfig(str,
		component.StringConfig{Alignment: component.SpanAlignmentCentered})
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
