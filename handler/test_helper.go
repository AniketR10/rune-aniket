package handler

import (
	"strings"
	"termbox"
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/component"
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

type testHandler struct {
	component.Fill
	Active bool
}

func newTestHandler() *testHandler {
	t := new(testHandler)
	t.Ch = 'A'
	t.Active = true
	return t
}

func (t *testHandler) Handle(termbox.Event) error {
	// signal that we handled the event
	t.Ch++
	return nil
}

func (t *testHandler) GetCursor() fractal.Coordinates {
	return fractal.Coordinates{0, 0}
}

func (t *testHandler) GetAttr() (fg termbox.Attribute, bg termbox.Attribute) {
	return termbox.ColorDefault, termbox.ColorDefault
}

func (t *testHandler) IsActive() bool {
	return t.Active
}

func (t *testHandler) Man() string {
	return ""
}
