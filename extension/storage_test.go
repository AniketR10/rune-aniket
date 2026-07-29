// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package extension

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// probeService records whether it actually served a request and how
// many times it was closed, so a test can prove that a registration
// serves the borrowed instance instead of one it constructed itself.
type probeService struct {
	storageapi.Service
	gets   int
	closes int
}

func (s *probeService) Get(ctx context.Context, id string, doc any) error {
	s.gets++
	return s.Service.Get(ctx, id, doc)
}

func (s *probeService) Close() error {
	s.closes++
	return nil
}

// serveStorageResources registers svc on a fresh gRPC server, as one
// workspace extension runner does, and returns a client for it.
func serveStorageResources(
	t *testing.T, svc storageapi.Service,
) storageapi.Service {
	t.Helper()

	resources := StorageResources(svc)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	srv := grpc.NewServer()
	closer, err := resources[extensionapi.PermissionStorage].Register(srv, new(sync.Mutex))
	require.NoError(t, err)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() {
		assert.NoError(t, closer.Close())
		srv.Stop()
	})

	client, err := storagerpc.NewClient(lis.Addr(), docbson.Marshaler(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

type storedDoc struct {
	Value string
}

// TestStorageResourcesSharesOneServiceAcrossWorkspaces guards the
// invariant that every workspace serves one process-wide storage
// service. Constructing a service per registration silently demotes all
// but the first workspace to a firstmover follower of its own process,
// which doubles the encode/decode work on every extension storage call.
func TestStorageResourcesSharesOneServiceAcrossWorkspaces(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	shared := &probeService{Service: storagestub.NewInMemoryService()}
	require.NoError(t, shared.Service.Set(ctx, "doc", storedDoc{Value: "seeded"}))

	first := serveStorageResources(t, shared)
	second := serveStorageResources(t, shared)

	// A registration that builds its own service would miss the document
	// seeded directly into the shared instance.
	for name, client := range map[string]storageapi.Service{
		"first workspace": first, "second workspace": second,
	} {
		var got storedDoc
		require.NoError(t, client.Get(ctx, "doc", &got), name)
		assert.Equal(t, "seeded", got.Value, name)
	}
	assert.Equal(t, 2, shared.gets,
		"every workspace must serve the shared storage service instance")

	assert.Zero(t, shared.closes,
		"registrations borrow the shared service and must not close it")
}
