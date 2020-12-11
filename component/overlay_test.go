package component

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
)

func TestDrawOverlay(t *testing.T) {
	cfg := SpanConfig{
		PadVertical:      -5,
		PadHorizontal:    -10,
		ContentAlignment: SpanAlignmentCentered,
	}
	background := &TestComponent{Ch: '*'}
	cover := NewFrame(String("a"))

	o := Overlay(background, cover, cfg)

	w := term.NewStringWriter(16, 9)

	tests := []testCase{
		{
			func() { o.Resize(16, 9) }, `
****************
****************
***┌────────┐***
***│        │***
***│   a    │***
***│        │***
***└────────┘***
****************
****************`,
		}, {
			func() { o.Resize(4, 4) }, `
┌──┐            
│a │            
│  │            
└──┘            
                
                
                
                
                `,
		}, {
			func() { o.Resize(2, 2) }, `
a               
                
                
                
                
                
                
                
                `,
		},
	}

	testWorkflow(t, o, w, tests)
}
