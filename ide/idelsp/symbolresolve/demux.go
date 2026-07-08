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

package symbolresolve

import (
	"context"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/debug"
)

// splitByQuery fans a single MultiResult stream into one
// iterator.Iterator[[]syntaxapi.Result] per query ID in ids. A single router
// goroutine pulls the shared stream once and routes each match to the
// channel for its QueryID, so the underlying workspace walk runs once while
// every sub-stream is consumed concurrently. Matches whose QueryID is not in
// ids are dropped.
//
// All returned sub-iterators must be consumed concurrently (each on its own
// goroutine): the router blocks while delivering to a sub-stream, so a
// sub-stream that is never drained would stall delivery to the others. Each
// sub-iterator only ever reads its own channel, so concurrent consumers
// cannot deadlock one another. Closing every sub-iterator (the demux is
// reference counted) tears down the router and closes the shared stream.
func splitByQuery(
	it iterator.Iterator[MultiResult], ids []int,
) map[int]iterator.Iterator[[]syntaxapi.Result] {
	d := &demux{
		it:       it,
		chans:    make(map[int]chan []syntaxapi.Result, len(ids)),
		stopOnce: sync.Once{},
	}
	d.stop = make(chan struct{})
	d.refs = len(ids)
	for _, id := range ids {
		d.chans[id] = make(chan []syntaxapi.Result)
	}

	go debug.CapturePanicReport(d.route)

	out := make(map[int]iterator.Iterator[[]syntaxapi.Result], len(ids))
	for _, id := range ids {
		ch := d.chans[id]
		out[id] = iterator.FromFunc(
			func(ctx context.Context) ([]syntaxapi.Result, bool, error) {
				select {
				case <-ctx.Done():
					return nil, false, ctx.Err()
				case <-d.stop:
					r, ok := <-ch
					return r, ok, d.routeErr()
				case r, ok := <-ch:
					if !ok {
						return nil, false, d.routeErr()
					}
					return r, true, nil
				}
			},
			d.release,
		)
	}
	return out
}

type demux struct {
	it    iterator.Iterator[MultiResult]
	chans map[int]chan []syntaxapi.Result
	stop  chan struct{}

	mu       sync.Mutex
	err      error
	refs     int
	stopOnce sync.Once
}

// route pulls the shared stream once and forwards each match to the channel
// registered for its QueryID until the stream ends or teardown is requested.
func (d *demux) route() {
	ctx := context.Background()
	defer func() {
		for _, ch := range d.chans {
			close(ch)
		}
	}()
	for {
		r, ok := d.it.Next(ctx)
		if !ok {
			d.mu.Lock()
			d.err = d.it.Err()
			d.mu.Unlock()
			return
		}
		ch, wanted := d.chans[r.QueryID]
		if !wanted {
			continue
		}
		select {
		case ch <- r.Match:
		case <-d.stop:
			return
		}
	}
}

func (d *demux) routeErr() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

// release tears the demux down once every sub-iterator has been closed.
func (d *demux) release() error {
	d.mu.Lock()
	d.refs--
	last := d.refs == 0
	d.mu.Unlock()
	if !last {
		return nil
	}
	d.stopOnce.Do(func() { close(d.stop) })
	return d.it.Close()
}
