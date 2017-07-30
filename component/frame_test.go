package component

import (
	"testing"

	"github.com/ernestrc/fractal"
	"github.com/ernestrc/fractal/writer"
)

func TestNewFrame(t *testing.T) {
	u := &testComponent{fill: '*'}
	f, err := NewFrame(u, 8, 4, 1, 1, fractal.ColorWhite, fractal.ColorBlack)

	if err != nil {
		t.Fatal(err)
	}

	if f.content.(*testComponent) != u {
		t.Errorf("did not set content correctly")
	}

	if f.Height() != 4 || f.Width() != 8 {
		t.Errorf("did not set size correctly")
	}

	if x, y := f.Position(); x != 1 || y != 1 {
		t.Errorf("did not set position correctly")
	}

	if f.Bg != fractal.ColorBlack || f.Fg != fractal.ColorWhite {
		t.Errorf("did not set attributes correctly")
	}
}

func TestDrawFrame(t *testing.T) {
	u := &testComponent{fill: 'X'}
	f, err := NewFrame(u, 8, 4, 0, 0, fractal.ColorWhite, fractal.ColorBlack)

	if err != nil {
		t.Fatal(err)
	}

	w := writer.String(8, 4)

	tests := []testCase{
		{
			nil, `
┌──────┐
│XXXXXX│
│XXXXXX│
└──────┘`,
		}, {
			func() { f.SetContent(&testComponent{fill: '*'}) }, `
┌──────┐
│******│
│******│
└──────┘`,
		}, {
			func() { f.Resize(4, 4) }, `
┌──┐    
│**│    
│**│    
└──┘    `,
		}, {
			func() { f.Move(4, 0) }, `
    ┌──┐
    │**│
    │**│
    └──┘`,
		}, {
			func() { f.SetContent(&testComponent{fill: 'T'}) }, `
    ┌──┐
    │TT│
    │TT│
    └──┘`,
		},
	}

	testWorkflow(t, f, w, tests)
}
