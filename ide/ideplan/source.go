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
	"fmt"
	"strings"
	"time"

	"github.com/unstablebuild/ox-api/auth"
	"golang.org/x/oauth2"
)

// Source provides plan-gating decisions.
type Source interface {
	// Decision returns the gating decision from the cached token
	// without forcing a refresh. No token yet yields StatusExpired
	// with zero PlanEnds; bootstrap relies on this to represent the
	// "OAuth in progress" state.
	Decision(ctx context.Context) (Decision, error)

	// Refresh rotates the access token via the refresh_token grant
	// and re-evaluates gating. Daily-monitor use only: it must NOT
	// be used to switch identities or pick up a fresh plan — those
	// require a purge + browser login.
	Refresh(ctx context.Context) (Decision, error)
}

// CachedSource is the minimum surface JWTSource needs from a token
// cache. *auth.CachedTokenSource satisfies it.
type CachedSource interface {
	Cached(ctx context.Context) *oauth2.Token
	Token() (*oauth2.Token, error)
}

// NewJWTSource wraps an oauth2 token cache as a plan Source. now is
// indirected so tests can drive the grace-window computation
// deterministically; pass time.Now in production.
func NewJWTSource(cache CachedSource, now func() time.Time) *JWTSource {
	if now == nil {
		now = time.Now
	}
	return &JWTSource{cache: cache, now: now}
}

// JWTSource is the production Source. Signatures are not verified
// client-side: ox-api verified them before issuing the token.
type JWTSource struct {
	cache CachedSource
	now   func() time.Time
}

// NopSource is the default Source used when no plan gating is
// configured: every Decision/Refresh resolves to StatusActive so the
// IDE never enters the lockdown overlay or grace-period warning.
type NopSource struct{}

// Decision always returns StatusActive.
func (NopSource) Decision(context.Context) (Decision, error) {
	return Decision{Status: StatusActive}, nil
}

// Refresh always returns StatusActive.
func (NopSource) Refresh(context.Context) (Decision, error) {
	return Decision{Status: StatusActive}, nil
}

// Decision returns StatusExpired (without an error) when no token is
// cached so the bootstrap flow can drive a single prompt across "not
// signed in" and "signed in but not paid".
func (s *JWTSource) Decision(ctx context.Context) (Decision, error) {
	tok := s.cache.Cached(ctx)
	return s.decideFromToken(tok), nil
}

// Refresh is the daily keep-alive path: rotate the token via the
// existing session rather than starting a new browser login.
func (s *JWTSource) Refresh(ctx context.Context) (Decision, error) {
	tok, err := s.cache.Token()
	if err != nil {
		// Fall back to the cached token when Token() cannot mint
		// one (e.g. mid-bootstrap) so the "OAuth in progress"
		// prompt still works.
		tok = s.cache.Cached(ctx)
		if tok == nil {
			return Decision{Status: StatusExpired, SignedIn: SignedOut},
				fmt.Errorf("refresh token: %w", err)
		}
	}
	return s.decideFromToken(tok), nil
}

func (s *JWTSource) decideFromToken(tok *oauth2.Token) Decision {
	if tok == nil || tok.AccessToken == "" {
		return Decision{Status: StatusExpired, SignedIn: SignedOut}
	}
	role, planEnds, err := parseClaims(tok.AccessToken)
	if err != nil {
		return Decision{Status: StatusExpired, SignedIn: ParseClaimsError}
	}
	return decide(role, planEnds, s.now())
}

type jwtPayload struct {
	Extra struct {
		Role     auth.Role `json:"Role"`
		PlanEnds time.Time `json:"plan_ends"`
	} `json:"extra"`
}

// parseClaims decodes the JWT payload without verifying the
// signature: ox-api verifies tokens server-side.
func parseClaims(token string) (auth.Role, time.Time, error) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return 0, time.Time{}, fmt.Errorf("jwt: expected at least 2 segments, got %d", len(parts))
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("jwt: decode payload: %w", err)
	}
	var p jwtPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return 0, time.Time{}, fmt.Errorf("jwt: unmarshal payload: %w", err)
	}
	return p.Extra.Role, p.Extra.PlanEnds, nil
}
