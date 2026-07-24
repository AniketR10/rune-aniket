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

package idetutorial

import (
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/browser"
)

// OverlayBrowser owns the dedicated [browser.Component] that hosts
// tutorial step windows above the IDE root.
type OverlayBrowser struct {
	mu sync.Mutex
	c  *browser.Component
	// wins preserves opening order so Draw paints deterministically:
	// oldest first, newest on top.
	wins []browser.Window
	// dragging pins mouse routing to the browser between a press
	// inside a window and the matching release so a window drag that
	// leaves the window rectangle is not dropped mid-gesture.
	dragging bool
}

// NewOverlayBrowser wraps c, which must be used exclusively through
// the returned OverlayBrowser from then on.
func NewOverlayBrowser(c *browser.Component) *OverlayBrowser {
	return &OverlayBrowser{c: c}
}

// DefaultOverlayBrowserConfig returns the browser configuration the
// overlay expects: the window-manager frame stays enabled so
// window-bar interactions (close, drag, maximize) work, the unused
// tab bar shrinks to a single row so windows can sit near the top of
// the screen, and frame unioning is off because only the windows are
// ever drawn.
func DefaultOverlayBrowserConfig() browser.Config {
	cfg := browser.DefaultConfig()
	cfg.TabBarHeight = 1
	cfg.FrameUnion = false
	return cfg
}

// Floating opens a floating window on the overlay browser.
func (b *OverlayBrowser) Floating(
	h browser.Floating, cfg browserapi.FloatingConfig,
) browser.Window {
	b.mu.Lock()
	defer b.mu.Unlock()
	win := b.c.Floating(h, cfg)
	b.wins = append(b.wins, win)
	return win
}

// Prompt opens a confirm/choice prompt window on the overlay browser.
func (b *OverlayBrowser) Prompt(
	message string, options []string, bindings []term.KeyComb,
	ph handler.PromptHandler,
) browser.Window {
	b.mu.Lock()
	defer b.mu.Unlock()
	win := b.c.Prompt(message, options, bindings, ph)
	b.wins = append(b.wins, win)
	return win
}

// CloseWindow closes win through the browser's bookkeeping path. Safe
// to call with a nil or already-closed window.
func (b *OverlayBrowser) CloseWindow(win browser.Window) {
	if win == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if !win.Closed() {
		_ = win.Close()
	}
	b.pruneLocked()
}

// CloseAll closes every window the overlay opened.
func (b *OverlayBrowser) CloseAll() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, win := range b.wins {
		if !win.Closed() {
			_ = win.Close()
		}
	}
	b.wins = nil
}

// Windows reports the number of live overlay windows.
func (b *OverlayBrowser) Windows() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked()
	return len(b.wins)
}

// Resize forwards the full screen dimensions to the browser.
func (b *OverlayBrowser) Resize(width, height int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.c.Resize(width, height)
}

// Update runs fn while holding the overlay lock so callers can
// safely mutate the content of a hosted window (e.g. swap a hint
// window's markdown body). fn must not call back into the
// OverlayBrowser nor block.
func (b *OverlayBrowser) Update(fn func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	fn()
}

// Draw paints the live overlay windows, oldest first. The rest of the
// browser chrome (tab bar, wallpaper) is never drawn so the IDE root
// stays visible around the windows.
func (b *OverlayBrowser) Draw(w term.Writer) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked()
	for _, win := range b.wins {
		b.c.DrawWindow(win, w)
	}
}

// Handle forwards ev to the browser unconditionally. Used for key
// events that must reach the focused overlay window (e.g. prompt
// option navigation); mouse events should go through HandleMouse so
// clicks outside the overlay windows fall through to the IDE root.
func (b *OverlayBrowser) Handle(ev term.Event) (bool, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.c.Handle(ev)
}

// HandleMouse routes ev to the browser when it is a mouse event that
// falls inside an overlay window, or while a press that started
// inside one is still being dragged. routed=false when the event does
// not concern the overlay and should fall through to the tutorial and
// IDE root.
func (b *OverlayBrowser) HandleMouse(ev term.Event) (handled, routed bool) {
	if ev.Type != term.EventMouse {
		return false, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked()
	pos := term.Coordinates{X: ev.MouseX, Y: ev.MouseY}
	if !b.dragging && !b.coversLocked(pos) {
		return false, false
	}
	switch ev.Key {
	case term.MouseLeft:
		b.dragging = true
	case term.MouseRelease:
		b.dragging = false
	}
	_, handled = b.c.Handle(ev)
	return handled, true
}

// Covers reports whether pos falls inside a live overlay window.
func (b *OverlayBrowser) Covers(pos term.Coordinates) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.pruneLocked()
	return b.coversLocked(pos)
}

// WindowRect returns win's screen-space rectangle. ok=false when win
// is nil or closed.
func (b *OverlayBrowser) WindowRect(win browser.Window) (
	pos term.Coordinates, width, height int, ok bool,
) {
	if win == nil {
		return term.Coordinates{}, 0, 0, false
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if win.Closed() {
		return term.Coordinates{}, 0, 0, false
	}
	off := b.c.WindowManagerPosition()
	pos = win.Position()
	pos.X += off.X
	pos.Y += off.Y
	return pos, win.Width(), win.Height(), true
}

func (b *OverlayBrowser) coversLocked(pos term.Coordinates) bool {
	off := b.c.WindowManagerPosition()
	for _, win := range b.wins {
		p := win.Position()
		p.X += off.X
		p.Y += off.Y
		if pos.X >= p.X && pos.Y >= p.Y &&
			pos.X < p.X+win.Width() && pos.Y < p.Y+win.Height() {
			return true
		}
	}
	return false
}

func (b *OverlayBrowser) pruneLocked() {
	live := b.wins[:0]
	for _, win := range b.wins {
		if !win.Closed() {
			live = append(live, win)
		}
	}
	b.wins = live
}
