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

package ide_test

import (
	"context"
	"sync"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/internal/ide"
	"unstable.build/rune/internal/localstorage"
)

func newE2EStorage(t *testing.T, dataDir string) storageapi.Service {
	t.Helper()
	return localstorage.New(context.Background(), dataDir, docbson.Marshaler())
}

// hostScheduleNextTick mirrors a host event loop's UserFunc dispatch:
// fn runs on a fresh goroutine while holding mu, exactly like
// gui.Update does before invoking ev.UserFunc(). Tests use this to
// preserve the production contract that scheduled callbacks observe
// IDE state under the host lock.
func hostScheduleNextTick(mu sync.Locker) func(func()) bool {
	return func(fn func()) bool {
		go func() {
			mu.Lock()
			defer mu.Unlock()
			fn()
		}()
		return true
	}
}

// schedTracker wraps a scheduler with a pending-callback counter so
// the test can wait for every queued scheduled callback to actually
// run, not just the IDE flusher's onDone. The IDE flusher's
// WaitInflight only blocks for awaiter goroutines; text/component's
// dispatchFlush is scheduled independently and otherwise has no
// observable completion handle in tests.
type schedTracker struct {
	inner   func(func()) bool
	muCount sync.Mutex
	cond    *sync.Cond
	pending int
}

func newSchedTracker(inner func(func()) bool) *schedTracker {
	s := &schedTracker{inner: inner}
	s.cond = sync.NewCond(&s.muCount)
	return s
}

func (s *schedTracker) Schedule(fn func()) bool {
	s.muCount.Lock()
	s.pending++
	s.muCount.Unlock()
	return s.inner(func() {
		defer func() {
			s.muCount.Lock()
			s.pending--
			if s.pending == 0 {
				s.cond.Broadcast()
			}
			s.muCount.Unlock()
		}()
		fn()
	})
}

func (s *schedTracker) Wait() {
	s.muCount.Lock()
	for s.pending > 0 {
		s.cond.Wait()
	}
	s.muCount.Unlock()
}

// e2eLockedHandler mirrors the way the production event loop drives
// the IDE handler: every Handle/Draw acquires mu before delegating
// and releases it afterwards so scheduled callbacks (spawned by
// hostScheduleNextTick) can run in between. After each Handle, the
// wrapper waits for in-flight async saves/reloads so the next
// Draw/Handle observes the post-completion state.
type e2eLockedHandler struct {
	tui.Handler
	mu    *sync.Mutex
	ide   *ide.IDE
	sched *schedTracker
}

func (h e2eLockedHandler) Handle(ev term.Event) (bool, bool) {
	h.mu.Lock()
	quit, handled := h.Handler.Handle(ev)
	h.mu.Unlock()
	h.ide.WaitInflight()
	// WaitInflight only waits for the IDE flusher's awaiter
	// goroutines. text/component.dispatchFlush is scheduled
	// independently through the same scheduler and would otherwise
	// race the next Draw — wait for the scheduler to drain too.
	if h.sched != nil {
		h.sched.Wait()
	}
	return quit, handled
}

func (h e2eLockedHandler) Draw(w term.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Handler.Draw(w)
}

func (h e2eLockedHandler) Resize(width, height int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Handler.Resize(width, height)
}

func (h e2eLockedHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.Handler.Cursor()
}

func (h e2eLockedHandler) Selection() (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.Handler.Selection()
}
