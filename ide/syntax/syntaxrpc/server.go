// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

package syntaxrpc

import (
	"context"
	"fmt"

	"github.com/unstablebuild/blue/bluectx"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi/syntaxrpc"
	"github.com/unstablebuild/rune-go-sdk/term/termrpc"
	"google.golang.org/grpc"
)

// Server adapts a syntaxapi.Searcher to the generated SyntaxServer interface.
type Server struct {
	syntaxrpc.UnimplementedSyntaxServer
	ctx       context.Context
	cancelCtx func()
	searcher  syntaxapi.Searcher
}

// NewServer returns a new Server that delegates to s.
func NewServer(s syntaxapi.Searcher) *Server {
	ctx, cancelCtx := context.WithCancel(context.Background())
	return &Server{searcher: s, ctx: ctx, cancelCtx: cancelCtx}
}

// RegisterServer registers a syntaxapi.Searcher as a gRPC service.
func RegisterServer(registrar grpc.ServiceRegistrar, s syntaxapi.Searcher) {
	syntaxrpc.RegisterSyntaxServer(registrar, NewServer(s))
}

// Search satisfies SyntaxServer.
func (s *Server) Search(
	req *syntaxrpc.SearchRequest, stream grpc.ServerStreamingServer[syntaxrpc.SearchResponse],
) error {
	iter, err := s.searcher.Search(req.GetQuery(), req.GetCaptureNames())
	if err != nil {
		return fmt.Errorf("syntax search: %w", err)
	}
	defer iter.Close()
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()

	for {
		result, ok := iter.Next(ctx)
		if !ok {
			break
		}

		var from, to termrpc.Coordinates
		from.FromModel(result.From)
		to.FromModel(result.To)

		resp := syntaxrpc.SearchResponse{
			Uri:         result.File.String(),
			Text:        result.Text,
			From:        &from,
			To:          &to,
			CaptureName: result.CaptureName,
		}
		if err := stream.Send(&resp); err != nil {
			return fmt.Errorf("syntax search send: %w", err)
		}
	}
	if err := iter.Err(); err != nil {
		return fmt.Errorf("syntax search next: %w", err)
	}
	return nil
}

// Close cancels all ongoing queries.
func (s *Server) Close() error {
	s.cancelCtx()
	return nil
}
