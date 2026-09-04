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

package codex

import (
	"context"
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
		AccountID:    "account-123",
		PlanType:     "plus",
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
	assert.Equal(t, "account-123", status.AccountID)
	assert.True(t, status.HasRefreshToken)
}

func TestStatusMissingCredential(t *testing.T) {
	status, err := Status(context.Background(), storagestub.NewInMemoryService())
	require.NoError(t, err)
	assert.False(t, status.Authenticated)
}

func TestCredentialClientHeaders(t *testing.T) {
	headers := (Credential{AccountID: "account-123", FedRAMP: true}).ClientHeaders()
	assert.Equal(t, "account-123", headers["ChatGPT-Account-ID"])
	assert.Equal(t, "true", headers["X-OpenAI-Fedramp"])
}
