package cell

import (
	"strings"
	"testing"

	"github.com/ernestrc/fractal/term"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStatefulSearcher(t *testing.T) {
	fn := func(buf *Buffer) Searcher {
		searcher := NewSimpleSearcher(buf)
		return ResetSubscriberSearcher(buf, searcher)
	}

	testSearch(t, fn)
}

func TestStatefulSearcherAfterReset(t *testing.T) {
	fn := func(buf *Buffer) Searcher {
		searcher := NewSimpleSearcher(buf)
		s := ResetSubscriberSearcher(buf, searcher)
		s.Search("whatever")
		s.OnDidDelete(term.Coordinates{}, term.Coordinates{}, "")
		return s
	}

	testSearch(t, fn)
}

func newResetSubscriberSearcher(t *testing.T, content string) (*Buffer, SubscriberSearcher) {
	buf := NewBuffer()
	searcher := NewSimpleSearcher(buf)
	rs := ResetSubscriberSearcher(buf, searcher)

	_, err := buf.ReadFrom(strings.NewReader(content))
	require.NoError(t, err)

	return buf, rs
}

func TestResetSubscriberSearcher(t *testing.T) {
	t.Run("should skip stale matches that have been updated upon calls to Prev/NextResult", func(t *testing.T) {
		buf, rs := newResetSubscriberSearcher(t, "blah blah bleh")
		require.Equal(t, 2, rs.Search("blah"))
		pos, ok := rs.NextResult()
		require.True(t, ok)
		assert.Equal(t, pos, term.Coordinates{})

		buf.Insert(term.Coordinates{X: 7}, 'j')

		pos, ok = rs.NextResult()
		require.True(t, ok)
		assert.Equal(t, pos, term.Coordinates{})

		pos, ok = rs.NextResult()
		require.True(t, ok)
		assert.Equal(t, pos, term.Coordinates{})

		pos, ok = rs.PrevResult()
		require.True(t, ok)
		assert.Equal(t, pos, term.Coordinates{})
	})

	t.Run("should unsubscribe upon Unsubscribe", func(t *testing.T) {
		buf, rs := newResetSubscriberSearcher(t, "blah blah bleh")
		require.Equal(t, 0, rs.Search("oh"))
		rs.Unsubscribe()

		buf.InsertString(term.Coordinates{}, "oh")

		_, ok := rs.NextResult()
		require.False(t, ok)
	})
}
