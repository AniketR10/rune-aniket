package component

import (
	"testing"

	"github.com/ernestrc/fractal/writer"
)

func TestNewFrame(t *testing.T) {
	u := &Fill{Ch: '*'}
	f, err := NewFrame(u, 8, 4, 1, 1)

	if err != nil {
		t.Fatal(err)
	}

	if f.content.(*Fill) != u {
		t.Errorf("did not set content correctly")
	}

	if f.Height() != 4 || f.Width() != 8 {
		t.Errorf("did not set size correctly")
	}

	if x, y := f.Position(); x != 1 || y != 1 {
		t.Errorf("did not set position correctly")
	}
}

func TestDrawFrame(t *testing.T) {
	u := &Fill{Ch: 'X'}
	f, err := NewFrame(u, 8, 4, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	w := writer.String(9, 5)

	tests := []testCase{
		{
			nil, `
┌──────┐ 
│XXXXXX│ 
│XXXXXX│ 
└──────┘ 
         `,
		}, {
			func() { f.SetContent(&Fill{Ch: '*'}) }, `
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
			func() { f.Move(4, 0) }, `
    ┌──┐ 
    │**│ 
    │**│ 
    └──┘ 
         `,
		}, {
			func() { f.SetContent(&Fill{Ch: 'T'}) }, `
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
			func() { f.Resize(8, 4); f.Move(0, 0) }, `
┌──────┐ 
│TTTTTT│ 
│TTTTTT│ 
└──────┘ 
         `,
		},
	}

	testWorkflow(t, f, w, tests)
}
