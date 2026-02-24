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
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi/syntaxrpc"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestServerClientIntegration(t *testing.T) {
	testURI, err := workspaceapi.ParseURI("file:///tmp/test.go")
	require.NoError(t, err)
	testURI2, err := workspaceapi.ParseURI("file:///tmp/other.go")
	require.NoError(t, err)

	tsuite := []struct {
		name   string
		setup  func(*mockParser)
		action func(t *testing.T, client *syntaxrpc.Client)
	}{
		{
			name: "Search returns results",
			setup: func(m *mockParser) {
				m.searchResults = []syntaxapi.Result{
					{
						File:        testURI,
						Text:        "func main()",
						From:        term.Coordinates{X: 0, Y: 10},
						To:          term.Coordinates{X: 11, Y: 10},
						CaptureName: "function.name",
					},
					{
						File:        testURI2,
						Text:        "func helper()",
						From:        term.Coordinates{X: 0, Y: 5},
						To:          term.Coordinates{X: 13, Y: 5},
						CaptureName: "function.name",
					},
				}
			},
			action: func(t *testing.T, client *syntaxrpc.Client) {
				it, err := client.Search("(function_declaration)", []string{"function.name"})
				require.NoError(t, err)
				results := collectResults(t, it)
				assert.Len(t, results, 2)
				assert.Equal(t, "func main()", results[0].Text)
				assert.Equal(t, "func helper()", results[1].Text)
			},
		},
		{
			name: "Search returns empty results",
			setup: func(m *mockParser) {
				m.searchResults = nil
			},
			action: func(t *testing.T, client *syntaxrpc.Client) {
				it, err := client.Search("(nonexistent)", nil)
				require.NoError(t, err)
				results := collectResults(t, it)
				assert.Empty(t, results)
			},
		},
		{
			name: "Search returns error",
			setup: func(m *mockParser) {
				m.searchErr = errors.New("search failed")
			},
			action: func(t *testing.T, client *syntaxrpc.Client) {
				it, err := client.Search("(function_declaration)", nil)
				require.NoError(t, err)
				defer func() { _ = it.Close() }()
				ctx := context.Background()
				_, ok := it.Next(ctx)
				assert.False(t, ok)
				assert.Error(t, it.Err())
			},
		},
		{
			name: "SearchNode with single node type",
			setup: func(m *mockParser) {
				m.searchNodeResults = []syntaxapi.Result{
					{
						File:        testURI,
						Text:        "func TestFunc()",
						From:        term.Coordinates{X: 0, Y: 20},
						To:          term.Coordinates{X: 15, Y: 20},
						CaptureName: "definition.function",
					},
				}
			},
			action: func(t *testing.T, client *syntaxrpc.Client) {
				it, err := client.SearchNode(syntaxapi.NodeCaptureDefinitionFunc)
				require.NoError(t, err)
				results := collectResults(t, it)
				assert.Len(t, results, 1)
				assert.Equal(t, "func TestFunc()", results[0].Text)
				assert.Equal(t, "definition.function", results[0].CaptureName)
			},
		},
		{
			name: "SearchNode with multiple node types (bitflag)",
			setup: func(m *mockParser) {
				m.searchNodeResults = []syntaxapi.Result{
					{
						File:        testURI,
						Text:        "func TestFunc()",
						From:        term.Coordinates{X: 0, Y: 20},
						To:          term.Coordinates{X: 15, Y: 20},
						CaptureName: "definition.function",
					},
					{
						File:        testURI,
						Text:        "var x int",
						From:        term.Coordinates{X: 0, Y: 1},
						To:          term.Coordinates{X: 9, Y: 1},
						CaptureName: "definition.var",
					},
				}
			},
			action: func(t *testing.T, client *syntaxrpc.Client) {
				nodeTypes := syntaxapi.NodeCaptureDefinitionFunc | syntaxapi.NodeCaptureDefinitionVar
				it, err := client.SearchNode(nodeTypes)
				require.NoError(t, err)
				results := collectResults(t, it)
				assert.Len(t, results, 2)
			},
		},
		{
			name: "Query specific file",
			setup: func(m *mockParser) {
				m.queryResults = []syntaxapi.Result{
					{
						File:        testURI,
						Text:        "type MyStruct struct",
						From:        term.Coordinates{X: 0, Y: 15},
						To:          term.Coordinates{X: 20, Y: 15},
						CaptureName: "type.definition",
					},
				}
			},
			action: func(t *testing.T, client *syntaxrpc.Client) {
				it, err := client.Query(testURI, "(type_declaration)", []string{"type.definition"})
				require.NoError(t, err)
				results := collectResults(t, it)
				assert.Len(t, results, 1)
				assert.Equal(t, "type MyStruct struct", results[0].Text)
			},
		},
		{
			name: "QueryNode specific file",
			setup: func(m *mockParser) {
				m.queryNodeResults = []syntaxapi.Result{
					{
						File:        testURI,
						Text:        "myPackage",
						From:        term.Coordinates{X: 8, Y: 0},
						To:          term.Coordinates{X: 17, Y: 0},
						CaptureName: "definition.namespace",
					},
				}
			},
			action: func(t *testing.T, client *syntaxrpc.Client) {
				it, err := client.QueryNode(testURI, syntaxapi.NodeCaptureDefinitionNamespace)
				require.NoError(t, err)
				results := collectResults(t, it)
				assert.Len(t, results, 1)
				assert.Equal(t, "myPackage", results[0].Text)
			},
		},
		{
			name: "coordinates are preserved",
			setup: func(m *mockParser) {
				m.searchResults = []syntaxapi.Result{
					{
						File:        testURI,
						Text:        "test",
						From:        term.Coordinates{X: 5, Y: 100},
						To:          term.Coordinates{X: 50, Y: 100},
						CaptureName: "test",
					},
				}
			},
			action: func(t *testing.T, client *syntaxrpc.Client) {
				it, err := client.Search("test", nil)
				require.NoError(t, err)
				results := collectResults(t, it)
				require.Len(t, results, 1)
				assert.Equal(t, term.Coordinates{X: 5, Y: 100}, results[0].From)
				assert.Equal(t, term.Coordinates{X: 50, Y: 100}, results[0].To)
			},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			mock := &mockParser{}
			tcase.setup(mock)

			server, client, cleanup := setupServerClient(t, mock)
			defer cleanup()
			_ = server

			tcase.action(t, client)
		})
	}
}

func TestNodeCaptureBitflags(t *testing.T) {
	tsuite := []struct {
		name      string
		nodeTypes syntaxapi.NodeCaptureName
		wantValue uint32
	}{
		{"single scope", syntaxapi.NodeCaptureScope, 1},
		{"single definition ns", syntaxapi.NodeCaptureDefinitionNamespace, 2},
		{"single reference", syntaxapi.NodeCaptureReference, 4},
		{"single definition func", syntaxapi.NodeCaptureDefinitionFunc, 8},
		{"single definition var", syntaxapi.NodeCaptureDefinitionVar, 16},
		{
			"combined func and var",
			syntaxapi.NodeCaptureDefinitionFunc | syntaxapi.NodeCaptureDefinitionVar,
			24,
		},
		{
			"combined all",
			syntaxapi.NodeCaptureScope | syntaxapi.NodeCaptureDefinitionNamespace |
				syntaxapi.NodeCaptureReference | syntaxapi.NodeCaptureDefinitionFunc |
				syntaxapi.NodeCaptureDefinitionVar,
			31,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			assert.Equal(t, tcase.wantValue, uint32(tcase.nodeTypes))
		})
	}
}

func TestHighlight(t *testing.T) {
	tests := []struct {
		name      string
		uri       string
		content   string
		locations []textapi.Location
	}{
		{
			name:    "single location",
			uri:     "file:///tmp/test.go",
			content: "package main",
			locations: []textapi.Location{
				{
					From:    term.Coordinates{X: 0, Y: 0},
					To:      term.Coordinates{X: 7, Y: 0},
					Attr:    term.Attributes(tcell.Style{Fg: tcell.ColorBlue}),
					Message: "keyword",
				},
			},
		},
		{
			name:    "multiple locations",
			uri:     "file:///workspace/main.rs",
			content: "fn main() {}",
			locations: []textapi.Location{
				{
					From:    term.Coordinates{X: 0, Y: 0},
					To:      term.Coordinates{X: 2, Y: 0},
					Attr:    term.Attributes(tcell.Style{Fg: tcell.ColorRed}),
					Message: "keyword",
				},
				{
					From:    term.Coordinates{X: 3, Y: 0},
					To:      term.Coordinates{X: 7, Y: 0},
					Attr:    term.Attributes(tcell.Style{Fg: tcell.ColorGreen, Attrs: tcell.AttrBold}),
					Message: "function",
				},
			},
		},
		{
			name:      "empty locations",
			uri:       "file:///tmp/empty.txt",
			content:   "",
			locations: []textapi.Location{},
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &mockParser{locations: tt.locations}
			srv := grpc.NewServer()
			syntaxrpc.RegisterSyntaxServer(srv, NewServer(stub))

			// Use a short path to avoid unix socket path length limits.
			tmpDir, err := os.MkdirTemp("", "syn")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })
			sockPath := filepath.Join(tmpDir, fmt.Sprintf("%d.sock", i))
			lis, err := net.Listen("unix", sockPath)
			require.NoError(t, err)
			go func() { _ = srv.Serve(lis) }()
			t.Cleanup(srv.Stop)

			conn, err := grpc.NewClient(
				"unix:"+sockPath,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			require.NoError(t, err)
			t.Cleanup(func() { _ = conn.Close() })

			ctx := context.Background()
			client := syntaxrpc.NewClient(ctx, conn)

			uri, err := workspaceapi.ParseURI(tt.uri)
			require.NoError(t, err)

			it, err := client.Highlight(uri, tt.content)
			require.NoError(t, err)

			got, err := iterator.ToSlice(ctx, it)
			require.NoError(t, err)

			require.Equal(t, tt.uri, stub.lastURI.String())
			require.Equal(t, tt.content, stub.lastContent)
			require.Equal(t, tt.locations, got)
		})
	}
}

type mockParser struct {
	locations         []textapi.Location
	searchResults     []syntaxapi.Result
	searchErr         error
	searchNodeResults []syntaxapi.Result
	searchNodeErr     error
	queryResults      []syntaxapi.Result
	queryErr          error
	queryNodeResults  []syntaxapi.Result
	queryNodeErr      error
	lastContent       string
	lastURI           workspaceapi.URI
}

func (m *mockParser) Highlight(uri workspaceapi.URI, content string) (
	iterator.Iterator[textapi.Location], error,
) {
	m.lastURI = uri
	m.lastContent = content
	return iterator.FromSlice(m.locations), nil
}

func (m *mockParser) Search(_ string, _ []string) (
	iterator.Iterator[syntaxapi.Result], error,
) {
	if m.searchErr != nil {
		return nil, m.searchErr
	}
	return iterator.FromSlice(m.searchResults), nil
}

func (m *mockParser) SearchNode(
	_ syntaxapi.NodeCaptureName,
) (iterator.Iterator[syntaxapi.Result], error) {
	if m.searchNodeErr != nil {
		return nil, m.searchNodeErr
	}
	return iterator.FromSlice(m.searchNodeResults), nil
}

func (m *mockParser) Query(_ workspaceapi.URI, _ string, _ []string) (
	iterator.Iterator[syntaxapi.Result], error,
) {
	if m.queryErr != nil {
		return nil, m.queryErr
	}
	return iterator.FromSlice(m.queryResults), nil
}

func (m *mockParser) QueryNode(_ workspaceapi.URI, _ syntaxapi.NodeCaptureName) (
	iterator.Iterator[syntaxapi.Result], error,
) {
	if m.queryNodeErr != nil {
		return nil, m.queryNodeErr
	}
	return iterator.FromSlice(m.queryNodeResults), nil
}

func setupServerClient(t *testing.T, mock *mockParser) (*Server, *syntaxrpc.Client, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "syntaxrpc-test-*")
	require.NoError(t, err)
	socketPath := filepath.Join(tmpDir, "test.sock")

	listener, err := net.Listen("unix", socketPath)
	require.NoError(t, err)

	server := NewServer(mock)
	grpcServer := grpc.NewServer()
	syntaxrpc.RegisterSyntaxServer(grpcServer, server)

	go func() {
		_ = grpcServer.Serve(listener)
	}()

	ctx := context.Background()
	conn, err := grpc.NewClient(
		"unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	client := syntaxrpc.NewClient(ctx, conn)

	cleanup := func() {
		_ = conn.Close()
		grpcServer.Stop()
		_ = os.RemoveAll(tmpDir)
	}

	return server, client, cleanup
}

func collectResults(t *testing.T, it iterator.Iterator[syntaxapi.Result]) []syntaxapi.Result {
	t.Helper()
	defer func() { _ = it.Close() }()

	var results []syntaxapi.Result
	ctx := context.Background()
	for {
		result, ok := it.Next(ctx)
		if !ok {
			break
		}
		results = append(results, result)
	}
	require.NoError(t, it.Err())
	return results
}
