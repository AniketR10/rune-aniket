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
	"context"
	"fmt"
	"net/url"

	"github.com/unstablebuild/ox-api/auth"
	"unstable.build/go-tui/cmd/rune/ide/apiclient"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/ide/idenag"
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
