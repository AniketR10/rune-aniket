package component

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
)

func TestDrawFrame(t *testing.T) {
	u := &TestComponent{Ch: 'X'}
	f := NewFrame(u)

	f.Resize(8, 4)

	w := term.NewStringWriter(9, 5)

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
		}, {
			func() {
				fb := FrameCharSetDefault()
				fb.Horizontal = '┄'
				fb.Vertical = '┊'
				f.FrameCharSet = fb
			}, `
┌┄┄┄┄┄┄┐ 
┊TTTTTT┊ 
┊TTTTTT┊ 
└┄┄┄┄┄┄┘ 
         `,
		},
	}

	testWorkflow(t, f, w, tests)
}
