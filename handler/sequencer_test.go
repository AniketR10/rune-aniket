package handler

import (
	"fmt"
	"testing"
	"time"

	"github.com/ernestrc/go-tui/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequencer(t *testing.T) {
	t.Run("returns sequence match", func(t *testing.T) {
		interests := []Sequence{{
			First: term.Event{Type: term.EventKey, Ch: 'd'},
			Last:  term.Event{Type: term.EventKey, Ch: 'd'},
		}}
		s := NewSequencer(interests, 1*time.Hour)

		seq, match := s.Handle(term.Event{Type: term.EventKey, Ch: 'd'})
		assert.False(t, match)
		assert.Zero(t, seq)

		seq, match = s.Handle(term.Event{Type: term.EventKey, Ch: 'd'})
		require.True(t, match)
		assert.Equal(t, interests[0], seq)
	})
	t.Run("ignores superfluous event fields in interests arg", func(t *testing.T) {
		interests := []Sequence{{
			First: term.Event{Type: term.EventKey, MouseX: 10, Ch: 'd'},
			Last:  term.Event{Type: term.EventKey, MouseX: 10, Ch: 'd'},
		}}
		s := NewSequencer(interests, 1*time.Hour)

		seq, match := s.Handle(term.Event{Type: term.EventKey, Ch: 'd'})
		assert.False(t, match)
		assert.Zero(t, seq)

		seq, match = s.Handle(term.Event{Type: term.EventKey, Ch: 'd'})
		require.True(t, match)
		assert.Equal(t, Sequence{
			First: term.Event{Type: term.EventKey, Ch: 'd'},
			Last:  term.Event{Type: term.EventKey, Ch: 'd'},
		}, seq)
	})
	t.Run("ignores superfluous event fields in Handle", func(t *testing.T) {
		interests := []Sequence{{
			First: term.Event{Type: term.EventKey, Ch: 'd'},
			Last:  term.Event{Type: term.EventKey, Ch: 'd'},
		}}
		s := NewSequencer(interests, time.Hour)

		seq, match := s.Handle(term.Event{Type: term.EventKey, MouseY: 1, Ch: 'd'})
		assert.False(t, match)
		assert.Zero(t, seq)

		seq, match = s.Handle(term.Event{Type: term.EventKey, MouseY: 1, Ch: 'd'})
		require.True(t, match)
		assert.Equal(t, interests[0], seq)
	})
	t.Run("does not return match if it took too long for second event", func(t *testing.T) {
		interests := []Sequence{{
			First: term.Event{Type: term.EventKey, Ch: 'd'},
			Last:  term.Event{Type: term.EventKey, Ch: 'd'},
		}}
		s := NewSequencer(interests, 1*time.Nanosecond)

		seq, match := s.Handle(term.Event{Type: term.EventKey, MouseY: 1, Ch: 'd'})
		assert.False(t, match)
		assert.Zero(t, seq)

		time.Sleep(time.Millisecond)

		seq, match = s.Handle(term.Event{Type: term.EventKey, MouseY: 1, Ch: 'd'})
		require.False(t, match)
		assert.Zero(t, seq)
	})
	t.Run("returns sequence match even if last event failed to match with initial event", func(t *testing.T) {
		interests := []Sequence{{
			First: term.Event{Type: term.EventKey, Ch: 'd'},
			Last:  term.Event{Type: term.EventKey, Ch: 'd'},
		}}
		s := NewSequencer(interests, 25*time.Millisecond)

		seq, match := s.Handle(term.Event{Type: term.EventKey, MouseY: 1, Ch: 'd'})
		assert.False(t, match)
		assert.Zero(t, seq)

		time.Sleep(26 * time.Millisecond)

		seq, match = s.Handle(term.Event{Type: term.EventKey, MouseY: 1, Ch: 'd'})
		require.False(t, match)
		assert.Zero(t, seq)

		seq, match = s.Handle(term.Event{Type: term.EventKey, MouseY: 1, Ch: 'd'})
		require.True(t, match)
		assert.Equal(t, interests[0], seq)
	})
}

func TestParseSequence(t *testing.T) {
	tsuite := []struct {
		in      string
		wantSeq Sequence
		wantErr bool
	}{
		{"", Sequence{}, true},
		{"f", Sequence{}, true},
		{"<c-x>", Sequence{}, true},
		{"<><>", Sequence{}, true},
		{"<c-s> ", Sequence{}, true},
		{" <c-p>", Sequence{}, true},
		{"<c-pf", Sequence{}, true},
		{"f<c-p", Sequence{}, true},
		{"c-p>f", Sequence{}, true},
		{">c-p<f", Sequence{}, true},
		{"f>c-p<", Sequence{}, true},
		{"><><", Sequence{}, true},
		{"ff", Sequence{
			First: term.Event{Type: term.EventKey, Ch: 'f'},
			Last:  term.Event{Type: term.EventKey, Ch: 'f'},
		}, false},
		{">>", Sequence{
			First: term.Event{Type: term.EventKey, Ch: '>'},
			Last:  term.Event{Type: term.EventKey, Ch: '>'},
		}, false},
		{"<<", Sequence{
			First: term.Event{Type: term.EventKey, Ch: '<'},
			Last:  term.Event{Type: term.EventKey, Ch: '<'},
		}, false},
		{"f<c-p>", Sequence{
			First: term.Event{Type: term.EventKey, Ch: 'f'},
			Last:  term.Event{Type: term.EventKey, Key: term.KeyCtrlP},
		}, false},
		{"<c-p>f", Sequence{
			First: term.Event{Type: term.EventKey, Key: term.KeyCtrlP},
			Last:  term.Event{Type: term.EventKey, Ch: 'f'},
		}, false},
		{"<c-x><c-p>", Sequence{
			First: term.Event{Type: term.EventKey, Key: term.KeyCtrlX},
			Last:  term.Event{Type: term.EventKey, Key: term.KeyCtrlP},
		}, false},
		{"<m-c-]><c-p>", Sequence{
			First: term.Event{
				Type: term.EventKey,
				Key:  term.KeyCtrlRsqBracket,
				Mod:  term.ModAlt,
			},
			Last: term.Event{Type: term.EventKey, Key: term.KeyCtrlP},
		}, false},
	}

	for _, tcase := range tsuite {
		t.Run(fmt.Sprintf("test case: '%s'", tcase.in), func(t *testing.T) {
			actualSeq, actualErr := ParseSequence(tcase.in)
			if tcase.wantErr {
				require.Error(t, actualErr)
			} else {
				require.NoError(t, actualErr)
			}
			assert.Equal(t, tcase.wantSeq, actualSeq)
		})
	}
}
