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
	cases := []struct {
		name         string
		role         auth.Role
		planEnds     time.Time
		want         Status
		wantPlanEnds time.Time
		wantGrace    time.Time
	}{
		{
			name:         "paid role is always active",
			role:         auth.RolePaid,
			planEnds:     planEnds,
			want:         StatusActive,
			wantPlanEnds: planEnds,
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
			want:     StatusExpired,
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
			want:     StatusExpired,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := decide(tc.role, tc.planEnds, now)
			assert.Equal(t, tc.want, got.Status, "status")
			assert.Equal(t, tc.wantPlanEnds, got.PlanEnds, "plan ends")
			assert.Equal(t, tc.wantGrace, got.GraceUntil, "grace until")
		})
	}
}
