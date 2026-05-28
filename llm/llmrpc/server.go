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

// Package llmrpc implements the host-side gRPC server for llmapi.Service.
package llmrpc

import (
	"context"
	"errors"
	"sync"

	"github.com/unstablebuild/blue/bluectx"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi/llmrpc"
	"google.golang.org/grpc"
)

// Server adapts an llmapi.Service to the generated LLMServer interface.
//
// The host event loop drives every UI handler on a single goroutine
// guarded by a shared locker; gRPC handlers come in on grpc-go's own
// goroutine pool. Server takes the locker around every llmapi.Service
// call so the router and its provider clients (notably the local
// llama.cpp service cache) stay single-threaded with the rest of the
// host. The lock is released across the per-event stream drain so a
// long-running completion does not freeze the UI.
type Server struct {
	llmrpc.UnimplementedLLMServer
	ctx       context.Context
	cancelCtx func()
	svc       llmapi.Service
	locker    sync.Locker
}

// NewServer returns a new Server that delegates to svc. locker is
// applied around every llmapi.Service call to serialize the router
// with the host event loop; it must not be nil.
func NewServer(svc llmapi.Service, locker sync.Locker) *Server {
	if locker == nil {
		panic("llmrpc: NewServer: locker must not be nil")
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{svc: svc, ctx: ctx, cancelCtx: cancel, locker: locker}
}

// RegisterServer registers an llmapi.Service as a gRPC service. locker
// follows the same contract as NewServer.
func RegisterServer(registrar grpc.ServiceRegistrar, svc llmapi.Service, locker sync.Locker) {
	llmrpc.RegisterLLMServer(registrar, NewServer(svc, locker))
}

// Close cancels in-flight streams.
func (s *Server) Close() error {
	s.cancelCtx()
	return nil
}

// CreateCompletion satisfies LLMServer.
func (s *Server) CreateCompletion(
	req *llmrpc.CreateCompletionRequest,
	stream grpc.ServerStreamingServer[llmrpc.CreateCompletionResponse],
) error {
	apiReq, err := llmrpc.FromProtoRequest(req.GetRequest())
	if err != nil {
		return err
	}
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()
	model := llmrpc.FromProtoModelEntry(req.GetModel())
	s.locker.Lock()
	it, err := s.svc.CreateCompletion(ctx, model, apiReq)
	s.locker.Unlock()
	if err != nil {
		var cwErr *llmapi.ErrContextWindowExceeded
		if errors.As(err, &cwErr) {
			return llmrpc.ContextWindowExceededStatus(cwErr)
		}
		return err
	}
	defer func() {
		s.locker.Lock()
		_ = it.Close()
		s.locker.Unlock()
	}()
	for {
		s.locker.Lock()
		ev, ok := it.Next(ctx)
		s.locker.Unlock()
		if !ok {
			if iErr := it.Err(); iErr != nil {
				var cwErr *llmapi.ErrContextWindowExceeded
				if errors.As(iErr, &cwErr) {
					return llmrpc.ContextWindowExceededStatus(cwErr)
				}
				return iErr
			}
			return nil
		}
		if err := stream.Send(&llmrpc.CreateCompletionResponse{
			Event: llmrpc.ToProtoEvent(ev),
		}); err != nil {
			return err
		}
	}
}

// CountTokens satisfies LLMServer.
func (s *Server) CountTokens(
	_ context.Context, req *llmrpc.CountTokensRequest,
) (*llmrpc.CountTokensResponse, error) {
	msgs := make([]llmapi.Message, 0, len(req.GetMessages()))
	for _, m := range req.GetMessages() {
		apiReq, err := llmrpc.FromProtoRequest(&llmrpc.Request{Messages: []*llmrpc.Message{m}})
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, apiReq.Messages...)
	}
	model := llmrpc.FromProtoModelEntry(req.GetModel())
	s.locker.Lock()
	count, err := s.svc.CountTokens(model, msgs)
	s.locker.Unlock()
	if err != nil {
		return nil, err
	}
	return &llmrpc.CountTokensResponse{Count: int32(count)}, nil
}

// Models satisfies LLMServer.
func (s *Server) Models(
	_ *llmrpc.ModelsRequest,
	stream grpc.ServerStreamingServer[llmrpc.ModelsResponse],
) error {
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()
	s.locker.Lock()
	it := s.svc.Models()
	s.locker.Unlock()
	defer func() {
		s.locker.Lock()
		_ = it.Close()
		s.locker.Unlock()
	}()
	for {
		s.locker.Lock()
		entry, ok := it.Next(ctx)
		s.locker.Unlock()
		if !ok {
			return it.Err()
		}
		if err := stream.Send(&llmrpc.ModelsResponse{
			Model: llmrpc.ToProtoModelEntry(entry),
		}); err != nil {
			return err
		}
	}
}

// GetModel satisfies LLMServer.
func (s *Server) GetModel(
	ctx context.Context, req *llmrpc.GetModelRequest,
) (*llmrpc.GetModelResponse, error) {
	model := llmrpc.FromProtoModelEntry(req.GetModel())
	s.locker.Lock()
	entry, ok := s.svc.GetModel(ctx, model)
	s.locker.Unlock()
	if !ok {
		return &llmrpc.GetModelResponse{Found: false}, nil
	}
	return &llmrpc.GetModelResponse{
		Model: llmrpc.ToProtoModelEntry(entry),
		Found: true,
	}, nil
}
