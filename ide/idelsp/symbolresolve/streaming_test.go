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

package symbolresolve_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/ide/idelsp/symbolresolve"
)

// fakeSearcher drives Resolve from a scripted SearchMulti stream so the test
// controls exactly how results are produced. The other Searcher methods are
// unused by the reference-only path under test.
type fakeSearcher struct {
	it iterator.Iterator[symbolresolve.MultiResult]
}

func (f fakeSearcher) SearchMulti(
	[]symbolresolve.MultiQuery, ...string,
) (iterator.Iterator[symbolresolve.MultiResult], error) {
	return f.it, nil
}

func (f fakeSearcher) Search(
	string, []string, ...string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (f fakeSearcher) Search2(
	string, [2]string, ...string,
) (iterator.Iterator[[2]syntaxapi.Result], error) {
	return iterator.Empty[[2]syntaxapi.Result](), nil
}

func (f fakeSearcher) SearchNode(
	syntaxapi.NodeCaptureName, ...string,
) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (f fakeSearcher) QueryNode(
	workspaceapi.URI, syntaxapi.NodeCaptureName,
) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}

func (f fakeSearcher) Query2(
	workspaceapi.URI, string, [2]string,
) (iterator.Iterator[[2]syntaxapi.Result], error) {
	return iterator.Empty[[2]syntaxapi.Result](), nil
}

// streamSpec is a minimal reference-only spec: one ref query, no package
// clause and no import queries, so Resolve issues a single SearchMulti pass
// with exactly one sub-stream.
var streamSpec = &symbolresolve.Spec{
	LangID: "go",
	RefQueries: []symbolresolve.RefQuery{
		{Query: "(q)", Captures: []string{"pkg", "sym"}},
	},
}

// trackingMultiIter yields count reference matches for pkg.sym, tracking
// the peak number of produced-but-unconsumed elements
// so the test can prove Resolve consumes the stream incrementally rather than
// buffering it whole. ToSlice is deliberately never used by Resolve.
type trackingMultiIter struct {
	pkg, sym string
	count    int

	emitted  int
	live     int64
	peakLive int64
	closed   atomic.Bool
}

func (it *trackingMultiIter) Next(ctx context.Context) (symbolresolve.MultiResult, bool) {
	select {
	case <-ctx.Done():
		return symbolresolve.MultiResult{}, false
	default:
	}
	if it.emitted >= it.count {
		return symbolresolve.MultiResult{}, false
	}
	it.emitted++
	return it.result(), true
}

// result builds one grouped reference match: the package alias capture
// followed by the symbol capture.
func (it *trackingMultiIter) result() symbolresolve.MultiResult {
	live := atomic.AddInt64(&it.live, 1)
	for {
		peak := atomic.LoadInt64(&it.peakLive)
		if live <= peak || atomic.CompareAndSwapInt64(&it.peakLive, peak, live) {
			break
		}
	}
	uri, _ := workspaceapi.ParseURI(fmt.Sprintf("file:///f%d.go", it.emitted))
	return symbolresolve.MultiResult{
		QueryID: 0,
		Match: []syntaxapi.Result{
			{File: uri, Text: it.pkg, CaptureName: "pkg"},
			{
				File: uri, Text: it.sym,
				From:        term.Coordinates{X: 1, Y: it.emitted},
				CaptureName: "sym",
			},
		},
	}
}

// consumed is called by the test's wrapper iterator each time Resolve pulls
// an element, so the live gauge reflects produced-minus-consumed.
func (it *trackingMultiIter) consumed() {
	atomic.AddInt64(&it.live, -1)
}

func (it *trackingMultiIter) Err() error { return nil }

func (it *trackingMultiIter) Close() error {
	it.closed.Store(true)
	return nil
}

func TestResolveStreamsWithoutFullBuffer(t *testing.T) {
	t.Parallel()

	const count = 5000
	inner := &trackingMultiIter{pkg: "pkg", sym: "Sym", count: count}
	// Wrap so every delivered element decrements the live gauge, modelling
	// the consumer taking ownership as soon as it pulls.
	wrapped := iterator.FromFunc(
		func(ctx context.Context) (symbolresolve.MultiResult, bool, error) {
			r, ok := inner.Next(ctx)
			if ok {
				inner.consumed()
			}
			return r, ok, inner.Err()
		}, inner.Close,
	)

	parser := fakeSearcher{it: wrapped}
	matches, err := symbolresolve.Resolve(
		context.Background(), parser, specIter(streamSpec), "pkg.Sym", nil,
	)
	require.NoError(t, err)

	// Every match maps to a distinct file URI, so all are retained.
	assert.Len(t, matches, count)
	assert.True(t, inner.closed.Load(), "Resolve must close the SearchMulti stream")
	assert.LessOrEqualf(t, inner.peakLive, int64(16),
		"Resolve must consume the stream incrementally; peak live elements was %d of %d",
		inner.peakLive, count)
}

func TestResolveCancelsLongStream(t *testing.T) {
	t.Parallel()

	blocking := iterator.FromFunc(
		func(ctx context.Context) (symbolresolve.MultiResult, bool, error) {
			<-ctx.Done()
			return symbolresolve.MultiResult{}, false, ctx.Err()
		}, func() error { return nil },
	)
	parser := fakeSearcher{it: blocking}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = symbolresolve.Resolve(ctx, parser, specIter(streamSpec), "pkg.Sym", nil)
	}()

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Resolve did not return after context cancellation")
	}
}
