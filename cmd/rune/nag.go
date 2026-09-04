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
	"context"
	"fmt"
	"net/url"

	"github.com/unstablebuild/ox-api/auth"
	"unstable.build/rune/cmd/rune/ide/apiclient"
	"unstable.build/rune/ide"
	"unstable.build/rune/ide/idenag"
)

// nagPromptOption wires the weekly sign-in/upgrade reminder against
// the production API client and the configured website address.
func nagPromptOption(client *apiclient.Client) ide.Option {
	signupURL, checkoutURL := mustResolveNagURLs(*flagWebsiteAddress)
	return ide.WithNagPrompt(ide.NagPromptConfig{
		State:       nagState(client),
		SignupURL:   signupURL,
		CheckoutURL: checkoutURL,
	})
}

// nagState classifies the cached account claims for the weekly nag.
// An undecodable cached token reads as signed out so the user is
// nudged towards a fresh sign-in rather than left unclassified.
func nagState(client *apiclient.Client) func(context.Context) idenag.State {
	return func(ctx context.Context) idenag.State {
		user, ok, err := client.AccountStatus(ctx)
		if err != nil {
			return idenag.StateSignedOut
		}
		return nagStateFromAccount(user, ok)
	}
}

func nagStateFromAccount(user auth.RPCUser, signedIn bool) idenag.State {
	if !signedIn {
		return idenag.StateSignedOut
	}
	// RoleOneOff sorts above RolePaid but is not a subscription, so
	// entitlements are matched explicitly rather than via >=.
	switch user.Role {
	case auth.RolePaid, auth.RoleAdmin, auth.RoleSuperAdmin, auth.RoleOneOff:
		return idenag.StateActive
	}
	if user.PlanEnds.IsZero() {
		return idenag.StateNoPlan
	}
	return idenag.StateExpired
}

// mustResolveNagURLs resolves the signup and checkout pages against
// the configured website address. It panics on parse failure because
// in practice nobody sets -rune-website-address; the default points
// at production and a parse failure means the build itself is broken.
func mustResolveNagURLs(raw string) (signup, checkout string) {
	base, err := url.Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("parse -rune-website-address %q: %v", raw, err))
	}
	s := *base
	s.Path = "/signup"
	c := *base
	c.Path = "/checkout"
	q := c.Query()
	q.Set("source", "rune")
	c.RawQuery = q.Encode()
	return s.String(), c.String()
}
