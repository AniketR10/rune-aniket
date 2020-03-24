package proto

import (
	"testing"

	"github.com/ernestrc/go-tui/component"
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
			},
		},
	}

	for _, tcase := range tcase {
		comp := component.String(tcase.in)
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
	comp := component.String(str)
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
