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

//revive:disable:exported
package browsertest

import (
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/browser"
)

// WindowToAPIWindow is a convenience method to work
// with browserapi.Window and browser.Window in the same package.
type WindowToAPIWindow struct {
	Win browser.Window
}

var _ browserapi.Window = WindowToAPIWindow{}

func (a WindowToAPIWindow) SetContent(h browserapi.Handler) error {
	return a.Win.SetContent(h)
}

func (a WindowToAPIWindow) Focus() (bool, error) {
	return a.Win.Focus()
}

func (a WindowToAPIWindow) Close() error {
	return a.Win.Close()
}

func (a WindowToAPIWindow) Content() (browserapi.Handler, error) {
	return a.Win.Content()
}

func (a WindowToAPIWindow) WindowID() uint64 {
	return a.Win.WindowID()
}

// WindowFromAPIWindow is a convenience method to work
// with browserapi.Window and browser.Window in the same package.
type WindowFromAPIWindow struct {
	Browser browserapi.Browser
	Win     browserapi.Window
}

func (a WindowFromAPIWindow) SetContent(h browserapi.Handler) error {
	return a.Browser.SetWindowContent(a.Win, h)
}

func (a WindowFromAPIWindow) Content() (browserapi.Handler, error) {
	h, err := a.Win.(interface {
		Content() (browserapi.Browser, error)
	}).Content()
	return h.(browserapi.Handler), err
}

func (a WindowFromAPIWindow) WindowID() uint64 {
	return a.Win.WindowID()
}

func (a WindowFromAPIWindow) Focus() (bool, error) {
	return a.Win.(interface{ Focus() (bool, error) }).Focus()
}

func (a WindowFromAPIWindow) Close() error {
	return a.Browser.CloseWindow(a.Win)
}

func (w WindowFromAPIWindow) Closed() bool {
	return w.Win.(interface{ Closed() bool }).Closed()
}

func (w WindowFromAPIWindow) IsFloating() bool {
	return w.Win.(interface{ IsFloating() bool }).IsFloating()
}

func (w WindowFromAPIWindow) SetFrameAttr(t term.Attributes) (term.Attributes, bool) {
	return w.Win.(interface {
		SetFrameAttr(t term.Attributes) (term.Attributes, bool)
	}).SetFrameAttr(t)
}

// IsMinimized returns true if this is a floating window and it's minimized.
func (w WindowFromAPIWindow) IsMinimized() (component.Alignment, bool) {
	return w.Win.(interface {
		IsMinimized() (component.Alignment, bool)
	}).IsMinimized()
}

// MinimizeUp minimizes this window and displays it above the window manager,
// if this window is a floating window.
func (w WindowFromAPIWindow) MinimizeUp(padding int) bool {
	return w.Win.(interface{ MinimizeUp(int) bool }).MinimizeUp(padding)
}

// MinimizeDown minimizes this window and displays it below the window manager,
// if this window is a floating window.
func (w WindowFromAPIWindow) MinimizeDown(padding int) bool {
	return w.Win.(interface{ MinimizeDown(int) bool }).MinimizeDown(padding)
}

// MinimizeLeft minimizes this window and displays it left of the window manager,
// if this window is a floating window.
func (w WindowFromAPIWindow) MinimizeLeft(padding int) bool {
	return w.Win.(interface{ MinimizeLeft(int) bool }).MinimizeLeft(padding)
}

// MinimizeRight minimizes this window and displays it left of the window manager,
// if this window is a floating window.
func (w WindowFromAPIWindow) MinimizeRight(padding int) bool {
	return w.Win.(interface{ MinimizeRight(int) bool }).MinimizeRight(padding)
}

// Unminimize un-minimizes this window and displays it at the back at the front.
func (w WindowFromAPIWindow) Unminimize() bool {
	return w.Win.(interface{ Unminimize() bool }).Unminimize()
}

// Position returns the top-left coordinate of this window within
// its WindowManager.
func (w WindowFromAPIWindow) Position() term.Coordinates {
	p, ok := w.Win.(interface{ Position() term.Coordinates })
	if !ok {
		return term.Coordinates{}
	}
	return p.Position()
}

// Width returns the current rendered width of this window.
func (w WindowFromAPIWindow) Width() int {
	p, ok := w.Win.(interface{ Width() int })
	if !ok {
		return 0
	}
	return p.Width()
}

// Height returns the current rendered height of this window.
func (w WindowFromAPIWindow) Height() int {
	p, ok := w.Win.(interface{ Height() int })
	if !ok {
		return 0
	}
	return p.Height()
}
