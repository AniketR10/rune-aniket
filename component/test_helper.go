package component

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/writer"
)

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

type testCase struct {
	action   func()
	expected string
}

func testWorkflow(t *testing.T, m fractal.Component, w *writer.StringWriter, cases []testCase) {
	var err error

	for _, tcase := range cases {
		if err = w.Clear(0, 0); err != nil {
			t.Fatal(err)
		}

		if tcase.action != nil {
			tcase.action()
		}

		if err != nil {
			t.Fatal(err)
		}

		if err := m.Draw(w); err != nil {
			t.Fatal(err)
		}

		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		// for readability, we expected strings are written starting with \n
		expected := strings.TrimLeft(tcase.expected, "\n")
		if expected != w.String() {
			t.Errorf("expected %q found %q", expected, w.String())
		}
	}
}
