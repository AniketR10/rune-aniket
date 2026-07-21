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

package ideplan

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/ox-api/auth"
	"golang.org/x/oauth2"
	"unstable.build/go-tui/debug"
)

type stubCache struct {
	cached  *oauth2.Token
	refresh *oauth2.Token
	err     error
	calls   int
}

func (s *stubCache) Cached(context.Context) *oauth2.Token {
	s.calls++
	return s.cached
}

func (s *stubCache) Token() (*oauth2.Token, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	s.cached = s.refresh
	return s.refresh, nil
}

func makeJWT(t *testing.T, role auth.Role, planEnds time.Time) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	body := struct {
		Extra struct {
			Role     auth.Role `json:"Role"`
			PlanEnds time.Time `json:"plan_ends,omitzero"`
		} `json:"extra"`
	}{}
	body.Extra.Role = role
	body.Extra.PlanEnds = planEnds
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	return header + "." + base64.RawURLEncoding.EncodeToString(payload) + ".sig"
}

func TestJWTSourceDecision(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	planEnds := now.Add(-24 * time.Hour)

	t.Run("nil cache yields expired", func(t *testing.T) {
		src := NewJWTSource(&stubCache{})
		d, err := src.Decision(context.Background())
		require.NoError(t, err)
		assert.Equal(t, StatusExpired, d.Status)
		assert.Equal(t, SignedOut, d.SignedIn,
			"no cached token must report signed out")
	})

	t.Run("paid token yields active", func(t *testing.T) {
		cache := &stubCache{cached: &oauth2.Token{AccessToken: makeJWT(t, auth.RolePaid, now.Add(30*24*time.Hour))}}
		src := NewJWTSource(cache)
		d, err := src.Decision(context.Background())
		require.NoError(t, err)
		assert.Equal(t, StatusActive, d.Status)
	})

	t.Run("lapsed user is expired", func(t *testing.T) {
		cache := &stubCache{cached: &oauth2.Token{AccessToken: makeJWT(t, auth.RoleUser, planEnds)}}
		src := NewJWTSource(cache)
		d, err := src.Decision(context.Background())
		require.NoError(t, err)
		assert.Equal(t, StatusExpired, d.Status)
	})

	t.Run("malformed token yields expired without error", func(t *testing.T) {
		cache := &stubCache{cached: &oauth2.Token{AccessToken: "not-a-jwt"}}
		src := NewJWTSource(cache)
		d, err := src.Decision(context.Background())
		require.NoError(t, err)
		assert.Equal(t, StatusExpired, d.Status)
		assert.Equal(t, ParseClaimsError, d.SignedIn,
			"an unparseable token must report a parse-claims error")
	})

	t.Run("refresh propagates source error", func(t *testing.T) {
		cache := &stubCache{err: errors.New("network down")}
		src := NewJWTSource(cache)
		d, err := src.Refresh(context.Background())
		require.Error(t, err)
		assert.Equal(t, StatusExpired, d.Status)
		assert.Equal(t, SignedOut, d.SignedIn,
			"a refresh with no fallback token must report signed out")
	})

	t.Run("refresh re-evaluates with new token", func(t *testing.T) {
		cache := &stubCache{refresh: &oauth2.Token{AccessToken: makeJWT(t, auth.RolePaid, now.Add(30*24*time.Hour))}}
		src := NewJWTSource(cache)
		d, err := src.Refresh(context.Background())
		require.NoError(t, err)
		assert.Equal(t, StatusActive, d.Status)
	})

	t.Run("one-off token with build past entitlement is upgrade-expired", func(t *testing.T) {
		entitlement := now.Add(-24 * time.Hour)
		cache := &stubCache{cached: &oauth2.Token{AccessToken: makeJWT(t, auth.RoleOneOff, entitlement)}}
		src := NewJWTSource(cache)
		src.buildDate = now
		d, err := src.Decision(context.Background())
		require.NoError(t, err)
		assert.Equal(t, StatusUpgradeExpired, d.Status)
		assert.Equal(t, entitlement, d.PlanEnds)
	})

	t.Run("one-off token with build within entitlement is active", func(t *testing.T) {
		entitlement := now.Add(30 * 24 * time.Hour)
		cache := &stubCache{cached: &oauth2.Token{AccessToken: makeJWT(t, auth.RoleOneOff, entitlement)}}
		src := NewJWTSource(cache)
		src.buildDate = now
		d, err := src.Decision(context.Background())
		require.NoError(t, err)
		assert.Equal(t, StatusActive, d.Status)
	})
}

// TestBuildDateParsesInjectedStamp pins the ldflag -> parser contract:
// a value in the exact form cmd/buildstamp emits (debug.BuildDateLayout,
// UTC) must parse to the corresponding instant, and any empty or
// malformed injection must fall back to the zero time so a broken build
// stamp fails open (never locks) rather than mis-gating.
func TestBuildDateParsesInjectedStamp(t *testing.T) {
	orig := debug.BuildDate
	t.Cleanup(func() { debug.BuildDate = orig })

	t.Run("canonical utc stamp parses to the instant", func(t *testing.T) {
		debug.BuildDate = "2026-07-09T09:23:56Z"
		got := buildDate()
		assert.True(t, got.Equal(time.Date(2026, 7, 9, 9, 23, 56, 0, time.UTC)),
			"a buildstamp-format value must parse to its instant, got %s", got)
	})

	t.Run("empty stamp yields zero time", func(t *testing.T) {
		debug.BuildDate = ""
		assert.True(t, buildDate().IsZero(),
			"a development build (empty stamp) must never gate on build date")
	})

	t.Run("malformed stamp yields zero time", func(t *testing.T) {
		debug.BuildDate = "not-a-date"
		assert.True(t, buildDate().IsZero(),
			"an unparseable stamp must fail open, not lock every user")
	})
}
