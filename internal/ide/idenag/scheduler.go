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

// Package idenag schedules the weekly sign-in/upgrade reminder
// prompt. It persists the time of the last prompt through a
// storageapi.Service partition and arms a timer so the prompt fires
// at most once per interval, surviving restarts.
package idenag

import (
	"context"
	"errors"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/rune/internal/debug"
)

// Partition is the storage partition the scheduler persists under.
const Partition = "idenag"

const (
	docID   = "nag"
	docKind = "nag"
)

// Default scheduling knobs.
const (
	DefaultInterval  = 7 * 24 * time.Hour
	DefaultBootDelay = 30 * time.Minute
)

// State classifies the user's account standing for the nag prompt.
type State int

const (
	// StateSignedOut means no account token is cached.
	StateSignedOut State = iota
	// StateNoPlan means the user is signed in but has never held a
	// paid plan.
	StateNoPlan
	// StateExpired means the user is signed in and their paid plan
	// has lapsed.
	StateExpired
	// StateActive means the user holds an active entitlement; no
	// prompt is shown.
	StateActive
)

// Config collects the Scheduler dependencies. Storage, State and
// Show are required.
type Config struct {
	// Storage persists the last-prompt timestamp. Callers should
	// pass a service partitioned under Partition.
	Storage storageapi.Service
	// State classifies the account standing when the timer fires.
	State func(context.Context) State
	// Show surfaces the prompt for the given (non-active) state.
	Show func(State)
	// Now defaults to time.Now.
	Now func() time.Time
	// Interval is the pause between prompts. Defaults to
	// DefaultInterval.
	Interval time.Duration
	// BootDelay postpones an overdue prompt right after startup so
	// it does not race the user's first interactions. Defaults to
	// DefaultBootDelay.
	BootDelay time.Duration
}

// nagDoc is the persisted record of the last prompt.
type nagDoc struct {
	Kind string
	// LastNag is the UTC time the prompt last fired, in RFC3339Nano.
	LastNag string
}

// Scheduler fires the nag prompt on a weekly cadence.
type Scheduler struct {
	cfg Config

	mu      sync.Mutex
	timer   *time.Timer
	stopped bool
}

// New builds a Scheduler from cfg, applying defaults for Now,
// Interval and BootDelay.
func New(cfg Config) *Scheduler {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Interval == 0 {
		cfg.Interval = DefaultInterval
	}
	if cfg.BootDelay == 0 {
		cfg.BootDelay = DefaultBootDelay
	}
	return &Scheduler{cfg: cfg}
}

// Start reads the last-prompt timestamp and arms the timer. On the
// first run (no record) it stores the current time and waits a full
// interval without prompting. When the record is older than the
// interval the prompt fires BootDelay after startup; otherwise the
// timer fires when the interval since the last prompt completes.
func (s *Scheduler) Start(ctx context.Context) error {
	var doc nagDoc
	err := s.cfg.Storage.Get(ctx, docID, &doc)
	if err != nil && !errors.Is(err, storageapi.ErrNotFound) {
		return err
	}
	if err == nil {
		if last, perr := time.Parse(time.RFC3339Nano, doc.LastNag); perr == nil {
			if since := s.cfg.Now().Sub(last); since < s.cfg.Interval {
				s.schedule(s.cfg.Interval - since)
			} else {
				s.schedule(s.cfg.BootDelay)
			}
			return nil
		}
	}
	// First run (or unreadable record): count today as prompted so
	// new installs get a full quiet interval before the first nag.
	if err := s.record(ctx); err != nil {
		return err
	}
	s.schedule(s.cfg.Interval)
	return nil
}

// Stop cancels any pending prompt. The Scheduler cannot be
// restarted.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
}

func (s *Scheduler) schedule(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return
	}
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(d, func() {
		debug.CapturePanicReport(s.fire)
	})
}

// fire records the time, shows the prompt appropriate for the
// current account state, and re-arms the timer for the next
// interval. Recording happens before showing so observers of the
// storage see the timestamp by the time the prompt is visible.
// Active accounts are not prompted but still advance the timestamp,
// so a plan that lapses later is picked up within one interval.
func (s *Scheduler) fire() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.mu.Unlock()
	ctx := context.Background()
	state := s.cfg.State(ctx)
	if err := s.record(ctx); err != nil {
		log.WithError(err).Warn("idenag: record nag time")
	}
	if state != StateActive {
		s.cfg.Show(state)
	}
	s.schedule(s.cfg.Interval)
}

func (s *Scheduler) record(ctx context.Context) error {
	doc := nagDoc{
		Kind:    docKind,
		LastNag: s.cfg.Now().UTC().Format(time.RFC3339Nano),
	}
	return s.cfg.Storage.Set(ctx, docID, &doc)
}
