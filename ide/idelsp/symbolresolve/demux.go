// Copyright (C) 2017-2026 Unstable Build, LLC
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

package symbolresolve

import (
	"context"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/debug"
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
