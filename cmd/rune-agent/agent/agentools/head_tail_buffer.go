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

package agentools

import (
	"fmt"
	"sync"
	"time"
)

const (
	defaultHeadSize = 512 * 1024 // 512KB
	defaultTailSize = 512 * 1024 // 512KB
)

// HeadTailBuffer is a thread-safe io.Writer that keeps the first
// headSize bytes and the last tailSize bytes of everything written,
// dropping the middle. It supports blocking on new data via WaitForData.
type HeadTailBuffer struct {
	mu       sync.Mutex
	head     []byte
	tail     []byte
	headSize int
	tailSize int
	written  int64
	notify   chan struct{}
}

// NewHeadTailBuffer creates a buffer that retains the first headSize
// and last tailSize bytes. Zero values use the 512KB defaults.
func NewHeadTailBuffer(headSize, tailSize int) *HeadTailBuffer {
	if headSize <= 0 {
		headSize = defaultHeadSize
	}
	if tailSize <= 0 {
		tailSize = defaultTailSize
	}
	return &HeadTailBuffer{
		head:     make([]byte, 0, headSize),
		tail:     make([]byte, 0, tailSize),
		headSize: headSize,
		tailSize: tailSize,
		notify:   make(chan struct{}, 1),
	}
}

// Write implements io.Writer. It is safe for concurrent use.
func (b *HeadTailBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	b.mu.Lock()
	n := len(p)
	b.written += int64(n)

	// Fill head first.
	if len(b.head) < b.headSize {
		room := b.headSize - len(b.head)
		if room > n {
			room = n
		}
		b.head = append(b.head, p[:room]...)
		p = p[room:]
	}

	// Remaining goes into the tail ring buffer.
	if len(p) > 0 {
		if len(p) >= b.tailSize {
			// The new data alone exceeds or equals the tail — keep only the last tailSize bytes.
			b.tail = append(b.tail[:0], p[len(p)-b.tailSize:]...)
		} else {
			b.tail = append(b.tail, p...)
			if len(b.tail) > b.tailSize {
				// Trim from the front to keep only the last tailSize bytes.
				b.tail = append(b.tail[:0], b.tail[len(b.tail)-b.tailSize:]...)
			}
		}
	}
	b.mu.Unlock()

	// Signal waiting readers.
	select {
	case b.notify <- struct{}{}:
	default:
	}

	return n, nil
}

// String returns the buffered content. If data was truncated, the
// middle is replaced with a "[N bytes truncated]" marker.
func (b *HeadTailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	kept := int64(len(b.head)) + int64(len(b.tail))
	if kept >= b.written {
		// No truncation — everything fits in head + tail.
		out := make([]byte, 0, len(b.head)+len(b.tail))
		out = append(out, b.head...)
		out = append(out, b.tail...)
		return string(out)
	}

	truncated := b.written - kept
	marker := fmt.Sprintf("\n\n[%d bytes truncated]\n\n", truncated)
	out := make([]byte, 0, len(b.head)+len(marker)+len(b.tail))
	out = append(out, b.head...)
	out = append(out, marker...)
	out = append(out, b.tail...)
	return string(out)
}

// Len returns the total number of bytes written.
func (b *HeadTailBuffer) Len() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.written
}

// Reset clears all buffered data and the byte counter.
func (b *HeadTailBuffer) Reset() {
	b.mu.Lock()
	b.head = b.head[:0]
	b.tail = b.tail[:0]
	b.written = 0
	b.mu.Unlock()
}

// WaitForData blocks until new data arrives or the deadline is reached.
func (b *HeadTailBuffer) WaitForData(deadline time.Time) {
	timeout := time.Until(deadline)
	if timeout <= 0 {
		return
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-b.notify:
	case <-timer.C:
	}
}
