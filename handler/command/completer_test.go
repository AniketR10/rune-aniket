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

package command

import (
	"context"
	"io"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

func TestCommandOutputLinesCompleterCloseSignals(t *testing.T) {
	var pid workspaceapi.Pid
	var sig syscall.Signal

	exec := &mockExecutor{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			// don't close stdout: simulates a long-running process
			return 42, nil
		},
		signalFn: func(_pid workspaceapi.Pid, _sig syscall.Signal) error {
			pid = _pid
			sig = _sig
			return nil
		},
	}

	c := OutputLinesCompleter(exec, []string{"long-running"})
	iter, _, err := c.Complete(context.Background(), nil)
	require.NoError(t, err)

	err = iter.Close()
	require.NoError(t, err)

	assert.Equal(t, workspaceapi.Pid(42), pid)
	assert.Equal(t, syscall.SIGINT, sig)
}

func TestCommandOutputLinesCompleterContextCancellation(t *testing.T) {
	exec := &mockExecutor{
		startFn: func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
			// stdout stays open: process never exits on its own
			return 1, nil
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := OutputLinesCompleter(exec, []string{"hanging-cmd"})
	iter, _, err := c.Complete(ctx, nil)
	require.NoError(t, err)

	cancel()

	_, ok := iter.Next(ctx)
	assert.False(t, ok)
	err = iter.Err()
	if err != nil {
		assert.Contains(t, err.Error(), "closed")
	}
}

func TestCommandOutputLinesCompleter(t *testing.T) {
	tests := []struct {
		name      string
		cmdArgs   []string
		executor  *mockExecutor
		wantLines []string
		wantErr   string
	}{
		{
			name:    "empty args returns error",
			cmdArgs: nil,
			executor: &mockExecutor{
				startFn: func(_ context.Context, _ workspaceapi.Cmd) (workspaceapi.Pid, error) {
					t.Fatal("StartCommand should not be called")
					return 0, nil
				},
			},
			wantErr: "expected at least one argument",
		},
		{
			name:    "start command error propagates",
			cmdArgs: []string{"failing-cmd"},
			executor: &mockExecutor{
				startFn: func(_ context.Context, _ workspaceapi.Cmd) (workspaceapi.Pid, error) {
					return 0, io.ErrUnexpectedEOF
				},
			},
			wantErr: "unexpected EOF",
		},
		{
			name:    "single line",
			cmdArgs: []string{"echo", "hello"},
			executor: &mockExecutor{
				startFn: writeAndExit("hello\n", 1),
			},
			wantLines: []string{"hello"},
		},
		{
			name:    "multiple lines",
			cmdArgs: []string{"my-cmd"},
			executor: &mockExecutor{
				startFn: writeAndExit("alpha\nbeta\ngamma\n", 1),
			},
			wantLines: []string{"alpha", "beta", "gamma"},
		},
		{
			name:    "empty output",
			cmdArgs: []string{"my-cmd"},
			executor: &mockExecutor{
				startFn: writeAndExit("", 1),
			},
			wantLines: nil,
		},
		{
			name:    "trailing line without EOL",
			cmdArgs: []string{"my-cmd"},
			executor: &mockExecutor{
				startFn: writeAndExit("foo\nbar", 1),
			},
			wantLines: []string{"foo", "bar"},
		},
		{
			name:    "lines with spaces preserved",
			cmdArgs: []string{"my-cmd"},
			executor: &mockExecutor{
				startFn: writeAndExit("  leading\ntrailing  \n  both  \n", 1),
			},
			wantLines: []string{"  leading", "trailing  ", "  both  "},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := OutputLinesCompleter(tt.executor, tt.cmdArgs)
			iter, prefix, err := c.Complete(context.Background(), nil)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, iter)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "", prefix)

			got := collectAll(t, iter)
			assert.Equal(t, tt.wantLines, got)
		})
	}
}

type mockExecutor struct {
	startFn  func(ctx context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error)
	signalFn func(pid workspaceapi.Pid, sig syscall.Signal) error
}

func (m *mockExecutor) StartCommand(ctx context.Context, cmd workspaceapi.Cmd) (
	workspaceapi.Pid, error,
) {
	return m.startFn(ctx, cmd)
}

func (m *mockExecutor) Signal(pid workspaceapi.Pid, sig syscall.Signal) error {
	if m.signalFn != nil {
		return m.signalFn(pid, sig)
	}
	return nil
}

func (m *mockExecutor) Close() error {
	return nil
}

// collectAll drains the iterator and returns all values.
func collectAll(t *testing.T, iter iterator.Iterator[string]) []string {
	t.Helper()
	var out []string
	for {
		val, ok := iter.Next(context.Background())
		if !ok {
			require.NoError(t, iter.Err())
			break
		}
		out = append(out, val)
	}
	return out
}

// writeAndExit simulates a command that writes lines to stdout then exits.
func writeAndExit(output string, pid int) func(context.Context, workspaceapi.Cmd) (workspaceapi.Pid, error) {
	return func(_ context.Context, cmd workspaceapi.Cmd) (workspaceapi.Pid, error) {
		go func() {
			w := cmd.Stdout.(io.WriteCloser)
			io.WriteString(w, output)
			w.Close()
			//cmd.Watcher.WatchProcess() <- nil
		}()
		return workspaceapi.Pid(pid), nil
	}
}
