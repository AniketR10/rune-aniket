package handler

import (
	"strings"
	"termbox"
	"testing"

	"github.com/ernestrc/fractal"
)

type testCase struct {
	event    termbox.Event
	expected string
}

func testWorkflow(t *testing.T, handler fractal.Handler, cases []testCase, w *fractal.StringWriter) {
	var err error

	for _, tcase := range cases {
		if err = w.Clear(0, 0); err != nil {
			t.Fatal(err)
		}

		if err = handler.Handle(tcase.event); err != nil {
			t.Fatal(err)
		}

		if err = handler.Draw(w); err != nil {
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
