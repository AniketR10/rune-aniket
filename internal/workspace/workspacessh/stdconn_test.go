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

package workspacessh

import (
	"os"
	"sync"
	"sync/atomic"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestStdConnCloseConcurrent exercises the contract documented by the
// FIXME on stdConn: concurrent Close() calls must be goroutine-safe
// AND the close hook must run at most once.
//
// gRPC's Close path (used by remoteScheme.Close) and the watcher
// goroutine in connectScheme can both call into the dialed net.Conn at
// roughly the same time when an SSH workspace tears down — for example
// when the IDE shuts down while the remote process is also exiting.
// Without synchronization, callHook reads s.closeHook, then writes nil
// to it, and finally invokes the captured func. Two goroutines racing
// through that sequence both observe the non-nil hook and invoke it
// twice, which surfaces in production as duplicate "ssh connection
// closed unexpectedly" notifications and in tests as a `-race` data
// race on the func value.
func TestStdConnCloseConcurrent(t *testing.T) {
	// Two pipes wired into a single stdConn — the same shape
	// connectScheme installs in production.
	stdinR, stdinW, err := os.Pipe()
	require.NoError(t, err)
	defer stdinR.Close()
	defer stdinW.Close()
	stdoutR, stdoutW, err := os.Pipe()
	require.NoError(t, err)
	defer stdoutR.Close()
	defer stdoutW.Close()

	var hookCalls atomic.Int32
	conn, err := newStdConn(log.StandardLogger(), stdoutR, stdinW,
		false, /* stdio */
		func() { hookCalls.Add(1) },
	)
	require.NoError(t, err)

	// Race two Close() calls. -race must report no data race, and
	// the close hook must fire exactly once regardless of which
	// goroutine wins.
	const n = 32
	var wg sync.WaitGroup
	wg.Add(n)
	start := make(chan struct{})
	for range n {
		go func() {
			defer wg.Done()
			<-start
			_ = conn.Close()
		}()
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int32(1), hookCalls.Load(),
		"close hook must fire exactly once even when Close races "+
			"with itself; otherwise observers (e.g. the SSH "+
			"workspace's closeHook → setError chain) see the same "+
			"disconnect twice")
}
