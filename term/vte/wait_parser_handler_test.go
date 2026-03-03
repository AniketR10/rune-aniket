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

package vte

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/term/vte/vteparser"
)

func TestWaitParserHandler(t *testing.T) {
	t.Parallel()
	t.Run("schedules a callback", func(t *testing.T) {
		t.Parallel()
		mock := newMockBellHandler()
		ctx := t.Context()
		ph := newWaitParserHandler(ctx, mock)

		var callbackCalled, triggerCalled bool
		ph.useTrigger(func() {
			triggerCalled = true
		})
		ph.scheduleBellCallback(func() {
			callbackCalled = true
		})

		ph.Bell()
		assert.True(t, callbackCalled)
		assert.True(t, triggerCalled)
	})

	t.Run("schedules multiple callbacks, preserving order", func(t *testing.T) {
		t.Parallel()
		mock := newMockBellHandler()
		ctx := t.Context()
		ph := newWaitParserHandler(ctx, mock)

		n := 10
		ch := make(chan struct{}, n)

		var callbackCalled, triggerCalled int
		var wg sync.WaitGroup
		wg.Add(10)
		ph.useTrigger(func() {
			triggerCalled++
			ch <- struct{}{}
		})

		for i := range n {
			i := i
			ph.scheduleBellCallback(func() {
				defer wg.Done()
				assert.Equal(t, i, callbackCalled)
				callbackCalled++
			})
		}

		go func() {
			for {
				<-ch
				ph.Bell()
			}
		}()

		wg.Wait()
		assert.Equal(t, n, callbackCalled)
		assert.Equal(t, n, triggerCalled)
	})

	t.Run("is goroutine safe", func(t *testing.T) {
		t.Parallel()
		mock := newMockBellHandler()
		ctx := t.Context()
		ph := newWaitParserHandler(ctx, mock)

		n := 10000

		var callbackCalled, triggerCalled atomic.Int32
		ph.useTrigger(func() {
			triggerCalled.Add(1)
			ph.Bell()
		})

		var wg sync.WaitGroup
		wg.Add(n)
		go func() {
			for i := range n {
				i := i
				ph.scheduleBellCallback(func() {
					defer wg.Done()
					assert.Equal(t, i+1, int(callbackCalled.Add(1)))
				})
			}
		}()

		wg.Wait()
		assert.Equal(t, n, int(callbackCalled.Load()))
	})
}

type mockBellHandler struct {
	vteparser.Handler
}

func newMockBellHandler() *mockBellHandler {
	ret := new(mockBellHandler)
	ret.Handler = vteparser.NopHandler()
	return ret
}

func (m *mockBellHandler) Bell() {
}
