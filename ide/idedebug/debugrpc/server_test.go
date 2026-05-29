// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

package debugrpc_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/go-dap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/debugapi"
	clientpkg "github.com/unstablebuild/rune-go-sdk/api/debugapi/debugrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"unstable.build/go-tui/ide/idedebug/debugrpc"
)

// fakeDebugger is a minimal debugapi.Debugger implementation
// that the server-side debugrpc.Server wraps. Behaviour for
// CreateSession is fully controllable; the rest is unimplemented
// because these tests focus on the streaming session lifecycle.
type fakeDebugger struct {
	mu sync.Mutex

	// CreateSession behaviour.
	createCallsLang []string
	createCallsCaps []debugapi.ClientCapabilities
	sessionID       string
	caps            *dap.Capabilities
	createErr       error
	// subscriberCh is signalled with the EventSubscriber once
	// CreateSession is invoked. Tests can use it to drive the
	// event stream after SessionOpened has reached the client.
	subscriberCh chan debugapi.EventSubscriber
	// blockCreate, if set, blocks CreateSession until closed.
	// Used to test caller-side cancellation while the adapter
	// is still starting up.
	blockCreate chan struct{}
}

func (f *fakeDebugger) CreateSession(
	ctx context.Context, langID string,
	client debugapi.ClientCapabilities, sub debugapi.EventSubscriber,
) (string, *dap.Capabilities, error) {
	f.mu.Lock()
	f.createCallsLang = append(f.createCallsLang, langID)
	f.createCallsCaps = append(f.createCallsCaps, client)
	err := f.createErr
	sid := f.sessionID
	caps := f.caps
	subCh := f.subscriberCh
	block := f.blockCreate
	f.mu.Unlock()
	if err != nil {
		return "", nil, err
	}
	if subCh != nil {
		select {
		case subCh <- sub:
		default:
		}
	}
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return "", nil, ctx.Err()
		}
	}
	return sid, caps, nil
}

// The remaining methods are unused by these tests but must be
// present so fakeDebugger satisfies debugapi.Debugger.
func (f *fakeDebugger) Launch(context.Context, string, debugapi.LaunchRequestArguments) error {
	return nil
}
func (f *fakeDebugger) Attach(context.Context, string, debugapi.AttachRequestArguments) error {
	return nil
}
func (f *fakeDebugger) ConfigurationDone(context.Context, string) error { return nil }
func (f *fakeDebugger) Disconnect(context.Context, string, *dap.DisconnectArguments) error {
	return nil
}
func (f *fakeDebugger) Terminate(context.Context, string, *dap.TerminateArguments) error {
	return nil
}
func (f *fakeDebugger) Restart(context.Context, string) error { return nil }
func (f *fakeDebugger) SetBreakpoints(
	context.Context, string, *dap.SetBreakpointsArguments,
) ([]dap.Breakpoint, error) {
	return nil, nil
}
func (f *fakeDebugger) SetFunctionBreakpoints(
	context.Context, string, *dap.SetFunctionBreakpointsArguments,
) ([]dap.Breakpoint, error) {
	return nil, nil
}
func (f *fakeDebugger) SetExceptionBreakpoints(
	context.Context, string, *dap.SetExceptionBreakpointsArguments,
) ([]dap.Breakpoint, error) {
	return nil, nil
}
func (f *fakeDebugger) Continue(
	context.Context, string, *dap.ContinueArguments,
) (*dap.ContinueResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) Next(context.Context, string, *dap.NextArguments) error { return nil }
func (f *fakeDebugger) StepIn(context.Context, string, *dap.StepInArguments) error {
	return nil
}
func (f *fakeDebugger) StepOut(context.Context, string, *dap.StepOutArguments) error {
	return nil
}
func (f *fakeDebugger) StepBack(context.Context, string, *dap.StepBackArguments) error {
	return nil
}
func (f *fakeDebugger) ReverseContinue(context.Context, string, *dap.ReverseContinueArguments) error {
	return nil
}
func (f *fakeDebugger) Pause(context.Context, string, *dap.PauseArguments) error {
	return nil
}
func (f *fakeDebugger) Threads(context.Context, string) ([]dap.Thread, error) {
	return nil, nil
}
func (f *fakeDebugger) StackTrace(
	context.Context, string, *dap.StackTraceArguments,
) (*dap.StackTraceResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) Scopes(context.Context, string, *dap.ScopesArguments) ([]dap.Scope, error) {
	return nil, nil
}
func (f *fakeDebugger) Variables(
	context.Context, string, *dap.VariablesArguments,
) ([]dap.Variable, error) {
	return nil, nil
}
func (f *fakeDebugger) SetVariable(
	context.Context, string, *dap.SetVariableArguments,
) (*dap.SetVariableResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) Source(
	context.Context, string, *dap.SourceArguments,
) (*dap.SourceResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) Evaluate(
	context.Context, string, *dap.EvaluateArguments,
) (*dap.EvaluateResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) SetExpression(
	context.Context, string, *dap.SetExpressionArguments,
) (*dap.SetExpressionResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) Completions(
	context.Context, string, *dap.CompletionsArguments,
) ([]dap.CompletionItem, error) {
	return nil, nil
}
func (f *fakeDebugger) ExceptionInfo(
	context.Context, string, *dap.ExceptionInfoArguments,
) (*dap.ExceptionInfoResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) Modules(
	context.Context, string, *dap.ModulesArguments,
) (*dap.ModulesResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) LoadedSources(context.Context, string) ([]dap.Source, error) {
	return nil, nil
}
func (f *fakeDebugger) ReadMemory(
	context.Context, string, *dap.ReadMemoryArguments,
) (*dap.ReadMemoryResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) WriteMemory(
	context.Context, string, *dap.WriteMemoryArguments,
) (*dap.WriteMemoryResponseBody, error) {
	return nil, nil
}
func (f *fakeDebugger) Disassemble(
	context.Context, string, *dap.DisassembleArguments,
) ([]dap.DisassembledInstruction, error) {
	return nil, nil
}
func (f *fakeDebugger) GotoTargets(
	context.Context, string, *dap.GotoTargetsArguments,
) ([]dap.GotoTarget, error) {
	return nil, nil
}
func (f *fakeDebugger) Goto(context.Context, string, *dap.GotoArguments) error { return nil }

var _ debugapi.Debugger = (*fakeDebugger)(nil)

// recordingSubscriber captures every event/close delivered to a
// debugapi.EventSubscriber so tests can assert on the sequence
// of forwarded messages.
type recordingSubscriber struct {
	mu      sync.Mutex
	events  []dap.EventMessage
	closed  bool
	reason  string
	closeCh chan struct{}
}

func newRecordingSubscriber() *recordingSubscriber {
	return &recordingSubscriber{closeCh: make(chan struct{})}
}

func (s *recordingSubscriber) OnEvent(ev dap.EventMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
}

func (s *recordingSubscriber) OnClose(reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.closed = true
	s.reason = reason
	close(s.closeCh)
}

func (s *recordingSubscriber) waitClose(t *testing.T) string {
	t.Helper()
	select {
	case <-s.closeCh:
	case <-time.After(2 * time.Second):
		t.Fatal("OnClose never called")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reason
}

// testEnv runs a real Server over an in-process gRPC unix socket
// connected to a real Client, exercising the full RPC code path.
type testEnv struct {
	srv    *grpc.Server
	server *debugrpc.Server
	dbg    *fakeDebugger
	conn   *grpc.ClientConn
	client *clientpkg.Client
	cancel func()
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	tmpDir, err := os.MkdirTemp("", "dbgrpc")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(tmpDir) })

	sock := filepath.Join(tmpDir, "s.sock")
	lis, err := net.Listen("unix", sock)
	require.NoError(t, err)

	dbg := &fakeDebugger{
		sessionID: "session-1",
		caps:      &dap.Capabilities{SupportsTerminateRequest: true},
	}
	srv := grpc.NewServer()
	server := debugrpc.NewServer(dbg)
	server.Register(srv)
	go func() { _ = srv.Serve(lis) }()

	conn, err := grpc.NewClient(
		"unix:"+sock,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	client := clientpkg.NewClient(ctx, conn)

	t.Cleanup(func() {
		_ = client.Close()
		_ = server.Close()
		cancel()
		_ = conn.Close()
		srv.Stop()
	})
	return &testEnv{
		srv:    srv,
		server: server,
		dbg:    dbg,
		conn:   conn,
		client: client,
		cancel: cancel,
	}
}

// makeOutputEvent returns a marshallable DAP OutputEvent fixture.
func makeOutputEvent(category, output string) *dap.OutputEvent {
	return &dap.OutputEvent{
		Event: dap.Event{
			ProtocolMessage: dap.ProtocolMessage{Type: "event"},
			Event:           "output",
		},
		Body: dap.OutputEventBody{Category: category, Output: output},
	}
}

func TestServer_CreateSession(t *testing.T) {
	cases := []struct {
		name string
		// drive runs after CreateSession returns successfully
		// (i.e. SessionOpened has reached the client). It is
		// passed the server-side EventSubscriber so the test
		// can push events / OnClose deterministically.
		drive  func(env *testEnv, sub debugapi.EventSubscriber)
		assert func(t *testing.T, env *testEnv, sub *recordingSubscriber,
			sid string, caps *dap.Capabilities)
	}{
		{
			name: "returns sessionID and capabilities on success",
			drive: func(_ *testEnv, sub debugapi.EventSubscriber) {
				sub.OnClose("terminated")
			},
			assert: func(t *testing.T, env *testEnv, sub *recordingSubscriber,
				sid string, caps *dap.Capabilities) {
				assert.Equal(t, "session-1", sid)
				require.NotNil(t, caps)
				assert.True(t, caps.SupportsTerminateRequest)
				assert.Equal(t, []string{"go"}, env.dbg.createCallsLang)
				require.Len(t, env.dbg.createCallsCaps, 1)
				assert.Equal(t, "rune",
					env.dbg.createCallsCaps[0].ClientID)
				assert.Equal(t, "terminated", sub.waitClose(t))
			},
		},
		{
			name: "forwards DAP events to the subscriber in order",
			drive: func(_ *testEnv, sub debugapi.EventSubscriber) {
				sub.OnEvent(makeOutputEvent("stdout", "first\n"))
				sub.OnEvent(makeOutputEvent("stderr", "second\n"))
				sub.OnClose("terminated")
			},
			assert: func(t *testing.T, _ *testEnv, sub *recordingSubscriber,
				_ string, _ *dap.Capabilities) {
				assert.Equal(t, "terminated", sub.waitClose(t))
				sub.mu.Lock()
				defer sub.mu.Unlock()
				require.Len(t, sub.events, 2)
				e0, ok := sub.events[0].(*dap.OutputEvent)
				require.True(t, ok)
				assert.Equal(t, "stdout", e0.Body.Category)
				assert.Equal(t, "first\n", e0.Body.Output)
				e1, ok := sub.events[1].(*dap.OutputEvent)
				require.True(t, ok)
				assert.Equal(t, "stderr", e1.Body.Category)
				assert.Equal(t, "second\n", e1.Body.Output)
			},
		},
		{
			name: "OnClose carries adapter-supplied reason",
			drive: func(_ *testEnv, sub debugapi.EventSubscriber) {
				sub.OnClose("crashed")
			},
			assert: func(t *testing.T, _ *testEnv, sub *recordingSubscriber,
				_ string, _ *dap.Capabilities) {
				assert.Equal(t, "crashed", sub.waitClose(t))
			},
		},
		{
			name: "empty close reason maps to terminated",
			drive: func(_ *testEnv, sub debugapi.EventSubscriber) {
				// An adapter that closes without a
				// reason must surface as "terminated".
				sub.OnClose("")
			},
			assert: func(t *testing.T, _ *testEnv, sub *recordingSubscriber,
				_ string, _ *dap.Capabilities) {
				assert.Equal(t, "terminated", sub.waitClose(t))
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestEnv(t)
			env.dbg.subscriberCh = make(chan debugapi.EventSubscriber, 1)
			sub := newRecordingSubscriber()
			ctx, cancel := context.WithTimeout(context.Background(),
				2*time.Second)
			defer cancel()
			sid, caps, err := env.client.CreateSession(ctx, "go",
				debugapi.ClientCapabilities{
					ClientID: "rune", LinesStartAt1: true,
				}, sub)
			require.NoError(t, err)

			// At this point SessionOpened has reached the
			// client; drain the server-side subscriber and
			// drive whatever events the case requires.
			var serverSub debugapi.EventSubscriber
			select {
			case serverSub = <-env.dbg.subscriberCh:
			case <-time.After(2 * time.Second):
				t.Fatal("server never received subscriber")
			}
			tc.drive(env, serverSub)
			tc.assert(t, env, sub, sid, caps)
		})
	}
}
func TestServer_CreateSession_BubblesUpDebuggerError(t *testing.T) {
	env := newTestEnv(t)
	env.dbg.createErr = errors.New("adapter spawn failed")

	sub := newRecordingSubscriber()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := env.client.CreateSession(ctx, "go",
		debugapi.ClientCapabilities{}, sub)
	require.Error(t, err)
	st, ok := status.FromError(errors.Unwrap(err))
	if !ok {
		// Some grpc versions wrap differently; accept the raw err
		// as long as it carries the message.
		assert.Contains(t, err.Error(), "adapter spawn failed")
	} else {
		assert.Equal(t, codes.Unknown, st.Code())
		assert.Contains(t, st.Message(), "adapter spawn failed")
	}
	assert.False(t, sub.closed,
		"OnClose should not fire when the handshake itself fails")
}

func TestServer_CreateSession_CallerCancelAbortsHandshake(t *testing.T) {
	env := newTestEnv(t)
	// Block CreateSession on the server so the SessionOpened
	// handshake never reaches the client.
	env.dbg.blockCreate = make(chan struct{})
	t.Cleanup(func() { close(env.dbg.blockCreate) })

	sub := newRecordingSubscriber()
	ctx, cancel := context.WithTimeout(context.Background(),
		100*time.Millisecond)
	defer cancel()
	_, _, err := env.client.CreateSession(ctx, "go",
		debugapi.ClientCapabilities{}, sub)
	require.Error(t, err)
	assert.True(t,
		errors.Is(err, context.DeadlineExceeded) ||
			errors.Is(err, context.Canceled),
		"expected deadline/cancel, got %v", err)
}
