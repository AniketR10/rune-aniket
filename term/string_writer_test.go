package term

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteFlush(t *testing.T) {
	width, height := 5, 6
	writer := NewStringWriter(width, height)

	c := 'A'
	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			if i > j-1 {
				writer.SetCell(Coordinates{X: j, Y: i}, Cell{Ch: c})
			}
		}
		c++
	}

	// should be fine to wtry to write
	writer.SetCell(Coordinates{X: width + 1, Y: height + 1}, Cell{Ch: '='})

	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}

	expected := "A    \nBB   \nCCC  \nDDDD \nEEEEE\n     "
	assert.Equal(t, expected, writer.String())
}

// TODO
// func TestRuneLength(t *testing.T) {
// 	width, height := 5, 6
// 	writer := newStringWriter(width, height)
//
// 	c := '中'
// 	for i := 0; i < width; i++ {
// 		for j := 0; j < height; j++ {
// 			writer.Write(j, i, c, 0, 0)
// 		}
// 		c++
// 	}
//
// 	if err := writer.Flush(); err != nil {
// 		t.Fatal(err)
// 	}
//
// 	expected := "中中 \n丮丮 \n丯丯 \n丰丰 \n丱丱 \n     "
// 	if writer.String() != expected {
// 		t.Errorf("expected: %q; found: %q", expected, writer.String())
// 	}
// }
