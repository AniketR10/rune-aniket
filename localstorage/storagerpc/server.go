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
type Server struct {
	marshaler docmarshal.Marshaler
	other     storageapi.Service
	mu        sync.Mutex
	cache     map[string]*cachedPartition
	// locker serializes calls into the wrapped storageapi.Service with the
	// rest of the host (typically the event-loop mutex). Different RPCs
	// can otherwise race on the underlying service's internal fields.
	locker sync.Locker
	docpb.UnimplementedDocumentStoreServer
}

type cachedPartition struct {
	svc     storageapi.Service
	created []storageapi.Service
}

// NewServer allocates storage for a new Server and initializes it. locker
// serializes the wrapped storageapi.Service with the host event loop; it
// must not be nil.
func NewServer(other storageapi.Service, m docmarshal.Marshaler, locker sync.Locker) *Server {
	ret := new(Server)
	ret.Init(other, m, locker)
	return ret
}

// Init initializes this server with the given underlying storageapi.Service,
// marshaler, and host locker.
func (s *Server) Init(other storageapi.Service, m docmarshal.Marshaler, locker sync.Locker) {
	if locker == nil {
		panic("storagerpc: Init: locker must not be nil")
	}
	s.other = other
	s.marshaler = m
	s.cache = make(map[string]*cachedPartition)
	s.locker = locker
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

// Create satisfies proto.DocumentStoreServer
func (s *Server) Create(
	ctx context.Context, req *docpb.CreateDocumentRequest,
) (res *docpb.CreateDocumentResponse, err error) {
	id := req.GetId()
	data := req.GetData()

	var pr map[string]any
	err = storageapi.SafeDecode(s.marshaler, &pr, data)
	if err != nil {
		return
	}

	s.locker.Lock()
	defer s.locker.Unlock()
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return nil, err
	}
	err = svc.Create(ctx, id, pr)
	if err != nil {
		if errors.Is(err, storageapi.ErrAlreadyExists) {
			err = nil
			res = &docpb.CreateDocumentResponse{
				AlreadyExists: true,
			}
		}
		if errors.Is(err, storageapi.ErrPermissionDenied) {
			err = status.Error(codes.PermissionDenied, "")
		}
		return
	}

	res = &docpb.CreateDocumentResponse{}
	return
}

// Set satisfies proto.DocumentStoreServer
func (s *Server) Set(
	ctx context.Context, req *docpb.SetDocumentRequest,
) (res *docpb.DocumentResponse, err error) {
	id := req.GetId()
	data := req.GetData()

	var pr map[string]any
	err = storageapi.SafeDecode(s.marshaler, &pr, data)
	if err != nil {
		return
	}

	s.locker.Lock()
	defer s.locker.Unlock()
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return nil, err
	}
	err = svc.Set(ctx, id, &pr)
	if errors.Is(err, storageapi.ErrPermissionDenied) {
		err = status.Error(codes.PermissionDenied, "")
	}
	res = new(docpb.DocumentResponse)
	return
}

// Update satisfies proto.DocumentStoreServer
func (s *Server) Update(
	ctx context.Context, req *docpb.UpdateDocumentRequest,
) (res *docpb.UpdateDocumentResponse, err error) {
	updates, err := makeModelUpdates(s.marshaler, req.GetUpdates())
	if err != nil {
		return nil, err
	}
	preconds, err := makeModelPreconds(s.marshaler, req.GetPreconditions())
	if err != nil {
		return nil, err
	}
	// client should panic if no updates are passed
	// so the following is to avoid potential DOS from a malicious client.
	if len(updates) == 0 {
		err = errors.New("invalid request: no paths to update")
		return nil, err
	}
	s.locker.Lock()
	defer s.locker.Unlock()
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return nil, err
	}
	err = svc.Update(ctx, req.GetId(), updates, preconds...)
	if err != nil {
		switch err {
		case storageapi.ErrNotFound:
			return &docpb.UpdateDocumentResponse{NotFound: true}, nil
		case storageapi.ErrPreconditionFailed:
			return &docpb.UpdateDocumentResponse{PreconditionFailed: true}, nil
		case storageapi.ErrPermissionDenied:
			return nil, status.Error(codes.PermissionDenied, "")
		}
		return nil, err
	}
	return new(docpb.UpdateDocumentResponse), nil
}

// Get satisfies proto.DocumentStoreServer
func (s *Server) Get(
	ctx context.Context, req *docpb.GetDocumentRequest,
) (res *docpb.GetDocumentResponse, err error) {
	id := req.GetId()

	var pr map[string]any
	s.locker.Lock()
	defer s.locker.Unlock()
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		return nil, err
	}
	err = svc.Get(ctx, id, &pr)
	if err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			res = &docpb.GetDocumentResponse{NotFound: true}
			err = nil
		}
		if errors.Is(err, storageapi.ErrPermissionDenied) {
			err = status.Error(codes.PermissionDenied, "")
		}
		return
	}

	res = &docpb.GetDocumentResponse{
		Data: storageapi.Encode(s.marshaler, pr, false),
	}
	return
}

// Delete satisfies proto.DocumentStoreServer
func (s *Server) Delete(
	ctx context.Context, req *docpb.DeleteDocumentRequest,
) (res *docpb.DocumentResponse, err error) {
	id := req.GetId()

	s.locker.Lock()
	defer s.locker.Unlock()
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

func (s *Server) streamList(list docpb.DocumentStore_ListServer, it storageapi.Iterator, fields []string) (err error) {
	var fieldSet map[string]struct{}
	if len(fields) > 0 {
		fieldSet = make(map[string]struct{}, len(fields))
		for _, field := range fields {
			fieldSet[field] = struct{}{}
		}
	}

	for {
		s.locker.Lock()
		hasNext := it.HasNext()
		if !hasNext {
			s.locker.Unlock()
			return nil
		}
		var pr map[string]any
		err = it.NextTo(&pr)
		s.locker.Unlock()
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
	s.locker.Lock()
	svc, err := s.serviceForContext(ctx)
	if err != nil {
		s.locker.Unlock()
		return err
	}
	it, err := svc.List(ctx, filters)
	s.locker.Unlock()
	if err != nil {
		if errors.Is(err, storageapi.ErrPermissionDenied) {
			err = status.Error(codes.PermissionDenied, "")
		}
		return err
	}
	defer func() {
		s.locker.Lock()
		closeErr := it.Close()
		s.locker.Unlock()
		err = errors.Join(err, closeErr)
	}()

	return s.streamList(list, it, req.GetFields())
}
