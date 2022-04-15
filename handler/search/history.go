package search

import (
	"context"
	"fmt"
	"time"

	"github.com/ernestrc/blue/datastore/document"
)

const (
	defaultStoreTimeout = 100 * time.Millisecond
)

type historyDocument struct {
	Queries []string
}

// History acts as a persisted stack of queries. It provides operations
// to push queries and retrieve previously persisted queries.
// A pop operation is not provided explicitly although calling Add
// when the number of stored queries is equal to the maximum allowed
// will remove the oldest query.
type History struct {
	store   document.Service
	timeout time.Duration
	docID   string
	idx     int
	max     int
	doc     historyDocument
}

// NewHistory allocates storage for a new instance of History and initializes it.
func NewHistory(store document.Service, documentID string, maxHistory int) (
	*History, error,
) {
	ret := new(History)
	err := ret.Init(store, documentID, maxHistory)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Init initializes this history and loads any queries persisted in store.
func (h *History) Init(
	store document.Service, documentID string, maxHistory int,
) error {
	if documentID == "" || store == nil || maxHistory == 0 {
		err := fmt.Sprintf("invalid Init args: documentID=%q, store=%v, max=%d",
			documentID, store, maxHistory)
		panic(err)
	}
	h.store = store
	h.docID = documentID
	h.timeout = defaultStoreTimeout
	h.max = maxHistory
	return h.loadCommandHistory()
}

func (h *History) loadCommandHistory() error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultStoreTimeout)
	defer cancel()

	err := h.store.Get(ctx, h.docID, &h.doc)
	if err == document.ErrNotFound {
		err = nil
	}
	if err != nil {
		return fmt.Errorf("Failed to load search history from store: %s", err)
	}
	return nil
}

// Add adds query to the history. If the number of queries persisted
// is greater than the max permitted, then the first query is dropped.
func (h *History) Add(query string) error {
	// next Next should return query
	h.idx = 0

	if query == "" {
		return nil
	}

	h.doc.Queries = append(h.doc.Queries, "")
	copy(h.doc.Queries[1:], h.doc.Queries)
	h.doc.Queries[0] = query

	if len(h.doc.Queries) > h.max {
		h.doc.Queries = h.doc.Queries[:h.max]
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultStoreTimeout)
	defer cancel()

	err := h.store.Set(ctx, h.docID, &h.doc)
	if err != nil {
		return fmt.Errorf("could not set command history: %v", err)
	}
	return nil
}

// Next returns the next query and updates History
// such that the next call to Next would return the query after.
// If this method is called after the last query has been returned
// the first query is returned instead.
func (h *History) Next() string {
	if len(h.doc.Queries) == 0 {
		return ""
	}
	search := h.doc.Queries[h.idx]
	h.idx++
	if h.idx == len(h.doc.Queries) {
		h.idx = 0
	}
	return search
}
