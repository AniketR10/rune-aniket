package component

import (
	"testing"

	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestDrawBackgroundNoZero(t *testing.T) {
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

func TestDrawBackground(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	s := NewSpan(u, SpanConfig{
		PadHorizontalPerc: 0.2,
		PadVerticalPerc:   0.2,
	})
	b := WithBackground(s, term.Cell{Ch: 0})

	b.Resize(9, 5)

	w := term.NewStringWriter(9, 5)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
XXXXXXXX 
         `,
		},
	}

	testutil.TestComponent(t, b, w, tests)
}
