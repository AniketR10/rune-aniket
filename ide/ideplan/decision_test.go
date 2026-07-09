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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/ox-api/auth"
)

func TestDecide(t *testing.T) {
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	planEnds := now.Add(-24 * time.Hour)
	// buildDate defaults to the zero time (development build) unless a
	// case sets it; the zero time is never After any real planEnds, so
	// one-off cases that must lock set buildDate explicitly.
	cases := []struct {
		name         string
		role         auth.Role
		planEnds     time.Time
		buildDate    time.Time
		want         Status
		wantPlanEnds time.Time
		wantGrace    time.Time
		wantSignedIn SignInStatus
	}{
		{
			name:         "paid role is always active",
			role:         auth.RolePaid,
			planEnds:     planEnds,
			want:         StatusActive,
			wantPlanEnds: planEnds,
		},
		{
			name:         "paid role stays active even with a newer build",
			role:         auth.RolePaid,
			planEnds:     planEnds,
			buildDate:    now.Add(365 * 24 * time.Hour),
			want:         StatusActive,
			wantPlanEnds: planEnds,
		},
		{
			name:         "one-off build covered by entitlement is active",
			role:         auth.RoleOneOff,
			planEnds:     now.Add(30 * 24 * time.Hour),
			buildDate:    now,
			want:         StatusActive,
			wantPlanEnds: now.Add(30 * 24 * time.Hour),
		},
		{
			name:         "one-off build after entitlement is upgrade-expired",
			role:         auth.RoleOneOff,
			planEnds:     planEnds,
			buildDate:    now,
			want:         StatusUpgradeExpired,
			wantPlanEnds: planEnds,
		},
		{
			name:      "one-off with zero plan ends is expired",
			role:      auth.RoleOneOff,
			planEnds:  time.Time{},
			buildDate: now,
			want:      StatusExpired,
		},
		{
			name:     "admin is active regardless of plan",
			role:     auth.RoleAdmin,
			planEnds: time.Time{},
			want:     StatusActive,
		},
		{
			name:     "super admin is active regardless of plan",
			role:     auth.RoleSuperAdmin,
			planEnds: time.Time{},
			want:     StatusActive,
		},
		{
			name:     "user without plan ends is expired",
			role:     auth.RoleUser,
			planEnds: time.Time{},
			want:     StatusNeverSubscribed,
		},
		{
			name:         "user inside grace window",
			role:         auth.RoleUser,
			planEnds:     planEnds,
			want:         StatusGracePeriod,
			wantPlanEnds: planEnds,
			wantGrace:    planEnds.Add(GracePeriod),
		},
		{
			name:         "user past grace window is expired",
			role:         auth.RoleUser,
			planEnds:     now.Add(-8 * 24 * time.Hour),
			want:         StatusExpired,
			wantPlanEnds: now.Add(-8 * 24 * time.Hour),
			wantGrace:    now.Add(-8 * 24 * time.Hour).Add(GracePeriod),
		},
		{
			name:     "basic role is expired without plan",
			role:     auth.RoleBasic,
			planEnds: time.Time{},
			want:     StatusNeverSubscribed,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decide(tc.role, tc.planEnds, now, tc.buildDate)
			assert.Equal(t, tc.want, got.Status, "status")
			assert.Equal(t, tc.wantPlanEnds, got.PlanEnds, "plan ends")
			assert.Equal(t, tc.wantGrace, got.GraceUntil, "grace until")
			assert.Equal(t, tc.wantSignedIn, got.SignedIn,
				"decide must always report the signed-in axis")
		})
	}
}
