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
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
	"github.com/unstablebuild/blue/document/doctest"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"unstable.build/go-tui/cmd/ox-api/bluestore"
)

func TestStorageConcurrentInstances(t *testing.T) {
	doctest.TestDocumentService(t, func(t *testing.T) document.Service {
		name, err := os.MkdirTemp("", "workspace_document_service_test")
		t.Cleanup(func() {
			_ = os.RemoveAll(name)
		})
		require.NoError(t, err)
		instance := New(context.Background(), name, doctoml.Marshaler())
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
