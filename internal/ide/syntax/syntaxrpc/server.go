// Copyright (C) 2017-2026 The Rune Authors
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

package syntaxrpc

import (
	"context"
	"errors"
	"fmt"

	"github.com/unstablebuild/blue/bluectx"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi/syntaxrpc"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term/termrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server adapts a syntaxapi.Parser to the generated SyntaxServer interface.
//
// gRPC dispatches each request on grpc-go's goroutine pool without the host
// holding the event-loop locker, so the wrapped Parser must be safe for
// concurrent use.
type Server struct {
	syntaxrpc.UnimplementedSyntaxServer
	ctx       context.Context
	cancelCtx func()
	parser    syntaxapi.Parser
}

// NewServer returns a new Server that delegates to s. s must be safe for
// concurrent use; see the Server doc comment.
func NewServer(s syntaxapi.Parser) *Server {
	ctx, cancelCtx := context.WithCancel(context.Background())
	return &Server{parser: s, ctx: ctx, cancelCtx: cancelCtx}
}

// RegisterServer registers a syntaxapi.Parser as a gRPC service.
func RegisterServer(registrar grpc.ServiceRegistrar, s syntaxapi.Parser) {
	syntaxrpc.RegisterSyntaxServer(registrar, NewServer(s))
}

// Search satisfies SyntaxServer.
func (s *Server) Search(
	req *syntaxrpc.SearchRequest, stream grpc.ServerStreamingServer[syntaxrpc.SearchResponse],
) error {
	langs := req.GetLanguages()
	var it iterator.Iterator[syntaxapi.Result]
	var err error
	if langs == nil {
		it, err = s.parser.Search(req.GetQuery(), req.GetCaptureNames())
	} else {
		it, err = s.parser.Search(req.GetQuery(), req.GetCaptureNames(), req.GetLanguages()...)
	}
	if err != nil {
		return fmt.Errorf("syntax search: %w", err)
	}
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()
	return streamResults(ctx, stream, it)
}

// SearchNode implements SyntaxServer.
func (s *Server) SearchNode(
	req *syntaxrpc.SearchNodeRequest,
	stream grpc.ServerStreamingServer[syntaxrpc.SearchResponse],
) error {
	it, err := s.parser.SearchNode(syntaxapi.NodeCaptureName(req.GetNodeTypes()))
	if err != nil {
		return err
	}
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()
	return streamResults(ctx, stream, it)
}

// Query implements SyntaxServer.
func (s *Server) Query(
	req *syntaxrpc.QueryRequest,
	stream grpc.ServerStreamingServer[syntaxrpc.SearchResponse],
) error {
	uri, err := workspaceapi.ParseURI(req.GetUri())
	if err != nil {
		return err
	}
	it, err := s.parser.Query(uri, req.GetQuery(), req.GetCaptureNames())
	if err != nil {
		return err
	}
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()
	return streamResults(ctx, stream, it)
}

// QueryNode implements SyntaxServer.
func (s *Server) QueryNode(
	req *syntaxrpc.QueryNodeRequest,
	stream grpc.ServerStreamingServer[syntaxrpc.SearchResponse],
) error {
	uri, err := workspaceapi.ParseURI(req.GetUri())
	if err != nil {
		return err
	}
	it, err := s.parser.QueryNode(uri, syntaxapi.NodeCaptureName(req.GetNodeTypes()))
	if err != nil {
		return err
	}
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()
	return streamResults(ctx, stream, it)
}

// Highlight implements SyntaxServer.
func (s *Server) Highlight(
	req *syntaxrpc.HighlightRequest,
	stream grpc.ServerStreamingServer[syntaxrpc.HighlightResponse],
) error {
	uri, err := workspaceapi.ParseURI(req.GetUri())
	if err != nil {
		return err
	}
	it, err := s.parser.Highlight(uri, req.GetContent())
	if err != nil {
		return err
	}
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()
	defer func() { _ = it.Close() }()
	for {
		loc, ok := it.Next(ctx)
		if !ok {
			return it.Err()
		}
		var from, to termrpc.Coordinates
		from.FromModel(loc.From)
		to.FromModel(loc.To)
		var attr termrpc.Attributes
		attr.FromModel(loc.Attr)
		resp := syntaxrpc.HighlightResponse{
			From:    &from,
			To:      &to,
			Attr:    &attr,
			Message: loc.Message,
		}
		if err := stream.Send(&resp); err != nil {
			return err
		}
	}
}

// errNoDotDetail is the gRPC status message the client uses to reconstruct
// syntaxapi.ErrNoDot. It must match the constant in the SDK syntaxrpc client.
const errNoDotDetail = "syntaxapi.ErrNoDot"

// ResolveSymbol implements SyntaxServer.
func (s *Server) ResolveSymbol(
	req *syntaxrpc.ResolveSymbolRequest,
	stream grpc.ServerStreamingServer[syntaxrpc.ResolveSymbolResponse],
) error {
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()

	progress := syntaxapi.ProgressFunc(func(msg string, found int, step, total int64) {
		_ = stream.Send(&syntaxrpc.ResolveSymbolResponse{
			Payload: &syntaxrpc.ResolveSymbolResponse_Progress{
				Progress: &syntaxrpc.ResolveSymbolProgress{
					Message: msg,
					Found:   int64(found),
					Step:    step,
					Total:   total,
				},
			},
		})
	})

	it, err := s.parser.ResolveSymbol(ctx, req.GetName(), progress)
	if err != nil {
		if errors.Is(err, syntaxapi.ErrNoDot) {
			return status.Error(codes.InvalidArgument, errNoDotDetail)
		}
		return fmt.Errorf("syntax resolve symbol: %w", err)
	}
	defer it.Close() //nolint:errcheck

	for {
		m, ok := it.Next(ctx)
		if !ok {
			break
		}
		resp := &syntaxrpc.ResolveSymbolResponse{
			Payload: &syntaxrpc.ResolveSymbolResponse_Match{
				Match: &syntaxrpc.ResolveSymbolMatch{
					Uri:        m.URI,
					Line:       uint32(m.Pos.Y),
					Character:  uint32(m.Pos.X),
					Display:    m.Display,
					ImportPath: m.ImportPath,
				},
			},
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	if err := it.Err(); err != nil {
		if errors.Is(err, syntaxapi.ErrNoDot) {
			return status.Error(codes.InvalidArgument, errNoDotDetail)
		}
		return err
	}
	return nil
}

// ListReferencedSymbols implements SyntaxServer.
func (s *Server) ListReferencedSymbols(
	_ *syntaxrpc.ListReferencedSymbolsRequest,
	stream grpc.ServerStreamingServer[syntaxrpc.ListReferencedSymbolsResponse],
) error {
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()

	it, err := s.parser.ListReferencedSymbols(ctx)
	if err != nil {
		return fmt.Errorf("syntax list referenced symbols: %w", err)
	}
	defer it.Close() //nolint:errcheck

	for {
		name, ok := it.Next(ctx)
		if !ok {
			break
		}
		if err := stream.Send(&syntaxrpc.ListReferencedSymbolsResponse{Name: name}); err != nil {
			return err
		}
	}
	return it.Err()
}

func streamResults(
	ctx context.Context,
	stream grpc.ServerStreamingServer[syntaxrpc.SearchResponse],
	it iterator.Iterator[syntaxapi.Result],
) error {
	defer it.Close() //nolint:errcheck
	for {
		result, ok := it.Next(ctx)
		if !ok {
			break
		}
		var from, to termrpc.Coordinates
		from.FromModel(result.From)
		to.FromModel(result.To)
		resp := &syntaxrpc.SearchResponse{
			Uri:         result.File.String(),
			Text:        result.Text,
			From:        &from,
			To:          &to,
			CaptureName: result.CaptureName,
		}
		if err := stream.Send(resp); err != nil {
			return err
		}
	}
	return it.Err()
}

// Close cancels all ongoing queries.
func (s *Server) Close() error {
	s.cancelCtx()
	return nil
}
