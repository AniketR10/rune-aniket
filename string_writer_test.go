package fractal

import "testing"

func TestWriteFlush(t *testing.T) {
	width, height := 5, 6
	writer := newStringWriter(width, height)

	c := 'A'
	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			if i > j-1 {
				writer.Write(j, i, c, 0, 0)
			}
		}
		c++
	}

	// should be fine to wtry to write
	if err := writer.Write(width+1, height+1, '=', 0, 0); err != nil {
		t.Fatal(err)
	}

	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}

	expected := "A    \nBB   \nCCC  \nDDDD \nEEEEE\n     "
	if writer.String() != expected {
		t.Errorf("expected: %q; found: %q", expected, writer.String())
	}
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
