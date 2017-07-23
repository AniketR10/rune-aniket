package writer

import "testing"

func TestWriteFlush(t *testing.T) {
	width, height := 5, 6
	writer := New(width, height)

	c := 'A'
	for i := 0; i < width; i++ {
		for j := 0; j < height; j++ {
			writer.Write(j, i, c)
		}
		c++
	}

	// should be fine to wtry to write
	if err := writer.Write(width+1, height+1, '='); err != nil {
		t.Fatal(err)
	}

	if err := writer.Flush(); err != nil {
		t.Fatal(err)
	}

	expected := "AAAAA\nBBBBB\nCCCCC\nDDDDD\nEEEEE\n     "
	if writer.String() != expected {
		t.Errorf("expected: %q; found: %q", expected, writer.String())
	}
}
