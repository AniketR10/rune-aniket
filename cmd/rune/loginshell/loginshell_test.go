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

package loginshell

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/ox-api/auth"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	sdkiterator "github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cmd/rune/ide/apiclient"
)

type fakeClient struct {
	loginURL  string
	loginErr  error
	logoutErr error
	logoutHit bool
	account   auth.RPCUser
	hasToken  bool
}

func (c *fakeClient) Login(context.Context) apiclient.LoginSession {
	urlCh := make(chan *url.URL, 1)
	done := make(chan error, 1)
	if c.loginURL != "" {
		u, _ := url.Parse(c.loginURL)
		urlCh <- u
	}
	close(urlCh)
	done <- c.loginErr
	close(done)
	return apiclient.LoginSession{URL: urlCh, Done: done}
}

func (c *fakeClient) Logout(context.Context) error {
	c.logoutHit = true
	return c.logoutErr
}

func (c *fakeClient) AccountStatus(context.Context) (auth.RPCUser, bool, error) {
	return c.account, c.hasToken, nil
}

func TestLoginStreamsURLThenSuccess(t *testing.T) {
	t.Parallel()

	_, h := Login(&fakeClient{loginURL: "https://auth.example.com/oauth?code=abc"})
	out := runCommand(t, h)

	require.Len(t, out, 2)
	assert.Contains(t, out[0], "auth.example.com/oauth")
	assert.Contains(t, out[1], "Login successful")
}

func TestLoginRendersPaidAccountStatus(t *testing.T) {
	t.Parallel()

	_, h := Login(&fakeClient{
		loginURL: "https://auth.example.com/oauth",
		hasToken: true,
		account: auth.RPCUser{
			Email:    "user@example.com",
			Role:     auth.RolePaid,
			PlanEnds: time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		},
	})
	out := runCommand(t, h)

	require.Len(t, out, 2)
	assert.Contains(t, out[1], "Login successful")
	assert.Contains(t, out[1], "user@example.com")
	assert.Contains(t, out[1], "Paid")
	assert.Contains(t, out[1], "2026-07-01")
}

func TestLoginRendersFreeAccountStatus(t *testing.T) {
	t.Parallel()

	_, h := Login(&fakeClient{
		loginURL: "https://auth.example.com/oauth",
		hasToken: true,
		account:  auth.RPCUser{Email: "free@example.com", Role: auth.RoleUser},
	})
	out := runCommand(t, h)

	require.Len(t, out, 2)
	assert.Contains(t, out[1], "free@example.com")
	assert.Contains(t, out[1], "Free")
	assert.Contains(t, out[1], "Upgrade")
}

func TestLoginReportsFailure(t *testing.T) {
	t.Parallel()

	_, h := Login(&fakeClient{
		loginURL: "https://auth.example.com/oauth",
		loginErr: errors.New("boom"),
	})
	out := runCommand(t, h)

	require.Len(t, out, 2)
	assert.Contains(t, out[0], "auth.example.com/oauth")
	assert.Contains(t, out[1], "Login did not complete")
	assert.Contains(t, out[1], "boom")
}

func TestLogoutReportsSuccess(t *testing.T) {
	t.Parallel()

	c := &fakeClient{}
	_, h := Logout(c)
	out := runCommand(t, h)

	assert.True(t, c.logoutHit)
	require.Len(t, out, 1)
	assert.Contains(t, out[0], "Logged out")
}

func TestLogoutPropagatesError(t *testing.T) {
	t.Parallel()

	_, h := Logout(&fakeClient{logoutErr: errors.New("nope")})
	_, err := h.HandleCommand(context.Background(), repl.Command{Name: "logout"},
		repl.NopProgressWriter())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "nope")
}

func runCommand(t *testing.T, h textapi.REPLHandler) []string {
	t.Helper()
	it, err := h.HandleCommand(context.Background(),
		repl.Command{}, repl.NopProgressWriter())
	require.NoError(t, err)
	items, err := sdkiterator.ToSlice(context.Background(), it)
	require.NoError(t, err)
	return responsiveStrings(t, items)
}

func responsiveStrings(t *testing.T, items []component.Responsive) []string {
	t.Helper()
	out := make([]string, 0, len(items))
	for _, item := range items {
		width := 120
		height := item.Height(width)
		if height <= 0 {
			height = 1
		}
		writer := term.NewStringWriter(width, height)
		item.Resize(width, height)
		item.Draw(writer)
		require.NoError(t, writer.Flush())
		out = append(out, strings.TrimSpace(writer.String()))
	}
	return out
}
