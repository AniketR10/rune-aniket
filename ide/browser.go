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
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"

	"unstable.build/go-tui/browser"
)

type currentBrowser struct {
	root *workspaceManagerHandler
}

var _ browser.Browser = currentBrowser{}

func (c currentBrowser) browser() browser.Browser { return c.root.focusBrowser() }

func (c currentBrowser) Focus() (browser.Window, error) {
	return c.browser().Focus()
}

func (c currentBrowser) Split(
	o browserapi.Orientation, win browser.Window, h browserapi.Handler,
) (browser.Window, error) {
	return c.browser().Split(o, win, h)
}

func (c currentBrowser) Floating(
	h browser.Floating, cfg browserapi.FloatingConfig,
) (browser.Window, error) {
	return c.browser().Floating(h, cfg)
}

func (c currentBrowser) Bar(cfg browserapi.BarConfig, h tui.Handler) error {
	return c.browser().Bar(cfg, h)
}

func (c currentBrowser) Window(id uint64) (browser.Window, bool) {
	return c.browser().Window(id)
}

func (c currentBrowser) SetFocus(win browser.Window) (browser.Window, error) {
	return c.browser().SetFocus(win)
}

func (c currentBrowser) IterateWindows(fn func(browser.Window)) {
	c.browser().IterateWindows(fn)
}

func (c currentBrowser) Tab(
	uri workspaceapi.URI, icon rune, name string, h browserapi.Handler,
) (browserapi.Handler, error) {
	return c.browser().Tab(uri, icon, name, h)
}

func (c currentBrowser) SetTabName(
	uri workspaceapi.URI, name string, attr term.Attributes,
) error {
	return c.browser().SetTabName(uri, name, attr)
}

func (c currentBrowser) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return c.browser().Notify(level, msg, args...)
}

func (c currentBrowser) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return c.browser().NotifyOnce(level, msg, args...)
}

func (c currentBrowser) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	return c.browser().UpdateNotificationProgress(id, message, progress, total)
}

func (c currentBrowser) Open(uri workspaceapi.URI) (browserapi.Handler, error) {
	return c.browser().Open(uri)
}

func (c currentBrowser) Resource(uri workspaceapi.URI) (browserapi.Handler, bool) {
	return c.browser().Resource(uri)
}

func (c currentBrowser) PublishEvent(ev term.Event) error {
	return c.browser().PublishEvent(ev)
}

func (c currentBrowser) Close() error { return nil }
