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
	"net"
	"os"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi/syntaxrpc"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// fakeSearcher is a syntaxapi.Searcher backed by a slice of results.
type fakeSearcher struct {
	results []syntaxapi.Result
	err     error
}

func (f *fakeSearcher) Search(query string, captureNames []string) (iterator.Iterator[syntaxapi.Result], error) {
	if f.err != nil {
		return nil, f.err
	}
	if len(captureNames) == 0 {
		return iterator.FromSlice(f.results), nil
	}
	allow := make(map[string]struct{}, len(captureNames))
	for _, n := range captureNames {
		allow[n] = struct{}{}
	}
	var filtered []syntaxapi.Result
	for _, r := range f.results {
		if _, ok := allow[r.Text]; ok {
			filtered = append(filtered, r)
		}
	}
	return iterator.FromSlice(filtered), nil
}

func mustParseURI(t *testing.T, s string) workspaceapi.URI {
	t.Helper()
	u, err := workspaceapi.ParseURI(s)
	require.NoError(t, err)
	return u
}

var sockSeq atomic.Int64

// startServer creates a gRPC server on a unix socket and returns
// a connected client. The server and connection are cleaned up
// when the test finishes.
func startServer(t *testing.T, s syntaxapi.Searcher) *syntaxrpc.Client {
	t.Helper()

	// Use a short path to stay under macOS 108-char unix socket limit.
	sock := fmt.Sprintf("%s/syntaxrpc-%d.sock", os.TempDir(), sockSeq.Add(1))
	t.Cleanup(func() { os.Remove(sock) })
	lis, err := net.Listen("unix", sock)
	require.NoError(t, err)

	srv := grpc.NewServer()
	syntaxrpc.RegisterSyntaxServer(srv, NewServer(s))
	go srv.Serve(lis)
	t.Cleanup(srv.GracefulStop)

	conn, err := grpc.NewClient(
		"unix:"+sock,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })

	return syntaxrpc.NewClient(context.Background(), conn)
}

func TestSearch(t *testing.T) {
	uri := mustParseURI(t, "file:///src/main.go")

	tests := []struct {
		name         string
		fake         fakeSearcher
		query        string
		captureNames []string
		want         []syntaxapi.Result
		wantErr      bool
	}{
		{
			name: "multiple results round-trip",
			fake: fakeSearcher{results: []syntaxapi.Result{
				{File: uri, Text: "func main()", Position: term.Coordinates{X: 0, Y: 10}},
				{File: uri, Text: "func init()", Position: term.Coordinates{X: 0, Y: 1}},
			}},
			query: "main",
			want: []syntaxapi.Result{
				{File: uri, Text: "func main()", Position: term.Coordinates{X: 0, Y: 10}},
				{File: uri, Text: "func init()", Position: term.Coordinates{X: 0, Y: 1}},
			},
		},
		{
			name:  "empty results",
			fake:  fakeSearcher{},
			query: "nothing",
			want:  []syntaxapi.Result{},
		},
		{
			name: "capture names narrows results",
			fake: fakeSearcher{results: []syntaxapi.Result{
				{File: uri, Text: "match", Position: term.Coordinates{X: 5, Y: 3}},
				{File: uri, Text: "skip", Position: term.Coordinates{X: 0, Y: 0}},
				{File: uri, Text: "match", Position: term.Coordinates{X: 10, Y: 7}},
			}},
			query:        "query",
			captureNames: []string{"match"},
			want: []syntaxapi.Result{
				{File: uri, Text: "match", Position: term.Coordinates{X: 5, Y: 3}},
				{File: uri, Text: "match", Position: term.Coordinates{X: 10, Y: 7}},
			},
		},
		{
			name:    "searcher error propagates",
			fake:    fakeSearcher{err: net.UnknownNetworkError("test error")},
			query:   "q",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := startServer(t, &tt.fake)
			ctx := context.Background()

			iter, err := client.Search(tt.query, tt.captureNames)
			if err != nil {
				require.True(t, tt.wantErr, "unexpected Search error: %v", err)
				return
			}
			defer iter.Close()

			got, err := iterator.ToSlice(ctx, iter)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Len(t, got, len(tt.want))

			for i := range tt.want {
				assert.Equal(t, tt.want[i].File.String(), got[i].File.String())
				assert.Equal(t, tt.want[i].Text, got[i].Text)
				assert.Equal(t, tt.want[i].Position, got[i].Position)
			}
		})
	}
}
