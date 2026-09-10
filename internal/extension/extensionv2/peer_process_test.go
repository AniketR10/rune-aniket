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

package extensionv2

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/rune/internal/ide/ideauthorizer"
)

func TestPeerProcessListenerAttributesUnixConnections(t *testing.T) {
	t.Parallel()

	socket := filepath.Join("/tmp", fmt.Sprintf("rune-peer-%d.sock", os.Getpid()))
	_ = os.Remove(socket)
	t.Cleanup(func() { _ = os.Remove(socket) })
	listener, err := net.Listen("unix", socket)
	require.NoError(t, err)
	defer listener.Close()
	listener = newPeerProcessListener(listener)

	accepted := make(chan net.Conn, 1)
	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		accepted <- conn
	}()

	client, err := net.Dial("unix", socket)
	require.NoError(t, err)
	defer client.Close()

	var server net.Conn
	select {
	case err := <-acceptErr:
		require.NoError(t, err)
	case server = <-accepted:
	}
	defer server.Close()

	addr, ok := server.RemoteAddr().(ideauthorizer.PeerProcessAddr)
	require.True(t, ok)
	require.NoError(t, addr.Err)
	assert.Equal(t, os.Getpid(), addr.Process.PID)
	assert.NotEmpty(t, addr.Process.ProgramPath())
}
