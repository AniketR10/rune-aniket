// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

package localstorage

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/blue/document/doctest"
	"github.com/unstablebuild/ox-api/bluestore"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

type miniDoc struct {
	V string
}

// TestNewMultiProcessSafeOnBolt asserts that two localstorage.New
// instances pointing at the same directory cooperate via firstmover
// leader election rather than racing for the bolt OS flock. Before
// the leader-only-storage refactor, the second instance silently
// fell back to an in-memory stub when bolt.Open timed out, so
// follower reads returned ErrNotFound instead of leader writes.
func TestNewMultiProcessSafeOnBolt(t *testing.T) {
	dir := t.TempDir()
	m := docbson.Marshaler()

	a := New(context.Background(), dir, m)
	defer func() { _ = a.Close() }()
	b := New(context.Background(), dir, m)
	defer func() { _ = b.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	require.NoError(t, a.Set(ctx, "k", &miniDoc{V: "from-a"}))
	var got miniDoc
	require.NoError(t, b.Get(ctx, "k", &got))
	require.Equal(t, "from-a", got.V)
}

func TestStorageConcurrentInstances(t *testing.T) {
	doctest.TestDocumentService(t, func(t *testing.T) document.Service {
		name, err := os.MkdirTemp("", "workspace_document_service_test")
		t.Cleanup(func() {
			_ = os.RemoveAll(name)
		})
		require.NoError(t, err)
		instance := New(context.Background(), name, docbson.Marshaler())
		return bluestore.AdaptFrom(instance)
	})
}

func TestDelayedLoadingServiceGetHonorsContextWhileInitializing(t *testing.T) {
	svc := &delayedLoadingService{ready: make(chan struct{})}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		var doc any
		errCh <- svc.Get(ctx, "blocked", &doc)
	}()

	select {
	case err := <-errCh:
		require.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Get blocked waiting for delayed storage initialization")
	}

	svc.service = storagestub.NewInMemoryService()
	close(svc.ready)
}
