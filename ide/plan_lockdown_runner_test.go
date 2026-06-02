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
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/ide/ideplan"
)

type countingInner struct {
	handles, draws, resizes atomic.Int32
}

type staticPlanSource struct {
	dec ideplan.Decision
}

func (s staticPlanSource) Decision(context.Context) (ideplan.Decision, error) { return s.dec, nil }
func (s staticPlanSource) Refresh(context.Context) (ideplan.Decision, error)  { return s.dec, nil }

func (c *countingInner) Handle(term.Event) (bool, bool) {
	c.handles.Add(1)
	return false, true
}
func (c *countingInner) Draw(term.Writer) { c.draws.Add(1) }
func (c *countingInner) Resize(int, int)  { c.resizes.Add(1) }
func (c *countingInner) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, 0, false
}
func (c *countingInner) Selection() (string, bool) { return "", false }

// coloredInner is a tui.Handler that always paints every cell of its
// resize area with a known foreground color. Used to assert the
// gray-fade overlay applies to inner draws while locked.
type coloredInner struct {
	width, height int
	fg            term.Color
}

func (c *coloredInner) Handle(term.Event) (bool, bool) { return false, true }
func (c *coloredInner) Resize(w, h int)                { c.width, c.height = w, h }
func (c *coloredInner) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, 0, false
}
func (c *coloredInner) Selection() (string, bool) { return "", false }
func (c *coloredInner) Draw(w term.Writer) {
	for y := 0; y < c.height; y++ {
		for x := 0; x < c.width; x++ {
			w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
				Ch: 'X', Width: 1,
				Attributes: term.Attributes{Fg: c.fg},
			})
		}
	}
}

// recordingPrompt captures the dimensions it was resized to so tests
// can assert the lockdown runner clamps the prompt to its 70×30
// budget and centers the rectangle. Draw paints a single recognizable
// rune in its (0,0) so tests can locate where the centered box lands
// in screen space.
type recordingPrompt struct {
	resizedW, resizedH int
	drawnAt            term.Coordinates
	drawn              bool
}

func (p *recordingPrompt) Handle(term.Event) (bool, bool) { return false, true }
func (p *recordingPrompt) Resize(w, h int)                { p.resizedW, p.resizedH = w, h }
func (p *recordingPrompt) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, 0, false
}
func (p *recordingPrompt) Selection() (string, bool) { return "", false }
func (p *recordingPrompt) Draw(w term.Writer) {
	w.SetCell(term.Coordinates{X: 0, Y: 0}, term.Cell{Ch: 'P', Width: 1})
	p.drawn = true
	_ = p.drawnAt
}

// recordingWriter is a minimal term.Writer that captures SetCell
// calls so tests can inspect both the prompt's screen-space position
// after offset translation and the desaturated inner cells.
type recordingWriter struct {
	width, height int
	cells         map[term.Coordinates]term.Cell
}

func newRecordingWriter(w, h int) *recordingWriter {
	return &recordingWriter{width: w, height: h, cells: map[term.Coordinates]term.Cell{}}
}
func (r *recordingWriter) Context() context.Context                              { return context.Background() }
func (r *recordingWriter) SetCell(pos term.Coordinates, c term.Cell)             { r.cells[pos] = c }
func (r *recordingWriter) UnionAttributes(_ term.Coordinates, _ term.Attributes) {}

func TestPlanLockdownRunnerUnlockedForwards(t *testing.T) {
	inner := &countingInner{}
	r := newPlanLockdownRunner(inner, nil)
	r.Handle(term.Event{Type: term.EventKey, Ch: 'a'})
	assert.Equal(t, int32(1), inner.handles.Load())
}

func TestIDEWithPlanSourceLocksReturnedRoot(t *testing.T) {
	configFile, _ := makeTestFiles(t)
	dataDir := t.TempDir()
	i, err := New(t.TempDir(), configFile.Name(), dataDir,
		newTestStorage(t, dataDir),
		WithLocker(new(sync.Mutex)),
		WithPlanSource(PlanSourceConfig{
			Source: staticPlanSource{dec: ideplan.Decision{Status: ideplan.StatusExpired}},
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, i.Close()) })

	root := i.Ready()
	runner, ok := root.(*planLockdownRunner)
	require.True(t, ok)

	i.TickPlan(context.Background())

	assert.Same(t, runner, i.planLockdown)
	assert.True(t, runner.Locked())
	_, handled := root.Handle(term.Event{Type: term.EventKey, Ch: 'x'})
	assert.True(t, handled)
}

// TestIDELockdownE2ERendersOverlayAndSwallowsInput is the black-box
// e2e for the paid-plan lockdown feature: it builds a full IDE via
// the public ide.New constructor with a plan source that returns
// StatusExpired, drives it through handlertest.RunHandlerSequence
// the same way tui.Run would, and asserts that
//
//  1. the rendered frame contains the lockdown copy users see,
//  2. ordinary keys do not reach the inner workspace once locked.
//
// Per the [bootstrap-and-lockdown-need-black-box-e2e] memory note,
// this avoids exercising the lockdown helpers in isolation — the IDE
// is constructed, Ready'd, ticked, and driven through real
// term.Events so a regression in the wiring (plan source → monitor →
// runner → prompt → root Handler) trips this test instead of being
// masked by a per-helper unit fixture.
func TestIDELockdownE2ERendersOverlayAndSwallowsInput(t *testing.T) {
	configFile, _ := makeTestFiles(t)
	dataDir := t.TempDir()
	mu := new(sync.Mutex)
	i, err := New(t.TempDir(), configFile.Name(), dataDir,
		newTestStorage(t, dataDir),
		WithLocker(mu),
		WithPlanSource(PlanSourceConfig{
			Source: staticPlanSource{dec: ideplan.Decision{Status: ideplan.StatusExpired}},
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, i.Close()) })

	root := i.Ready()
	i.TickPlan(context.Background())

	wrapped := &lockedHandler{Handler: root, mu: mu}
	wrapped.Resize(80, 24)
	frame := handlertest.DrawHandler(wrapped, 80, 24)
	assert.Contains(t, frame, "Rune Pro subscription",
		"locked IDE must render the lockdown copy")
	assert.Contains(t, frame, "Upgrade to Pro",
		"locked IDE must render the Upgrade button")
	assert.Contains(t, frame, "Re-signin",
		"locked IDE must render the Re-signin button")

	_, handled := root.Handle(term.Event{Type: term.EventKey, Ch: 'x'})
	assert.True(t, handled,
		"locked IDE must claim ordinary keys so they do not fall through "+
			"to the inner workspace")
}

func TestPlanLockdownRunnerLockedRoutesToPrompt(t *testing.T) {
	inner := &countingInner{}
	makePrompt := func() tui.Handler {
		return newPlanLockdownPrompt(planLockdownPromptDeps{
			checkoutURL: "",
		})
	}
	r := newPlanLockdownRunner(inner, makePrompt)
	r.SetLocked(true)
	defer r.SetLocked(false)

	r.Handle(term.Event{Type: term.EventKey, Ch: 'a'})
	assert.Equal(t, int32(0), inner.handles.Load(), "inner handler must not receive events while locked")
}

func TestPlanLockdownRunnerSetLockedFalseTearsDown(t *testing.T) {
	inner := &countingInner{}
	makePrompt := func() tui.Handler {
		return newPlanLockdownPrompt(planLockdownPromptDeps{
			checkoutURL: "",
		})
	}
	r := newPlanLockdownRunner(inner, makePrompt)
	r.SetLocked(true)
	r.SetLocked(false)
	r.Handle(term.Event{Type: term.EventKey, Ch: 'b'})
	assert.Equal(t, int32(1), inner.handles.Load())
}

// While locked, the runner must claim every event it receives so that
// unhandled keys (e.g. <m-enter> bound to terminalneworsplit) cannot
// fall through to the inner IDE.
func TestPlanLockdownRunnerLockedSwallowsUnhandled(t *testing.T) {
	inner := &countingInner{}
	makePrompt := func() tui.Handler {
		return newPlanLockdownPrompt(planLockdownPromptDeps{
			checkoutURL: "",
		})
	}
	r := newPlanLockdownRunner(inner, makePrompt)
	r.SetLocked(true)
	defer r.SetLocked(false)

	_, handled := r.Handle(term.Event{
		Type: term.EventKey, Key: term.KeyEnter, Mod: term.ModMeta,
	})
	assert.True(t, handled, "locked runner must claim every event")
	assert.Equal(t, int32(0), inner.handles.Load(), "inner must not see the event")
}

// The lockdown prompt itself returns exit=true on Enter/Esc/option
// bindings. The runner must NOT propagate that exit to the event
// loop, or the IDE process quits when the user clicks Re-sign in or
// Open checkout. The overlay is dismissed by SetLocked(false) from
// onActive, not by a prompt exit.
func TestPlanLockdownRunnerLockedSwallowsPromptExit(t *testing.T) {
	inner := &countingInner{}
	makePrompt := func() tui.Handler {
		return newPlanLockdownPrompt(planLockdownPromptDeps{
			checkoutURL: "",
		})
	}
	r := newPlanLockdownRunner(inner, makePrompt)
	r.SetLocked(true)
	defer r.SetLocked(false)
	r.Resize(80, 24)

	exit, handled := r.Handle(term.Event{Type: term.EventKey, Ch: 'o'})
	assert.True(t, handled, "locked runner must claim every event")
	assert.False(t, exit, "lockdown overlay must never propagate exit")

	exit, _ = r.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	assert.False(t, exit, "lockdown overlay must never propagate exit on enter")

	exit, _ = r.Handle(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	assert.False(t, exit, "lockdown overlay must never propagate exit on esc")
}

// TestPlanLockdownRunnerLockedSequence drives the lockdown prompt
// end-to-end: it asserts the prompt renders the lockdown copy and
// that an <m-enter> sneak attempt is swallowed without reaching the
// inner handler.
func TestPlanLockdownRunnerLockedSequence(t *testing.T) {
	inner := &countingInner{}
	makePrompt := func() tui.Handler {
		return newPlanLockdownPrompt(planLockdownPromptDeps{
			checkoutURL: "",
		})
	}
	r := newPlanLockdownRunner(inner, makePrompt)
	r.SetLocked(true)
	defer r.SetLocked(false)
	r.Resize(80, 24)

	// Draw the lockdown overlay and assert the lockdown copy
	// appears. We do not match the entire frame because the
	// overlay padding depends on Prompt's internal layout.
	frame := handlertest.DrawHandler(r, 80, 24)
	assert.Contains(t, frame, "Rune Pro subscription")
	assert.Contains(t, frame, "Upgrade to Pro")

	// Pressing the bound key 'u' must not reach the inner handler.
	// openCheckoutURL with an empty string is a no-op so no browser
	// is opened from the test process.
	r.Handle(term.Event{Type: term.EventKey, Ch: 'u'})
	assert.Equal(t, int32(0), inner.handles.Load(), "inner must not see events while locked")

	// An <m-enter> press (terminalneworsplit) must NOT reach inner.
	_, handled := r.Handle(term.Event{
		Type: term.EventKey, Key: term.KeyEnter, Mod: term.ModMeta,
	})
	assert.True(t, handled)
	assert.Equal(t, int32(0), inner.handles.Load())

}

type recordingNotifier struct {
	mu    sync.Mutex
	calls []string
}

func (r *recordingNotifier) Notify(
	_ browserapi.NotificationLevel, format string, _ ...any,
) (string, error) {
	r.mu.Lock()
	r.calls = append(r.calls, format)
	r.mu.Unlock()
	return "", nil
}
func (r *recordingNotifier) NotifyOnce(
	level browserapi.NotificationLevel, format string, args ...any,
) (string, error) {
	return r.Notify(level, format, args...)
}
func (r *recordingNotifier) UpdateNotificationProgress(string, string, int64, int64) error {
	return nil
}

// TestPlanLockdownReSignInInvokesCallback proves that pressing the
// Re-signin button fires the onReSignIn callback. The callback owns
// the purge + re-login + monitor-tick sequence; the prompt itself
// just routes the click.
func TestPlanLockdownReSignInInvokesCallback(t *testing.T) {
	var reSignCalled atomic.Int32
	deps := planLockdownPromptDeps{
		checkoutURL:   "",
		onReSignIn:    func() { reSignCalled.Add(1) },
		notifications: &recordingNotifier{},
	}
	p := newPlanLockdownPrompt(deps)
	p.Resize(80, 24)
	p.Handle(term.Event{Type: term.EventKey, Ch: 'r'})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if reSignCalled.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	assert.Equal(t, int32(1), reSignCalled.Load(),
		"Re-signin button must invoke the onReSignIn callback")
}

// TestPlanLockdownPromptRendersUpgradeAndReSignin documents the
// final button copy so a future rename trips the test.
func TestPlanLockdownPromptRendersUpgradeAndReSignin(t *testing.T) {
	deps := planLockdownPromptDeps{
		checkoutURL:   "",
		onReSignIn:    func() {},
		notifications: &recordingNotifier{},
	}
	p := newPlanLockdownPrompt(deps)
	p.Resize(80, 24)
	frame := handlertest.DrawHandler(p, 80, 24)
	assert.Contains(t, frame, "Upgrade to Pro")
	assert.Contains(t, frame, "Re-signin")
}

// TestPlanLockdownRunnerResizeClampsPromptBox proves the lockdown
// runner resizes the inner handler to the full window but the prompt
// to at most 70×30 so the overlay reads as a centered box, not as a
// full-screen takeover.
func TestPlanLockdownRunnerResizeClampsPromptBox(t *testing.T) {
	inner := &countingInner{}
	prompt := &recordingPrompt{}
	r := newPlanLockdownRunner(inner, func() tui.Handler { return prompt })
	r.SetLocked(true)
	defer r.SetLocked(false)

	r.Resize(200, 60)

	assert.Equal(t, planLockdownPromptMaxWidth, prompt.resizedW,
		"prompt width must clamp to the lockdown overlay budget")
	assert.Equal(t, planLockdownPromptMaxHeight, prompt.resizedH,
		"prompt height must clamp to the lockdown overlay budget")
}

// TestPlanLockdownRunnerResizeFitsTinyWindow proves the runner does
// not over-extend the prompt past the available window when it is
// smaller than the 70×30 budget, so the overlay still fits.
func TestPlanLockdownRunnerResizeFitsTinyWindow(t *testing.T) {
	inner := &countingInner{}
	prompt := &recordingPrompt{}
	r := newPlanLockdownRunner(inner, func() tui.Handler { return prompt })
	r.SetLocked(true)
	defer r.SetLocked(false)

	r.Resize(40, 10)

	assert.Equal(t, 40, prompt.resizedW, "prompt width must shrink to window width")
	assert.Equal(t, 10, prompt.resizedH, "prompt height must shrink to window height")
}

// TestPlanLockdownRunnerDrawCentersPrompt proves the lockdown
// overlay draws the centered prompt offset by (window-promptBox)/2,
// so the recognizable 'P' rune lands at the expected screen-space
// coordinates instead of (0, 0).
func TestPlanLockdownRunnerDrawCentersPrompt(t *testing.T) {
	inner := &coloredInner{fg: term.NewRGBColor(255, 0, 0)}
	prompt := &recordingPrompt{}
	r := newPlanLockdownRunner(inner, func() tui.Handler { return prompt })
	r.SetLocked(true)
	defer r.SetLocked(false)

	const W, H = 200, 60
	r.Resize(W, H)
	w := newRecordingWriter(W, H)
	r.Draw(w)

	wantX := (W - planLockdownPromptMaxWidth) / 2
	wantY := (H - planLockdownPromptMaxHeight) / 2
	cellAtCenter, ok := w.cells[term.Coordinates{X: wantX, Y: wantY}]
	require.True(t, ok, "expected prompt to draw at centered offset (%d,%d)", wantX, wantY)
	assert.Equal(t, rune('P'), cellAtCenter.Ch, "prompt's (0,0) cell must land at the centered offset")
}

// TestPlanLockdownRunnerDrawDesaturatesInner proves the inner UI is
// rendered through the gray-fade desaturation while locked. The
// coloredInner paints fully-saturated red, and the runner must rewrite
// every visible cell to the per-channel luminance gray before the
// prompt is overlaid on top.
func TestPlanLockdownRunnerDrawDesaturatesInner(t *testing.T) {
	red := term.NewRGBColor(255, 0, 0)
	inner := &coloredInner{fg: red}
	r := newPlanLockdownRunner(inner, func() tui.Handler { return &recordingPrompt{} })
	r.SetLocked(true)
	defer r.SetLocked(false)

	const W, H = 200, 60
	r.Resize(W, H)
	w := newRecordingWriter(W, H)
	r.Draw(w)

	// A cell well outside the centered prompt rectangle must come
	// from the gray-faded inner draw. Its foreground must no longer
	// be the saturated red the inner painted.
	outside := term.Coordinates{X: 1, Y: 1}
	cell, ok := w.cells[outside]
	require.True(t, ok, "expected inner draw to populate (%d,%d)", outside.X, outside.Y)
	assert.NotEqual(t, red, cell.Attributes.Fg,
		"gray-fade overlay must desaturate the inner foreground while locked")
}

// TestPlanLockdownRunnerHandleTranslatesMouse proves mouse events
// targeted at screen-space coordinates inside the centered prompt
// reach the prompt with coordinates re-based into its local
// 70×30 frame. Without translation, the prompt would never receive a
// click on its options because they live at (0, ~promptH-1) in
// local space, far from where the cursor sits on the full screen.
func TestPlanLockdownRunnerHandleTranslatesMouse(t *testing.T) {
	inner := &countingInner{}
	var got term.Event
	prompt := &mouseRecordingPrompt{out: &got}
	r := newPlanLockdownRunner(inner, func() tui.Handler { return prompt })
	r.SetLocked(true)
	defer r.SetLocked(false)

	const W, H = 200, 60
	r.Resize(W, H)

	wantOX := (W - planLockdownPromptMaxWidth) / 2
	wantOY := (H - planLockdownPromptMaxHeight) / 2
	r.Handle(term.Event{
		Type:   term.EventMouse,
		Key:    term.MouseLeft,
		MouseX: wantOX + 5,
		MouseY: wantOY + 7,
	})
	assert.Equal(t, 5, got.MouseX, "mouse X must be translated into the prompt's local frame")
	assert.Equal(t, 7, got.MouseY, "mouse Y must be translated into the prompt's local frame")
}

type mouseRecordingPrompt struct {
	out *term.Event
}

func (p *mouseRecordingPrompt) Handle(ev term.Event) (bool, bool) {
	*p.out = ev
	return false, true
}
func (p *mouseRecordingPrompt) Resize(int, int)  {}
func (p *mouseRecordingPrompt) Draw(term.Writer) {}
func (p *mouseRecordingPrompt) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, 0, false
}
func (p *mouseRecordingPrompt) Selection() (string, bool) { return "", false }
