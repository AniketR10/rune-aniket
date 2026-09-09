// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"unstable.build/rune/auth"
	"unstable.build/rune/internal/ide/idenag"
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
