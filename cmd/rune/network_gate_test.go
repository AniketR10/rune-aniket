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

package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"unstable.build/rune/auth"
	"unstable.build/rune/cmd/rune/ide/apiclient"
	"unstable.build/rune/internal/ide"
	"unstable.build/rune/internal/runenet"
)

type stubNetworkAPI struct {
	user       auth.RPCUser
	signedIn   bool
	statusErr  error
	controlURL string
	authKey    string
	credErr    error
	credCalls  int
}

func (s *stubNetworkAPI) AccountStatus(context.Context) (auth.RPCUser, bool, error) {
	return s.user, s.signedIn, s.statusErr
}

func (s *stubNetworkAPI) NetworkCredentials(context.Context) (string, string, error) {
	s.credCalls++
	return s.controlURL, s.authKey, s.credErr
}

func TestNetworkGateCheck(t *testing.T) {
	planEnds := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tsuite := []struct {
		name string
		api  stubNetworkAPI
		want error
	}{
		{"signed out", stubNetworkAPI{}, runenet.ErrNotAuthenticated},
		{
			"undecodable token",
			stubNetworkAPI{statusErr: errors.New("bad jwt")},
			runenet.ErrNotAuthenticated,
		},
		{
			"never subscribed",
			stubNetworkAPI{user: auth.RPCUser{Role: auth.RoleUser}, signedIn: true},
			runenet.ErrSubscriptionRequired,
		},
		{
			"lapsed plan",
			stubNetworkAPI{
				user:     auth.RPCUser{Role: auth.RoleUser, PlanEnds: planEnds},
				signedIn: true,
			},
			runenet.ErrSubscriptionRequired,
		},
		{
			"paid",
			stubNetworkAPI{user: auth.RPCUser{Role: auth.RolePaid}, signedIn: true},
			nil,
		},
		{
			"one-off purchase",
			stubNetworkAPI{user: auth.RPCUser{Role: auth.RoleOneOff}, signedIn: true},
			nil,
		},
		{
			"admin",
			stubNetworkAPI{user: auth.RPCUser{Role: auth.RoleAdmin}, signedIn: true},
			nil,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			api := tcase.api
			gate := newNetworkGate(&api)
			err := gate.check(context.Background())
			if tcase.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.True(t, errors.Is(err, tcase.want), "got %v", err)
		})
	}
}

func TestNetworkGateCredentials(t *testing.T) {
	t.Run("mints credentials for an entitled account", func(t *testing.T) {
		api := &stubNetworkAPI{
			user:       auth.RPCUser{Role: auth.RolePaid},
			signedIn:   true,
			controlURL: "https://control.example.com",
			authKey:    "tskey-1",
		}
		controlURL, authKey, err := newNetworkGate(api).
			credentials(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "https://control.example.com", controlURL)
		assert.Equal(t, "tskey-1", authKey)
	})

	t.Run("does not ask for credentials without a plan", func(t *testing.T) {
		api := &stubNetworkAPI{
			user:     auth.RPCUser{Role: auth.RoleUser},
			signedIn: true,
		}
		_, _, err := newNetworkGate(api).credentials(context.Background())
		assert.True(t, errors.Is(err, runenet.ErrSubscriptionRequired), "got %v", err)
		assert.Zero(t, api.credCalls)
	})

	t.Run("maps the API entitlement errors", func(t *testing.T) {
		tsuite := []struct {
			name string
			err  error
			want error
		}{
			{"expired session", auth.ErrNotAuthenticated, runenet.ErrNotAuthenticated},
			{
				"plan lapsed server-side",
				apiclient.ErrSubscriptionRequired,
				runenet.ErrSubscriptionRequired,
			},
		}
		for _, tcase := range tsuite {
			t.Run(tcase.name, func(t *testing.T) {
				api := &stubNetworkAPI{
					user:     auth.RPCUser{Role: auth.RolePaid},
					signedIn: true,
					credErr:  tcase.err,
				}
				_, _, err := newNetworkGate(api).credentials(context.Background())
				assert.True(t, errors.Is(err, tcase.want), "got %v", err)
			})
		}
	})
}

func TestNetworkPromptFor(t *testing.T) {
	message, options, bindings, ok := networkPromptFor(runenet.ErrNotAuthenticated)
	require.True(t, ok)
	assert.Equal(t, networkSignedOutMessage, message)
	assert.Equal(t,
		[]string{networkOptSignIn, networkOptSignUp, networkOptCancel}, options)
	assert.Len(t, bindings, len(options))

	message, options, _, ok = networkPromptFor(runenet.ErrSubscriptionRequired)
	require.True(t, ok)
	assert.Equal(t, networkUpgradeMessage, message)
	assert.Equal(t, []string{networkOptUpgrade, networkOptCancel}, options)

	_, _, _, ok = networkPromptFor(errors.New("network unreachable"))
	assert.False(t, ok)
}

// The gate, prompter, and network take only mandatory dependencies. A
// nil is a wiring bug that must crash at construction, never a state
// the code quietly tolerates.
func TestNetworkNilDependenciesPanic(t *testing.T) {
	assert.Panics(t, func() { newNetworkGate(nil) })
	assert.Panics(t, func() {
		newNetwork(config.NopConfig(), t.TempDir(), nil)
	})
	assert.Panics(t, func() {
		newNetworkPrompter(nil, func(func()) bool { return true })
	})
	assert.Panics(t, func() { newNetworkPrompter(&ide.IDE{}, nil) })
}

// The stack is fully assembled whatever the configuration says —
// auto_join only decides whether boot joins — so `network up` can
// join later without restarting Rune.
func TestNewNetworkAlwaysAssembles(t *testing.T) {
	tsuite := []struct {
		name string
		cfg  map[string]any
	}{
		{"no network section", map[string]any{}},
		{"auto_join off", map[string]any{
			"network": map[string]any{"auto_join": false},
		}},
		{"malformed section falls back to defaults", map[string]any{
			"network": map[string]any{"port": "not-a-number"},
		}},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			net := newNetwork(config.MapConfig(tcase.cfg), t.TempDir(),
				newNetworkGate(&stubNetworkAPI{}))
			t.Cleanup(func() { _ = net.Close() })
			require.NotNil(t, net.node)
			require.NotNil(t, net.gate)
			assert.False(t, net.autoJoin)
		})
	}
}
