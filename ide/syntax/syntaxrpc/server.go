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
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi/syntaxrpc"
	"github.com/unstablebuild/rune-go-sdk/term/termrpc"
	"google.golang.org/grpc"
)

// Server adapts a syntaxapi.Searcher to the generated SyntaxServer interface.
type Server struct {
	syntaxrpc.UnimplementedSyntaxServer
	searcher syntaxapi.Searcher
}

// NewServer returns a new Server that delegates to s.
func NewServer(s syntaxapi.Searcher) *Server {
	return &Server{searcher: s}
}

// RegisterServer registers a syntaxapi.Searcher as a gRPC service.
func RegisterServer(registrar grpc.ServiceRegistrar, s syntaxapi.Searcher) {
	syntaxrpc.RegisterSyntaxServer(registrar, NewServer(s))
}

// Search satisfies SyntaxServer.
func (s *Server) Search(
	req *syntaxrpc.SearchRequest, stream grpc.ServerStreamingServer[syntaxrpc.SearchResponse],
) error {
	ctx := stream.Context()
	iter, err := s.searcher.Search(req.GetQuery(), req.GetCaptureNames())
	if err != nil {
		return fmt.Errorf("syntax search: %w", err)
	}
	defer iter.Close()

	for {
		result, ok := iter.Next(ctx)
		if !ok {
			break
		}

		var pos termrpc.Coordinates
		pos.FromModel(result.Position)

		resp := syntaxrpc.SearchResponse{
			Uri:      result.File.String(),
			Text:     result.Text,
			Position: &pos,
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
