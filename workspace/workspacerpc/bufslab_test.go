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

package workspacerpc

import (
	"runtime"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pacing: true pretends the purge pacer is already armed, so put never
// schedules an async GC-driven purge. Structural tests use it to keep
// pool contents deterministic; aging is exercised by calling purge
// directly, and the real pacer end to end in
// TestBufSlabIdleBuffersEventuallyFreed.
func TestBufSlabBestFitReuse(t *testing.T) {
	t.Parallel()
	s := &bufSlab{pacing: true}
	b4 := make([]byte, 4<<10)
	s.put(make([]byte, 2<<10))
	s.put(make([]byte, 8<<10))
	s.put(b4)

	got := s.get(3 << 10)

	assert.Len(t, got, 3<<10)
	assert.Equal(t, 4<<10, cap(got))
	assert.Same(t, &b4[0], &got[0])
	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Len(t, s.bufs, 2)
}

func TestBufSlabEvictsSmallestWhenFull(t *testing.T) {
	t.Parallel()
	s := &bufSlab{pacing: true}
	for i := range maxRecycledBufs {
		s.put(make([]byte, (2<<10)+i))
	}

	s.put(make([]byte, 64<<10))

	s.mu.Lock()
	defer s.mu.Unlock()
	require.Len(t, s.bufs, maxRecycledBufs)
	assert.Equal(t, (2<<10)+1, cap(s.bufs[0]))
	assert.Equal(t, 64<<10, cap(s.bufs[len(s.bufs)-1]))
}

func TestBufSlabSmallReadsBypassPool(t *testing.T) {
	t.Parallel()
	s := &bufSlab{pacing: true}
	s.put(make([]byte, 4<<10))

	got := s.get(16)
	empty := s.get(0)

	assert.Len(t, got, 16)
	assert.Empty(t, empty)
	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Len(t, s.bufs, 1)
}

func TestBufSlabRecyclesReleasedBuffer(t *testing.T) {
	t.Parallel()
	s := &bufSlab{pacing: true}
	buf := s.get(4 << 10)
	// uintptr does not extend the backing array's liveness.
	base := uintptr(unsafe.Pointer(&buf[0]))
	buf = nil
	_ = buf

	require.Eventually(t, func() bool {
		runtime.GC()
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.bufs) == 1
	}, 10*time.Second, 10*time.Millisecond,
		"released buffer never returned to the slab")

	got := s.get(4 << 10)
	assert.Equal(t, base, uintptr(unsafe.Pointer(&got[0])))
}

func TestBufSlabGetFallsBackToVictim(t *testing.T) {
	t.Parallel()
	s := &bufSlab{pacing: true}
	b4 := make([]byte, 4<<10)
	s.put(b4)

	s.purge()
	got := s.get(4 << 10)

	assert.Same(t, &b4[0], &got[0])
	s.mu.Lock()
	defer s.mu.Unlock()
	assert.Empty(t, s.bufs)
	assert.Empty(t, s.victim)
}

func TestBufSlabPurgeDropsIdleGeneration(t *testing.T) {
	t.Parallel()
	s := &bufSlab{pacing: true}
	b4 := make([]byte, 4<<10)
	s.put(b4)

	s.purge()
	s.purge()

	s.mu.Lock()
	assert.Empty(t, s.bufs)
	assert.Empty(t, s.victim)
	s.mu.Unlock()
	got := s.get(4 << 10)
	assert.NotSame(t, &b4[0], &got[0])
}

func TestBufSlabIdleBuffersEventuallyFreed(t *testing.T) {
	t.Parallel()
	s := &bufSlab{}
	s.put(make([]byte, 4<<10))

	require.Eventually(t, func() bool {
		runtime.GC()
		s.mu.Lock()
		defer s.mu.Unlock()
		return len(s.bufs) == 0 && len(s.victim) == 0
	}, 10*time.Second, 10*time.Millisecond,
		"idle pooled buffer never aged out")
}
