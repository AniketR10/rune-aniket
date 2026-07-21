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

// Package ideplan implements the subscription gating (active /
// expired) that Rune drives entirely from the JWT claim plan_ends.
// The package owns the decision function, a JWT-backed Source, the
// daily monitor that signals the IDE lockdown wrapper, and the
// checkout URL helper. Gating alone never locks: the idelockdown
// planner interprets the lock signal against usage evidence.
package ideplan

import (
	"time"

	"github.com/unstablebuild/ox-api/auth"
)

// Status is the gating state for a user at a moment in time.
type Status int

const (
	// StatusActive grants full IDE access. Either the user is at
	// RolePaid or above, or they are an admin/super-admin (which is
	// "sticky" — paid status is not required for staff).
	StatusActive Status = iota

	// StatusExpired marks a previously paid user whose plan has
	// lapsed (role dropped below paid).
	StatusExpired

	// StatusNeverSubscribed blocks IDE interaction like StatusExpired
	// but distinguishes a user who has never held a paid plan
	// (PlanEnds is zero) from one whose paid plan lapsed. Enforcement
	// is identical to StatusExpired; the distinction only drives the
	// lockdown prompt copy.
	StatusNeverSubscribed

	// StatusUpgradeExpired blocks IDE interaction like StatusExpired
	// but marks a one-off buyer running a build newer than their
	// upgrade entitlement (the compile-time build date is after
	// PlanEnds). Enforcement is identical to StatusExpired; the
	// distinction only drives the lockdown prompt copy, which offers
	// downgrade or renewal instead of subscription upgrade.
	StatusUpgradeExpired
)

func (s Status) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusExpired:
		return "expired"
	case StatusNeverSubscribed:
		return "never_subscribed"
	case StatusUpgradeExpired:
		return "upgrade_expired"
	default:
		return "unknown"
	}
}

// SignInStatus is the authentication axis of a Decision, orthogonal
// to Status. It distinguishes a signed-in user from one whose token
// is missing or unparseable, so the lockdown prompt can offer the
// right sign-in copy without conflating auth failures with plan
// state.
type SignInStatus int

const (
	// SignedIn means a token was present and its claims parsed. This
	// is the zero value so every Decision literal that omits the
	// field reads as signed in.
	SignedIn SignInStatus = iota
	// SignedOut means no token was cached.
	SignedOut
	// ParseClaimsError means a token was present but its claims could
	// not be decoded.
	ParseClaimsError
)

func (s SignInStatus) String() string {
	switch s {
	case SignedIn:
		return "signed_in"
	case SignedOut:
		return "signed_out"
	case ParseClaimsError:
		return "parse_claims_error"
	default:
		return "unknown"
	}
}

// Decision is the full result of evaluating gating against a user's
// claims.
type Decision struct {
	Status   Status
	SignedIn SignInStatus
	PlanEnds time.Time
}

func decide(role auth.Role, planEnds, buildDate time.Time) Decision {
	if role == auth.RoleOneOff {
		if planEnds.IsZero() {
			return Decision{Status: StatusExpired}
		}
		if buildDate.After(planEnds) {
			return Decision{Status: StatusUpgradeExpired, PlanEnds: planEnds}
		}
		return Decision{Status: StatusActive, PlanEnds: planEnds}
	}
	if role >= auth.RolePaid {
		return Decision{Status: StatusActive, PlanEnds: planEnds}
	}
	if planEnds.IsZero() {
		return Decision{Status: StatusNeverSubscribed}
	}
	return Decision{Status: StatusExpired, PlanEnds: planEnds}
}
