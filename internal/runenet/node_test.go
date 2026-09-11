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

package runenet

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"tailscale.com/ipn"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tailcfg"
)

func TestStartCredentials(t *testing.T) {
	t.Run("propagates the entitlement error without joining", func(t *testing.T) {
		node := New(Config{
			Hostname: "workstation",
			Port:     DefaultPort,
			Dir:      t.TempDir(),
			Credentials: func(context.Context) (string, string, error) {
				return "", "", ErrSubscriptionRequired
			},
		})
		t.Cleanup(func() { _ = node.Close() })

		err := node.Start(context.Background())
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrSubscriptionRequired))

		_, err = node.Peers(context.Background())
		assert.True(t, errors.Is(err, ErrNotStarted))
	})

	t.Run("records the failure for status", func(t *testing.T) {
		credErr := errors.New("credentials endpoint unreachable")
		node := New(Config{
			Hostname: "workstation",
			Port:     DefaultPort,
			Dir:      t.TempDir(),
			Credentials: func(context.Context) (string, string, error) {
				return "", "", credErr
			},
		})
		t.Cleanup(func() { _ = node.Close() })

		require.Error(t, node.Start(context.Background()))

		st, err := node.Status(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "NotRunning", st.State)
		assert.Equal(t, credErr.Error(), st.LastError)
	})

	t.Run("joins with the minted credentials", func(t *testing.T) {
		var gotCtxValue any
		node := New(Config{
			Hostname: "workstation",
			Port:     DefaultPort,
			Dir:      t.TempDir(),
			Credentials: func(ctx context.Context) (string, string, error) {
				gotCtxValue = ctx.Value(credentialsCtxKey{})
				return "https://control.example.com", "tskey-auth-secret", nil
			},
		})
		t.Cleanup(func() { _ = node.Close() })

		require.NoError(t, node.Start(context.WithValue(
			context.Background(), credentialsCtxKey{}, "called")))
		assert.Equal(t, "called", gotCtxValue)
		assert.Equal(t, "https://control.example.com", node.srv.ControlURL)
		assert.Equal(t, "tskey-auth-secret", node.srv.AuthKey)
	})
}

// Node keys expire on purpose: re-registering is what re-checks the
// account and the machines its plan covers, so a backend waiting for
// a login must mint fresh credentials rather than wait for a browser
// login the mesh never asks a Rune user for.
func TestNeedsReauth(t *testing.T) {
	assert.True(t, needsReauth(&ipnstate.Status{
		BackendState: ipn.NeedsLogin.String()}))
	assert.False(t, needsReauth(&ipnstate.Status{
		BackendState: ipn.Running.String()}))
	assert.False(t, needsReauth(nil))
}

func TestReauthenticate(t *testing.T) {
	t.Run("is a no-op without a credentials source", func(t *testing.T) {
		node := New(Config{
			Hostname: "workstation", Port: DefaultPort, Dir: t.TempDir(),
			AuthKey: "tskey-static",
		})
		t.Cleanup(func() { _ = node.Close() })
		assert.NoError(t, node.reauthenticate(context.Background()))
	})

	t.Run("needs a running node", func(t *testing.T) {
		node := New(Config{
			Hostname: "workstation", Port: DefaultPort, Dir: t.TempDir(),
			Credentials: func(context.Context) (string, string, error) {
				return "https://control.example.com", "tskey-auth-secret", nil
			},
		})
		t.Cleanup(func() { _ = node.Close() })
		assert.True(t, errors.Is(
			node.reauthenticate(context.Background()), ErrNotStarted))
	})
}

// A machine removed while it slept wakes up on a netmap the
// coordination server has already forgotten, so Rejoin must re-mint
// without waiting for the backend to admit it is logged out.
func TestRejoin(t *testing.T) {
	t.Run("is a no-op without a credentials source", func(t *testing.T) {
		node := New(Config{
			Hostname: "workstation", Port: DefaultPort, Dir: t.TempDir(),
			AuthKey: "tskey-static",
		})
		t.Cleanup(func() { _ = node.Close() })
		assert.NoError(t, node.Rejoin(context.Background()))
	})

	t.Run("needs a running node", func(t *testing.T) {
		node := New(Config{
			Hostname: "workstation", Port: DefaultPort, Dir: t.TempDir(),
			Credentials: func(context.Context) (string, string, error) {
				return "https://control.example.com", "tskey-auth-secret", nil
			},
		})
		t.Cleanup(func() { _ = node.Close() })
		assert.True(t, errors.Is(
			node.Rejoin(context.Background()), ErrNotStarted))
	})
}

// Only the move into NeedsLogin is news. The bus repeats the current
// state every time the watch is re-established, and a machine that is
// logged out stays logged out.
func TestEnteredNeedsLogin(t *testing.T) {
	cases := []struct {
		name string
		prev ipn.State
		next ipn.State
		want bool
	}{
		{"from running", ipn.Running, ipn.NeedsLogin, true},
		{"from no state", ipn.NoState, ipn.NeedsLogin, true},
		{"from starting", ipn.Starting, ipn.NeedsLogin, true},
		{"repeated on re-watch", ipn.NeedsLogin, ipn.NeedsLogin, false},
		{"joining", ipn.NeedsLogin, ipn.Starting, false},
		{"joined", ipn.Starting, ipn.Running, false},
		{"stopped", ipn.Running, ipn.Stopped, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, enteredNeedsLogin(tc.prev, tc.next))
		})
	}
}

// The machine list the account server returns is keyed by this id, so
// it is what tells this machine whether it is still one of the
// account's. A node that has not registered has none.
func TestSelfMachineID(t *testing.T) {
	assert.Equal(t, "", selfMachineID(nil))
	assert.Equal(t, "", selfMachineID(&ipnstate.Status{}))
	assert.Equal(t, "42", selfMachineID(&ipnstate.Status{
		Self: &ipnstate.PeerStatus{ID: tailcfg.StableNodeID("42")},
	}))
}

func TestStatusBeforeStart(t *testing.T) {
	node := New(Config{
		Hostname: "workstation",
		Port:     DefaultPort,
		Dir:      t.TempDir(),
	})
	t.Cleanup(func() { _ = node.Close() })

	st, err := node.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "workstation", st.Hostname)
	assert.Equal(t, "NotRunning", st.State)
	assert.Empty(t, st.LastError)
}

type credentialsCtxKey struct{}
