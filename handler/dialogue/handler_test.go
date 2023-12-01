package dialogue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

func TestHandlerIntegration(t *testing.T) {
	clip := clipboard.NewInMemory()
	interrupt := make(chan struct{})
	h, tx, rx := Handler(NewComponent(ComponentConfig{}), term.FuncInterrupter(func() error {
		interrupt <- struct{}{}
		return nil
	}), clip)
	defer close(tx)
	h.Resize(20, 9)

	t.Run("interrupt should be called after tx channel send msg", func(t *testing.T) {
		for _, ch := range "Hello assistant!" {
			_, handled := h.Handle(term.Event{Type: term.EventKey, Ch: ch})
			assert.True(t, handled)
		}
		go func() {
			msg := <-rx
			assert.Equal(t, "Hello assistant!", msg)
		}()
		_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
		assert.True(t, handled)

		tx <- "Well, hello Sir."

		<-interrupt

		w := term.NewStringWriter(20, 9)
		h.Draw(w)
		err := w.Flush()
		require.NoError(t, err)

		out := w.String()
		assert.Equal(t, `Hello assistant!    
Well, hello Sir.    
                    
                    
                    
                    
 ┌──────────────┐   
 │              │   
 └──────────────┘   `, out)
	})

	t.Run("double click should select and copy to clipboard", func(t *testing.T) {
		_, handled := h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
		assert.True(t, handled)
		_, handled = h.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft})
		assert.True(t, handled)

		data, err := clip.Paste(clipboard.DefaultRegisterID)
		require.NoError(t, err)
		assert.Equal(t, "Hello assistant!", data.Text)

		_, handled = h.Handle(term.Event{Type: term.EventMouse, MouseY: 1, Key: term.MouseLeft})
		assert.True(t, handled)

		data, err = clip.Paste(clipboard.DefaultRegisterID)
		require.NoError(t, err)
		assert.Equal(t, "Well, hello Sir.", data.Text)
	})
}
