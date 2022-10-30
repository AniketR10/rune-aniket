package search

import (
	"testing"

	"github.com/ernestrc/blue/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistory(t *testing.T) {
	store := document.NewInMemoryService()
	history := NewHistory(store, "id", 4)
	err := history.Load()
	require.NoError(t, err)

	require.NoError(t, history.Add("jmac"))
	require.NoError(t, history.Add("jj"))

	assert.Equal(t, "jj", history.Next())
	assert.Equal(t, "jmac", history.Next())
	assert.Equal(t, "jj", history.Next())

	// adding another one resets history
	require.NoError(t, history.Add("Kom"))
	assert.Equal(t, "Kom", history.Next())
	assert.Equal(t, "jj", history.Next())

	// queries are persisted across stores
	history2 := NewHistory(store, "id", 4)
	err = history2.Load()
	require.NoError(t, err)
	assert.Equal(t, "Kom", history2.Next())
	assert.Equal(t, "jj", history2.Next())
	assert.Equal(t, "jmac", history2.Next())
	assert.Equal(t, "Kom", history2.Next())

	// history pointers are kept in isolation
	assert.Equal(t, "jmac", history.Next())
	assert.Equal(t, "jj", history2.Next())

	// test max
	require.NoError(t, history.Add("4"))
	require.NoError(t, history.Add("5"))
	assert.Equal(t, "5", history.Next())
	assert.Equal(t, "4", history.Next())
	assert.Equal(t, "Kom", history.Next())
	assert.Equal(t, "jj", history.Next())
	assert.Equal(t, "5", history.Next())

	// queries are NOT persisted across stores with diff IDs
	history3 := NewHistory(store, "id2", 4)
	err = history3.Load()
	require.NoError(t, err)
	assert.Equal(t, "", history3.Next())
	require.NoError(t, history3.Add("a"))
	assert.Equal(t, "a", history3.Next())
	assert.Equal(t, "a", history3.Next())
}
