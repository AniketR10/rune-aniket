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

package runenet

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
