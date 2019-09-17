package fractal

import (
	"testing"
)

func TestDrawFrame(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	f := NewFrame(u, 0, 0)

	f.Resize(8, 4)

	w := newStringWriter(9, 5)

	tests := []testCase{
		{
			nil, `
┌──────┐ 
│XXXXXX│ 
│XXXXXX│ 
└──────┘ 
         `,
		}, {
			func() { f.SetContent(&TestComponent{Ch: '*'}) }, `
┌──────┐ 
│******│ 
│******│ 
└──────┘ 
         `,
		}, {
			func() { f.Resize(4, 4) }, `
┌──┐     
│**│     
│**│     
└──┘     
         `,
		}, {
			func() { f.SetContent(&TestComponent{Ch: 'T'}) }, `
┌──┐     
│TT│     
│TT│     
└──┘     
         `,
		}, {
			func() { f.Resize(2, 2) }, `
TT       
TT       
         
         
         `,
		}, {
			func() { f.Resize(8, 4) }, `
┌──────┐ 
│TTTTTT│ 
│TTTTTT│ 
└──────┘ 
         `,
		},
	}

	testWorkflow(t, f, w, tests)
}
