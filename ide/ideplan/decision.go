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

// Package ideplan implements the three-state subscription gating
// (active / grace period / expired) that Rune drives entirely from the
// JWT claim plan_ends. The package owns the decision function, a
// JWT-backed Source, the reusable upgrade prompt, the daily monitor
// that pins a soft-warning notification and signals the IDE lockdown
// wrapper, and the checkout URL helper.
package ideplan

import (
	"time"

	"github.com/unstablebuild/ox-api/auth"
)

// GracePeriod is the duration after PlanEnds during which a lapsed
// paid user keeps full IDE access while seeing a daily pinned warning
// notification. Past this window the lockdown wrapper takes over.
const GracePeriod = 7 * 24 * time.Hour

// Status is the gating state for a user at a moment in time.
type Status int

const (
	// StatusActive grants full IDE access. Either the user is at
	// RolePaid or above, or they are an admin/super-admin (which is
	// "sticky" — paid status is not required for staff).
	StatusActive Status = iota

	// StatusGracePeriod grants full IDE access but triggers a daily
	// pinned warning notification. Reached when a previously paid
	// user has lapsed (role dropped below paid) but PlanEnds+grace
	// has not yet elapsed.
	StatusGracePeriod

	// StatusExpired blocks IDE interaction behind the lockdown
	// prompt. Reached when the user is below paid AND either has
	// never been paid (PlanEnds is zero) or the grace window has
	// elapsed.
	StatusExpired
)

func (s Status) String() string {
	switch s {
	case StatusActive:
		return "active"
	case StatusGracePeriod:
		return "grace_period"
	case StatusExpired:
		return "expired"
	default:
		return "unknown"
	}
}

// Decision is the full result of evaluating gating against a user's
// claims. GraceUntil is meaningful only when Status == StatusGracePeriod
// (callers that need it for the warning copy in other states should
// inspect PlanEnds instead).
type Decision struct {
	Status     Status
	PlanEnds   time.Time
	GraceUntil time.Time
}

// decide returns the gating decision for the given role and PlanEnds at
// the given moment. Admins and super-admins are always StatusActive
// regardless of PlanEnds. Paid users are always StatusActive. Anyone
// else falls into StatusGracePeriod when PlanEnds is non-zero and
// PlanEnds+grace is still in the future, and StatusExpired otherwise.
func decide(role auth.Role, planEnds time.Time, now time.Time) Decision {
	if role >= auth.RolePaid {
		return Decision{Status: StatusActive, PlanEnds: planEnds}
	}
	if planEnds.IsZero() {
		return Decision{Status: StatusExpired}
	}
	graceUntil := planEnds.Add(GracePeriod)
	if now.Before(graceUntil) {
		return Decision{
			Status:     StatusGracePeriod,
			PlanEnds:   planEnds,
			GraceUntil: graceUntil,
		}
	}
	return Decision{Status: StatusExpired, PlanEnds: planEnds, GraceUntil: graceUntil}
}
