package test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
)

// ComponentTestCase represents an action and how a component
// is expected to be drawn after this action.
type ComponentTestCase struct {
	Action   func()
	Expected string
}

// StringerWriter is a term.Writer that also is able to produce a string screen output.
type StringerWriter interface {
	term.Writer
	fmt.Stringer
	Flush() error
	Clear(attr term.Attributes) (err error)
}

// TestComponent tests a given component against a set of ComponentTestCase.
func TestComponent(
	t *testing.T, m tui.Component,
	w StringerWriter, cases []ComponentTestCase,
) {
	var err error

	for i, tcase := range cases {
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
		assert.Equal(t, expected, w.String(), "testcase %d failed", i)
	}
}
