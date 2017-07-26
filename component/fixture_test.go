package component

import "github.com/ernestrc/fractal"

type testComponent struct {
	fill                rune
	x, y, width, height int
}

func (t *testComponent) Resize(width, height int) (err error) {
	t.width, t.height = width, height
	return nil
}

func (t *testComponent) Move(x, y int) error {
	t.x, t.y = x, y
	return nil
}

func (t *testComponent) Draw(w fractal.Writer) (err error) {
	for tx := t.x + t.width - 1; tx >= t.x; tx-- {
		for ty := t.y + t.height - 1; ty >= t.y; ty-- {
			w.Write(tx, ty, t.fill, 0, 0)
		}
	}
	return nil
}

func (t *testComponent) Height() int {
	return t.height
}

func (t *testComponent) Width() int {
	return t.width
}

func (t *testComponent) Position() (int, int) {
	return t.x, t.y
}
