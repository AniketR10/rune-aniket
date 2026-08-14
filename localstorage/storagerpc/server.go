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
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	sdkstoragerpc "github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagerpc/docpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server wraps another storageapi.Service and exposes it through a grpc interface.
//
// gRPC dispatches each request on grpc-go's goroutine pool without the host
// holding the event-loop locker, so the Server must be safe for concurrent
// use: it guards its partition cache with s.mu, and the wrapped service must
// itself be safe for concurrent use.
type Server struct {
	marshaler docmarshal.Marshaler
	other     storageapi.Service
	mu        sync.Mutex
	cache     map[string]*cachedPartition
	docpb.UnimplementedDocumentStoreServer
}

type cachedPartition struct {
	svc     storageapi.Service
	created []storageapi.Service
}

// maxChunkBytes bounds a single streamed data chunk. It mirrors the client's
// chunk size and stays below gRPC's default 4 MiB frame so the server can
// stream a document of any size back to the client in Get.
const maxChunkBytes = 1 << 20

// NewServer allocates storage for a new Server and initializes it. The
// supplied service must be safe for concurrent use; see the Server doc
// comment.
func NewServer(other storageapi.Service, m docmarshal.Marshaler) *Server {
	ret := new(Server)
	ret.Init(other, m)
	return ret
}

// Init initializes this server with the given underlying storageapi.Service
// and marshaler.
func (s *Server) Init(other storageapi.Service, m docmarshal.Marshaler) {
	s.other = other
	s.marshaler = m
	s.cache = make(map[string]*cachedPartition)
}

func (s *Server) serviceForContext(ctx context.Context) (storageapi.Service, error) {
	partitions := sdkstoragerpc.PartitionsFromIncomingContext(ctx)
	if len(partitions) == 0 {
		return s.other, nil
	}
	key := partitionCacheKey(partitions)

	s.mu.Lock()
	defer s.mu.Unlock()
	if cached := s.cache[key]; cached != nil {
		return cached.svc, nil
	}

	svc := s.other
	var created []storageapi.Service
	closeCreated := func() (err error) {
		for i := len(created) - 1; i >= 0; i-- {
			err = errors.Join(err, created[i].Close())
		}
		return err
	}

	for _, partition := range partitions {
		var err error
		svc, err = svc.Partition(partition)
		if err != nil {
			return nil, errors.Join(err, closeCreated())
		}
		created = append(created, svc)
	}
	s.cache[key] = &cachedPartition{svc: svc, created: created}
	return svc, nil
}

func partitionCacheKey(partitions []string) string {
	var b strings.Builder
	for _, partition := range partitions {
		b.WriteString(strconv.Itoa(len(partition)))
		b.WriteByte(':')
		b.WriteString(partition)
	}
	return b.String()
}

// Close closes cached partition services created by this server. It does not
// close the underlying service passed to Init; that service remains owned by
// the caller that created the server.
func (s *Server) Close() (err error) {
	s.mu.Lock()
	cache := s.cache
	s.cache = make(map[string]*cachedPartition)
	s.mu.Unlock()

	for _, cached := range cache {
		for i := len(cached.created) - 1; i >= 0; i-- {
			err = errors.Join(err, cached.created[i].Close())
		}
	}
	return err
}

// Create satisfies proto.DocumentStoreServer. The request is streamed: the id
// arrives on the first message and the document data is reassembled from the
// data chunk on every message.
func (s *Server) Create(stream docpb.DocumentStore_CreateServer) error {
	id, data, err := recvCreateStream(stream)
	if err != nil {
		return err
	}

	var pr map[string]any
	if err := storageapi.SafeDecode(s.marshaler, &pr, data); err != nil {
		return err
	}

	ctx := noSyncIncomingContext(stream.Context())
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return err
	}
	err = svc.Create(ctx, id, pr)
	if err != nil {
		if errors.Is(err, storageapi.ErrAlreadyExists) {
			return stream.SendAndClose(&docpb.CreateDocumentResponse{AlreadyExists: true})
		}
		if errors.Is(err, storageapi.ErrPermissionDenied) {
			return status.Error(codes.PermissionDenied, "")
		}
		return err
	}
	return stream.SendAndClose(&docpb.CreateDocumentResponse{})
}

// recvCreateStream reassembles the id and concatenated data from a Create
// stream. The id is read from the first message only.
func recvCreateStream(stream docpb.DocumentStore_CreateServer) (string, []byte, error) {
	var id string
	var data []byte
	first := true
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return id, data, nil
		}
		if err != nil {
			return "", nil, err
		}
		if first {
			id = req.GetId()
			first = false
		}
		data = append(data, req.GetData()...)
	}
}

// Set satisfies proto.DocumentStoreServer. See Create for the stream framing.
func (s *Server) Set(stream docpb.DocumentStore_SetServer) error {
	id, data, err := recvSetStream(stream)
	if err != nil {
		return err
	}

	var pr map[string]any
	if err := storageapi.SafeDecode(s.marshaler, &pr, data); err != nil {
		return err
	}

	ctx := noSyncIncomingContext(stream.Context())
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return err
	}
	err = svc.Set(ctx, id, &pr)
	if errors.Is(err, storageapi.ErrPermissionDenied) {
		return status.Error(codes.PermissionDenied, "")
	}
	if err != nil {
		return err
	}
	return stream.SendAndClose(new(docpb.DocumentResponse))
}

func recvSetStream(stream docpb.DocumentStore_SetServer) (string, []byte, error) {
	var id string
	var data []byte
	first := true
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return id, data, nil
		}
		if err != nil {
			return "", nil, err
		}
		if first {
			id = req.GetId()
			first = false
		}
		data = append(data, req.GetData()...)
	}
}

// Update satisfies proto.DocumentStoreServer. The request is streamed: the id
// arrives on the first message and the updates/preconditions are accumulated
// across messages, reassembling a field's data from its chunks by field path.
func (s *Server) Update(stream docpb.DocumentStore_UpdateServer) error {
	id, updateFields, precondFields, err := recvUpdateStream(stream)
	if err != nil {
		return err
	}
	updates, err := makeModelUpdates(s.marshaler, updateFields)
	if err != nil {
		return err
	}
	preconds, err := makeModelPreconds(s.marshaler, precondFields)
	if err != nil {
		return err
	}
	// client should panic if no updates are passed
	// so the following is to avoid potential DOS from a malicious client.
	if len(updates) == 0 {
		return errors.New("invalid request: no paths to update")
	}
	ctx := noSyncIncomingContext(stream.Context())
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return err
	}
	err = svc.Update(ctx, id, updates, preconds...)
	if err != nil {
		switch {
		case errors.Is(err, storageapi.ErrNotFound):
			return stream.SendAndClose(&docpb.UpdateDocumentResponse{NotFound: true})
		case errors.Is(err, storageapi.ErrPreconditionFailed):
			return stream.SendAndClose(&docpb.UpdateDocumentResponse{PreconditionFailed: true})
		case errors.Is(err, storageapi.ErrPermissionDenied):
			return status.Error(codes.PermissionDenied, "")
		}
		return err
	}
	return stream.SendAndClose(new(docpb.UpdateDocumentResponse))
}

// recvUpdateStream reassembles the id, updates and preconditions from an
// Update stream. The id is read from the first message; each field's data is
// concatenated across consecutive messages that share its field path.
func recvUpdateStream(stream docpb.DocumentStore_UpdateServer) (
	id string,
	updates, preconds []*docpb.UpdateDocumentRequest_Field,
	err error,
) {
	first := true
	for {
		req, recvErr := stream.Recv()
		if recvErr == io.EOF {
			return id, updates, preconds, nil
		}
		if recvErr != nil {
			return "", nil, nil, recvErr
		}
		if first {
			id = req.GetId()
			first = false
		}
		updates = appendFieldChunks(updates, req.GetUpdates())
		preconds = appendFieldChunks(preconds, req.GetPreconditions())
	}
}

// appendFieldChunks merges streamed field entries into dst. A field whose
// FieldPath is empty is a continuation chunk of the previous field's data;
// otherwise it starts a new field.
func appendFieldChunks(
	dst, fields []*docpb.UpdateDocumentRequest_Field,
) []*docpb.UpdateDocumentRequest_Field {
	for _, field := range fields {
		if len(field.GetFieldPath()) == 0 && len(dst) > 0 {
			last := dst[len(dst)-1]
			last.Data = append(last.Data, field.GetData()...)
			continue
		}
		dst = append(dst, &docpb.UpdateDocumentRequest_Field{
			FieldPath: field.GetFieldPath(),
			Data:      append([]byte(nil), field.GetData()...),
		})
	}
	return dst
}

// Get satisfies proto.DocumentStoreServer. The response is streamed: a single
// message carries not_found when the document is missing, otherwise the
// encoded document is split into data chunks across messages.
func (s *Server) Get(
	req *docpb.GetDocumentRequest, stream docpb.DocumentStore_GetServer,
) error {
	id := req.GetId()

	var pr map[string]any
	svc, err := s.serviceForContext(stream.Context())
	if err != nil {
		return err
	}
	err = svc.Get(stream.Context(), id, &pr)
	if err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			return stream.Send(&docpb.GetDocumentResponse{NotFound: true})
		}
		if errors.Is(err, storageapi.ErrPermissionDenied) {
			return status.Error(codes.PermissionDenied, "")
		}
		return err
	}

	data := storageapi.Encode(s.marshaler, pr, false)
	for {
		chunk := data
		if len(chunk) > maxChunkBytes {
			chunk = chunk[:maxChunkBytes]
		}
		if err := stream.Send(&docpb.GetDocumentResponse{Data: chunk}); err != nil {
			return err
		}
		data = data[len(chunk):]
		if len(data) == 0 {
			return nil
		}
	}
}

// Delete satisfies proto.DocumentStoreServer
func (s *Server) Delete(
	ctx context.Context, req *docpb.DeleteDocumentRequest,
) (res *docpb.DocumentResponse, err error) {
	id := req.GetId()

	ctx = noSyncIncomingContext(ctx)
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return nil, err
	}
	err = svc.Delete(ctx, id)
	if errors.Is(err, storageapi.ErrPermissionDenied) {
		err = status.Error(codes.PermissionDenied, "")
	}
	res = new(docpb.DocumentResponse)
	return
}

// Drop satisfies proto.DocumentStoreServer. It drops the partition addressed
// by the request metadata; sub-partitions are not dropped, since the service
// has no way to enumerate them.
func (s *Server) Drop(
	ctx context.Context, _ *docpb.DropRequest,
) (*docpb.DropResponse, error) {
	ctx = noSyncIncomingContext(ctx)
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return nil, err
	}
	droppable, ok := svc.(storageapi.DroppableService)
	if !ok {
		return nil, status.Error(codes.FailedPrecondition,
			"underlying service cannot be dropped")
	}
	if err := droppable.Drop(ctx); err != nil {
		if errors.Is(err, storageapi.ErrPermissionDenied) {
			return nil, status.Error(codes.PermissionDenied, "")
		}
		return nil, err
	}
	return new(docpb.DropResponse), nil
}

// Batch satisfies proto.DocumentStoreServer. The request is streamed with
// the same chunked framing as Create/Set/Update: an op entry marked as a
// continuation carries further chunks of the op that precedes it.
func (s *Server) Batch(stream docpb.DocumentStore_BatchServer) error {
	entries, err := recvBatchStream(stream)
	if err != nil {
		return err
	}
	ops, err := s.makeModelBatchOps(entries)
	if err != nil {
		return err
	}

	ctx := noSyncIncomingContext(stream.Context())
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return err
	}
	writer, ok := svc.(storageapi.BatchWriter)
	if !ok {
		return status.Error(codes.Unimplemented,
			"underlying service does not support batching")
	}
	results, err := writer.ApplyBatch(ctx, ops)
	if err != nil {
		if errors.Is(err, storageapi.ErrPermissionDenied) {
			return status.Error(codes.PermissionDenied, "")
		}
		return err
	}

	res := docpb.BatchResponse{
		Results: make([]*docpb.BatchResponse_OpResult, len(results)),
	}
	for i, result := range results {
		entry := new(docpb.BatchResponse_OpResult)
		switch {
		case errors.Is(result.Err, storageapi.ErrAlreadyExists):
			entry.AlreadyExists = true
		case errors.Is(result.Err, storageapi.ErrNotFound):
			entry.NotFound = true
		case errors.Is(result.Err, storageapi.ErrPreconditionFailed):
			entry.PreconditionFailed = true
		case result.Err != nil:
			return result.Err
		}
		res.Results[i] = entry
	}
	return stream.SendAndClose(&res)
}

// recvBatchStream reassembles the operations of a Batch stream, merging
// every continuation entry into the operation it continues.
func recvBatchStream(
	stream docpb.DocumentStore_BatchServer,
) ([]*docpb.BatchRequest_Op, error) {
	var ops []*docpb.BatchRequest_Op
	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return ops, nil
		}
		if err != nil {
			return nil, err
		}
		for _, entry := range req.GetOps() {
			if !entry.GetContinuation() {
				ops = append(ops, entry)
				continue
			}
			if len(ops) == 0 {
				return nil, errors.New(
					"invalid request: batch continuation without operation")
			}
			last := ops[len(ops)-1]
			last.Data = append(last.Data, entry.GetData()...)
			last.Updates = appendFieldChunks(last.Updates, entry.GetUpdates())
			last.Preconditions = appendFieldChunks(
				last.Preconditions, entry.GetPreconditions())
		}
	}
}

func (s *Server) makeModelBatchOps(
	entries []*docpb.BatchRequest_Op,
) ([]storageapi.BatchOp, error) {
	ops := make([]storageapi.BatchOp, len(entries))
	for i, entry := range entries {
		op := storageapi.BatchOp{ID: entry.GetId()}
		switch entry.GetType() {
		case docpb.BatchRequest_Create, docpb.BatchRequest_Set:
			op.Type = storageapi.BatchCreate
			if entry.GetType() == docpb.BatchRequest_Set {
				op.Type = storageapi.BatchSet
			}
			var doc map[string]any
			err := storageapi.SafeDecode(s.marshaler, &doc, entry.GetData())
			if err != nil {
				return nil, err
			}
			op.Doc = doc
		case docpb.BatchRequest_Update:
			op.Type = storageapi.BatchUpdate
			updates, err := makeModelUpdates(s.marshaler, entry.GetUpdates())
			if err != nil {
				return nil, err
			}
			// client should panic if no updates are passed, so the
			// following is to avoid potential DOS from a malicious client.
			if len(updates) == 0 {
				return nil, errors.New("invalid request: no paths to update")
			}
			preconds, err := makeModelPreconds(
				s.marshaler, entry.GetPreconditions())
			if err != nil {
				return nil, err
			}
			op.Updates = updates
			op.Preconditions = preconds
		case docpb.BatchRequest_Delete:
			op.Type = storageapi.BatchDelete
		default:
			return nil, errors.New("invalid request: unknown batch operation")
		}
		ops[i] = op
	}
	return ops, nil
}

func (s *Server) streamList(list docpb.DocumentStore_ListServer, it storageapi.Iterator, fields []string) (err error) {
	var fieldSet map[string]struct{}
	if len(fields) > 0 {
		fieldSet = make(map[string]struct{}, len(fields))
		for _, field := range fields {
			fieldSet[field] = struct{}{}
		}
	}

	for it.HasNext() {
		var pr map[string]any
		err = it.NextTo(&pr)
		res := docpb.ListDocumentResponse{}
		if err != nil {
			res.Error = err.Error()
		} else {
			if fieldSet != nil {
				for key := range pr {
					if _, ok := fieldSet[key]; !ok {
						delete(pr, key)
					}
				}
			}
			res.Data = storageapi.Encode(s.marshaler, pr, false)
		}
		err = list.SendMsg(&res)
		if err != nil {
			return
		}
		if res.Error != "" {
			return
		}
	}
	return
}

// List satisfies proto.DocumentStoreServer
func (s *Server) List(
	req *docpb.ListDocumentRequest, list docpb.DocumentStore_ListServer,
) (err error) {
	ctx := list.Context()
	filters, err := makeModelFilters(s.marshaler, req.GetFilters())
	if err != nil {
		return err
	}
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return err
	}
	it, err := svc.List(ctx, filters)
	if err != nil {
		if errors.Is(err, storageapi.ErrPermissionDenied) {
			err = status.Error(codes.PermissionDenied, "")
		}
		return err
	}
	defer func() {
		err = errors.Join(err, it.Close())
	}()

	return s.streamList(list, it, req.GetFields())
}
