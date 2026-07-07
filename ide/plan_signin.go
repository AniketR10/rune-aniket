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

package ide

import (
	"context"
	"net/url"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
)

// SignInSession exposes the asynchronous state of an in-flight
// browser sign-in flow started by PlanSourceConfig.SignIn. URL emits
// the OAuth authorization URL once the flow computes it, then closes.
// Done receives the flow result (nil on success), then closes.
type SignInSession struct {
	URL  <-chan *url.URL
	Done <-chan error
}

// NopSignIn is the default PlanSourceConfig.SignIn: it resolves
// immediately with no browser flow. Used when no real login client is
// wired (pre-IDE, tests), where the plan source never reports expired
// so the flow is never actually reached.
func NopSignIn(context.Context) SignInSession {
	url := make(chan *url.URL)
	done := make(chan error)
	close(url)
	close(done)
	return SignInSession{URL: url, Done: done}
}

const (
	optSignInCopy   = "  Copy URL  "
	optSignInCancel = "  Cancel  "
)

// planSignInFlow owns the wait-prompt UI of one sign-in attempt. It is a
// small state machine — oauthURL (empty means "contacting"), copied, and
// dismissed — that renders the current state onto at most one surface: the
// lockdown overlay or a floating workspace prompt. All UI mutations run on
// the event loop via scheduleNextTick.
type planSignInFlow struct {
	ide    *IDE
	cancel context.CancelFunc

	mu        sync.Mutex
	oauthURL  string
	copied    bool
	dismissed bool
	win       browser.Window
}

// scheduleRender records an optional new OAuth URL and schedules a render
// of the current state on the event loop.
func (f *planSignInFlow) scheduleRender(oauthURL string) {
	f.ide.ideConfig.scheduleNextTick(func() {
		f.mu.Lock()
		if oauthURL != "" {
			f.oauthURL = oauthURL
		}
		f.mu.Unlock()
		f.render()
	})
}

// render draws the current prompt state onto the active surface. It runs on
// the event loop and keeps at most one surface live at a time.
func (f *planSignInFlow) render() {
	f.mu.Lock()
	if f.dismissed {
		f.mu.Unlock()
		return
	}
	msg, options, bindings := f.promptContentLocked()
	win := f.win
	f.win = nil
	f.mu.Unlock()
	if win != nil {
		_ = win.Close()
	}
	onSelect := func(_ int, opt string) {
		switch opt {
		case optSignInCopy:
			f.copyURL()
		case optSignInCancel:
			f.cancel()
			f.dismiss()
		}
	}
	spec := lockdownPromptSpec{
		message:  msg,
		options:  options,
		bindings: bindings,
		onSelect: onSelect,
	}
	if f.ide.planLockdown.Locked() {
		f.renderInOverlay(spec)
		return
	}
	f.renderInWorkspace(spec)
}

// renderInOverlay swaps the lockdown overlay prompt for the current state.
func (f *planSignInFlow) renderInOverlay(spec lockdownPromptSpec) {
	f.ide.planLockdown.setPrompt(newLockdownPrompt(f.ide.planPromptDeps, spec))
}

// renderInWorkspace opens the floating workspace prompt for the current
// state and records the window, closing it again if a dismiss raced the open.
func (f *planSignInFlow) renderInWorkspace(spec lockdownPromptSpec) {
	w := f.ide.Prompt(spec.message, spec.options, spec.bindings,
		handler.FuncPromptHandler(spec.onSelect, func() error { return nil }))
	f.mu.Lock()
	if f.dismissed {
		f.mu.Unlock()
		if w != nil {
			_ = w.Close()
		}
		return
	}
	f.win = w
	f.mu.Unlock()
}

func (f *planSignInFlow) promptContentLocked() (
	msg string, options []string, bindings []term.KeyComb,
) {
	if f.oauthURL == "" {
		return "**Signing you in.**\n\nContacting the sign-in service…",
			[]string{optSignInCancel}, []term.KeyComb{{Ch: 'c'}}
	}
	msg = "**Follow the instructions in your browser.**\n\n" +
		"If your browser did not open automatically, copy this link:\n\n" +
		"`" + f.oauthURL + "`"
	if f.copied {
		msg += "\n\nURL copied to clipboard."
	}
	return msg, []string{optSignInCopy, optSignInCancel},
		[]term.KeyComb{{Ch: 'p'}, {Ch: 'c'}}
}

func (f *planSignInFlow) copyURL() {
	f.mu.Lock()
	u := f.oauthURL
	f.mu.Unlock()
	if u == "" {
		return
	}
	err := f.ide.workspaceHandler.clip.Copy(
		clipboard.DefaultRegisterID, clipboard.Data{Text: u})
	if err != nil {
		log.WithError(err).Warn("plan sign-in: copy url")
		_, _ = f.ide.Notifications().Notify(browserapi.LevelWarn,
			"copy to clipboard failed: %v", err)
		return
	}
	f.mu.Lock()
	f.copied = true
	f.mu.Unlock()
	if f.ide.planLockdown.Locked() {
		f.render()
		return
	}
	_, _ = f.ide.Notifications().Notify(browserapi.LevelSuccess,
		"OAuth URL copied to clipboard")
}

func (f *planSignInFlow) dismiss() {
	f.ide.ideConfig.scheduleNextTick(func() {
		f.mu.Lock()
		if f.dismissed {
			f.mu.Unlock()
			return
		}
		f.dismissed = true
		win := f.win
		f.win = nil
		f.mu.Unlock()
		if win != nil {
			_ = win.Close()
		}
		if f.ide.planLockdown.Locked() {
			f.ide.planLockdown.setPrompt(
				newPlanLockdownPrompt(f.ide.planPromptDeps, f.ide.planLockdownReason()))
		}
	})
}
