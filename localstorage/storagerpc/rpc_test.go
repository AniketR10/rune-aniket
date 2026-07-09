// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
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

package storagerpc

import (
	"context"
	"bytes"
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
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc/docpb"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"github.com/unstablebuild/ox-api/bluestore"
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
