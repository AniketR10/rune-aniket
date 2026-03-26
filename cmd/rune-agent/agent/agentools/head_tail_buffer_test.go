// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentools

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHeadTailBuffer_small_write_fits_in_head(t *testing.T) {
	buf := NewHeadTailBuffer(32, 32)
	n, err := buf.Write([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, n)
	assert.Equal(t, "hello", buf.String())
	assert.Equal(t, int64(5), buf.Len())
}

func TestHeadTailBuffer_head_fills_then_tail(t *testing.T) {
	buf := NewHeadTailBuffer(4, 4)
	_, _ = buf.Write([]byte("HHHHTTTT"))

	assert.Equal(t, "HHHHTTTT", buf.String())
	assert.Equal(t, int64(8), buf.Len())
}

func TestHeadTailBuffer_truncation(t *testing.T) {
	buf := NewHeadTailBuffer(4, 4)
	// Write 4 head + 10 more (only last 4 kept in tail)
	_, _ = buf.Write([]byte("HHHH"))
	_, _ = buf.Write([]byte("0123456789"))

	s := buf.String()
	assert.Contains(t, s, "HHHH")
	assert.Contains(t, s, "6789")
	assert.Contains(t, s, "bytes truncated")
	assert.Equal(t, int64(14), buf.Len())
}

func TestHeadTailBuffer_single_large_write(t *testing.T) {
	buf := NewHeadTailBuffer(4, 4)
	// Single write that exceeds head+tail.
	_, _ = buf.Write([]byte("ABCDxxxxxxYZWQ"))

	s := buf.String()
	assert.True(t, strings.HasPrefix(s, "ABCD"))
	assert.True(t, strings.HasSuffix(s, "YZWQ"))
	assert.Contains(t, s, "bytes truncated")
}

func TestHeadTailBuffer_tail_overflow_keeps_latest(t *testing.T) {
	buf := NewHeadTailBuffer(2, 4)
	_, _ = buf.Write([]byte("HH"))   // fills head
	_, _ = buf.Write([]byte("aaaa")) // fills tail
	_, _ = buf.Write([]byte("bb"))   // overwrites oldest tail bytes

	s := buf.String()
	assert.True(t, strings.HasPrefix(s, "HH"))
	assert.True(t, strings.HasSuffix(s, "aabb"))
}

func TestHeadTailBuffer_no_truncation_when_exact_fit(t *testing.T) {
	buf := NewHeadTailBuffer(4, 4)
	_, _ = buf.Write([]byte("ABCD"))
	// Exactly head size, no tail, no truncation.
	assert.Equal(t, "ABCD", buf.String())
	assert.NotContains(t, buf.String(), "truncated")
}

func TestHeadTailBuffer_Reset(t *testing.T) {
	buf := NewHeadTailBuffer(16, 16)
	_, _ = buf.Write([]byte("some data"))
	buf.Reset()
	assert.Equal(t, int64(0), buf.Len())
	assert.Equal(t, "", buf.String())
}

func TestHeadTailBuffer_empty_write(t *testing.T) {
	buf := NewHeadTailBuffer(16, 16)
	n, err := buf.Write([]byte{})
	require.NoError(t, err)
	assert.Equal(t, 0, n)
	assert.Equal(t, int64(0), buf.Len())
}

func TestHeadTailBuffer_concurrent_writes(t *testing.T) {
	buf := NewHeadTailBuffer(64, 64)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, _ = buf.Write([]byte("x"))
			}
		}()
	}
	wg.Wait()
	assert.Equal(t, int64(1000), buf.Len())
}

func TestHeadTailBuffer_WaitForData_returns_on_write(t *testing.T) {
	buf := NewHeadTailBuffer(16, 16)
	done := make(chan struct{})
	go func() {
		buf.WaitForData(time.Now().Add(5 * time.Second))
		close(done)
	}()

	// Small delay to let the goroutine block.
	time.Sleep(10 * time.Millisecond)
	_, _ = buf.Write([]byte("wake"))

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("WaitForData did not return after Write")
	}
}

func TestHeadTailBuffer_WaitForData_returns_on_deadline(t *testing.T) {
	buf := NewHeadTailBuffer(16, 16)
	start := time.Now()
	buf.WaitForData(time.Now().Add(50 * time.Millisecond))
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 40*time.Millisecond)
}

func TestHeadTailBuffer_WaitForData_past_deadline(t *testing.T) {
	buf := NewHeadTailBuffer(16, 16)
	start := time.Now()
	buf.WaitForData(time.Now().Add(-time.Second))
	assert.Less(t, time.Since(start), 10*time.Millisecond)
}

func TestHeadTailBuffer_defaults(t *testing.T) {
	buf := NewHeadTailBuffer(0, 0)
	assert.Equal(t, defaultHeadSize, buf.headSize)
	assert.Equal(t, defaultTailSize, buf.tailSize)
}
