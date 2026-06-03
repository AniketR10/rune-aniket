// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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
	"unstable.build/go-tui/localstorage"
	"unstable.build/go-tui/localstorage/storagerpc"
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
