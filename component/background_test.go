package component

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
)

func TestDrawBackground(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	s := NewSpan(u, SpanConfig{
		PadHorizontalPerc: 0.2,
		PadVerticalPerc:   0.2,
	})
	b := WithBackground(s, term.Cell{Ch: 'O'})

	b.Resize(9, 5)

	w := term.NewStringWriter(9, 5)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
XXXXXXXXO
XXXXXXXXO
XXXXXXXXO
XXXXXXXXO
OOOOOOOOO`,
		},
	}

	testutil.TestComponent(t, b, w, tests)
}
