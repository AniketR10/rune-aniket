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

package storagerpc

import (
	"bytes"
	"context"
	"net"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	bluebolt "github.com/unstablebuild/blue/document/bolt"
	"github.com/unstablebuild/blue/document/docmarshal/docbson"
	"github.com/unstablebuild/blue/document/docmarshal/docjson"
	"github.com/unstablebuild/blue/document/docmarshal/doctoml"
	"github.com/unstablebuild/blue/document/doctest"
	"github.com/unstablebuild/ox-api/bluestore"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc/docpb"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func tcpListener() (net.Listener, error) {
	return net.Listen("tcp", ":0")
}

func runDatastoreServerOverListener(
	t *testing.T, other storageapi.Service,
	listener func() (net.Listener, error),
	marshaler docmarshal.Marshaler,
	register func(grpc.ServiceRegistrar, docpb.DocumentStoreServer),
	opts ...grpc.ServerOption,
) (net.Addr, func()) {
	gsrv := grpc.NewServer(opts...)

	srv := new(Server)
	register(gsrv, srv)
	srv.Init(other, marshaler)

	lis, err := listener()
	require.NoError(t, err)

	teardown := func() {
		require.NoError(t, srv.Close())
		gsrv.Stop()
		lis.Close()
	}

	addr := lis.Addr()
	go func() {
		_ = gsrv.Serve(lis)
	}()

	return addr, teardown
}

func runDatastoreServer(
	t *testing.T, other storageapi.Service, marshaler docmarshal.Marshaler,
) (net.Addr, func()) {
	return runDatastoreServerOverListener(t, other, tcpListener, marshaler,
		docpb.RegisterDocumentStoreServer)
}

func testRPCDatastoreOverListener(
	t *testing.T, listener func() (net.Listener, error),
	marshaler docmarshal.Marshaler,
) {
	teardowns := []func(){}

	doctest.TestDocumentService(t, func(t *testing.T) document.Service {
		cache := document.NewInMemoryServiceWithMarshaler(marshaler)
		addr, teardown := runDatastoreServerOverListener(t, bluestore.AdaptTo(cache),
			listener, marshaler, docpb.RegisterDocumentStoreServer)
		teardowns = append(teardowns, teardown)

		store, err := storagerpc.NewClient(addr, marshaler,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)

		return bluestore.AdaptFrom(store)
	})

	for _, fn := range teardowns {
		fn()
	}
}

func testRPCPartitionedDatastoreOverListener(
	t *testing.T, listener func() (net.Listener, error),
	marshaler docmarshal.Marshaler,
) {
	teardowns := []func(){}

	doctest.TestDocumentService(t, func(t *testing.T) document.Service {
		cache := storagestub.NewInMemoryServiceWithMarshaler(marshaler)
		addr, teardown := runDatastoreServerOverListener(t, cache,
			listener, marshaler, docpb.RegisterDocumentStoreServer)
		teardowns = append(teardowns, teardown)

		store, err := storagerpc.NewClient(addr, marshaler,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)

		part, err := store.Partition("suite")
		require.NoError(t, err)

		return bluestore.AdaptFrom(part)
	})

	for _, fn := range teardowns {
		fn()
	}
}

// tempUnixListener creates a temp file and exposes it
// as a unix domain sockets net.Listener.
func tempUnixListener() (net.Listener, error) {
	tf, err := os.CreateTemp("", "plugin")
	if err != nil {
		return nil, err
	}
	path := tf.Name()

	// Close the file and remove it because it has to not exist for
	// the domain socket.
	if err := tf.Close(); err != nil {
		return nil, err
	}
	if err := os.Remove(path); err != nil {
		return nil, err
	}

	l, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}

	return l, nil
}

func TestRPC(t *testing.T) {
	t.Run("over TCP", func(t *testing.T) {
		testRPCDatastoreOverListener(t, tcpListener, docbson.Marshaler())
	})

	t.Run("over Unix domain sockets", func(t *testing.T) {
		testRPCDatastoreOverListener(t, tempUnixListener, docbson.Marshaler())
	})
}

func TestRPCPartitioned(t *testing.T) {
	t.Run("over TCP", func(t *testing.T) {
		testRPCPartitionedDatastoreOverListener(t, tcpListener, docbson.Marshaler())
	})

	t.Run("over Unix domain sockets", func(t *testing.T) {
		testRPCPartitionedDatastoreOverListener(t, tempUnixListener, docbson.Marshaler())
	})
}

func TestListWithFieldProjection(t *testing.T) {
	for name, marshaler := range map[string]docmarshal.Marshaler{
		"bson": docbson.Marshaler(),
		"toml": doctoml.Marshaler(),
		"json": docjson.Marshaler(),
	} {
		t.Run(name, func(t *testing.T) {
			cache := document.NewInMemoryServiceWithMarshaler(marshaler)
			addr, teardown := runDatastoreServer(t, bluestore.AdaptTo(cache), marshaler)
			defer teardown()

			store, err := storagerpc.NewClient(addr, marshaler,
				grpc.WithTransportCredentials(insecure.NewCredentials()))
			require.NoError(t, err)
			defer store.Close()

			err = store.Create(context.Background(), "doc-1", map[string]any{
				"name":   "Ada",
				"role":   "admin",
				"secret": "hidden",
			})
			require.NoError(t, err)

			ctx := storagerpc.WithFields(context.Background(), "name", "role")
			it, err := store.List(ctx, nil)
			require.NoError(t, err)
			defer it.Close()

			require.True(t, it.HasNext())

			var got map[string]any
			err = it.NextTo(&got)
			require.NoError(t, err)
			require.Equal(t, "Ada", got["name"])
			require.Equal(t, "admin", got["role"])
			require.NotContains(t, got, "secret")
			require.NotContains(t, got, storageapi.DefaultCreatedAtField)
			require.NotContains(t, got, storageapi.LowerCreatedAtField)

			for key := range got {
				require.Contains(t, []string{
					"name",
					"role",
					storageapi.DefaultUpdatedAtField,
					storageapi.LowerUpdatedAtField,
				}, key)
			}
			require.False(t, it.HasNext())
		})
	}
}

type interopHelper struct {
	read  document.Service
	write document.Service
}

// nonDroppableService hides the Drop method of the service it wraps, since
// embedding an interface only promotes that interface's methods.
type nonDroppableService struct {
	storageapi.Service
}

func TestDrop(t *testing.T) {
	marshaler := docbson.Marshaler()
	ctx := context.Background()

	t.Run("drops only the addressed partition", func(t *testing.T) {
		backing := storagestub.NewInMemoryServiceWithMarshaler(marshaler)
		addr, teardown := runDatastoreServer(t, backing, marshaler)
		defer teardown()

		store, err := storagerpc.NewClient(addr, marshaler,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer store.Close()

		dropped, err := store.Partition("dropped")
		require.NoError(t, err)
		kept, err := store.Partition("kept")
		require.NoError(t, err)
		nested, err := dropped.Partition("nested")
		require.NoError(t, err)

		for _, svc := range []storageapi.Service{dropped, kept, nested} {
			require.NoError(t, svc.Create(ctx, "doc", map[string]any{"v": "1"}))
		}

		require.NoError(t, dropped.(storageapi.DroppableService).Drop(ctx))

		var doc map[string]any
		require.ErrorIs(t, dropped.Get(ctx, "doc", &doc), storageapi.ErrNotFound)
		require.NoError(t, kept.Get(ctx, "doc", &doc))
		// Drop is not recursive: owners of sub-partitions drop them.
		require.NoError(t, nested.Get(ctx, "doc", &doc))

		// the service must remain usable after being dropped
		require.NoError(t, dropped.Create(ctx, "doc", map[string]any{"v": "2"}))
		require.NoError(t, dropped.Get(ctx, "doc", &doc))
	})

	t.Run("service that cannot be dropped", func(t *testing.T) {
		backing := nonDroppableService{
			storagestub.NewInMemoryServiceWithMarshaler(marshaler),
		}
		addr, teardown := runDatastoreServer(t, backing, marshaler)
		defer teardown()

		store, err := storagerpc.NewClient(addr, marshaler,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer store.Close()

		require.ErrorIs(t, store.(storageapi.DroppableService).Drop(ctx),
			storageapi.ErrPreconditionFailed)
	})
}

func TestBatch(t *testing.T) {
	marshaler := docbson.Marshaler()
	ctx := context.Background()

	t.Run("applies every operation of the batch", func(t *testing.T) {
		backing := storagestub.NewInMemoryServiceWithMarshaler(marshaler)
		addr, teardown := runDatastoreServer(t, backing, marshaler)
		defer teardown()

		store, err := storagerpc.NewClient(addr, marshaler,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer store.Close()

		part, err := store.Partition("p")
		require.NoError(t, err)
		type rec struct{ V string }
		require.NoError(t, part.Create(ctx, "taken", rec{V: "1"}))
		require.NoError(t, part.Create(ctx, "doomed", rec{V: "1"}))
		require.NoError(t, part.Create(ctx, "bumped", rec{V: "1"}))

		writer, ok := part.(storageapi.BatchWriter)
		require.True(t, ok, "the rpc client must support batching")
		results, err := writer.ApplyBatch(ctx, []storageapi.BatchOp{
			{Type: storageapi.BatchCreate, ID: "fresh", Doc: rec{V: "1"}},
			{Type: storageapi.BatchCreate, ID: "taken", Doc: rec{V: "2"}},
			{Type: storageapi.BatchSet, ID: "taken", Doc: rec{V: "2"}},
			{Type: storageapi.BatchDelete, ID: "doomed"},
			{Type: storageapi.BatchUpdate, ID: "bumped", Updates: []storageapi.Update{
				{FieldPath: []string{"V"}, Value: "2"},
			}, Preconditions: []storageapi.Precondition{
				{FieldPath: []string{"V"}, Value: "1"},
			}},
			{Type: storageapi.BatchUpdate, ID: "bumped", Updates: []storageapi.Update{
				{FieldPath: []string{"V"}, Value: "3"},
			}, Preconditions: []storageapi.Precondition{
				{FieldPath: []string{"V"}, Value: "1"},
			}},
			{Type: storageapi.BatchUpdate, ID: "ghost", Updates: []storageapi.Update{
				{FieldPath: []string{"V"}, Value: "1"},
			}},
		})
		require.NoError(t, err)
		require.Len(t, results, 7)
		assert.NoError(t, results[0].Err)
		assert.ErrorIs(t, results[1].Err, storageapi.ErrAlreadyExists)
		assert.NoError(t, results[2].Err)
		assert.NoError(t, results[3].Err)
		assert.NoError(t, results[4].Err)
		assert.ErrorIs(t, results[5].Err, storageapi.ErrPreconditionFailed)
		assert.ErrorIs(t, results[6].Err, storageapi.ErrNotFound)

		var doc map[string]any
		require.NoError(t, part.Get(ctx, "fresh", &doc))
		assert.Equal(t, "1", doc["v"])
		require.NoError(t, part.Get(ctx, "taken", &doc))
		assert.Equal(t, "2", doc["v"])
		require.NoError(t, part.Get(ctx, "bumped", &doc))
		assert.Equal(t, "2", doc["v"])
		assert.ErrorIs(t, part.Get(ctx, "doomed", &doc), storageapi.ErrNotFound)
	})

	t.Run("documents larger than a chunk", func(t *testing.T) {
		backing := storagestub.NewInMemoryServiceWithMarshaler(marshaler)
		addr, teardown := runDatastoreServer(t, backing, marshaler)
		defer teardown()

		store, err := storagerpc.NewClient(addr, marshaler,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer store.Close()

		big := strings.Repeat("x", 3<<20)
		type rec struct{ V string }
		writer := store.(storageapi.BatchWriter)
		results, err := writer.ApplyBatch(ctx, []storageapi.BatchOp{
			{Type: storageapi.BatchSet, ID: "big", Doc: rec{V: big}},
			{Type: storageapi.BatchSet, ID: "small", Doc: rec{V: "s"}},
		})
		require.NoError(t, err)
		require.Len(t, results, 2)
		require.NoError(t, results[0].Err)
		require.NoError(t, results[1].Err)

		var doc rec
		require.NoError(t, store.Get(ctx, "big", &doc))
		assert.Equal(t, big, doc.V)
		require.NoError(t, store.Get(ctx, "small", &doc))
		assert.Equal(t, "s", doc.V)
	})

	t.Run("service that cannot batch", func(t *testing.T) {
		backing := nonBatchingService{
			storagestub.NewInMemoryServiceWithMarshaler(marshaler),
		}
		addr, teardown := runDatastoreServer(t, backing, marshaler)
		defer teardown()

		store, err := storagerpc.NewClient(addr, marshaler,
			grpc.WithTransportCredentials(insecure.NewCredentials()))
		require.NoError(t, err)
		defer store.Close()

		_, err = store.(storageapi.BatchWriter).ApplyBatch(ctx,
			[]storageapi.BatchOp{{Type: storageapi.BatchDelete, ID: "x"}})
		require.Equal(t, codes.Unimplemented, status.Code(err))
	})
}

// nonBatchingService hides the ApplyBatch method of the service it wraps,
// since embedding an interface only promotes that interface's methods.
type nonBatchingService struct {
	storageapi.Service
}

func (h interopHelper) Create(ctx context.Context, ID string, doc any) error {
	return h.write.Create(ctx, ID, doc)
}

func (h interopHelper) Set(ctx context.Context, ID string, doc any) error {
	return h.write.Set(ctx, ID, doc)
}

func (h interopHelper) Update(
	ctx context.Context, ID string, updates []document.Update,
	preconds ...document.Precondition,
) error {
	return h.write.Update(ctx, ID, updates, preconds...)
}

func (h interopHelper) Get(ctx context.Context, ID string, doc any) error {
	return h.read.Get(ctx, ID, doc)
}

func (h interopHelper) Delete(ctx context.Context, ID string) error {
	return h.write.Delete(ctx, ID)
}

func (h interopHelper) List(
	ctx context.Context, filters []document.Filter,
) (document.Iterator, error) {
	return h.read.List(ctx, filters)
}

func (h interopHelper) Close() error {
	err1 := h.write.Close()
	err2 := h.read.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func TestRPCInterop(t *testing.T) {
	teardowns := []func(){}

	t.Cleanup(func() {
		for _, fn := range teardowns {
			fn()
		}
	})

	for name, marshaler := range map[string]docmarshal.Marshaler{
		"bson": docbson.Marshaler(),
		"toml": doctoml.Marshaler(),
		"json": docjson.Marshaler(),
	} {
		t.Run(name, func(t *testing.T) {
			t.Run("writes by client/server are readable by underlying service", func(t *testing.T) {
				doctest.TestDocumentService(t, func(t *testing.T) document.Service {
					cache := document.NewInMemoryServiceWithMarshaler(marshaler)
					addr, teardown := runDatastoreServer(t, bluestore.AdaptTo(cache), marshaler)
					teardowns = append(teardowns, teardown)

					store, err := storagerpc.NewClient(addr, marshaler,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					require.NoError(t, err)

					return interopHelper{read: cache, write: bluestore.AdaptFrom(store)}
				})
			})

			t.Run("writes by underlying service are readable by client/server", func(t *testing.T) {
				doctest.TestDocumentService(t, func(t *testing.T) document.Service {
					cache := document.NewInMemoryServiceWithMarshaler(marshaler)
					addr, teardown := runDatastoreServer(t, bluestore.AdaptTo(cache), marshaler)
					teardowns = append(teardowns, teardown)

					store, err := storagerpc.NewClient(addr, marshaler,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					require.NoError(t, err)

					return interopHelper{read: bluestore.AdaptFrom(store), write: cache}
				})
			})

			t.Run("partitioned writes by client/server are readable by underlying service", func(t *testing.T) {
				doctest.TestDocumentService(t, func(t *testing.T) document.Service {
					cache := storagestub.NewInMemoryServiceWithMarshaler(marshaler)
					addr, teardown := runDatastoreServer(t, cache, marshaler)
					teardowns = append(teardowns, teardown)

					store, err := storagerpc.NewClient(addr, marshaler,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					require.NoError(t, err)

					storePart, err := store.Partition("suite")
					require.NoError(t, err)
					cachePart, err := cache.Partition("suite")
					require.NoError(t, err)

					return interopHelper{read: bluestore.AdaptFrom(cachePart), write: bluestore.AdaptFrom(storePart)}
				})
			})

			t.Run("partitioned writes by underlying service are readable by client/server", func(t *testing.T) {
				doctest.TestDocumentService(t, func(t *testing.T) document.Service {
					cache := storagestub.NewInMemoryServiceWithMarshaler(marshaler)
					addr, teardown := runDatastoreServer(t, cache, marshaler)
					teardowns = append(teardowns, teardown)

					store, err := storagerpc.NewClient(addr, marshaler,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					require.NoError(t, err)

					storePart, err := store.Partition("suite")
					require.NoError(t, err)
					cachePart, err := cache.Partition("suite")
					require.NoError(t, err)

					return interopHelper{read: bluestore.AdaptFrom(storePart), write: bluestore.AdaptFrom(cachePart)}
				})
			})

			t.Run("single instance preconditions", func(t *testing.T) {
				doctest.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
					cache := document.NewInMemoryServiceWithMarshaler(marshaler)
					addr, teardown := runDatastoreServer(t, bluestore.AdaptTo(cache), marshaler)
					teardowns = append(teardowns, teardown)

					store, err := storagerpc.NewClient(addr, marshaler,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					require.NoError(t, err)

					return interopHelper{read: cache, write: bluestore.AdaptFrom(store)}
				})
			})

			t.Run("partitioned single instance preconditions", func(t *testing.T) {
				doctest.TestDocumentServicePreconditions(t, func(t *testing.T) document.Service {
					cache := storagestub.NewInMemoryServiceWithMarshaler(marshaler)
					addr, teardown := runDatastoreServer(t, cache, marshaler)
					teardowns = append(teardowns, teardown)

					store, err := storagerpc.NewClient(addr, marshaler,
						grpc.WithTransportCredentials(insecure.NewCredentials()))
					require.NoError(t, err)

					storePart, err := store.Partition("suite")
					require.NoError(t, err)
					cachePart, err := cache.Partition("suite")
					require.NoError(t, err)

					return interopHelper{read: bluestore.AdaptFrom(cachePart), write: bluestore.AdaptFrom(storePart)}
				})
			})
		})
	}
}

func TestRPCPartitionIsolation(t *testing.T) {
	cache := storagestub.NewInMemoryServiceWithMarshaler(doctoml.Marshaler())
	addr, teardown := runDatastoreServer(t, cache, doctoml.Marshaler())
	defer teardown()

	store, err := storagerpc.NewClient(addr, doctoml.Marshaler(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer store.Close()

	partA, err := store.Partition("a")
	require.NoError(t, err)
	partB, err := store.Partition("b")
	require.NoError(t, err)

	require.NoError(t, partA.Set(context.Background(), "doc", map[string]any{"name": "A"}))
	require.NoError(t, partB.Set(context.Background(), "doc", map[string]any{"name": "B"}))

	var gotA map[string]any
	require.NoError(t, partA.Get(context.Background(), "doc", &gotA))
	assert.Equal(t, "A", gotA["name"])

	var gotB map[string]any
	require.NoError(t, partB.Get(context.Background(), "doc", &gotB))
	assert.Equal(t, "B", gotB["name"])

	itA, err := partA.List(context.Background(), nil)
	require.NoError(t, err)
	defer itA.Close()
	require.True(t, itA.HasNext())
	var listedA map[string]any
	require.NoError(t, itA.NextTo(&listedA))
	assert.Equal(t, "A", listedA["name"])
	assert.False(t, itA.HasNext())

	itB, err := partB.List(context.Background(), nil)
	require.NoError(t, err)
	defer itB.Close()
	require.True(t, itB.HasNext())
	var listedB map[string]any
	require.NoError(t, itB.NextTo(&listedB))
	assert.Equal(t, "B", listedB["name"])
	assert.False(t, itB.HasNext())
}

func TestRPCCachesAndClosesRequestPartitions(t *testing.T) {
	base := &partitionCloseCountingService{Service: storagestub.NewInMemoryServiceWithMarshaler(doctoml.Marshaler())}
	addr, teardown := runDatastoreServer(t, base, doctoml.Marshaler())

	store, err := storagerpc.NewClient(addr, doctoml.Marshaler(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	part, err := store.Partition("suite")
	require.NoError(t, err)
	nested, err := part.Partition("nested")
	require.NoError(t, err)

	require.NoError(t, nested.Create(context.Background(), "doc", map[string]any{"name": "Ada"}))
	require.Equal(t, int32(2), base.partitionCount.Load())
	require.Equal(t, int32(0), base.partitionCloseCount.Load())

	require.NoError(t, nested.Set(context.Background(), "doc", map[string]any{"name": "Grace"}))
	require.Equal(t, int32(2), base.partitionCount.Load())
	require.Equal(t, int32(0), base.partitionCloseCount.Load())

	require.NoError(t, nested.Update(context.Background(), "doc", []storageapi.Update{
		{FieldPath: []string{"name"}, Value: "Katherine"},
	}))
	require.Equal(t, int32(2), base.partitionCount.Load())
	require.Equal(t, int32(0), base.partitionCloseCount.Load())

	var got map[string]any
	require.NoError(t, nested.Get(context.Background(), "doc", &got))
	require.Equal(t, int32(2), base.partitionCount.Load())
	require.Equal(t, int32(0), base.partitionCloseCount.Load())

	it, err := nested.List(context.Background(), nil)
	require.NoError(t, err)
	require.True(t, it.HasNext())
	require.NoError(t, it.NextTo(&got))
	require.NoError(t, it.Close())
	require.Equal(t, int32(2), base.partitionCount.Load())
	require.Equal(t, int32(0), base.partitionCloseCount.Load())

	require.NoError(t, nested.Delete(context.Background(), "doc"))
	require.Equal(t, int32(2), base.partitionCount.Load())
	require.Equal(t, int32(0), base.partitionCloseCount.Load())

	require.NoError(t, store.Close())
	teardown()
	require.Equal(t, int32(2), base.partitionCloseCount.Load())
}

func TestServerPartitionCacheKeyDistinguishesEmbeddedSeparators(t *testing.T) {
	base := &partitionCloseCountingService{Service: storagestub.NewInMemoryServiceWithMarshaler(doctoml.Marshaler())}
	srv := NewServer(base, doctoml.Marshaler())

	ctxA := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		storagerpc.PartitionMetadataKey, "a\x00b",
		storagerpc.PartitionMetadataKey, "c",
	))
	ctxB := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		storagerpc.PartitionMetadataKey, "a",
		storagerpc.PartitionMetadataKey, "b\x00c",
	))

	svcA, err := srv.serviceForContext(ctxA)
	require.NoError(t, err)
	svcB, err := srv.serviceForContext(ctxB)
	require.NoError(t, err)
	require.NotSame(t, svcA, svcB)
	require.Equal(t, int32(4), base.partitionCount.Load())

	svcAAgain, err := srv.serviceForContext(ctxA)
	require.NoError(t, err)
	require.Same(t, svcA, svcAAgain)
	require.Equal(t, int32(4), base.partitionCount.Load())

	require.NoError(t, srv.Close())
	require.Equal(t, int32(4), base.partitionCloseCount.Load())
}

type partitionCloseCountingService struct {
	storageapi.Service
	partitionCount      atomic.Int32
	partitionCloseCount atomic.Int32
}

func (s *partitionCloseCountingService) Partition(name string) (storageapi.Service, error) {
	s.partitionCount.Add(1)
	partitioned, err := s.Service.Partition(name)
	if err != nil {
		return nil, err
	}
	return &countedPartitionService{Service: partitioned, parent: s}, nil
}

type countedPartitionService struct {
	storageapi.Service
	parent *partitionCloseCountingService
}

func (s *countedPartitionService) Partition(name string) (storageapi.Service, error) {
	s.parent.partitionCount.Add(1)
	partitioned, err := s.Service.Partition(name)
	if err != nil {
		return nil, err
	}
	return &countedPartitionService{Service: partitioned, parent: s.parent}, nil
}

func (s *countedPartitionService) Close() error {
	s.parent.partitionCloseCount.Add(1)
	return s.Service.Close()
}

// oldMaxMessageSize is firstmover's historical gRPC frame cap. Before the
// doc-carrying RPCs were converted to chunked streaming, a document whose
// marshaled size exceeded this limit failed to load with
// codes.ResourceExhausted. The tests below configure both ends with this cap
// and round-trip a document well beyond it to prove the cap is no longer a
// correctness boundary.
const oldMaxMessageSize = 4 << 20

// runDatastoreServerCapped starts a server whose gRPC send/recv frames are
// bounded by oldMaxMessageSize, mirroring firstmover's leader configuration.
func runDatastoreServerCapped(
	t *testing.T, other storageapi.Service, marshaler docmarshal.Marshaler,
) (net.Addr, func()) {
	return runDatastoreServerOverListener(t, other, tcpListener, marshaler,
		docpb.RegisterDocumentStoreServer,
		grpc.MaxSendMsgSize(oldMaxMessageSize),
		grpc.MaxRecvMsgSize(oldMaxMessageSize),
	)
}

// cappedClient dials addr with frames bounded by oldMaxMessageSize, mirroring
// firstmover's follower configuration.
func cappedClient(
	t *testing.T, addr net.Addr, marshaler docmarshal.Marshaler,
) storageapi.Service {
	store, err := storagerpc.NewClient(addr, marshaler,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(
			grpc.MaxCallSendMsgSize(oldMaxMessageSize),
			grpc.MaxCallRecvMsgSize(oldMaxMessageSize),
		),
	)
	require.NoError(t, err)
	return store
}

func TestStreamingRoundTripExceedsMessageCap(t *testing.T) {
	for name, marshaler := range map[string]docmarshal.Marshaler{
		"bson": docbson.Marshaler(),
		"toml": doctoml.Marshaler(),
		"json": docjson.Marshaler(),
	} {
		t.Run(name, func(t *testing.T) {
			cache := document.NewInMemoryServiceWithMarshaler(marshaler)
			addr, teardown := runDatastoreServerCapped(t, bluestore.AdaptTo(cache), marshaler)
			defer teardown()

			store := cappedClient(t, addr, marshaler)
			defer store.Close()

			// ~10 MB payload, comfortably over the 4 MiB cap and matching the
			// magnitude in the original ResourceExhausted report.
			big := strings.Repeat("rune-245-", 10*1024*1024/len("rune-245-"))
			require.Greater(t, len(big), oldMaxMessageSize)

			ctx := context.Background()

			require.NoError(t, store.Set(ctx, "doc", map[string]any{"blob": big}))
			var afterSet map[string]any
			require.NoError(t, store.Get(ctx, "doc", &afterSet))
			assert.Equal(t, big, afterSet["blob"])

			require.NoError(t, store.Create(ctx, "doc2", map[string]any{"blob": big}))
			var afterCreate map[string]any
			require.NoError(t, store.Get(ctx, "doc2", &afterCreate))
			assert.Equal(t, big, afterCreate["blob"])

			require.NoError(t, store.Update(ctx, "doc2", []storageapi.Update{
				{FieldPath: []string{"blob"}, Value: big + "-updated"},
			}))
			var afterUpdate map[string]any
			require.NoError(t, store.Get(ctx, "doc2", &afterUpdate))
			assert.Equal(t, big+"-updated", afterUpdate["blob"])
		})
	}
}

func TestStreamingGetBinaryFidelity(t *testing.T) {
	marshaler := docbson.Marshaler()
	cache := document.NewInMemoryServiceWithMarshaler(marshaler)
	addr, teardown := runDatastoreServerCapped(t, bluestore.AdaptTo(cache), marshaler)
	defer teardown()

	store := cappedClient(t, addr, marshaler)
	defer store.Close()

	// A multi-chunk binary blob exercises chunk reassembly on both ends and
	// asserts the bytes survive byte-for-byte across the streamed frames.
	blob := make([]byte, 9_970_883)
	for i := range blob {
		blob[i] = byte(i * 31)
	}

	ctx := context.Background()
	require.NoError(t, store.Set(ctx, "doc", map[string]any{"blob": blob}))

	var got map[string]any
	require.NoError(t, store.Get(ctx, "doc", &got))
	gotBlob, ok := got["blob"].([]byte)
	require.True(t, ok, "blob should decode as []byte, got %T", got["blob"])
	assert.True(t, bytes.Equal(blob, gotBlob))
}

// noSyncSpyService records whether each write arrived with the bluebolt
// no-sync request restored on its context.
type noSyncSpyService struct {
	storageapi.Service
	noSync map[string]bool
}

func (s *noSyncSpyService) Create(ctx context.Context, ID string, doc any) error {
	s.noSync["create"] = bluebolt.NoSyncRequested(ctx)
	return s.Service.Create(ctx, ID, doc)
}

func (s *noSyncSpyService) Set(ctx context.Context, ID string, doc any) error {
	s.noSync["set"] = bluebolt.NoSyncRequested(ctx)
	return s.Service.Set(ctx, ID, doc)
}

func (s *noSyncSpyService) Update(
	ctx context.Context, ID string,
	updates []storageapi.Update, preconds ...storageapi.Precondition,
) error {
	s.noSync["update"] = bluebolt.NoSyncRequested(ctx)
	return s.Service.Update(ctx, ID, updates, preconds...)
}

func (s *noSyncSpyService) Delete(ctx context.Context, ID string) error {
	s.noSync["delete"] = bluebolt.NoSyncRequested(ctx)
	return s.Service.Delete(ctx, ID)
}

func TestNoSyncRequestPropagatesOverRPC(t *testing.T) {
	marshaler := doctoml.Marshaler()
	spy := &noSyncSpyService{
		Service: storagestub.NewInMemoryServiceWithMarshaler(marshaler),
		noSync:  make(map[string]bool),
	}
	addr, teardown := runDatastoreServer(t, spy, marshaler)
	defer teardown()

	opts := append(NoSyncDialOptions(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	store, err := storagerpc.NewClient(addr, marshaler, opts...)
	require.NoError(t, err)
	defer store.Close()

	writeAll := func(ctx context.Context, id string) {
		t.Helper()
		require.NoError(t, store.Create(ctx, id, map[string]any{"name": "Ada"}))
		require.NoError(t, store.Set(ctx, id, map[string]any{"name": "Grace"}))
		require.NoError(t, store.Update(ctx, id, []storageapi.Update{
			{FieldPath: []string{"name"}, Value: "Katherine"},
		}))
		require.NoError(t, store.Delete(ctx, id))
	}

	writeAll(context.Background(), "doc-plain")
	for op, noSync := range spy.noSync {
		assert.False(t, noSync, "op %s should not carry no-sync", op)
	}

	writeAll(bluebolt.ContextWithNoSync(context.Background()), "doc-nosync")
	for _, op := range []string{"create", "set", "update", "delete"} {
		assert.True(t, spy.noSync[op], "op %s should carry no-sync", op)
	}
}
