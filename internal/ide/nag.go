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

package ide

import (
	"context"
	"net/url"

	"github.com/ernestrc/sensible/browser"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	sdkbrowser "unstable.build/rune/internal/browser"
	"unstable.build/rune/internal/ide/idenag"
)

// NagPromptConfig collects the dependencies WithNagPrompt needs to
// run the weekly sign-in/upgrade reminder. State classifies the
// account standing when the timer fires; SignupURL is opened by the
// signed-out prompt's Sign up button; CheckoutURL is opened by the
// Upgrade button of the no-plan and expired-plan prompts.
type NagPromptConfig struct {
	State       func(context.Context) idenag.State
	SignupURL   string
	CheckoutURL string
}

// WithNagPrompt enables the weekly sign-in/upgrade reminder prompt.
// Callers that want no reminders omit this option.
func WithNagPrompt(cfg NagPromptConfig) Option {
	if cfg.State == nil {
		panic("ide.WithNagPrompt: nil State")
	}
	return func(opts *options) {
		opts.nagPrompt = cfg
	}
}

const nagSignedOutMessage = "**Enjoying Rune?**\n\n" +
	"Rune is free to use, at work and at home. Create a free account to hear\n" +
	"about new releases and to participate in what we're building next.\n\n" +
	"It takes a minute, and it means a lot to a small, self-funded company."

const nagNoPlanMessage = "**Rune seems to be working out for you.**\n\n" +
	"Much of the industry is betting that we won't be writing code for much longer.\n" +
	"We are betting that the technologists, the systems programmers, the\n" +
	"hackers who love this craft, and the ones who'd rather read the source than the\n" +
	"docs, will outlive every company betting against them. " +
	"When prod is down at 3am, nobody pages the product manager.\n\n" +
	"Rune Pro supports a small, self-funded company that wants to keep you in the driver's seat."

const nagExpiredMessage = "**Your Rune Pro plan has expired.**\n\n" +
	"Thank you for backing Rune. Support from people like you is what keeps\n" +
	"a small, self-funded company independent. Rune keeps working; renewing\n" +
	"restores Rune Network and premium support."

const (
	nagOptSignIn  = "  Sign in  "
	nagOptSignUp  = "  Sign up  "
	nagOptUpgrade = "  Upgrade  "
	nagOptCancel  = "  Cancel  "
)

// nagPromptOpener returns the Show callback the nag scheduler
// invokes when the weekly timer fires for a non-active account.
func (i *IDE) nagPromptOpener(cfg NagPromptConfig) func(idenag.State) {
	return func(state idenag.State) {
		switch state {
		case idenag.StateSignedOut:
			i.openSignedOutNagPrompt(cfg)
		case idenag.StateNoPlan:
			i.openUpgradeNagPrompt(nagNoPlanMessage, cfg.CheckoutURL)
		case idenag.StateExpired:
			i.openUpgradeNagPrompt(nagExpiredMessage, cfg.CheckoutURL)
		}
	}
}

// openSignedOutNagPrompt offers a browser sign-up alongside the
// console `login` flow, which already owns the OAuth URL streaming
// and account-status UX.
func (i *IDE) openSignedOutNagPrompt(cfg NagPromptConfig) {
	i.ideConfig.scheduleNextTick(func() {
		var win sdkbrowser.Window
		win = i.Prompt(nagSignedOutMessage,
			[]string{nagOptSignIn, nagOptSignUp, nagOptCancel},
			[]term.KeyComb{{Ch: 's'}, {Ch: 'u'}, {Ch: 'c'}},
			handler.FuncPromptHandler(func(_ int, opt string) {
				if win != nil {
					_ = win.Close()
				}
				switch opt {
				case nagOptSignIn:
					i.workspaceHandler.focusEx().Dispatch("console", "login")
				case nagOptSignUp:
					openBrowser(cfg.SignupURL)
				}
			}, func() error { return nil }),
		)
	})
}

func (i *IDE) openUpgradeNagPrompt(message, checkoutURL string) {
	i.ideConfig.scheduleNextTick(func() {
		var win sdkbrowser.Window
		win = i.Prompt(message,
			[]string{nagOptUpgrade, nagOptCancel},
			[]term.KeyComb{{Ch: 'u'}, {Ch: 'c'}},
			handler.FuncPromptHandler(func(_ int, opt string) {
				if win != nil {
					_ = win.Close()
				}
				if opt == nagOptUpgrade {
					openBrowser(checkoutURL)
				}
			}, func() error { return nil }),
		)
	})
}

func openBrowser(raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		log.WithError(err).Warnf("idenag: parse url %q", raw)
		return
	}
	if err := browser.Browse(u); err != nil {
		log.WithError(err).Warn("idenag: open browser")
	}
}
