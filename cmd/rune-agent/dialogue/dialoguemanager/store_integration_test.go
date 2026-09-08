// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package dialoguemanager

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	sdkstoragerpc "github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc/docpb"
	"unstable.build/rune/internal/localstorage"
	"unstable.build/rune/internal/localstorage/storagerpc"
)

// newRealBackend stands up the production storage stack used by the agent:
// a bbolt/firstmover service exposed over a storagerpc gRPC server, reached
// through a storagerpc client. This is the path that the in-memory stub used
// by the rest of the store tests never exercises.
func newRealBackend(t *testing.T) storageapi.Service {
	t.Helper()
	marshaler := docbson.Marshaler()

	backend := localstorage.New(context.Background(), t.TempDir(), marshaler)
	srv := storagerpc.NewServer(backend, marshaler)

	gsrv := grpc.NewServer()
	docpb.RegisterDocumentStoreServer(gsrv, srv)

	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	go func() {
		_ = gsrv.Serve(lis)
	}()
	t.Cleanup(func() {
		gsrv.Stop()
		_ = srv.Close()
		_ = lis.Close()
	})

	client, err := sdkstoragerpc.NewClient(lis.Addr(), marshaler,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	return client
}

func TestStoreRealBackendRetainsContextAcrossTurns(t *testing.T) {
	ctx := context.Background()
	s := NewStore(newRealBackend(t), t.TempDir())

	turn1 := []llmapi.Message{
		{Role: llmapi.RoleSystem, Content: "system"},
		{Role: llmapi.RoleUser, Content: "choose 1 or 2"},
		{Role: llmapi.RoleAssistant, Content: "option 1 or option 2?"},
	}
	require.NoError(t, s.Create(ctx, Dialogue{ID: "d1", Messages: turn1}))

	d, err := s.Get(ctx, "d1")
	require.NoError(t, err)

	turn2 := []llmapi.Message{
		{Role: llmapi.RoleUser, Content: "lets go with 1"},
		{Role: llmapi.RoleAssistant, Content: "going with option 1"},
	}
	require.NoError(t, s.AppendMessages(ctx, d, turn2, llmapi.DialogueUsage{}))

	got, err := s.Get(ctx, "d1")
	require.NoError(t, err)
	assert.NotEmpty(t, got.MessagesPath)

	want := append(append([]llmapi.Message(nil), turn1...), turn2...)
	assert.Equal(t, want, got.Messages,
		"all messages from both turns must survive reload")
}

// wrappingBackend mirrors the production gRPC/firstmover path, which can return
// storage sentinels wrapped in extra context (e.g. "layer: %w"). Identity
// comparisons (err == storageapi.ErrNotFound) miss these; only errors.Is
// recovers them. The in-memory stub returns bare sentinels and never exercises
// this regression in the store's index bookkeeping.
type wrappingBackend struct {
	storageapi.Service
}

func wrapSentinel(err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("backend layer: %w", err)
}

func (b wrappingBackend) Get(ctx context.Context, id string, doc any) error {
	return wrapSentinel(b.Service.Get(ctx, id, doc))
}

func (b wrappingBackend) Create(ctx context.Context, id string, doc any) error {
	return wrapSentinel(b.Service.Create(ctx, id, doc))
}

func (b wrappingBackend) Update(
	ctx context.Context, id string,
	updates []storageapi.Update, precond ...storageapi.Precondition,
) error {
	return wrapSentinel(b.Service.Update(ctx, id, updates, precond...))
}

func TestStoreWrappedSentinelBackendRetainsContext(t *testing.T) {
	ctx := context.Background()
	s := NewStore(wrappingBackend{storagestub.NewInMemoryService()}, t.TempDir())

	turn1 := []llmapi.Message{
		{Role: llmapi.RoleSystem, Content: "system"},
		{Role: llmapi.RoleUser, Content: "choose 1 or 2"},
		{Role: llmapi.RoleAssistant, Content: "option 1 or option 2?"},
	}
	require.NoError(t, s.Create(ctx, Dialogue{ID: "d1", Messages: turn1}))

	d, err := s.Get(ctx, "d1")
	require.NoError(t, err)

	turn2 := []llmapi.Message{
		{Role: llmapi.RoleUser, Content: "lets go with 1"},
		{Role: llmapi.RoleAssistant, Content: "going with option 1"},
	}
	require.NoError(t, s.AppendMessages(ctx, d, turn2, llmapi.DialogueUsage{}))

	got, err := s.Get(ctx, "d1")
	require.NoError(t, err)
	assert.NotEmpty(t, got.MessagesPath)

	want := append(append([]llmapi.Message(nil), turn1...), turn2...)
	assert.Equal(t, want, got.Messages,
		"all messages from both turns must survive reload")
}
