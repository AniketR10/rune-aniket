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
