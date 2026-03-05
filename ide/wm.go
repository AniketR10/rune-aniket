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
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/browser"
)

// routes calls to WindowManager to the current open workspace
type currentWorkspaceWindowManager struct {
	root *workspaceManagerHandler
}

var _ browserapi.WindowManager = currentWorkspaceWindowManager{}

func (c currentWorkspaceWindowManager) Focus() (browserapi.Window, error) {
	return c.root.focusBrowser().Focus()
}

// Split splits the current window in focus in two, and installs
// Handler in the new window.
func (c currentWorkspaceWindowManager) Split(
	o browserapi.Orientation, win browserapi.Window, h browserapi.Handler,
) (browserapi.Window, error) {
	return c.root.focusBrowser().Split(o, win.(browser.Window), h)
}

// Floating creates a new floating window at coordinates,
// with static width and height.
func (c currentWorkspaceWindowManager) Floating(
	h browserapi.Floating, cfg browserapi.FloatingConfig,
) (browserapi.Window, error) {
	return c.root.focusBrowser().Floating(h, cfg)
}

// Bar creates a status bar with Orientation and Handler.
// Bars differ from Split and Floating windows in that they can't
// be in focus and can only receive mouse events.
func (c currentWorkspaceWindowManager) Bar(
	cfg browserapi.BarConfig, h tui.Handler,
) error {
	return c.root.focusBrowser().Bar(cfg, h)
}

// Tab creates a new tab with h and returns a handle that can be
// used with the rest of methods that take a browser.Handler.
// URI is used to uniquely identify a tab and name is used as a label
// to display it in the tab bar.
func (c currentWorkspaceWindowManager) Tab(
	uri workspaceapi.URI, icon rune, name string, h browserapi.Handler) (
	browserapi.Handler, error,
) {
	return c.root.focusBrowser().Tab(uri, icon, name, h)
}

// SetWindowContent sets the content of the given window to the given handler.
func (c currentWorkspaceWindowManager) SetWindowContent(
	win browserapi.Window, h browserapi.Handler,
) error {
	return win.(browser.Window).SetContent(h)
}

// Close closes the given window.
func (c currentWorkspaceWindowManager) CloseWindow(win browserapi.Window) error {
	return win.(browser.Window).Close()
}
