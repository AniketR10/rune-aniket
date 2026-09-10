// Copyright (C) 2017-2026 The Rune Authors
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

package workspacerpc

import (
	"runtime"
	"slices"
	"sort"
	"sync"
	"unsafe"
)

const (
	// Below this size the finalizer bookkeeping costs more than the
	// allocation it would save.
	minRecycledBufSize = 1 << 10
	maxRecycledBufs    = 32
)

// readBufs is shared by all Server instances: buffers carry no
// workspace affinity, and a single process-wide slab bounds retention
// at maxRecycledBufs total instead of per server.
var readBufs bufSlab

// bufSlab recycles read buffers whose lifetime is unknown to the
// handler: gRPC marshals the response, and may retain it in
// interceptors or binary logging, after the handler returns. Instead of
// an explicit release, each handed-out buffer carries a finalizer that
// resurrects the backing array into the slab once nothing references it
// anymore. The zero value is ready to use.
//
// Idle retention is bounded sync.Pool style: each GC cycle demotes the
// primary generation to victim and drops the previous victim, so a
// buffer unused for two cycles is freed. The runtime forces a GC every
// two minutes even in an idle process, so idle slabs drain without any
// timer or scavenger goroutine of our own.
type bufSlab struct {
	mu     sync.Mutex
	bufs   [][]byte // idle buffers sorted by capacity, ascending
	victim [][]byte // previous generation, dropped at the next purge
	pacing bool     // a purge cleanup is armed for the next GC cycle
}

// get returns a buffer of length n, reusing the smallest idle buffer
// that fits, and arms its release-to-slab finalizer.
func (s *bufSlab) get(n int) []byte {
	if n < minRecycledBufSize {
		return make([]byte, n)
	}
	s.mu.Lock()
	buf := takeFit(&s.bufs, n)
	if buf == nil {
		buf = takeFit(&s.victim, n)
	}
	s.mu.Unlock()
	if buf == nil {
		buf = make([]byte, n)
	}
	s.arm(buf)
	return buf
}

// takeFit removes and returns the smallest buffer with capacity at
// least n, or nil. The slab mutex must be held.
func takeFit(bufs *[][]byte, n int) []byte {
	i := sort.Search(len(*bufs), func(i int) bool { return cap((*bufs)[i]) >= n })
	if i == len(*bufs) {
		return nil
	}
	buf := (*bufs)[i][:n]
	*bufs = slices.Delete(*bufs, i, i+1)
	return buf
}

// arm sets the finalizer on the backing array's base pointer. The
// closure must not capture buf: an object reachable from its own
// finalizer is never collected. It captures only the capacity and
// rebuilds the slice when the finalizer fires. Pooled buffers are
// unarmed (the runtime clears a finalizer when it runs), so re-arming
// on every hand-out never double-sets.
func (s *bufSlab) arm(buf []byte) {
	c := cap(buf)
	runtime.SetFinalizer(&buf[0], func(p *byte) {
		s.put(unsafe.Slice(p, c))
	})
}

// put inserts buf keeping bufs sorted by capacity. When the slab is
// full it evicts the smallest buffer: keeping the larger ones satisfies
// more future requests and causes less allocation churn.
func (s *bufSlab) put(buf []byte) {
	s.mu.Lock()
	i := sort.Search(len(s.bufs), func(i int) bool { return cap(s.bufs[i]) >= cap(buf) })
	s.bufs = slices.Insert(s.bufs, i, buf[:cap(buf)])
	if len(s.bufs) > maxRecycledBufs {
		s.bufs = slices.Delete(s.bufs, 0, 1)
	}
	pace := !s.pacing
	s.pacing = true
	s.mu.Unlock()
	if pace {
		s.schedulePurge()
	}
}

// schedulePurge arms a purge for the next GC cycle: the sentinel is
// unreachable as soon as this returns, so its cleanup fires when the
// next cycle collects it. The sentinel is 16 bytes to stay out of the
// tiny allocator, which batches smaller pointer-free objects and would
// delay collection indefinitely.
func (s *bufSlab) schedulePurge() {
	runtime.AddCleanup(new([16]byte), func(s *bufSlab) { s.purge() }, s)
}

// purge ages the generations and keeps pacing GC cycles until the slab
// is fully drained, so an idle slab costs nothing.
func (s *bufSlab) purge() {
	s.mu.Lock()
	s.victim = s.bufs
	s.bufs = nil
	again := len(s.victim) > 0
	s.pacing = again
	s.mu.Unlock()
	if again {
		s.schedulePurge()
	}
}
