package test

import (
	"strings"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
)

// ComponentTestCase represents an action and how a component
// is expected to be drawn after this action.
type ComponentTestCase struct {
	Action   func()
	Expected string
}

// TestComponent tests a given component against a set of ComponentTestCase.
func TestComponent(
	t *testing.T, m tui.Component,
	w *term.StringWriter, cases []ComponentTestCase,
) {
	var err error

	for _, tcase := range cases {
		if err = w.Clear(term.Attributes{}); err != nil {
			t.Fatal(err)
		}

		if tcase.Action != nil {
			tcase.Action()
		}

		m.Draw(w)

		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		// for readability, we expected strings are written starting with \n
		expected := strings.TrimLeft(tcase.Expected, "\n")
		assert.Equal(t, expected, w.String())
	}
}
