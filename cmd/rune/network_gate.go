// Copyright (C) 2017-2026 The Rune Authors
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
	"errors"
	"net/url"

	extbrowser "github.com/ernestrc/sensible/browser"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/auth"
	"unstable.build/rune/cmd/rune/ide/apiclient"
	"unstable.build/rune/internal/browser"
	"unstable.build/rune/internal/ide"
	"unstable.build/rune/internal/ide/idenag"
	"unstable.build/rune/internal/runenet"
)

const networkFeatureBlurb = "**The network is part of a paid Rune plan.**\n\n" +
	"It links the machines you sign in on into a private, encrypted network,\n" +
	"so any of them can be opened as a workspace with\n" +
	"`workspaceopen rune://<machine>/`.\n\n"

const networkSignedOutMessage = networkFeatureBlurb +
	"Sign in with the account that holds your plan, or create one to get started."

const networkUpgradeMessage = networkFeatureBlurb +
	"Upgrade to use it. Once you have, run the `login` command again so this\n" +
	"machine picks up your new plan."

const (
	networkOptSignIn  = "  Sign in  "
	networkOptSignUp  = "  Sign up  "
	networkOptUpgrade = "  Upgrade  "
	networkOptCancel  = "  Cancel  "
)

// networkAPI is the account-facing half of [apiclient.Client] the gate
// needs.
type networkAPI interface {
	AccountStatus(ctx context.Context) (auth.RPCUser, bool, error)
	NetworkCredentials(ctx context.Context) (controlURL, authKey string, err error)
}

// networkGate decides whether this machine may join the network. The
// entitlement is enforced by the API when it mints the credentials;
// the cached account claims are consulted first so an unentitled user
// is told what is missing instead of watching a request fail. The
// gate is pure entitlement logic: prompting the user to fix a failed
// check belongs to [networkPrompter].
type networkGate struct {
	client networkAPI
}

func newNetworkGate(client networkAPI) *networkGate {
	if client == nil {
		panic("newNetworkGate: client must not be nil")
	}
	return &networkGate{client: client}
}

// check reports whether the cached account claims cover the network.
// It performs no request, so it is safe to call from the event loop.
func (g *networkGate) check(ctx context.Context) error {
	user, signedIn, err := g.client.AccountStatus(ctx)
	if err != nil {
		// An undecodable token reads as signed out, so the user is
		// nudged towards a fresh sign-in rather than left stuck.
		return runenet.ErrNotAuthenticated
	}
	switch nagStateFromAccount(user, signedIn) {
	case idenag.StateActive:
		return nil
	case idenag.StateSignedOut:
		return runenet.ErrNotAuthenticated
	default:
		return runenet.ErrSubscriptionRequired
	}
}

// credentials satisfies [runenet.CredentialsFunc].
func (g *networkGate) credentials(
	ctx context.Context,
) (controlURL, authKey string, err error) {
	if err := g.check(ctx); err != nil {
		return "", "", err
	}
	controlURL, authKey, err = g.client.NetworkCredentials(ctx)
	switch {
	case errors.Is(err, auth.ErrNotAuthenticated):
		return "", "", runenet.ErrNotAuthenticated
	case errors.Is(err, apiclient.ErrSubscriptionRequired):
		return "", "", runenet.ErrSubscriptionRequired
	case err != nil:
		return "", "", err
	}
	return controlURL, authKey, nil
}

// networkPrompter turns an entitlement error into the sign-in or
// upgrade prompt. It is constructed from a live IDE, so it can always
// prompt; a nil dependency is a wiring bug, not a state to tolerate.
type networkPrompter struct {
	ide              *ide.IDE
	scheduleNextTick func(func()) bool
	signupURL        string
	checkoutURL      string
}

func newNetworkPrompter(
	i *ide.IDE, scheduleNextTick func(func()) bool,
) *networkPrompter {
	if i == nil {
		panic("newNetworkPrompter: ide must not be nil")
	}
	if scheduleNextTick == nil {
		panic("newNetworkPrompter: scheduleNextTick must not be nil")
	}
	signupURL, checkoutURL := mustResolveNagURLs(*flagWebsiteAddress)
	return &networkPrompter{
		ide:              i,
		scheduleNextTick: scheduleNextTick,
		signupURL:        signupURL,
		checkoutURL:      checkoutURL,
	}
}

// prompt offers the way out of err, and does nothing for errors that
// are not about entitlement.
func (p *networkPrompter) prompt(err error) {
	message, options, bindings, ok := networkPromptFor(err)
	if !ok {
		return
	}
	p.scheduleNextTick(func() {
		var win browser.Window
		win = p.ide.Prompt(message, options, bindings,
			handler.FuncPromptHandler(func(_ int, opt string) {
				if win != nil {
					_ = win.Close()
				}
				switch opt {
				case networkOptSignIn:
					if err := p.ide.DispatchCommand("console", "login"); err != nil {
						log.Warnf("network: dispatch login: %v", err)
					}
				case networkOptSignUp:
					openWebPage(p.signupURL)
				case networkOptUpgrade:
					openWebPage(p.checkoutURL)
				}
			}, func() error { return nil }),
		)
	})
}

// networkPromptFor maps an entitlement error to the prompt that
// resolves it. ok is false for every other error: a transport failure
// is not something the user can buy their way out of.
func networkPromptFor(err error) (
	message string, options []string, bindings []term.KeyComb, ok bool,
) {
	switch {
	case errors.Is(err, runenet.ErrNotAuthenticated):
		return networkSignedOutMessage,
			[]string{networkOptSignIn, networkOptSignUp, networkOptCancel},
			[]term.KeyComb{{Ch: 's'}, {Ch: 'u'}, {Ch: 'c'}}, true
	case errors.Is(err, runenet.ErrSubscriptionRequired):
		return networkUpgradeMessage,
			[]string{networkOptUpgrade, networkOptCancel},
			[]term.KeyComb{{Ch: 'u'}, {Ch: 'c'}}, true
	default:
		return "", nil, nil, false
	}
}

func openWebPage(raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		log.Warnf("network: parse url %q: %v", raw, err)
		return
	}
	if err := extbrowser.Browse(u); err != nil {
		log.Warnf("network: open browser: %v", err)
	}
}
