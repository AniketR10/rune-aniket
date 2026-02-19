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
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/browser"
)

var _ browserapi.Browser = (*browserAdapter)(nil)

func newBrowserAdapter(b browser.Browser) browserapi.Browser {
	return browserAdapter{b: b}
}

type browserAdapter struct {
	b browser.Browser
}

func (a browserAdapter) Focus() (
	browserapi.Window, error,
) {
	return a.b.Focus()
}

func (a browserAdapter) Split(
	o browserapi.Orientation,
	w browserapi.Window,
	h browserapi.Handler,
) (browserapi.Window, error) {
	bw, ok := a.b.Window(w.WindowID())
	if !ok {
		return nil, fmt.Errorf("window %d not found", w.WindowID())
	}
	return a.b.Split(o, bw, h)
}

func (a browserAdapter) Floating(
	h browserapi.Floating,
	cfg browserapi.FloatingConfig,
) (browserapi.Window, error) {
	return a.b.Floating(h, cfg)
}

func (a browserAdapter) Bar(
	cfg browserapi.BarConfig, h tui.Handler,
) error {
	return a.b.Bar(cfg, h)
}

func (a browserAdapter) Tab(
	uri workspaceapi.URI, icon rune, name string, h browserapi.Handler,
) (browserapi.Handler, error) {
	return a.b.Tab(uri, icon, name, h)
}

func (a browserAdapter) SetWindowContent(
	w browserapi.Window, h browserapi.Handler,
) error {
	bw, ok := a.b.Window(w.WindowID())
	if !ok {
		return fmt.Errorf("window %d not found", w.WindowID())
	}
	return bw.SetContent(h)
}

func (a browserAdapter) CloseWindow(
	w browserapi.Window,
) error {
	bw, ok := a.b.Window(w.WindowID())
	if !ok {
		return fmt.Errorf("window %d not found", w.WindowID())
	}
	return bw.Close()
}

func (a browserAdapter) Interrupt(
	ctx context.Context,
) error {
	return a.b.PublishEvent(term.Event{
		Type:    term.EventInterrupt,
		Context: ctx,
	})
}

func (a browserAdapter) PublishEventNone() error {
	return a.b.PublishEvent(term.Event{
		Type: term.EventNone,
	})
}

func (a browserAdapter) Open(
	uri workspaceapi.URI,
) (browserapi.Handler, error) {
	return a.b.Open(uri)
}

func (a browserAdapter) Notify(
	level browserapi.NotificationLevel,
	msg string,
	args ...any,
) (string, error) {
	return a.b.Notify(level, msg, args...)
}

func (a browserAdapter) NotifyOnce(
	level browserapi.NotificationLevel,
	msg string,
	args ...any,
) (string, error) {
	return a.b.NotifyOnce(level, msg, args...)
}

func (a browserAdapter) UpdateNotificationProgress(
	id, message string,
	progress, total int64,
) error {
	return a.b.UpdateNotificationProgress(
		id, message, progress, total,
	)
}

func (a browserAdapter) Close() error {
	return a.b.Close()
}
