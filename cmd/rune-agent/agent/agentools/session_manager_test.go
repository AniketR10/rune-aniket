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
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionManager_Create_and_exit(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	defer func() { _ = mgr.Close() }()

	sess, err := mgr.Create("echo hello", t.TempDir(), "", false, nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, sess.ID, minSessionID)
	assert.Less(t, sess.ID, maxSessionID)

	exited := sess.Wait(5 * time.Second)
	assert.True(t, exited)
	assert.Equal(t, 0, sess.ExitCode())
	assert.Contains(t, sess.Output(), "hello")
}

func TestSessionManager_Get(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	defer func() { _ = mgr.Close() }()

	sess, err := mgr.Create("echo ok", t.TempDir(), "", false, nil)
	require.NoError(t, err)

	got, ok := mgr.Get(sess.ID)
	assert.True(t, ok)
	assert.Equal(t, sess.ID, got.ID)

	_, ok = mgr.Get(99999)
	assert.False(t, ok)
}

func TestSessionManager_nonzero_exit(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	defer func() { _ = mgr.Close() }()

	sess, err := mgr.Create("exit 42", t.TempDir(), "", false, nil)
	require.NoError(t, err)

	exited := sess.Wait(5 * time.Second)
	assert.True(t, exited)
	assert.Equal(t, 42, sess.ExitCode())
}

func TestSessionManager_long_running_yield(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	defer func() { _ = mgr.Close() }()

	sess, err := mgr.Create("sleep 10", t.TempDir(), "", false, nil)
	require.NoError(t, err)

	// Process should still be running after a short wait.
	exited := sess.Wait(100 * time.Millisecond)
	assert.False(t, exited)
	assert.False(t, sess.Exited())
	assert.Greater(t, sess.WallTime(), 0.0)
}

func TestSessionManager_WriteStdin(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	defer func() { _ = mgr.Close() }()

	// cat reads from stdin and echoes to stdout.
	sess, err := mgr.Create("cat", t.TempDir(), "", false, nil)
	require.NoError(t, err)

	err = sess.WriteStdin([]byte("hello from stdin\n"))
	require.NoError(t, err)

	// Close stdin to let cat exit.
	_ = sess.stdin.Close()
	exited := sess.Wait(5 * time.Second)
	assert.True(t, exited)
	assert.Contains(t, sess.Output(), "hello from stdin")
}

func TestSessionManager_Close_kills_sessions(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)

	sess, err := mgr.Create("sleep 60", t.TempDir(), "", false, nil)
	require.NoError(t, err)
	assert.False(t, sess.Exited())

	_ = mgr.Close()

	// After Close, the session should exit (context cancelled + SIGKILL).
	select {
	case <-sess.done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("session did not exit after Close")
	}
}

func TestSessionManager_eviction(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	defer func() { _ = mgr.Close() }()

	dir := t.TempDir()

	// Fill up to max.
	sessions := make([]*Session, maxSessions)
	for i := range maxSessions {
		sess, err := mgr.Create("echo "+strings.Repeat("x", i), dir, "", false, nil)
		require.NoError(t, err, "session %d", i)
		sessions[i] = sess
	}

	// Wait for first session to complete so it's evictable.
	sessions[0].Wait(5 * time.Second)
	require.True(t, sessions[0].Exited())

	// Creating one more should evict the oldest completed.
	sess, err := mgr.Create("echo new", dir, "", false, nil)
	require.NoError(t, err)
	require.NotNil(t, sess)

	// Original session 0 should no longer be gettable.
	_, ok := mgr.Get(sessions[0].ID)
	assert.False(t, ok)
}

func TestSessionManager_closed_manager(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	_ = mgr.Close()

	_, err := mgr.Create("echo hi", t.TempDir(), "", false, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session manager closed")
}

func TestSessionManager_custom_shell(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	defer func() { _ = mgr.Close() }()

	sess, err := mgr.Create("echo shell_test", t.TempDir(), "sh", false, nil)
	require.NoError(t, err)

	exited := sess.Wait(5 * time.Second)
	assert.True(t, exited)
	assert.Contains(t, sess.Output(), "shell_test")
}

func TestSession_WallTime(t *testing.T) {
	mgr := NewSessionManager(context.Background(), localExec{}, nil)
	defer func() { _ = mgr.Close() }()

	sess, err := mgr.Create("sleep 0.1", t.TempDir(), "", false, nil)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	assert.Greater(t, sess.WallTime(), 0.01)
}
