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

package claude

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

func TestSaveLoadStatusCredential(t *testing.T) {
	ctx := context.Background()
	db := storagestub.NewInMemoryService()
	expiry := time.Now().Add(time.Hour).UTC()
	lastRefresh := time.Now().Add(-time.Minute).UTC()
	require.NoError(t, SaveCredential(ctx, db, Credential{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		Email:        "user@example.com",
		AccountID:    "org-123",
		PlanType:     "max",
		Expiry:       expiry,
		LastRefresh:  lastRefresh,
	}))

	cred, err := LoadCredential(ctx, db)
	require.NoError(t, err)
	assert.Equal(t, "access-token", cred.AccessToken)
	assert.Equal(t, "user@example.com", cred.Email)

	status, err := Status(ctx, db)
	require.NoError(t, err)
	assert.True(t, status.Authenticated)
	assert.False(t, status.Expired)
	assert.Equal(t, "user@example.com", status.Email)
	assert.Equal(t, "org-123", status.AccountID)
	assert.Equal(t, "max", status.PlanType)
	assert.True(t, status.HasRefreshToken)
}

func TestStatusMissingCredential(t *testing.T) {
	status, err := Status(context.Background(), storagestub.NewInMemoryService())
	require.NoError(t, err)
	assert.False(t, status.Authenticated)
}

func TestCredentialClientHeaders(t *testing.T) {
	headers := Credential{}.ClientHeaders()
	assert.Equal(t, agentBetaHeader, headers["anthropic-beta"])
	assert.Contains(t, headers["anthropic-beta"], "oauth-2025-04-20")
	assert.Equal(t, anthropicVersion, headers["anthropic-version"])
}

func TestLoginEndToEnd(t *testing.T) {
	ctx := context.Background()
	db := storagestub.NewInMemoryService()

	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var got map[string]string
		require.NoError(t, json.Unmarshal(body, &got))
		assert.Equal(t, "authorization_code", got["grant_type"])
		assert.Equal(t, "the-code", got["code"])
		assert.NotEmpty(t, got["code_verifier"])
		assert.NotEmpty(t, got["state"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{
			"access_token": "claude-access",
			"refresh_token": "claude-refresh",
			"token_type": "Bearer",
			"expires_in": 3600,
			"scope": "user:inference",
			"account": {"uuid": "acc-1", "email_address": "dev@example.com"},
			"organization": {"uuid": "org-1", "subscription_type": "max_20x"}
		}`)
	}))
	defer tokenSrv.Close()

	restore := overrideTokenURL(tokenSrv.URL)
	defer restore()

	session, err := StartLogin(ctx, db,
		WithBrowserOpener(nil),
		WithHTTPClient(tokenSrv.Client()),
		WithCallbackAddr("127.0.0.1:0", ""),
	)
	require.NoError(t, err)
	defer func() { _ = session.Close() }()

	authURL, err := url.Parse(session.AuthURL())
	require.NoError(t, err)
	state := authURL.Query().Get("state")
	require.NotEmpty(t, state)

	go func() {
		callbackURL := "http://" + session.listener.Addr().String() +
			"/callback?code=the-code&state=" + url.QueryEscape(state)
		resp, derr := tokenSrv.Client().Get(callbackURL)
		if derr == nil {
			_ = resp.Body.Close()
		}
	}()

	cred, err := session.Wait(ctx)
	require.NoError(t, err)
	assert.Equal(t, "claude-access", cred.AccessToken)
	assert.Equal(t, "claude-refresh", cred.RefreshToken)
	assert.Equal(t, "dev@example.com", cred.Email)
	assert.Equal(t, "org-1", cred.AccountID)
	assert.Equal(t, "max_20x", cred.PlanType)

	status, err := Status(ctx, db)
	require.NoError(t, err)
	assert.True(t, status.Authenticated)
	assert.Equal(t, "dev@example.com", status.Email)
}
