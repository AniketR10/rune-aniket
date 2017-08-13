package fractal

import (
	"testing"
)

func TestNewFrame(t *testing.T) {
	u := &TestComponent{Ch: '*'}
	f, err := NewFrame(u, 8, 4, 1, 1)

	if err != nil {
		t.Fatal(err)
	}

	if f.Content().(*TestComponent) != u {
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
	u := &TestComponent{Ch: 'X'}
	f, err := NewFrame(u, 8, 4, 0, 0)

	if err != nil {
		t.Fatal(err)
	}

	w := NewStringWriter(9, 5)

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
			func() { f.Move(4, 0) }, `
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
