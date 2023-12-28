package rpc

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/term"
)

func TestAsyncClientDraw(t *testing.T) {
	t.Run("draws loading while waiting for response", func(t *testing.T) {
		const (
			width  = 7
			height = 4
		)
		var wg sync.WaitGroup
		c := NewAsyncClient(waitGroupInterrupter(&wg),
			&mockHandlerClient{
				remote: handler.Nop(component.NewStringWithConfig("WOW",
					component.StringConfig{Alignment: component.SpanAlignmentCentered})),
			},
		)
		c.Resize(width, height)
		expected := `       
LOADING
       
       `
		wg.Add(1)
		writer := term.NewStringWriter(width, height)
		c.Draw(writer)
		writer.Flush()
		assert.Equal(t, expected, writer.String())

		assert.NoError(t, c.Close())
	})

	t.Run("draws response after interrupt triggers draw", func(t *testing.T) {
		const (
			width  = 7
			height = 4
		)
		writer := term.NewStringWriter(width, height)
		var wg sync.WaitGroup
		c := NewAsyncClient(term.FuncInterrupter(func(ctx context.Context) error {
			writer.SetContext = ctx
			wg.Done()
			return nil
		}), &mockHandlerClient{
			remote: handler.Nop(component.NewStringWithConfig("WOW",
				component.StringConfig{Alignment: component.SpanAlignmentCentered})),
		})
		c.Resize(width, height)

		expected := `       
  WOW  
       
       `
		wg.Add(1)
		c.Draw(writer)

		wg.Wait()
		c.Draw(writer)
		writer.Flush()
		assert.Equal(t, expected, writer.String())

		assert.NoError(t, c.Close())
	})

	t.Run("draws previous response while waiting for new response", func(t *testing.T) {
		const (
			width  = 7
			height = 4
		)
		writer := term.NewStringWriter(width, height)
		var wg sync.WaitGroup
		c := NewAsyncClient(term.FuncInterrupter(func(ctx context.Context) error {
			writer.SetContext = ctx
			wg.Done()
			return nil
		}), &mockHandlerClient{
			remote: handler.Nop(component.NewStringWithConfig("WOW",
				component.StringConfig{Alignment: component.SpanAlignmentCentered})),
		})
		c.Resize(width, height)

		wg.Add(1)
		c.Draw(writer)

		wg.Wait()
		c.Draw(writer)

		expected := `       
  WOW  
       
       `
		wg.Add(1)
		c.Draw(writer)
		writer.Flush()
		assert.Equal(t, expected, writer.String())

		assert.NoError(t, c.Close())
	})

	t.Run("draws error if Draw is closed after Close", func(t *testing.T) {
		c := NewAsyncClient(term.NopInterrupter(), &mockHandlerClient{
			remote: handler.Nop(component.Nop()),
		})

		const (
			width  = 32
			height = 18
		)
		c.Resize(width, height)
		require.NoError(t, c.Close())
		expected := `                                
                                
                                
          ___                   
         /___/\_                
        _\   \/_/\__            
      __\       \/_/\           
      \   __    __ \ \          
     __\  \_\   \_\ \ \   __    
    /_/\\   __   __  \ \_/_/\   
    \_\/_\__\/\__\/\__\/_\_\/   
       \_\/_/\       /_\_\/     
          \_\/       \_\/       
                                
                                
Uh, Houston, we've had a problem
                                
                                `

		writer := term.NewStringWriter(width, height)
		c.Draw(writer)
		writer.Flush()
		assert.Equal(t, expected, writer.String())

		writer.Reset()
		c.Draw(writer)
		writer.Flush()
		assert.Equal(t, expected, writer.String())
	})

	t.Run("draws error if Draw returns error", func(t *testing.T) {
		const (
			width  = 32
			height = 18
		)
		var wg sync.WaitGroup
		mock := &mockHandlerClient{
			remote:   handler.Nop(component.Nop()),
			rpcError: errors.New("whoopsie"),
		}
		c := NewAsyncClient(waitGroupInterrupter(&wg), mock)
		c.Resize(width, height)

		wg.Add(1)
		writer := term.NewStringWriter(width, height)
		c.Draw(writer)
		// loading

		expected := `                                
                                
                                
          ___                   
         /___/\_                
        _\   \/_/\__            
      __\       \/_/\           
      \   __    __ \ \          
     __\  \_\   \_\ \ \   __    
    /_/\\   __   __  \ \_/_/\   
    \_\/_\__\/\__\/\__\/_\_\/   
       \_\/_/\       /_\_\/     
          \_\/       \_\/       
                                
                                
Uh, Houston, we've had a problem
                                
                                `
		wg.Wait()
		writer.Reset()
		wg.Add(1)
		c.Draw(writer)
		writer.Flush()
		assert.Equal(t, expected, writer.String())

		mock.rpcError = nil
		require.NoError(t, c.Close())
	})

	t.Run("is goroutine safe", func(t *testing.T) {
		const (
			width  = 800
			height = 600
		)
		var wg sync.WaitGroup
		const n = 100
		c := NewAsyncClient(term.NopInterrupter(), &mockHandlerClient{
			remote: handler.Nop(component.NewStringWithConfig("boogie",
				component.StringConfig{Alignment: component.SpanAlignmentCentered})),
		})
		c.Resize(width, height)

		wg.Add(n)
		for i := 0; i < n; i++ {
			go func(i int) {
				defer wg.Done()
				ctx := context.Background()
				writer := term.NewStringWriter(width, height)
				writer.SetContext = term.ContextWithPayload(ctx, []byte(strconv.Itoa(i)))
				c.Draw(writer)
			}(i)
		}

		wg.Wait()
		assert.NoError(t, c.Close())
	})
}

func TestAsyncClientClose(t *testing.T) {
	t.Run("is idempotent", func(t *testing.T) {
		c := NewAsyncClient(term.NopInterrupter(), &mockHandlerClient{
			remote: handler.Nop(component.Nop()),
		})
		require.NoError(t, c.Close())
		assert.NoError(t, c.Close())
	})
}

func TestUnitAsyncClientHandlerManual(t *testing.T) {
	c := NewAsyncClient(term.NopInterrupter(), &mockHandlerClient{
		remote: testHandler(),
	})
	testRPCHandlerManual(t, c)
	assert.NoError(t, c.Close())
}

func TestUnitAsyncClientHandlerCursor(t *testing.T) {
	c := NewAsyncClient(term.NopInterrupter(), &mockHandlerClient{
		remote: testHandler(),
	})
	testRPCHandlerCursor(t, c)
	assert.NoError(t, c.Close())
}

func waitGroupInterrupter(wg *sync.WaitGroup) term.Interrupter {
	return term.FuncInterrupter(func(context.Context) error { wg.Done(); return nil })
}
