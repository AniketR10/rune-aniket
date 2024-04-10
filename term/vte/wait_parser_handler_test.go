package vte

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/term/vte/parser"
)

func TestWaitParserHandler(t *testing.T) {
	t.Run("schedules a callback", func(t *testing.T) {
		mock := newMockBellHandler()
		ph := newWaitParserHandler(mock)

		var callbackCalled, triggerCalled bool
		ph.useTrigger(func() {
			triggerCalled = true
		})
		ph.scheduleBellCallback(1*time.Second, func() {
			callbackCalled = true
		})

		ph.Bell()
		assert.True(t, callbackCalled)
		assert.True(t, triggerCalled)
	})

	t.Run("schedules multiple callbacks, preserving order", func(t *testing.T) {
		mock := newMockBellHandler()
		ph := newWaitParserHandler(mock)

		n := 10
		ch := make(chan struct{}, n)

		var callbackCalled, triggerCalled int
		var wg sync.WaitGroup
		wg.Add(10)
		ph.useTrigger(func() {
			triggerCalled++
			ch <- struct{}{}
		})

		for i := 0; i < n; i++ {
			i := i
			ph.scheduleBellCallback(1*time.Second, func() {
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
		mock := newMockBellHandler()
		ph := newWaitParserHandler(mock)

		n := 10000

		var callbackCalled, triggerCalled atomic.Int32
		ph.useTrigger(func() {
			triggerCalled.Add(1)
			ph.Bell()
		})

		var wg sync.WaitGroup
		wg.Add(n)
		go func() {
			for i := 0; i < n; i++ {
				i := i
				ph.scheduleBellCallback(10*time.Millisecond, func() {
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
	parser.Handler
}

func newMockBellHandler() *mockBellHandler {
	ret := new(mockBellHandler)
	ret.Handler = parser.NopHandler()
	return ret
}

func (m *mockBellHandler) Bell() {
}
