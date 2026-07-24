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

package starlarktutorial

import (
	"fmt"
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/component/markdown"
	mdhandler "unstable.build/go-tui/handler/markdown"
)

var _ browser.ScrollableFloating = (*floatingWindowContent)(nil)

// floatingWindowContent is the browser-window content of a
// floating_window step or a wait_* hint: the step's markdown body
// behind a less-like mouse-scrollable viewer. Dimensions reproduces
// the historical 60%-of-screen width heuristic from the live screen
// size; the window manager re-queries it every draw so terminal
// resizes reflow the window automatically.
type floatingWindowContent struct {
	*mdhandler.Handler
	span *handler.Span
	// screen packs the last known screen size (width<<32 | height).
	// Written by Tutorial.Resize on the TUI loop and read during
	// window layout under the overlay-browser lock, possibly from
	// the run goroutine at open time.
	screen atomic.Uint64
	// hint caps the window height so it stays above the command
	// prompt's anchor row, keeping the prompt the user is asked to
	// open visible beneath the hint.
	hint bool
	// onClose runs when the browser releases the content: the
	// window-bar close click or a programmatic window close. It must
	// only stamp state — it is called while the overlay-browser lock
	// is held.
	onClose func()
}

func newFloatingWindowContent(
	md *markdown.Component, width, height int, onClose func(),
) *floatingWindowContent {
	mdh := mdhandler.New(md)
	c := &floatingWindowContent{
		Handler: mdh,
		span: handler.NewSpan(mdh, component.SpanConfig{
			PadHorizontal: 2,
			PadVertical:   1,
			ContentAlignment: component.AlignmentBottom |
				component.AlignmentHorizontallyCentered,
		}),
		onClose: onClose,
	}
	c.setScreen(width, height)
	return c
}

func (c *floatingWindowContent) setScreen(width, height int) {
	c.screen.Store(uint64(uint32(width))<<32 | uint64(uint32(height)))
}

func (c *floatingWindowContent) Draw(w term.Writer) {
	c.span.Draw(w)
}

func (c *floatingWindowContent) Resize(width, height int) {
	c.span.Resize(width, height)
}

func (c *floatingWindowContent) Handle(ev term.Event) (bool, bool) {
	return c.span.Handle(ev)
}

func (c *floatingWindowContent) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return c.span.Cursor()
}

// Dimensions reports the ideal content size: 60% of the screen width
// (clamped like the bespoke floating window did) and the markdown
// height at that width, capped so the window chrome still fits on
// screen — or, for hint windows, stays above the command prompt.
func (c *floatingWindowContent) Dimensions() (int, int) {
	packed := c.screen.Load()
	sw := int(uint32(packed >> 32))
	sh := int(uint32(packed))
	innerW := (sw * 6) / 10
	innerW = max(innerW, 20)
	innerW = min(innerW, sw-2)
	contentW := max(innerW-2, 4)
	markdownW := max(contentW-2, 1)
	contentH := max(c.Handler.Height(markdownW), 1) + 1
	switch {
	case c.hint:
		hintCap := int(float64(sh)*commandPromptTopFraction) +
			hintBoxMaxHeightSlack - 2
		hintCap = max(hintCap, hintBoxMinInnerH-2)
		contentH = min(contentH, hintCap)
	case sh > 4:
		contentH = min(contentH, sh-4)
	}
	return contentW, contentH
}

// Close stamps the close-notification state before releasing the
// underlying viewer.
func (c *floatingWindowContent) Close() error {
	if c.onClose != nil {
		c.onClose()
	}
	return c.Handler.Close()
}

// openFloatingWindow opens the browser window for a reqFloatingWindow
// request. It runs on the run goroutine before the request becomes
// active; the overlay browser's lock makes that safe.
func (t *Tutorial) openFloatingWindow(r *request, width, height int) {
	if t.winOverlay == nil || r.md == nil {
		return
	}
	content := newFloatingWindowContent(r.md, width, height, func() {
		r.winClosed.Store(true)
	})
	align := r.align
	if align == 0 {
		align = component.AlignmentCentered
	}
	offset := r.offset
	offset.X = max(offset.X, 0)
	offset.Y = max(offset.Y, 0)
	r.winContent = content
	r.win = t.winOverlay.Floating(content, browserapi.FloatingConfig{
		Alignment: align,
		Offset:    offset,
		Title:     floatingWindowTitle(r),
	})
}

// closeRequestWindow closes r's browser window, if any. Idempotent.
func (t *Tutorial) closeRequestWindow(r *request) {
	if t.winOverlay == nil || r.win == nil {
		return
	}
	t.winOverlay.CloseWindow(r.win)
}

// openPromptWindow opens the confirm/choice prompt window for r on
// the overlay browser. It runs on the run goroutine before the
// request becomes active. OnSelect stamps the request's pending
// response; the prompt window closing (selection, Esc, or the ✕
// click) fires OnClose, which stamps winClosed so the TUI loop
// resolves the request with the stamped or per-kind dismissal
// response. Both callbacks fire while the overlay-browser lock is
// held and must only stamp state.
func (t *Tutorial) openPromptWindow(r *request) {
	if t.winOverlay == nil {
		return
	}
	ph := handler.FuncPromptHandler(
		func(idx int, option string) {
			value := option
			if idx >= 0 && idx < len(r.options) {
				value = r.options[idx]
			}
			r.pendingResp = response{
				selectedIdx:   idx,
				selectedValue: value,
				selected:      true,
			}
			if r.kind == reqConfirm {
				r.pendingResp.confirmed = idx == 0
			}
			r.pendingSelected = true
		},
		func() error {
			r.winClosed.Store(true)
			return nil
		},
	)
	r.win = t.winOverlay.Prompt(
		r.message, padPromptOptions(r.options), nil, ph)
}

// openHintWindow opens the non-modal hint window for a wait_* request
// near the top of the screen. Keys are never routed to it — they fall
// through to the IDE root while the request is armed — but the user
// can drag it aside, scroll it, or close it via the window bar
// without resolving the step.
func (t *Tutorial) openHintWindow(r *request, width, height int) {
	if t.winOverlay == nil {
		return
	}
	md, ok := newHintMarkdown(t.hintBody(r))
	if !ok {
		return
	}
	content := newFloatingWindowContent(md, width, height, func() {
		r.winClosed.Store(true)
	})
	content.hint = true
	r.winContent = content
	r.win = t.winOverlay.Floating(content, browserapi.FloatingConfig{
		Alignment: component.AlignmentTop |
			component.AlignmentHorizontallyCentered,
		Title: floatingWindowTitle(r),
	})
}

// refreshHintWindow re-renders r's hint body into its live window,
// used when ObserveCommand swaps in the on_error recovery hint.
func (t *Tutorial) refreshHintWindow(r *request) {
	if t.winOverlay == nil || r.winContent == nil {
		return
	}
	md, ok := newHintMarkdown(t.hintBody(r))
	if !ok {
		return
	}
	t.winOverlay.Update(func() {
		r.winContent.SetComponent(md)
	})
}

// hintBody composes the markdown body of a wait_* hint window.
func (t *Tutorial) hintBody(r *request) string {
	switch r.kind {
	case reqWaitKey:
		return "Press " + r.waitKey + " to continue."
	case reqWaitCommand:
		return buildWaitCommandHint(r, t.commandKeyDisplay,
			t.commandManualLookup, t.keyForCommand)
	case reqWaitShell:
		return buildWaitShellHint(r, t.commandKeyDisplay)
	case reqWaitEvent:
		return r.text
	}
	return ""
}

func newHintMarkdown(body string) (*markdown.Component, bool) {
	if body == "" {
		return nil, false
	}
	mdCfg := markdown.DefaultConfig()
	mdCfg.HeaderPrefix = false
	md, err := markdown.NewWithConfig(body, mdCfg)
	if err != nil {
		return nil, false
	}
	return md, true
}

func floatingWindowTitle(r *request) string {
	switch {
	case r.title != "" && r.stepNum > 0:
		return fmt.Sprintf("Step %d — %s", r.stepNum, r.title)
	case r.title != "":
		return r.title
	case r.stepNum > 0:
		return fmt.Sprintf("Step %d", r.stepNum)
	}
	return ""
}
