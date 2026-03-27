// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.

package localstorage

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
	"github.com/unstablebuild/blue/document/doctest"
	"unstable.build/go-tui/localstorage/bluestore"
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
