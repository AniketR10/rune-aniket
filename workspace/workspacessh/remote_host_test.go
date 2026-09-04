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
	"context"
	"errors"
	"io"

	"os"
	"sync"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi/workspacerpc"
	"google.golang.org/grpc"
	"unstable.build/rune/workspace"
	tworkspacerpc "unstable.build/rune/workspace/workspacerpc"
)

// TestKeepaliveContract pins the invariant that ties the SSH-tunneled
// gRPC client's ping cadence to the server's enforcement policy. If a
// future edit lowers clientKeepalive.Time below serverEnforcement.MinTime,
// or flips PermitWithoutStream off, the server would answer the client's
// keepalive pings with GOAWAY too_many_pings and tear down the single
// HTTP/2 connection carried over the SSH pipe — dropping terminals,
// invalidating cached pty fds, and forcing a reconnect that re-runs
// remote provisioning.
func TestKeepaliveContract(t *testing.T) {
	assert.LessOrEqual(t, serverEnforcement.MinTime, clientKeepalive.Time,
		"server MinTime must permit the client ping interval, else "+
			"the server sends GOAWAY too_many_pings")
	assert.True(t, serverEnforcement.PermitWithoutStream,
		"client pings with PermitWithoutStream, so the server must "+
			"permit pings without an active stream")
	assert.Greater(t, serverEnforcement.MinTime, time.Duration(0),
		"MinTime must be a positive floor against a genuinely abusive peer")
}

func TestReaderWriterListener(t *testing.T) {
	t.Run("one accept", func(t *testing.T) {
		inRead, inWrite, err := os.Pipe()
		require.NoError(t, err)

		outRead, outWrite, err := os.Pipe()
		require.NoError(t, err)

		l := newStdioListener(logger, inRead, outWrite, false, func() {})

		_, err = inWrite.WriteString("JJ")
		require.NoError(t, err)

		// sut
		conn, err := l.Accept()
		require.NoError(t, err)

		n, err := conn.Write([]byte("\n"))
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		var buf [3]byte
		n, err = io.ReadAtLeast(outRead, buf[:], 1)
		require.NoError(t, err)
		require.Equal(t, 1, n)
		assert.Equal(t, "\n", string(buf[:1]))

		n, err = conn.Read(buf[:])
		require.NoError(t, err)
		require.Equal(t, 2, n)
		assert.Equal(t, "JJ", string(buf[:2]))

		err = conn.Close()
		require.NoError(t, err)

		t.Cleanup(func() {
			inRead.Close()
			inWrite.Close()
			outWrite.Close()
			outRead.Close()
		})
	})

	t.Run("close of stdio returns", func(t *testing.T) {
		grpcServer := grpc.NewServer()
		inRead, inWrite, err := os.Pipe()
		require.NoError(t, err)

		outRead, outWrite, err := os.Pipe()
		require.NoError(t, err)

		lis := newStdioListener(logger, inRead, outWrite, false, func() {
			go grpcServer.Stop()
		})
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		scheme, err := workspace.NewMemoryScheme(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)

		server := tworkspacerpc.NewServer(scheme,
			tworkspacerpc.CommandAuthorizerFunc(
				func(context.Context, workspaceapi.Cmd) error { return nil }))
		workspacerpc.RegisterSchemeServer(grpcServer, server)
		go func() {
			inWrite.Close()
		}()
		grpcServer.Serve(lis)

		t.Cleanup(func() {
			inRead.Close()
			inWrite.Close()
			outWrite.Close()
			outRead.Close()
		})
	})
}

var logger = log.New()

func init() {
	logger.Out = io.Discard
}

type closer struct {
	mu     sync.Mutex
	closed bool
}

func (c *closer) Read(p []byte) (n int, err error) {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return 0, io.EOF
	}
	// timeout tests
	time.Sleep(300 * time.Minute)
	return 0, io.EOF
}

func (c *closer) Write(p []byte) (n int, err error) {
	return 0, errors.New("nope")
}

func (c *closer) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}
