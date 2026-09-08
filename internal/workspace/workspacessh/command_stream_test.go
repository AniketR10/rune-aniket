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
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	sdkworkspacerpc "github.com/unstablebuild/rune-go-sdk/api/workspaceapi/workspacerpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"unstable.build/rune/internal/debug"
	"unstable.build/rune/internal/workspace"
	tworkspacerpc "unstable.build/rune/internal/workspace/workspacerpc"
)

func TestNestedCommandStreamWaitsForProcessCompletion(t *testing.T) {
	fileURI, err := workspaceapi.ParseURI("file://" + t.TempDir())
	require.NoError(t, err)
	fileScheme, err := workspace.NewFileScheme(
		context.Background(), config.NopConfig(), fileURI)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, fileScheme.Close()) })

	innerClient := newNestedCommandRPCClient(t, fileScheme)

	remoteURI, err := workspaceapi.ParseURI("ssh://test@host/tmp")
	require.NoError(t, err)
	remote := newRemoteScheme(context.Background(),
		func(context.Context, workspaceapi.URI, func(error)) (schemeapi.Scheme, error) {
			return innerClient, nil
		}, remoteURI)
	t.Cleanup(func() { require.NoError(t, remote.Close()) })

	outerClient := newNestedCommandRPCClient(t, remote)

	stdoutRead, stdoutWrite, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stdoutRead.Close()) })
	stderrRead, stderrWrite, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, stderrRead.Close()) })

	type readResult struct {
		data []byte
		err  error
	}
	stdoutResult := make(chan readResult, 1)
	stderrResult := make(chan readResult, 1)
	go debug.CapturePanicReport(func() {
		data, err := io.ReadAll(stdoutRead)
		stdoutResult <- readResult{data: data, err: err}
	})
	go debug.CapturePanicReport(func() {
		data, err := io.ReadAll(stderrRead)
		stderrResult <- readResult{data: data, err: err}
	})

	watched := make(chan error)
	_, err = outerClient.StartCommand(context.Background(), workspaceapi.Cmd{
		Path: "sh",
		Args: []string{"-c",
			"printf 'finder-output\\n'; printf 'finder-error\\n' >&2"},
		Stdout:  stdoutWrite,
		Stderr:  stderrWrite,
		Watcher: workspaceapi.ChanProcessWatcher(watched),
	})
	require.NoError(t, err)

	select {
	case err = <-watched:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for command result")
	}
	require.NoError(t, err)

	stdout := <-stdoutResult
	require.NoError(t, stdout.err)
	require.Equal(t, "finder-output\n", string(stdout.data))
	stderr := <-stderrResult
	require.NoError(t, stderr.err)
	require.Equal(t, "finder-error\n", string(stderr.data))
}

func newNestedCommandRPCClient(
	t *testing.T, scheme schemeapi.Scheme,
) *sdkworkspacerpc.Client {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := grpc.NewServer()
	rpcServer := tworkspacerpc.NewServer(scheme,
		tworkspacerpc.CommandAuthorizerFunc(
			func(context.Context, workspaceapi.Cmd) error { return nil }))
	sdkworkspacerpc.RegisterExecutorServer(server, rpcServer)
	go debug.CapturePanicReport(func() {
		_ = server.Serve(listener)
	})

	conn, err := grpc.NewClient(listener.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	client := sdkworkspacerpc.NewClient(context.Background(), conn)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
		server.Stop()
	})
	return client
}
