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

package gui

import (
	"context"
	"sync"
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text/clipboard"
)

func TestUpdate(t *testing.T) {
	t.Run("passes iteration in Draw context to root handler", func(t *testing.T) {
		var called int
		mock := mockHandler{assertDraw: func(w term.Writer) {
			actualIteration, ok := tui.IterationFromContext(w.Context())
			require.True(t, ok)
			assert.Equal(t, int64(0), actualIteration)
			called++
		}}
		gui, _ := newTestGUI(t, &mock)

		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)
	})

	t.Run("Update DOES call Draw if ebiten calls Layout with DIFFERENT height/width", func(t *testing.T) {
		var called int
		mock := mockHandler{assertDraw: func(w term.Writer) {
			called++
		}}
		gui, _ := newTestGUI(t, &mock)

		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)

		gui.Layout(1600, 900)

		require.NoError(t, gui.Update())
		require.Equal(t, 2, called)
	})

	t.Run("Update DOES NOT calls Draw if ebiten calls Layout with SAME height/width", func(t *testing.T) {
		var called int
		mock := mockHandler{assertDraw: func(w term.Writer) {
			called++
		}}
		gui, _ := newTestGUI(t, &mock)

		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)

		gui.Layout(defaultWidth, defaultHeight)

		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)
	})

	t.Run("delegates events to handler", func(t *testing.T) {
		var expectedIterationID int64
		var called int
		mock := mockHandler{
			assertDraw: func(w term.Writer) {
				actualIteration, ok := tui.IterationFromContext(w.Context())
				require.True(t, ok)
				assert.Equal(t, expectedIterationID, actualIteration)
			},
			assertEvent: func(ev term.Event) (exit, handled bool) {
				called++
				assert.Equal(t, term.EventKey, ev.Type)
				assert.Equal(t, term.KeyEnter, ev.Key)
				return
			},
		}
		gui, input := newTestGUI(t, &mock)

		input.pressedKeys[ebiten.KeyEnter] = struct{}{}
		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)

		expectedIterationID++
		input.pressedKeys[ebiten.KeyEnter] = struct{}{}
		input.pressedKeys[ebiten.KeyMeta] = struct{}{}
		require.NoError(t, gui.Update())
		require.Equal(t, 2, called)

		delete(input.pressedKeys, ebiten.KeyMeta)
		delete(input.pressedKeys, ebiten.KeyEnter)
		require.NoError(t, gui.Update())
		require.Equal(t, 2, called)
	})

	t.Run("delivers pasted text to handler via EventPasteStart/End", func(t *testing.T) {
		const text = "SOA"
		var expectedIterationID int64
		var called int
		mock := mockHandler{
			assertDraw: func(w term.Writer) {
				actualIteration, ok := tui.IterationFromContext(w.Context())
				require.True(t, ok)
				assert.Equal(t, expectedIterationID, actualIteration)
			},
			assertEvent: func(ev term.Event) (exit, handled bool) {
				switch called {
				case 0:
					assert.Equal(t, term.EventPasteStart, ev.Type)
				case 4:
					assert.Equal(t, term.EventPasteEnd, ev.Type)
				default:
					assert.Equal(t, term.EventKey, ev.Type, called)
					switch called {
					case 1:
						assert.Equal(t, 'S', ev.Ch)
						assert.Equal(t, []byte{'S'}, ev.Raw)
					case 2:
						assert.Equal(t, 'O', ev.Ch)
						assert.Equal(t, []byte{'O'}, ev.Raw)
					case 3:
						assert.Equal(t, 'A', ev.Ch)
						assert.Equal(t, []byte{'A'}, ev.Raw)
					}
				}
				called++
				return
			},
		}
		clip := clipboard.NewInMemory()
		gui, _ := newTestGUI(t, &mock, WithClipboard(clip))
		require.NoError(t, clip.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: text}))
		require.NoError(t, gui.PasteFromClipboard())

		require.NoError(t, gui.Update())
		require.Equal(t, 5, called)
	})

	t.Run("delegates events to handler", func(t *testing.T) {
		var expectedIterationID int64
		var called int
		mock := mockHandler{
			assertDraw: func(w term.Writer) {
				actualIteration, ok := tui.IterationFromContext(w.Context())
				require.True(t, ok)
				assert.Equal(t, expectedIterationID, actualIteration)
			},
			assertEvent: func(ev term.Event) (exit, handled bool) {
				called++
				assert.Equal(t, term.EventKey, ev.Type)
				assert.Equal(t, term.KeyEnter, ev.Key)
				return
			},
		}
		gui, input := newTestGUI(t, &mock)

		input.pressedKeys[ebiten.KeyEnter] = struct{}{}
		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)

		expectedIterationID++
		input.pressedKeys[ebiten.KeyEnter] = struct{}{}
		input.pressedKeys[ebiten.KeyMeta] = struct{}{}
		require.NoError(t, gui.Update())
		require.Equal(t, 2, called)

		delete(input.pressedKeys, ebiten.KeyMeta)
		delete(input.pressedKeys, ebiten.KeyEnter)
		require.NoError(t, gui.Update())
		require.Equal(t, 2, called)
	})

	t.Run("process interrupts by calling Draw, with reset context", func(t *testing.T) {
		var called int
		mock := mockHandler{assertDraw: func(w term.Writer) {
			if called != 0 {
				_, ok := tui.IterationFromContext(w.Context())
				require.False(t, ok)

				_, ok = term.PayloadFromContext(w.Context())
				require.False(t, ok)
			}
			called++
		}}
		gui, _ := newTestGUI(t, &mock)

		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)

		// simulate publish
		gui.pendingEvents = append(gui.pendingEvents, term.Event{Type: term.EventInterrupt})

		require.NoError(t, gui.Update())
		require.Equal(t, 2, called)
	})

	t.Run("if interrupt contains .Raw payload, this is passed along in next call to Draw", func(t *testing.T) {
		var called int
		mock := mockHandler{assertDraw: func(w term.Writer) {
			if called != 0 {
				payload, ok := term.PayloadFromContext(w.Context())
				require.True(t, ok)
				assert.Equal(t, "X1234", string(payload))
			}

			called++
		}}
		gui, _ := newTestGUI(t, &mock)

		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)

		// simulate publish
		gui.pendingEvents = append(gui.pendingEvents, term.Event{
			Type: term.EventInterrupt,
			Raw:  []byte("X1234"),
		})

		require.NoError(t, gui.Update())
		require.Equal(t, 2, called)
	})

	t.Run("if interrupt contains iteration ID, this is passed along in next call to Draw", func(t *testing.T) {
		var called int
		var actualDrawContext context.Context
		mock := mockHandler{assertDraw: func(w term.Writer) {
			if called != 0 {
				actualIterationID, ok := tui.IterationFromContext(w.Context())
				require.True(t, ok)
				assert.Equal(t, int64(0), actualIterationID)
			}

			actualDrawContext = w.Context()
			called++
		}}
		gui, _ := newTestGUI(t, &mock)

		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)

		payload, ok := term.PayloadFromContext(actualDrawContext)
		require.True(t, ok)

		// simulate publish
		gui.pendingEvents = append(gui.pendingEvents, term.Event{
			Type: term.EventInterrupt,
			Raw:  payload,
		})

		require.NoError(t, gui.Update())
		require.Equal(t, 2, called)
	})

	t.Run("if interrupt contains user function this is called before next call to Draw", func(t *testing.T) {
		var called int
		var userFnCalled int
		mock := mockHandler{assertDraw: func(w term.Writer) {
			if called != 0 {
				assert.Equal(t, 1, userFnCalled)
			}
			called++
		}}
		gui, _ := newTestGUI(t, &mock)

		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)

		var wg sync.WaitGroup
		wg.Add(1)

		// simulate publish; UserFunc is processed asynchronously
		go gui.consumeEvents()
		gui.updateChan <- term.Event{
			Type: term.EventInterrupt,
			UserFunc: func() {
				defer wg.Done()
				userFnCalled++
			},
		}

		wg.Wait()
		require.NoError(t, gui.Update())
		require.Equal(t, 1, called)
		assert.Equal(t, 1, userFnCalled)
	})

	t.Run("returns ErrHandlerExited if handler exits", func(t *testing.T) {
		mock := mockHandler{
			assertEvent: func(ev term.Event) (bool, bool) {
				return true, true
			},
		}
		gui, input := newTestGUI(t, &mock)

		input.pressedKeys[ebiten.KeyEnter] = struct{}{}
		require.Equal(t, ErrHandlerExited, gui.Update())
	})
}

func TestLayout(t *testing.T) {
	t.Run("does not panic on layout 0 width and height", func(t *testing.T) {
		mock := mockHandler{}
		gui, _ := newTestGUI(t, &mock)
		assert.NotPanics(t, func() {
			gui.Layout(0, 0)
		})
	})

	t.Run("resizes underlying handler if width/height are different", func(t *testing.T) {
		mock := mockHandler{}
		gui, _ := newTestGUI(t, &mock)

		mock.width = 0
		mock.height = 0
		gui.Layout(1200, 900)
		assert.NotZero(t, mock.width)
		assert.NotZero(t, mock.height)
	})

	t.Run("does not resize underlying handler if width/height are the same", func(t *testing.T) {
		mock := mockHandler{}
		gui, _ := newTestGUI(t, &mock)

		mock.width = 0
		mock.height = 0
		gui.Layout(defaultWidth, defaultHeight)
		assert.Zero(t, mock.width)
		assert.Zero(t, mock.height)
	})
}

func newTestGUI(t *testing.T, mock *mockHandler, opts ...Option) (*GUI, *mockInputManager) {
	gui, err := New(mock, opts...)
	require.NoError(t, err)

	ret := &mockInputManager{pressedKeys: make(map[ebiten.Key]struct{})}
	gui.input.input = ret
	gui.input.keyPressDelay = 0
	gui.input.keyPressRepeat = 0

	return gui, ret
}

type mockHandler struct {
	width, height int
	assertDraw    func(term.Writer)
	assertEvent   func(term.Event) (bool, bool)
}

func (m *mockHandler) Resize(width, height int) {
	m.width, m.height = width, height
}

func (m *mockHandler) Draw(w term.Writer) {
	m.assertDraw(w)
}

func (m *mockHandler) Handle(ev term.Event) (exit, handled bool) {
	return m.assertEvent(ev)
}

func (m *mockHandler) Cursor() (c term.Coordinates, s term.CursorStyle, show bool) {
	return
}

func (m *mockHandler) Man() tui.Manual {
	return tui.Manual{}
}
