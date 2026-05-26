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

// Package ideplan defines the shared plan-gating types used across
// the IDE: a Source interface that decides whether the current user
// may use gated features, the Decision returned by that interface,
// and the ErrSubscriptionRequired sentinel that callers wrap to
// signal "logged in but not subscribed" or "logged out".
package ideplan

import (
	"context"
	"errors"
	"time"
)

// Kind identifies the outcome of a plan check.
type Kind int

const (
	// Allowed means the user holds a valid paid subscription.
	Allowed Kind = iota
	// GracePeriod means the user's last-seen plan was paid but the
	// persisted token is expired within an offline grace window.
	GracePeriod
	// Denied means the user is logged out, not subscribed, or past
	// the offline grace window.
	Denied
)

// Decision is the result returned by a Source.
type Decision struct {
	Kind           Kind
	Reason         string
	GraceExpiresAt time.Time
}

// Source decides whether the current user is allowed to use
// plan-gated IDE features (extensions, package downloads, runectl
// RPCs).
type Source interface {
	PlanDecision(ctx context.Context) Decision
}

// ErrSubscriptionRequired is returned by gated paths when a Source
// reports Denied. Callers typically wrap this with a transport
// sentinel (e.g. blueauth.ErrForbidden for RPCs, idepkg.ErrForbidden
// for downloads) so existing branches keep working alongside
// errors.Is(err, ideplan.ErrSubscriptionRequired).
var ErrSubscriptionRequired = errors.New("subscription required: " +
	"run the `login` command and ensure your account has an active subscription")

// DefaultGraceWindow is the duration after a persisted token's Expiry
// during which a previously-paid user can keep using gated features
// while offline.
const DefaultGraceWindow = 7 * 24 * time.Hour

// AlwaysAllowed returns a Source that always reports Allowed. Intended
// for tests and self-hosted setups where the IDE should not gate
// features on subscription status.
func AlwaysAllowed() Source { return alwaysAllowed{} }

type alwaysAllowed struct{}

func (alwaysAllowed) PlanDecision(context.Context) Decision {
	return Decision{Kind: Allowed}
}
