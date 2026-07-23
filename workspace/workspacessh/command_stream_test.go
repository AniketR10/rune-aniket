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
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/workspace"
	tworkspacerpc "unstable.build/go-tui/workspace/workspacerpc"
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
