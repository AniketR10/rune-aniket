package component

import (
	"strings"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	action   func()
	expected string
}

func testWorkflow(
	t *testing.T, m tui.Component,
	w *term.StringWriter, cases []testCase,
) {
	var err error

	for _, tcase := range cases {
		if err = w.Clear(term.Attributes{}); err != nil {
			t.Fatal(err)
		}

		if tcase.action != nil {
			tcase.action()
		}

		m.Draw(w)

		if err := w.Flush(); err != nil {
			t.Fatal(err)
		}

		// for readability, we expected strings are written starting with \n
		expected := strings.TrimLeft(tcase.expected, "\n")
		assert.Equal(t, expected, w.String())
	}
}
