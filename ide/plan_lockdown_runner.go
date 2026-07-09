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
	"sync/atomic"

	"github.com/ernestrc/sensible/browser"
	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component/markdown"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide/idelockdown"
)

// Lockdown prompt size caps. Prevent the overlay from consuming the
// full screen on large terminals while still fitting on small ones.
const (
	planLockdownPromptMaxWidth  = 80
	planLockdownPromptMaxHeight = 30
)

type planLockdownRunner struct {
	tui.Handler

	mu               sync.Mutex
	locked           atomic.Bool
	prompt           tui.Handler
	width, height    int
	promptW, promptH int
	offsetX, offsetY int
	// defaultAttr resolves term.ColorDefault for the gray-fade
	// shader; nil falls back to plain terminal defaults.
	defaultAttr func() term.Attributes
}

func newPlanLockdownRunner(inner tui.Handler) *planLockdownRunner {
	return &planLockdownRunner{Handler: inner}
}

// setDefaultAttr lets the overlay desaturation match the IDE's
// configured default background instead of falling back to plain
// terminal black.
func (r *planLockdownRunner) setDefaultAttr(get func() term.Attributes) {
	r.mu.Lock()
	r.defaultAttr = get
	r.mu.Unlock()
}

// SetLocked toggles the gray-fade overlay and input swallowing.
// Callers own what is shown on top via setPrompt; unlocking clears
// the prompt so no UI resources are held while unlocked.
func (r *planLockdownRunner) SetLocked(locked bool) {
	if r.locked.Load() == locked {
		return
	}
	r.mu.Lock()
	if !locked {
		r.prompt = nil
	}
	r.locked.Store(locked)
	r.mu.Unlock()
}

func (r *planLockdownRunner) Locked() bool { return r.locked.Load() }

// setPrompt sets the handler drawn on top of the overlay. Callers
// decide what to show for the current phase (default lockdown prompt,
// sign-in wait prompt, etc.). No-op when unlocked.
func (r *planLockdownRunner) setPrompt(p tui.Handler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.locked.Load() {
		return
	}
	r.prompt = p
	r.recomputePromptBoxLocked()
	if p != nil && r.promptW > 0 && r.promptH > 0 {
		p.Resize(r.promptW, r.promptH)
	}
}

func (r *planLockdownRunner) Resize(width, height int) {
	r.mu.Lock()
	r.width, r.height = width, height
	r.recomputePromptBoxLocked()
	p := r.prompt
	pw, ph := r.promptW, r.promptH
	r.mu.Unlock()
	r.Handler.Resize(width, height)
	if p != nil {
		p.Resize(pw, ph)
	}
}

// recomputePromptBoxLocked uses the embedded prompt's Floating
// dimensions when available so the overlay mirrors the proportions
// of an in-workspace prompt; otherwise falls back to the size caps.
func (r *planLockdownRunner) recomputePromptBoxLocked() {
	pw := planLockdownPromptMaxWidth
	ph := planLockdownPromptMaxHeight
	if f, ok := r.prompt.(component.Floating); ok {
		dw, dh := f.Dimensions()
		if dw > 0 {
			pw = dw
		}
		if dh > 0 {
			ph = dh
		}
	}
	if pw > planLockdownPromptMaxWidth {
		pw = planLockdownPromptMaxWidth
	}
	if ph > planLockdownPromptMaxHeight {
		ph = planLockdownPromptMaxHeight
	}
	if pw > r.width {
		pw = r.width
	}
	if ph > r.height {
		ph = r.height
	}
	if pw < 0 {
		pw = 0
	}
	if ph < 0 {
		ph = 0
	}
	r.promptW, r.promptH = pw, ph
	r.offsetX = (r.width - r.promptW) / 2
	r.offsetY = (r.height - r.promptH) / 2
	if r.offsetX < 0 {
		r.offsetX = 0
	}
	if r.offsetY < 0 {
		r.offsetY = 0
	}
}

// Draw captures the inner handler into a cell buffer and gray-fades
// it before overlaying the prompt so the desaturation lives entirely
// on the captured copy; the inner handler never sees it.
func (r *planLockdownRunner) Draw(w term.Writer) {
	if !r.locked.Load() {
		r.Handler.Draw(w)
		return
	}
	r.mu.Lock()
	p := r.prompt
	width, height := r.width, r.height
	ox, oy := r.offsetX, r.offsetY
	pw, ph := r.promptW, r.promptH
	getAttr := r.defaultAttr
	r.mu.Unlock()
	if width <= 0 || height <= 0 {
		return
	}
	r.drawDesaturatedInner(w, width, height, getAttr)
	if p != nil && pw > 0 && ph > 0 {
		p.Draw(translateWriter{w: w, dx: ox, dy: oy})
	}
}

func (r *planLockdownRunner) drawDesaturatedInner(
	w term.Writer, width, height int,
	getAttr func() term.Attributes,
) {
	var defAttr term.Attributes
	if getAttr != nil {
		defAttr = getAttr()
	}
	buf := cell.NewBufferWriter(w.Context(), width, height)
	_ = buf.Clear(defAttr)
	r.Handler.Draw(buf)
	sh := shader.GrayFade(shader.DefaultGrayFadeParams(), defAttr)
	sh.Shade(0, 1, buf.RawCells())
	rows := buf.RawCells()
	for y, row := range rows {
		for x, c := range row {
			w.SetCell(term.Coordinates{X: x, Y: y}, c)
		}
	}
}

// translateWriter offsets every coordinate before forwarding so the
// centered prompt can be placed without modifying its layout.
type translateWriter struct {
	w      term.Writer
	dx, dy int
}

func (t translateWriter) Context() context.Context { return t.w.Context() }
func (t translateWriter) SetCell(pos term.Coordinates, c term.Cell) {
	t.w.SetCell(term.Coordinates{X: pos.X + t.dx, Y: pos.Y + t.dy}, c)
}
func (t translateWriter) UnionAttributes(pos term.Coordinates, attr term.Attributes) {
	t.w.UnionAttributes(term.Coordinates{X: pos.X + t.dx, Y: pos.Y + t.dy}, attr)
}

func (r *planLockdownRunner) Handle(ev term.Event) (exit, handled bool) {
	if !r.locked.Load() {
		return r.Handler.Handle(ev)
	}
	r.mu.Lock()
	p := r.prompt
	ox, oy := r.offsetX, r.offsetY
	r.mu.Unlock()
	if p == nil {
		// Locked with no prompt yet: swallow every event so nothing
		// leaks to the inner IDE.
		return false, true
	}
	if ev.Type == term.EventMouse {
		ev.MouseX -= ox
		ev.MouseY -= oy
	}
	p.Handle(ev)
	// Drop exit: the overlay is owned by SetLocked, so prompt
	// Enter/Esc/option bindings must not quit the IDE. Claim every
	// event so unhandled keys (e.g. <m-enter>) cannot fall through.
	return false, true
}

// Cursor translates the prompt's local cursor coordinates by the
// overlay offset so the terminal cursor lands on the highlighted
// button in screen space.
func (r *planLockdownRunner) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	if r.locked.Load() {
		r.mu.Lock()
		p := r.prompt
		ox, oy := r.offsetX, r.offsetY
		r.mu.Unlock()
		if p != nil {
			pos, style, ok := p.Cursor()
			if ok {
				pos.X += ox
				pos.Y += oy
			}
			return pos, style, ok
		}
	}
	return r.Handler.Cursor()
}

func (r *planLockdownRunner) Selection() (string, bool) {
	if r.locked.Load() {
		r.mu.Lock()
		p := r.prompt
		r.mu.Unlock()
		if p != nil {
			return p.Selection()
		}
	}
	return r.Handler.Selection()
}

type planLockdownPromptDeps struct {
	checkoutURL   string
	downgradeURL  string
	onReSignIn    func()
	notifications browserapi.Notifications
	frameCharSet  component.FrameCharSet
	textAttr      term.Attributes
	highlightAttr term.Attributes
	backgroundBg  term.Attributes
}

type lockdownPromptSpec struct {
	message  string
	options  []string
	bindings []term.KeyComb
	onSelect func(idx int, opt string)
}

// newLockdownPrompt builds a framed prompt styled for the lockdown
// overlay. Frame is omitted from PromptConfig on purpose: the SDK
// wraps each option in its own NewFrame when Frame is set, producing
// 3-line buttons. Mirror the workspace prompts by leaving Frame empty
// and wrapping the whole prompt in handler.NewFrame.
func newLockdownPrompt(deps planLockdownPromptDeps, spec lockdownPromptSpec) tui.Handler {
	cfg := handler.PromptConfig{
		OptionBindings: spec.bindings,
		PromptConfig: component.PromptConfig{
			Message:              spec.message,
			Options:              spec.options,
			BackgroundAttributes: deps.backgroundBg,
			NewMessage:           newPromptMarkdownMessage,
		},
		OptionAttr:    deps.textAttr,
		HighlightAttr: deps.highlightAttr,
		PromptHandler: handler.FuncPromptHandler(
			spec.onSelect,
			func() error { return nil },
		),
	}
	prompt := handler.NewPrompt(cfg)
	frame := handler.NewFrame(prompt)
	if deps.frameCharSet != (component.FrameCharSet{}) {
		frame.FrameCharSet = deps.frameCharSet
	}
	frame.Attributes = term.Attributes{Bg: deps.backgroundBg.Bg}
	return frame
}

// Lockdown prompt button labels. The verb differs by reason: a
// signed-out or unverifiable session says "Sign in", while an
// already-authenticated lapsed/never-paid account says "Re-sign in".
const (
	optLockUpgrade   = " Upgrade to Pro "
	optLockSignIn    = " Sign in "
	optLockReSignIn  = " Re-sign in "
	optLockDowngrade = " Downgrade Rune "
	optLockRenew     = " Renew license "
)

// newPlanLockdownPrompt builds the default lockdown overlay prompt for
// the given lock reason. The button set and copy match the user's
// actual situation: an auth problem offers only sign-in, while a
// plan problem offers Upgrade alongside a (re-)sign-in escape hatch.
func newPlanLockdownPrompt(
	deps planLockdownPromptDeps, reason idelockdown.LockReason,
) tui.Handler {
	message, options, bindings := planLockdownPromptSpec(reason)
	return newLockdownPrompt(deps, lockdownPromptSpec{
		message:  message,
		options:  options,
		bindings: bindings,
		onSelect: func(_ int, opt string) {
			switch opt {
			case optLockUpgrade:
				openBrowser(deps.checkoutURL)
			case optLockRenew:
				openBrowser(deps.checkoutURL)
			case optLockDowngrade:
				openBrowser(deps.downgradeURL)
			case optLockSignIn, optLockReSignIn:
				if deps.onReSignIn != nil {
					go debug.CapturePanicReport(deps.onReSignIn)
				}
			}
		},
	})
}

// planLockdownPromptSpec returns the message, button labels, and key
// bindings for the given lock reason. Copy avoids em dashes per house
// style.
func planLockdownPromptSpec(reason idelockdown.LockReason) (
	message string, options []string, bindings []term.KeyComb,
) {
	switch reason {
	case idelockdown.LockSignedOut:
		return "**You're signed out.** Sign in to keep using Rune.",
			[]string{optLockSignIn}, []term.KeyComb{{Ch: 's'}}
	case idelockdown.LockParseError:
		return "**We couldn't verify your session.** " +
				"Please sign in again to continue.",
			[]string{optLockSignIn}, []term.KeyComb{{Ch: 's'}}
	case idelockdown.LockNeverSubscribed:
		return "**Rune requires a Pro subscription.** " +
				"Upgrade to Pro, or re-sign in if you've already upgraded your account.",
			[]string{optLockUpgrade, optLockSignIn},
			[]term.KeyComb{{Ch: 'u'}, {Ch: 's'}}
	case idelockdown.LockUpgradeExpired:
		return "**This Rune update isn't covered by your license.** " +
				"Your one-off purchase covers updates released on or before " +
				"your entitlement date. Downgrade to a covered version, or " +
				"renew to unlock the latest updates.",
			[]string{optLockDowngrade, optLockRenew},
			[]term.KeyComb{{Ch: 'd'}, {Ch: 'r'}}
	default:
		return "**Your Pro subscription has lapsed.** " +
				"Upgrade to Pro, or re-sign in if you've already renewed.",
			[]string{optLockUpgrade, optLockReSignIn},
			[]term.KeyComb{{Ch: 'u'}, {Ch: 's'}}
	}
}

// planLocker adapts the lockdown runner to idelockdown.Locker so the
// IDE owns overlay content: locking installs the reason-appropriate
// lockdown prompt, unlocking clears it. Transient prompts (sign-in)
// are swapped in via planLockdownRunner.setPrompt and restored to the
// default for the last reason here and in the sign-in flow.
type planLocker struct{ ide *IDE }

func (l planLocker) SetLocked(locked bool, reason idelockdown.LockReason) {
	l.ide.planLockdown.SetLocked(locked)
	if !locked {
		return
	}
	l.ide.planLockReason.Store(int32(reason))
	l.ide.planLockdown.setPrompt(newPlanLockdownPrompt(l.ide.planPromptDeps, reason))
}

func newPromptMarkdownMessage(str string) component.Floating {
	mcfg := markdown.DefaultConfig()
	mcfg.HeaderPrefix = false
	mkd, err := markdown.NewWithConfig(str, mcfg)
	if err == nil {
		return component.NewAspectRatioFloatingResponsive(
			component.NewSpan(mkd, component.SpanConfig{
				PadHorizontal:    4,
				PadVertical:      2,
				ContentAlignment: component.AlignmentCentered,
			}), component.DefaultAspectRatio)
	}
	cfg := component.StringResponsiveConfig{
		NoSplitWords: true,
		StringConfig: component.StringConfig{
			PaddingVertical:   4,
			PaddingHorizontal: 4,
			Alignment:         component.AlignmentCentered,
		},
	}
	return component.NewAspectRatioFloatingResponsive(
		component.NewResponsiveString(str, cfg), component.DefaultAspectRatio)
}

func openBrowser(raw string) {
	u, err := url.Parse(raw)
	if err != nil {
		log.WithError(err).Warnf("ideplan: parse checkout url %q", raw)
		return
	}
	if err := browser.Browse(u); err != nil {
		log.WithError(err).Warn("ideplan: open checkout url")
	}
}
