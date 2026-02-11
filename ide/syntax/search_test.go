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

package syntax

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/go-multierror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
)

func TestListSymbolsIteratorNext(t *testing.T) {
	tests := []struct {
		name       string
		send       []syntaxapi.Result
		closeCh    bool
		cancelCtx  bool
		cancelSelf bool
		wantCount  int
		wantErr    bool
	}{
		{
			name: "receives all results then channel closes",
			send: []syntaxapi.Result{
				{Text: "foo", CaptureName: "name"},
				{Text: "bar", CaptureName: "name"},
			},
			closeCh:   true,
			wantCount: 2,
		},
		{
			name:      "empty channel closed immediately",
			closeCh:   true,
			wantCount: 0,
		},
		{
			name: "single result",
			send: []syntaxapi.Result{
				{Text: "only", CaptureName: "definition"},
			},
			closeCh:   true,
			wantCount: 1,
		},
		{
			name:      "external context cancellation returns error",
			cancelCtx: true,
			wantCount: 0,
			wantErr:   true,
		},
		{
			name:       "iterator context cancellation returns error",
			cancelSelf: true,
			wantCount:  0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			it := newTestIterator(len(tt.send))

			for _, r := range tt.send {
				it.ch <- r
			}
			if tt.closeCh {
				close(it.ch)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			if tt.cancelCtx {
				cancel()
			}
			if tt.cancelSelf {
				it.cancel()
			}

			got := drainIterator(t, it, ctx)
			assert.Equal(t, tt.wantCount, got, "result count")

			err := it.Err()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestListSymbolsIteratorNextPreservesResultValues(t *testing.T) {
	it := newTestIterator(2)

	want := []syntaxapi.Result{
		{Text: "alpha", CaptureName: "function"},
		{Text: "beta", CaptureName: "type"},
	}
	for _, r := range want {
		it.ch <- r
	}
	close(it.ch)

	ctx := context.Background()
	var got []syntaxapi.Result
	for {
		r, ok := it.Next(ctx)
		if !ok {
			break
		}
		got = append(got, r)
	}

	require.Len(t, got, 2)
	assert.Equal(t, want[0].Text, got[0].Text)
	assert.Equal(t, want[0].CaptureName, got[0].CaptureName)
	assert.Equal(t, want[1].Text, got[1].Text)
	assert.Equal(t, want[1].CaptureName, got[1].CaptureName)
}

func TestListSymbolsIteratorErr(t *testing.T) {
	tests := []struct {
		name      string
		setErr    error
		cancelCtx bool
		wantNil   bool
		wantWraps error
	}{
		{
			name:    "no error and live context returns nil",
			wantNil: true,
		},
		{
			name:   "accumulated error surfaces",
			setErr: errors.New("something went wrong"),
		},
		{
			name:      "cancelled context without explicit error",
			cancelCtx: true,
			wantWraps: context.Canceled,
		},
		{
			name:      "both accumulated error and cancelled context",
			setErr:    errors.New("read failure"),
			cancelCtx: true,
			wantWraps: context.Canceled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			it := &listSymbolsIterator{
				ctx:    ctx,
				err:    tt.setErr,
				cancel: cancel,
			}
			if tt.cancelCtx {
				cancel()
			}

			err := it.Err()
			if tt.wantNil {
				assert.NoError(t, err)
				return
			}

			require.Error(t, err)
			if tt.wantWraps != nil {
				assert.ErrorIs(t, err, tt.wantWraps)
			}
		})
	}
}

func TestListSymbolsIteratorErrMultierrorUnwrap(t *testing.T) {
	sentinel := errors.New("sentinel")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	it := &listSymbolsIterator{
		ctx:    ctx,
		err:    multierror.Append(nil, sentinel),
		cancel: cancel,
	}

	err := it.Err()
	require.Error(t, err)

	var merr *multierror.Error
	require.ErrorAs(t, err, &merr)
	require.GreaterOrEqual(t, len(merr.Errors), 1)

	found := false
	for _, e := range merr.Errors {
		if errors.Is(e, sentinel) {
			found = true
		}
	}
	assert.True(t, found, "sentinel error not found in multierror chain")
}

func TestListSymbolsIteratorErrNilErrReturnsCtxErr(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	<-ctx.Done() // ensure it's expired

	it := &listSymbolsIterator{ctx: ctx, cancel: cancel}

	err := it.Err()
	require.Error(t, err)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestListSymbolsIteratorClose(t *testing.T) {
	tests := []struct {
		name  string
		delay time.Duration
	}{
		{name: "immediate goroutine completion", delay: 0},
		{name: "blocks until goroutine finishes", delay: 50 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			closeWaitCh := make(chan struct{})

			it := &listSymbolsIterator{
				ctx:         ctx,
				ch:          make(chan syntaxapi.Result),
				cancel:      cancel,
				closeWaitCh: closeWaitCh,
			}

			var done bool
			var mu sync.Mutex

			go func() {
				if tt.delay > 0 {
					time.Sleep(tt.delay)
				}
				close(closeWaitCh)
				mu.Lock()
				done = true
				mu.Unlock()
			}()

			err := it.Close()
			require.NoError(t, err)

			mu.Lock()
			assert.True(t, done, "Close() returned before closeWaitCh was closed")
			mu.Unlock()
		})
	}
}

func TestListSymbolsIteratorClose_CancelsContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	closeWaitCh := make(chan struct{})

	it := &listSymbolsIterator{
		ctx:         ctx,
		ch:          make(chan syntaxapi.Result),
		cancel:      cancel,
		closeWaitCh: closeWaitCh,
	}

	go func() { close(closeWaitCh) }()
	_ = it.Close()

	assert.Error(t, ctx.Err(), "context should be cancelled after Close")
}

func TestListSymbolsIteratorConcurrentAccess(t *testing.T) {
	const (
		numResults   = 50
		numConsumers = 4
	)

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan syntaxapi.Result, numResults)
	closeWaitCh := make(chan struct{})

	it := &listSymbolsIterator{
		ctx:         ctx,
		ch:          ch,
		cancel:      cancel,
		closeWaitCh: closeWaitCh,
	}

	go func() {
		for i := range numResults {
			ch <- syntaxapi.Result{Text: string(rune('A' + i%26))}
		}
		close(ch)
		close(closeWaitCh)
	}()

	var wg sync.WaitGroup
	counts := make([]int, numConsumers)
	for i := range numConsumers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for {
				_, ok := it.Next(ctx)
				if !ok {
					return
				}
				counts[idx]++
			}
		}(i)
	}
	wg.Wait()

	total := 0
	for _, c := range counts {
		total += c
	}
	assert.Equal(t, numResults, total, "total results across goroutines")

	var errWg sync.WaitGroup
	for range 10 {
		errWg.Go(func() {
			_ = it.Err()
		})
	}
	errWg.Wait()
}

func newTestIterator(bufSize int) *listSymbolsIterator {
	ctx, cancel := context.WithCancel(context.Background())
	return &listSymbolsIterator{
		ctx:         ctx,
		ch:          make(chan syntaxapi.Result, bufSize),
		cancel:      cancel,
		closeWaitCh: make(chan struct{}),
	}
}

func drainIterator(t *testing.T, it *listSymbolsIterator, ctx context.Context) int {
	t.Helper()
	count := 0
	for {
		_, ok := it.Next(ctx)
		if !ok {
			return count
		}
		count++
	}
}
