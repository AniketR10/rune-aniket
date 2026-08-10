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

package vte

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/pty"
	"unstable.build/go-tui/workspace/workspacetest"
)

func TestPtyGather(t *testing.T) {
	t.Parallel()

	t.Run("rejects non-local pty files", func(t *testing.T) {
		t.Parallel()
		_, ok := newPtyGather(t.Context(), &workspacetest.File{})
		assert.False(t, ok)
	})

	t.Run("preserves content across batches with a slow consumer", func(t *testing.T) {
		t.Parallel()
		master, tty, err := pty.Open()
		require.NoError(t, err)
		defer master.Close()

		g, ok := newPtyGather(t.Context(), master)
		require.True(t, ok)

		// Alphanumeric payload only: the tty output discipline rewrites
		// control bytes (e.g. ONLCR turns \n into \r\n).
		payload := make([]byte, 4<<20)
		for i := range payload {
			payload[i] = 'a' + byte(i%26)
		}
		go func() {
			defer tty.Close()
			for chunk := payload; len(chunk) > 0; {
				n := min(len(chunk), 8192)
				if _, err := tty.Write(chunk[:n]); err != nil {
					return
				}
				chunk = chunk[n:]
			}
		}()

		var got bytes.Buffer
		for batch := range g.ready {
			require.LessOrEqual(t, len(batch), gatherBatchSize)
			require.NotEmpty(t, batch)
			got.Write(batch)
			g.release(batch)
			// A stalled parse stage must not corrupt or drop data:
			// flow control pushes back through the ring and the
			// kernel queue.
			if got.Len() > len(payload)/2 {
				time.Sleep(time.Millisecond)
			}
		}
		require.True(t, isNormalPtyExit(g.err), "unexpected stream end: %v", g.err)
		require.Equal(t, len(payload), got.Len())
		assert.Equal(t, payload, got.Bytes())
	})

	t.Run("delivers an interactive trickle without further input", func(t *testing.T) {
		t.Parallel()
		master, tty, err := pty.Open()
		require.NoError(t, err)
		defer master.Close()
		defer tty.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		g, ok := newPtyGather(ctx, master)
		require.True(t, ok)

		_, err = tty.WriteString("hello")
		require.NoError(t, err)

		select {
		case batch := <-g.ready:
			assert.Equal(t, "hello", string(batch))
			g.release(batch)
		case <-time.After(5 * time.Second):
			t.Fatal("trickle was not delivered")
		}
	})

	t.Run("context cancellation ends the stream", func(t *testing.T) {
		t.Parallel()
		master, tty, err := pty.Open()
		require.NoError(t, err)
		defer master.Close()
		defer tty.Close()

		ctx, cancel := context.WithCancel(context.Background())
		g, ok := newPtyGather(ctx, master)
		require.True(t, ok)

		cancel()
		select {
		case batch, open := <-g.ready:
			require.False(t, open, "expected closed ready channel, got batch %q", batch)
		case <-time.After(5 * time.Second):
			t.Fatal("gather did not stop on cancellation")
		}
		assert.True(t, isNormalPtyExit(g.err), "cancellation must read as a normal exit: %v", g.err)
	})
}

// BenchmarkPtyGatherDrain measures the gather stage's sustained drain
// rate through a real pty, the bound that `cat large_file` sees.
func BenchmarkPtyGatherDrain(b *testing.B) {
	payload := make([]byte, 8<<20)
	for i := range payload {
		payload[i] = 'a' + byte(i%26)
	}

	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()

	for range b.N {
		master, tty, err := pty.Open()
		if err != nil {
			b.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		g, ok := newPtyGather(ctx, master)
		if !ok {
			b.Fatal("gather rejected local pty")
		}
		// Closing the tty discards any bytes the kernel has not yet
		// handed to the master, so the writer must stay open until the
		// reader has drained the whole payload. It waits on drained
		// before closing, and the reader stops on the byte count rather
		// than on a close-driven EOF that would race the final bytes.
		drained := make(chan struct{})
		go func() {
			defer tty.Close()
			for chunk := payload; len(chunk) > 0; {
				n := min(len(chunk), 65536)
				if _, err := tty.Write(chunk[:n]); err != nil {
					return
				}
				chunk = chunk[n:]
			}
			<-drained
		}()
		var total int
		for total < len(payload) {
			batch, ok := <-g.ready
			if !ok {
				b.Fatalf("drained %d of %d bytes: %v", total, len(payload), g.err)
			}
			total += len(batch)
			g.release(batch)
		}
		close(drained)
		cancel()
		master.Close()
	}
}
