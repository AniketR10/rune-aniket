// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package component

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/component/comptest"
	"unstable.build/go-tui/term"
)

func TestAsync(t *testing.T) {
	t.Run("draws placeholder if contstructor is not done", func(t *testing.T) {
		interrupt := term.NopInterrupter()
		a := Async[tui.Component](interrupt, &TestComponent{Ch: 'a'},
			func() (tui.Component, error) {
				<-t.Context().Done()
				return &TestComponent{Ch: 'x'}, nil
			})
		a.Resize(20, 9)
		w := term.NewStringWriter(20, 9)
		tests := []comptest.TestCase{
			{
				nil, `
aaaaaaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaa
aaaaaaaaaaaaaaaaaaaa`,
			},
		}
		comptest.TestComponent(t, a, w, tests)
	})

	t.Run("draws constructed component if done", func(t *testing.T) {
		var sema sync.Mutex
		sema.Lock()
		interrupt := term.FuncInterrupter(func(context.Context) error {
			sema.Unlock()
			return nil
		})
		a := Async[tui.Component](interrupt, &TestComponent{Ch: 'a'},
			func() (tui.Component, error) {
				return &TestComponent{Ch: 'x'}, nil
			})
		a.Resize(8, 8)
		w := term.NewStringWriter(20, 9)
		tests := []comptest.TestCase{
			{
				nil, `
xxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxx
xxxxxxxxxxxxxxxxxxxx`,
			},
		}
		sema.Lock()
		a.Resize(20, 9)
		comptest.TestComponent(t, a, w, tests)
	})

	t.Run("draws error when component constructor errors", func(t *testing.T) {
		var sema sync.Mutex
		sema.Lock()
		interrupt := term.FuncInterrupter(func(context.Context) error {
			sema.Unlock()
			return nil
		})
		a := Async[tui.Component](interrupt, &TestComponent{Ch: 'a'},
			func() (tui.Component, error) {
				return nil, errors.New("boom")
			})
		w := term.NewStringWriter(20, 9)
		tests := []comptest.TestCase{
			{
				nil, `
                    
                    
            ___     
                    
  /___/\_           
                    
          _\        
  \/_/\__           
                    `,
			},
		}
		sema.Lock()
		a.Resize(20, 9)
		comptest.TestComponent(t, a, w, tests)
	})

	t.Run("Height", func(t *testing.T) {
		var sema sync.Mutex
		sema.Lock()
		interrupt := term.FuncInterrupter(func(context.Context) error {
			sema.Unlock()
			return nil
		})
		a := Async[Responsive](interrupt, &TestResponsive{},
			func() (Responsive, error) {
				return &TestResponsive{WantHeight: 10}, nil
			})
		sema.Lock()
		assert.Equal(t, 10, a.Height(99))
	})

	t.Run("Dimensions", func(t *testing.T) {
		var sema sync.Mutex
		sema.Lock()
		interrupt := term.FuncInterrupter(func(context.Context) error {
			sema.Unlock()
			return nil
		})
		a := Async[Floating](interrupt, &TestResponsive{},
			func() (Floating, error) {
				return &TestResponsive{WantWidth: 10, WantHeight: 10}, nil
			})
		sema.Lock()
		width, height := a.Dimensions()
		assert.Equal(t, 10, width)
		assert.Equal(t, 10, height)
	})

	t.Run("SetAttr", func(t *testing.T) {
		var sema sync.Mutex
		sema.Lock()
		interrupt := term.FuncInterrupter(func(context.Context) error {
			sema.Unlock()
			return nil
		})
		attrs := term.Attributes{Fg: tcell.ColorYellow, Bg: tcell.ColorBlue}
		a := Async[WithAttributes](interrupt, &TestComponent{},
			func() (WithAttributes, error) {
				return &TestComponent{Attributes: attrs}, nil
			})
		sema.Lock()
		repv := a.SetAttr(term.Attributes{})
		assert.Equal(t, tcell.ColorBlue, repv.Bg)
		assert.Equal(t, tcell.ColorYellow, repv.Fg)
	})

	t.Run("SetAttr is called on constructed after constructor returns", func(t *testing.T) {
		var sema1, sema2 sync.Mutex
		sema1.Lock()
		sema2.Lock()
		interrupt := term.FuncInterrupter(func(context.Context) error {
			sema2.Unlock()
			return nil
		})
		tc := &TestComponent{}
		a := Async[WithAttributes](interrupt, &TestComponent{},
			func() (WithAttributes, error) {
				sema1.Lock()
				return tc, nil
			})
		attrs := term.Attributes{Fg: tcell.ColorYellow, Bg: tcell.ColorBlue}
		a.SetAttr(attrs)
		sema1.Unlock()
		sema2.Lock()
		assert.Equal(t, attrs, tc.Attributes)
	})
}
