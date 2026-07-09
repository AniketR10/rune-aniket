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

package idelockdown

import (
	"context"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide/ideplan"
)

// Locker abstracts the ability to lock the IDE. The reason explains
// why the lock is requested so the lockdown overlay can pick the
// matching prompt copy; it is LockNone on unlock.
type Locker interface {
	SetLocked(locked bool, reason LockReason)
}

// Config collects the dependencies of Planner. All fields except Now
// are required; New panics on missing dependencies so mis-wiring
// fails loudly at construction instead of silently skipping
// enforcement.
type Config struct {
	// Storage is the pre-partitioned storage service usage days are
	// persisted under.
	Storage storageapi.Service
	// Source provides the login/plan state.
	Source ideplan.Source
	// Policies are evaluated together on every usage or enforcement
	// signal; the strongest Action wins and the earliest returned
	// time schedules the next evaluation.
	Policies []Policy
	// Locker is the lockdown overlay the planner locks and unlocks.
	Locker Locker
	// ShowNagPrompt opens the nag prompt. The caller is responsible
	// for scheduling onto the event loop.
	ShowNagPrompt func()
	// ShowAskMoreTimePrompt opens the ask-more-time prompt shown in
	// place of the last nag before lockdown. The caller is
	// responsible for scheduling onto the event loop.
	ShowAskMoreTimePrompt func()
	// Tampered reports that install-ID resolution detected a wiped
	// data directory. Load persists it into the usage ledger, where
	// it outlives the self-healing detection signal until an entitled
	// session clears it.
	Tampered bool
	// Now defaults to time.Now.
	Now func() time.Time
}

// Planner records usage days and applies enforcement policies. It
// satisfies ideplan.Locker so the plan monitor's SetLocked(true)
// signal is interpreted as "enforcement requested" and re-evaluated
// against the usage history instead of locking unconditionally.
type Planner struct {
	cfg     Config
	tracker *tracker

	mu         sync.Mutex
	loaded     bool
	started    bool
	stopCh     chan struct{}
	nextCancel chan struct{}
}

// New constructs a Planner. Call Start to load state and begin the
// policy evaluation chain. Panics when a required dependency is
// missing.
func New(cfg Config) *Planner {
	if cfg.Storage == nil {
		panic("idelockdown: Config.Storage is required")
	}
	if cfg.Source == nil {
		panic("idelockdown: Config.Source is required")
	}
	if len(cfg.Policies) == 0 {
		panic("idelockdown: Config.Policies is required")
	}
	if cfg.Locker == nil {
		panic("idelockdown: Config.Locker is required")
	}
	if cfg.ShowNagPrompt == nil {
		panic("idelockdown: Config.ShowNagPrompt is required")
	}
	if cfg.ShowAskMoreTimePrompt == nil {
		panic("idelockdown: Config.ShowAskMoreTimePrompt is required")
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	return &Planner{cfg: cfg, tracker: newTracker(cfg.Storage), stopCh: make(chan struct{})}
}

// Load fetches the usage history and records the launch usage
// sample. It is split from Start so callers can perform the storage
// I/O before other subsystems contend for the store; Start calls it
// when the caller does not. Safe to call multiple times: only the
// first call does work.
func (p *Planner) Load(ctx context.Context) error {
	p.mu.Lock()
	if p.loaded {
		p.mu.Unlock()
		return nil
	}
	p.loaded = true
	p.mu.Unlock()
	if err := p.tracker.load(ctx); err != nil {
		return err
	}
	if p.cfg.Tampered {
		if err := p.tracker.setTampered(ctx, true); err != nil {
			return err
		}
	}
	return p.tracker.recordUsage(ctx, bucketStart(p.cfg.Now(), usageBucket))
}

// Start loads the usage history (unless Load already ran), records
// the launch usage sample, and runs the first policy evaluation,
// which schedules the next one at the earliest time the policies
// request. Safe to call once.
func (p *Planner) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return nil
	}
	p.started = true
	p.mu.Unlock()
	if err := p.Load(ctx); err != nil {
		return err
	}
	p.evaluate(ctx)
	return nil
}

// Stop halts the evaluation chain. Safe to call multiple times.
func (p *Planner) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.stopCh:
		return
	default:
	}
	close(p.stopCh)
}

// SetLocked satisfies ideplan.Locker, whose contract carries no
// reason. Unlock requests pass through to the underlying lockdown
// runner; lock requests (the monitor saw an enforcing status) trigger
// a policy evaluation instead of locking unconditionally, and the
// reason is derived there from the current decision.
func (p *Planner) SetLocked(locked bool) {
	if !locked {
		p.cfg.Locker.SetLocked(false, LockNone)
		return
	}
	p.evaluate(context.Background())
}

func (p *Planner) snapshot(ctx context.Context) Snapshot {
	dec, err := p.cfg.Source.Decision(ctx)
	if err != nil {
		log.WithError(err).Debug("idelockdown: plan decision")
	}
	return Snapshot{
		Now:      p.cfg.Now().UTC(),
		Usage:    p.tracker.usage(),
		Plan:     dec,
		Tampered: p.tracker.tampered(),
	}
}

// evaluate records the current usage sample, applies the strongest
// Action across the configured policies, and schedules the next
// evaluation at the earliest time they request.
func (p *Planner) evaluate(ctx context.Context) {
	if err := p.tracker.recordUsage(ctx, bucketStart(p.cfg.Now(), usageBucket)); err != nil {
		log.WithError(err).Debug("idelockdown: record usage sample")
	}
	s := p.snapshot(ctx)
	action := ActionNone
	strongest := ""
	var next time.Time
	for _, pol := range p.cfg.Policies {
		at, a := pol.Evaluate(s)
		if a > action {
			action = a
			strongest = pol.Name()
		}
		if !at.IsZero() && (next.IsZero() || at.Before(next)) {
			next = at
		}
	}
	log.WithFields(log.Fields{
		"action": action,
		"policy": strongest,
		"status": s.Plan.Status.String(),
		"usage":  len(s.Usage),
		"next":   next.Format(time.RFC3339),
	}).Debug("idelockdown: evaluate")
	isGated, reason := gated(s.Plan)
	// An entitled session forgives a past tamper so a later lapse
	// gets the normal usage runway instead of an immediate lock.
	if !isGated && s.Tampered {
		if err := p.tracker.setTampered(ctx, false); err != nil {
			log.WithError(err).Debug("idelockdown: clear tamper mark")
		}
	}
	switch action {
	case ActionLockdown:
		p.cfg.Locker.SetLocked(true, reason)
	case ActionAskMoreTime:
		p.maybePrompt(ctx, s.Now, p.cfg.ShowAskMoreTimePrompt)
	case ActionPrompt:
		p.maybePrompt(ctx, s.Now, p.cfg.ShowNagPrompt)
	case ActionNone:
		p.cfg.Locker.SetLocked(false, LockNone)
	}
	p.scheduleEvaluate(ctx, next)
}

// scheduleEvaluate spawns a goroutine that re-evaluates at the given
// time, replacing any previously scheduled evaluation so concurrent
// triggers (the timer and out-of-band SetLocked signals) never fork
// the chain. A zero time ends the chain.
func (p *Planner) scheduleEvaluate(ctx context.Context, at time.Time) {
	if at.IsZero() {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	select {
	case <-p.stopCh:
		return
	default:
	}
	if p.nextCancel != nil {
		close(p.nextCancel)
	}
	cancel := make(chan struct{})
	p.nextCancel = cancel
	timer := time.NewTimer(at.Sub(p.cfg.Now()))
	go debug.CapturePanicReport(func() {
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-p.stopCh:
		case <-cancel:
		case <-timer.C:
			p.evaluate(ctx)
		}
	})
}

// maybePrompt shows a prompt at most once per nagCooldown, persisted
// across restarts. The nag and ask-more-time prompts share the mark,
// so the ask-more-time prompt fires on the cadence the nag would
// have, replacing it. The CAS mark happens before the prompt so a
// concurrent session cannot double-prompt.
func (p *Planner) maybePrompt(ctx context.Context, now time.Time, show func()) {
	marked, err := p.tracker.markNagged(ctx, now, nagCooldown)
	if err != nil {
		log.WithError(err).Debug("idelockdown: mark nagged")
		return
	}
	if !marked {
		return
	}
	show()
}
