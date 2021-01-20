package component

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
)

func TestDrawFrameUnion(t *testing.T) {
	one := Virtual{C: NewFrame(&TestComponent{Ch: 'X'})}
	two := Virtual{C: NewFrame(&TestComponent{Ch: 'A'})}
	f := NewFrameUnion(&one, &two)
	one.Resize(10, 3)
	f.Resize(10, 8)

	w := term.NewStringWriter(10, 8)

	tests := []testutil.ComponentTestCase{
		{
			nil, `
┌────────┐
│XXXXXXXX│
├────────┤
│AAAAAAAA│
│AAAAAAAA│
│AAAAAAAA│
│AAAAAAAA│
└────────┘`,
		}, {
			func() {
				f.Left = '┊'
				f.Right = '┊'
			}, `
┌────────┐
│XXXXXXXX│
┊────────┊
│AAAAAAAA│
│AAAAAAAA│
│AAAAAAAA│
│AAAAAAAA│
└────────┘`,
		}, {
			func() {
				one.Resize(1, 2)
				f.Resize(2, 2)
			}, `
XX        
AA        
          
          
          
          
          
          `,
		},
	}

	testutil.TestComponent(t, f, w, tests)
}
