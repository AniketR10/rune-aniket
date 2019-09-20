package fractal

import (
	"strings"
	"github.com/nsf/termbox-go"
	"testing"
)

type handlerTestCase struct {
	event    termbox.Event
	expected string
}

func testHandlerWorkflow(t *testing.T, handler Handler, cases []handlerTestCase, w *stringWriter) {
	var err error

	for _, tcase := range cases {
		if err = w.Clear(0, 0); err != nil {
			t.Fatal(err)
		}

		handler.Handle(tcase.event)

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

type testCase struct {
	action   func()
	expected string
}

func testWorkflow(t *testing.T, m Component, w *stringWriter, cases []testCase) {
	var err error

	for _, tcase := range cases {
		if err = w.Clear(0, 0); err != nil {
			t.Fatal(err)
		}

		if tcase.action != nil {
			tcase.action()
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
