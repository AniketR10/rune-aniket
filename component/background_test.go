package component

import (
	"testing"

	"github.com/ernestrc/fractal/term"
)

func TestDrawBackground(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	s := NewSpan(u)
	s.Padding.HorizontalPerc = 0.2
	s.Padding.VerticalPerc = 0.2
	b := WithBackground(s, term.Cell{Ch: 'O'})

	b.Resize(9, 5)

	w := term.NewStringWriter(9, 5)

	tests := []testCase{
		{
			nil, `
XXXXXXXXO
XXXXXXXXO
XXXXXXXXO
XXXXXXXXO
OOOOOOOOO`,
		},
	}

	testWorkflow(t, b, w, tests)
}
