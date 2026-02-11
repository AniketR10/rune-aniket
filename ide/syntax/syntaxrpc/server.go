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

package syntaxrpc

import (
	"context"
	"fmt"

	"github.com/unstablebuild/blue/bluectx"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi/syntaxrpc"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
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
	it, err := s.searcher.Search(req.GetQuery(), req.GetCaptureNames())
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
	it, err := s.searcher.SearchNode(syntaxapi.NodeCaptureName(req.GetNodeTypes()))
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
	it, err := s.searcher.Query(uri, req.GetQuery(), req.GetCaptureNames())
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
	it, err := s.searcher.QueryNode(uri, syntaxapi.NodeCaptureName(req.GetNodeTypes()))
	if err != nil {
		return err
	}
	ctx, cancel := bluectx.First(stream.Context(), s.ctx)
	defer cancel()
	return streamResults(ctx, stream, it)
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
