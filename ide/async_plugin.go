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
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/debug"
)

var _ pluginHandler = (*asyncPlugin)(nil)

// asyncPlugin is a permanent pluginHandler wrapper that runs the
// blocking plugin build (plugin.New -> vte.NewHandler -> NewPty and
// StartCommand RPCs) off the host event loop. The floating window
// opens immediately with a loading animation; once the build settles
// the wrapper forwards every call to the real handler and replays the
// queued resize/focus/key operations.
//
// All mutable state is event-loop-owned: the factory goroutine hands
// its result back through ScheduleNextTick, so no locking is needed.
type asyncPlugin struct {
	e *ex
	// title mirrors plugin.Handler's default title (the joined argv)
	// so the floating window bar is identical before and after the
	// swap.
	title    string
	maxWidth int

	anim        *component.Animation
	animStopped bool

	real    pluginHandler
	err     error
	errComp component.String
	closed  bool

	width, height int
	resized       bool

	queuedEvents []term.Event
	// queuedFocus preserves every pre-ready focus transition in
	// order: consumers such as the ephemeral-close logic count
	// individual transitions, so collapsing to the last value would
	// change behavior.
	queuedFocus []bool
}

// newAsyncPlugin returns immediately with a placeholder handler and
// runs factory on a background goroutine. The result is installed on
// the host event loop via e.sched.
func newAsyncPlugin(
	e *ex, title string, maxWidth int,
	factory func() (pluginHandler, error),
) *asyncPlugin {
	ap := &asyncPlugin{e: e, title: title, maxWidth: maxWidth}
	frames, sequence := component.ProgressAnimationFrames()
	ap.anim = component.NewAnimation(
		browser.EventPublisherInterrupter(e.Browser()), frames, sequence, 0)
	ap.anim.Resize(2, 1)
	e.asyncVTELoads.Add(1)
	go debug.CapturePanicReport(func() {
		defer e.asyncVTELoads.Done()
		h, ferr := factory()
		if !e.sched(func() { ap.complete(h, ferr) }) {
			// The event loop is gone: nothing will ever install the
			// result, so release the freshly-built handler.
			if ferr == nil && h != nil {
				_ = h.Close()
			}
			ap.stopAnimation()
		}
	})
	return ap
}

// complete installs the factory result. It runs on the host event
// loop.
func (ap *asyncPlugin) complete(h pluginHandler, err error) {
	if ap.closed {
		// Only a successful build hands over an owned, fully
		// initialized handler; closing anything else is unsafe.
		if err == nil && h != nil {
			_ = h.Close()
		}
		return
	}
	ap.stopAnimation()
	if err != nil {
		ap.err = err
		ap.errComp = component.NewStringWithConfig(
			fmt.Sprintf("%s: %v", ap.title, err),
			component.StringConfig{Alignment: component.AlignmentCentered})
		ap.errComp.Resize(ap.width, ap.height)
		_, _ = ap.e.notifications.Notify(browserapi.LevelError,
			"%s: %v", ap.title, err)
		return
	}
	ap.real = h
	if ap.resized {
		h.Resize(ap.width, ap.height)
	}
	for _, focus := range ap.queuedFocus {
		h.OnFocusChange(focus)
	}
	ap.queuedFocus = nil
	for _, ev := range ap.queuedEvents {
		_, _ = h.Handle(ev)
	}
	ap.queuedEvents = nil
}

func (ap *asyncPlugin) stopAnimation() {
	if ap.animStopped {
		return
	}
	ap.animStopped = true
	_ = ap.anim.Close()
}

func (ap *asyncPlugin) Draw(w term.Writer) {
	if ap.real != nil {
		ap.real.Draw(w)
		return
	}
	if ap.err != nil {
		ap.errComp.Draw(w)
		return
	}
	dx := max(0, (ap.width-2)/2)
	dy := max(0, ap.height/2)
	ap.anim.Draw(translateWriter{w: w, dx: dx, dy: dy})
}

func (ap *asyncPlugin) Resize(width, height int) {
	ap.width, ap.height = width, height
	ap.resized = true
	if ap.real != nil {
		ap.real.Resize(width, height)
		return
	}
	if ap.err != nil {
		ap.errComp.Resize(width, height)
	}
}

func (ap *asyncPlugin) Handle(ev term.Event) (exit, handled bool) {
	if ap.real != nil {
		return ap.real.Handle(ev)
	}
	if ev.Type != term.EventKey {
		return false, false
	}
	// Esc / ctrl-c abandon the pending window; complete then closes
	// the freshly-built handler, which kills the spawned process.
	if ev.Key == term.KeyEsc || (ev.Ch == 'c' && ev.Mod == term.ModCtrl) {
		return true, true
	}
	if len(ap.queuedEvents) < maxQueuedAsyncVTEEvents {
		ap.queuedEvents = append(ap.queuedEvents, ev)
	}
	// Queued but unclaimed, mirroring asyncVTE: global bindings keep
	// working while the spawn is in flight.
	return false, false
}

func (ap *asyncPlugin) Cursor() (
	c term.Coordinates, s term.CursorStyle, show bool,
) {
	if ap.real != nil {
		return ap.real.Cursor()
	}
	return
}

func (ap *asyncPlugin) Selection() (string, bool) {
	if ap.real != nil {
		return ap.real.Selection()
	}
	return "", false
}

// Dimensions mirrors plugin.Handler's pre-completion interactive
// sizing so the floating window does not jump when the real handler
// lands.
func (ap *asyncPlugin) Dimensions() (int, int) {
	if ap.real != nil {
		return ap.real.Dimensions()
	}
	width := int(float64(ap.maxWidth) * 0.8)
	return width, width * 9 / 16
}

func (ap *asyncPlugin) OnFocusChange(inFocus bool) {
	if ap.real != nil {
		ap.real.OnFocusChange(inFocus)
		return
	}
	if ap.closed {
		return
	}
	ap.queuedFocus = append(ap.queuedFocus, inFocus)
}

func (ap *asyncPlugin) Title() string {
	if ap.real != nil {
		return ap.real.Title()
	}
	return ap.title
}

func (ap *asyncPlugin) Close() error {
	if ap.closed {
		return nil
	}
	ap.closed = true
	ap.stopAnimation()
	if ap.real != nil {
		return ap.real.Close()
	}
	// Pre-ready: complete observes ap.closed and closes the
	// freshly-built handler silently (the user abandoned the window).
	return nil
}
