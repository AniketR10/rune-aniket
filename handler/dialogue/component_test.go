package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
)

func testDrawComponent(t *testing.T, cfg ComponentConfig, expectedOut string) {
	s := NewComponent(cfg)

	s.Resize(20, 10)

	w := term.NewStringWriter(21, 11)

	tests := []testutil.ComponentTestCase{
		{Expected: expectedOut},
	}

	testutil.TestComponent(t, s, w, tests)
}

func TestComponentDraw(t *testing.T) {
	for i := 0; i < 2; i++ {
		var desc string
		if i == 0 {
			desc = "resize before adding history"
		} else {
			desc = "resize after adding history"
		}

		t.Run(desc, func(t *testing.T) {
			comp := NewComponent(ComponentConfig{})
			if i == 0 {
				comp.Resize(20, 10)
			}

			comp.AddSendMessage("wasup bro")
			comp.AddReceiveMessage("I am a AI assistant blabla.")
			comp.AddSendMessage("Speak in bro.")
			comp.AddReceiveMessage("Yo, wa'tup.")

			if i == 1 {
				comp.Resize(20, 10)
			}

			w := term.NewStringWriter(21, 11)
			tests := []testutil.ComponentTestCase{
				{
					Action: func() {},
					Expected: `wasup bro            
I am a AI assistant  
blabla.              
Speak in bro.        
Yo, wa'tup.          
                     
                     
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						comp.AddSendMessage("This is rather bore")
						comp.AddSendMessage("This is rather bore")
						comp.AddSendMessage("This is rather bore")
					},
					Expected: `I am a AI assistant  
blabla.              
Speak in bro.        
Yo, wa'tup.          
This is rather bore  
This is rather bore  
This is rather bore  
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						assert.True(t, comp.SeekUp())
						assert.False(t, comp.SeekUp())
					},
					Expected: `wasup bro            
I am a AI assistant  
blabla.              
Speak in bro.        
Yo, wa'tup.          
This is rather bore  
This is rather bore  
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
				{
					Action: func() {
						assert.True(t, comp.SeekDown())
						assert.False(t, comp.SeekDown())
					},
					Expected: `I am a AI assistant  
blabla.              
Speak in bro.        
Yo, wa'tup.          
This is rather bore  
This is rather bore  
This is rather bore  
 ┌──────────────┐    
 │              │    
 └──────────────┘    
                     `,
				},
			}
			testutil.TestComponent(t, comp, w, tests)
		})
	}
}

func TestComponentInputPosition(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.Resize(20, 10)

	assert.Equal(t, term.Coordinates{Y: 7, X: 1}, comp.InputPosition())
}
