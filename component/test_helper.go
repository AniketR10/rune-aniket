package component

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal"
)

type testCase struct {
	action   func()
	expected string
}

func testWorkflow(t *testing.T, m fractal.Component, w *fractal.StringWriter, cases []testCase) {
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
