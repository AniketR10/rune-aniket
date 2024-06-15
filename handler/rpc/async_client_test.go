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
