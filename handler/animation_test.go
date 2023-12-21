package handler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

func newTestInterrupter() (term.Interrupter, chan struct{}) {
	ch := make(chan struct{})
	interrupter := term.FuncInterrupter(func(context.Context) error {
		ch <- struct{}{}
		return nil
	})
	return interrupter, ch
}

func TestAnimationPlayer(t *testing.T) {
	fps := 30
	interrupter, ch := newTestInterrupter()

	frames := []string{
		"0000    \n0000    \n    0000\n    0000",
		"1111    \n1111    \n    1111\n    1111",
	}
	sequences := []int{0, 0, 1}

	c := component.NewAnimation(interrupter, frames, sequences, fps)

	// sut
	h := AnimationPlayer(c)
	h.Resize(8, 4)

	for i := 0; i < 3; i++ {
		w := term.NewStringWriter(8, 4)
		<-ch
		h.Draw(w)
		var expected string
		switch i {
		case 0:
			expected = "0000    \n0000    \n    0000\n    0000"
		case 1:
			expected = "0000    \n0000    \n    0000\n    0000"
		case 2:
			expected = "1111    \n1111    \n    1111\n    1111"
		default:
			panic("hmmm")
		}

		require.NoError(t, w.Flush())
		assert.Equal(t, expected, w.String(), i)
	}

	// pause
	exit, handled := h.Handle(term.Event{Key: term.KeySpace, Type: term.EventKey})
	assert.True(t, handled)
	assert.False(t, exit)

	for i := 0; i < 3; i++ {
		w := term.NewStringWriter(8, 4)
		<-ch
		h.Draw(w)
		expected := "0000    \n0000    \n    0000\n    0000"
		require.NoError(t, w.Flush())
		assert.Equal(t, expected, w.String(), i)
	}

	// unpause
	exit, handled = h.Handle(term.Event{Key: term.KeySpace, Type: term.EventKey})
	assert.True(t, handled)
	assert.False(t, exit)

	for i := 0; i < 3; i++ {
		w := term.NewStringWriter(8, 4)
		<-ch
		h.Draw(w)
		var expected string
		switch i {
		case 2:
			expected = "0000    \n0000    \n    0000\n    0000"
		case 0:
			expected = "0000    \n0000    \n    0000\n    0000"
		case 1:
			expected = "1111    \n1111    \n    1111\n    1111"
		default:
			panic("hmmm")
		}

		require.NoError(t, w.Flush())
		assert.Equal(t, expected, w.String(), i)
	}

	// close
	exit, handled = h.Handle(term.Event{Key: term.KeyEsc, Type: term.EventKey})
	assert.True(t, handled)
	assert.True(t, exit)
	goleak.VerifyNone(t)

	// after close animation should be paused and cause no panics
	for i := 0; i < 3; i++ {
		w := term.NewStringWriter(8, 4)
		h.Draw(w)
		expected := "0000    \n0000    \n    0000\n    0000"
		require.NoError(t, w.Flush())
		assert.Equal(t, expected, w.String(), i)
	}
}
