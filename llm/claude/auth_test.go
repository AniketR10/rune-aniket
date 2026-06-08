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
