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

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/ox-api/auth"
	"unstable.build/go-tui/ide/idenag"
)

func TestNagStateFromAccount(t *testing.T) {
	planEnds := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		user     auth.RPCUser
		signedIn bool
		want     idenag.State
	}{
		{"signed out", auth.RPCUser{}, false, idenag.StateSignedOut},
		{"basic never subscribed", auth.RPCUser{Role: auth.RoleBasic}, true, idenag.StateNoPlan},
		{"user never subscribed", auth.RPCUser{Role: auth.RoleUser}, true, idenag.StateNoPlan},
		{"lapsed plan", auth.RPCUser{Role: auth.RoleUser, PlanEnds: planEnds}, true, idenag.StateExpired},
		{"paid", auth.RPCUser{Role: auth.RolePaid, PlanEnds: planEnds}, true, idenag.StateActive},
		{"one-off", auth.RPCUser{Role: auth.RoleOneOff, PlanEnds: planEnds}, true, idenag.StateActive},
		{"admin", auth.RPCUser{Role: auth.RoleAdmin}, true, idenag.StateActive},
		{"super admin", auth.RPCUser{Role: auth.RoleSuperAdmin}, true, idenag.StateActive},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, nagStateFromAccount(tt.user, tt.signedIn))
		})
	}
}

func TestMustResolveNagURLs(t *testing.T) {
	signup, checkout := mustResolveNagURLs("https://rune.test")
	assert.Equal(t, "https://rune.test/signup", signup)
	assert.Equal(t, "https://rune.test/checkout?source=rune", checkout)
}
