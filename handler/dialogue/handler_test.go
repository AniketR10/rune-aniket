// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package dialogue

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

func TestHandlerIntegration(t *testing.T) {
	clip := clipboard.NewInMemory()
	interrupt := make(chan struct{})
	h, tx, rx := Handler(new(sync.Mutex),
		NewComponent(ComponentConfig{}), term.FuncInterrupter(func(context.Context) error {
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
