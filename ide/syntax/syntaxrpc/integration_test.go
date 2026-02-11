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
				{
					File:        uri,
					Text:        "func main()",
					From:        term.Coordinates{X: 0, Y: 10},
					To:          term.Coordinates{X: 11, Y: 10},
					CaptureName: "function",
				},
				{
					File:        uri,
					Text:        "func init()",
					From:        term.Coordinates{X: 0, Y: 1},
					To:          term.Coordinates{X: 11, Y: 1},
					CaptureName: "function",
				},
			}},
			query: "main",
			want: []syntaxapi.Result{
				{
					File:        uri,
					Text:        "func main()",
					From:        term.Coordinates{X: 0, Y: 10},
					To:          term.Coordinates{X: 11, Y: 10},
					CaptureName: "function",
				},
				{
					File:        uri,
					Text:        "func init()",
					From:        term.Coordinates{X: 0, Y: 1},
					To:          term.Coordinates{X: 11, Y: 1},
					CaptureName: "function",
				},
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
				{
					File:        uri,
					Text:        "match",
					From:        term.Coordinates{X: 5, Y: 3},
					To:          term.Coordinates{X: 10, Y: 3},
					CaptureName: "identifier",
				},
				{
					File:        uri,
					Text:        "skip",
					From:        term.Coordinates{X: 0, Y: 0},
					To:          term.Coordinates{X: 4, Y: 0},
					CaptureName: "comment",
				},
				{
					File:        uri,
					Text:        "match",
					From:        term.Coordinates{X: 10, Y: 7},
					To:          term.Coordinates{X: 15, Y: 7},
					CaptureName: "identifier",
				},
			}},
			query:        "query",
			captureNames: []string{"match"},
			want: []syntaxapi.Result{
				{
					File:        uri,
					Text:        "match",
					From:        term.Coordinates{X: 5, Y: 3},
					To:          term.Coordinates{X: 10, Y: 3},
					CaptureName: "identifier",
				},
				{
					File:        uri,
					Text:        "match",
					From:        term.Coordinates{X: 10, Y: 7},
					To:          term.Coordinates{X: 15, Y: 7},
					CaptureName: "identifier",
				},
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

				assert.Equal(t, tt.want[i].From, got[i].From)
				assert.Equal(t, tt.want[i].To, got[i].To)
				assert.Equal(t, tt.want[i].CaptureName, got[i].CaptureName)
			}
		})
	}
}
