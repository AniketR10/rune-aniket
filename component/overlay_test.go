package component

import (
	"testing"

	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func TestDrawOverlay(t *testing.T) {
	cfg := SpanConfig{
		PadVertical:      -5,
		PadHorizontal:    -10,
		ContentAlignment: SpanAlignmentCentered,
	}
	background := &TestComponent{Ch: '*'}
	cover := NewFrame(NewStringWithConfig("a", StringConfig{Alignment: SpanAlignmentCentered}))

	o := NewOverlay(background, cover, term.Attributes{}, cfg)

	w := term.NewStringWriter(16, 9)

	tests := []testutil.ComponentTestCase{
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

	testutil.TestComponent(t, o, w, tests)
}
