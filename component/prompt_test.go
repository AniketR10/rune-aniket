package component

import (
	"testing"

	"github.com/ernestrc/go-tui/term"
	testutil "github.com/ernestrc/go-tui/util/test"
	"github.com/stretchr/testify/assert"
)

func testDrawPrompt(t *testing.T, cfg PromptConfig, expectedOut string) {
	s := NewPrompt(cfg)

	s.Resize(20, 10)

	w := term.NewStringWriter(21, 11)

	tests := []testutil.ComponentTestCase{
		{Expected: expectedOut},
	}

	testutil.TestComponent(t, s, w, tests)
}

func TestDrawPrompt(t *testing.T) {
	t.Run("no frame", func(t *testing.T) {
		testDrawPrompt(t, PromptConfig{
			Message: "Do you?",
			Options: []string{"Yay", "Nay"},
		}, `
                     
                     
      Do you?        
                     
                     
                     
                     
   Yay       Nay     
                     
                     
                     `,
		)
	})

	t.Run("with frame", func(t *testing.T) {
		testDrawPrompt(t, PromptConfig{
			Message: "Do you?",
			Options: []string{"Yay", "Nay"},
			Frame:   FrameCharSetDefault(),
		}, `┌──────────────────┐ 
│                  │ 
│     Do you?      │ 
│                  │ 
│                  │ 
│ ┌─────┐  ┌─────┐ │ 
│ │ Yay │  │ Nay │ │ 
│ └─────┘  └─────┘ │ 
│                  │ 
└──────────────────┘ 
                     `,
		)
	})

	t.Run("with frame overflow options", func(t *testing.T) {
		testDrawPrompt(t, PromptConfig{
			Message: "Why soooooo serious?",
			Options: []string{"Yay", "Nay", "Say", "Wey"},
			Frame:   FrameCharSetDefault(),
		}, `┌──────────────────┐ 
│                  │ 
│Why soooooo seriou│ 
│                  │ 
│                  │ 
│┌──┐┌──┐┌───┐┌───┐│ 
││Ya││Na││Say││Wey││ 
│└──┘└──┘└───┘└───┘│ 
│                  │ 
└──────────────────┘ 
                     `,
		)
	})

	t.Run("with frame overflow options", func(t *testing.T) {
		testDrawPrompt(t, PromptConfig{
			Message: "Why soooooo serious?",
			Options: []string{"Yay", "Nay", "Say", "Wey", "They", "May"},
		}, `                     
                     
Why soooooo serious? 
                     
                     
                     
                     
YayNaySayWeyTheyMay  
                     
                     
                     `,
		)
	})
}

func TestPromptSetOptionAttr(t *testing.T) {
	t.Run("does not panic with frame", func(t *testing.T) {
		p := NewPrompt(PromptConfig{
			Message: "Wasup?",
			Options: []string{"Meh", "Bleh"},
			Frame:   FrameCharSetDefault(),
		})
		p.SetOptionAttr(0, term.Attributes{})
		p.SetOptionAttr(1, term.Attributes{})
	})

	t.Run("does not panic without frame", func(t *testing.T) {
		p := NewPrompt(PromptConfig{
			Message: "Wasup?",
			Options: []string{"Meh", "Bleh"},
		})
		p.SetOptionAttr(0, term.Attributes{})
		p.SetOptionAttr(1, term.Attributes{})
	})
}

func TestPromptDefaults(t *testing.T) {
	t.Run("panics on emptym message", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = NewPrompt(PromptConfig{
				Message: "", Options: []string{"a"},
			})
		})
	})
	t.Run("panics on empty options", func(t *testing.T) {
		assert.Panics(t, func() {
			_ = NewPrompt(PromptConfig{
				Message: "blah", Options: []string{},
			})
		})
	})
}
