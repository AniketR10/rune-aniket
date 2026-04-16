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

package extensionv2

import (
	"context"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/auth"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"google.golang.org/grpc"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/workspace/processctx"
)

type recordingExecutor struct {
	ctx context.Context
	cmd workspaceapi.Cmd
}

var _ schemeapi.Executor = (*recordingExecutor)(nil)

func (r *recordingExecutor) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
	r.ctx = ctx
	r.cmd = cmd
	return 1, nil
}

func (r *recordingExecutor) Signal(workspaceapi.Pid, syscall.Signal) error {
	return nil
}

func (r *recordingExecutor) Close() error {
	return nil
}

func TestWorkspaceRunnerStartCommandPreservesCallerEnv(t *testing.T) {
	t.Parallel()

	keys, err := auth.GenerateKeys()
	require.NoError(t, err)

	uri, err := workspaceapi.ParseURI("file:///tmp")
	require.NoError(t, err)

	exec := &recordingExecutor{}
	runner := newWorkspaceRunner(
		exec,
		nil, // grantor is not used by StartCommand
		uri,
		"/tmp/ext.sock",
		"/tmp/ext-data",
		[]byte("cert"),
		keys,
	)

	_, err = runner.StartCommand(context.Background(), workspaceapi.Cmd{
		Path: "/bin/zsh",
		Args: []string{"--login", "-i"},
		Env:  []string{"ZDOTDIR=/Applications/Rune.app/Contents/Resources/zdot", "FOO=bar"},
	})
	require.NoError(t, err)

	assert.Contains(t, exec.cmd.Env, "ZDOTDIR=/Applications/Rune.app/Contents/Resources/zdot")
	assert.Contains(t, exec.cmd.Env, "FOO=bar")
	assert.Contains(t, exec.cmd.Env, "IDE_SOCKET=/tmp/ext.sock")
	assert.Contains(t, exec.cmd.Env, "IDE_DATADIR=/tmp/ext-data")
	assert.NotEmpty(t, exec.cmd.Dir)
	assert.Equal(t, "/bin/zsh", exec.cmd.Path)
	assert.Equal(t, []string{"--login", "-i"}, exec.cmd.Args)
}

func TestWorkspaceRunnerStartCommandMarksTokenPlugin(t *testing.T) {
	t.Parallel()

	keys, err := auth.GenerateKeys()
	require.NoError(t, err)
	verifyKeys, err := keys.Verify(context.Background())
	require.NoError(t, err)
	require.NotEmpty(t, verifyKeys)

	uri, err := workspaceapi.ParseURI("file:///tmp")
	require.NoError(t, err)

	runner := newWorkspaceRunner(
		&recordingExecutor{},
		nil, // grantor is not used by StartCommand
		uri,
		"/tmp/ext.sock",
		"/tmp/ext-data",
		[]byte("cert"),
		keys,
	)

	env, err := runner.commandEnvs(context.Background(), "/bin/zsh",
		[]string{"--login", "-i"})
	require.NoError(t, err)

	var token string
	for _, e := range env {
		if value, ok := strings.CutPrefix(e, runner.cfg.authTokenEnv+"="); ok {
			token = value
			break
		}
	}
	require.NotEmpty(t, token)

	claims, err := auth.VerifyToken[Extension](verifyKeys[0], token)
	require.NoError(t, err)
	assert.True(t, claims.Extra.Plugin)
	assert.Equal(t, "/bin/zsh", claims.Extra.Path)
	assert.Equal(t, []string{"--login", "-i"}, claims.Extra.Args)
	assert.Equal(t, extensionapi.AllPermissions(), claims.Extra.Permissions)
}

func TestWorkspaceRunnerRunCarriesExtensionID(t *testing.T) {
	t.Parallel()

	keys, err := auth.GenerateKeys()
	require.NoError(t, err)

	uri, err := workspaceapi.ParseURI("file:///tmp")
	require.NoError(t, err)

	exec := &recordingExecutor{}
	runner := newWorkspaceRunner(
		exec,
		nil, // grantor is not used before process execution in this test
		uri,
		"/tmp/ext.sock",
		"/tmp/ext-data",
		[]byte("cert"),
		keys,
	)
	require.NoError(t, runner.Run("test-extension", "/bin/ext", config.NopConfig()))

	extensionID, ok := processctx.ExtensionIDFromContext(exec.ctx)
	require.True(t, ok)
	assert.Equal(t, "test-extension", extensionID)

	processIfc, ok := runner.pids.Load("test-extension")
	require.True(t, ok)
	process := processIfc.(extensionProcess)
	assert.Equal(t, workspaceapi.Pid(1), process.pid)
	require.NotNil(t, process.cancel)
}

type testServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s testServerStream) Context() context.Context {
	return s.ctx
}

func TestExtensionIDStreamInterceptorTagsContext(t *testing.T) {
	t.Parallel()

	ctx := auth.ContextWithClaims(context.Background(), auth.UserClaims[Extension]{
		Extra: Extension{Metadata: extensionapi.Metadata{
			ExtensionID: "test-extension",
		}},
	})
	stream := testServerStream{ctx: ctx}

	called := false
	err := extensionIDStreamInterceptor()(nil, stream, &grpc.StreamServerInfo{},
		func(_ any, stream grpc.ServerStream) error {
			called = true
			extensionID, ok := processctx.ExtensionIDFromContext(stream.Context())
			require.True(t, ok)
			assert.Equal(t, "test-extension", extensionID)
			return nil
		})
	require.NoError(t, err)
	assert.True(t, called)
}

var _ extension.Runner = (*workspaceRunner)(nil)
