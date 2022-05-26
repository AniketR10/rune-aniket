package component

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
)

func TestReferenceDraw(t *testing.T) {
	s := NewReference(nil)

	// valid sequence of calls
	s.Resize(8, 4)

	s.Init(&TestComponent{Ch: 'X'})

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
	testutil.TestComponent(t, s, w, tests)

	s.Init(&TestComponent{Ch: 'Y'})

	tests = []testutil.ComponentTestCase{
		{
			nil, `
YYYYYYYY 
YYYYYYYY 
YYYYYYYY 
YYYYYYYY 
         `,
		},
	}
	testutil.TestComponent(t, s, w, tests)
}
