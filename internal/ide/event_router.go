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

package ide

import (
	"context"
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

type eventRouter struct {
	publish func(term.Event) bool
	// focusURI is mirrored from h.focus so workspace-bound publishers
	// invoked from extension RPC and vte goroutines (which do not hold
	// h.mu) can compare against the focused workspace without racing.
	focusURI atomic.Pointer[workspaceapi.URI]
}

func newEventRouter(publish func(term.Event) bool) *eventRouter {
	r := &eventRouter{publish: publish}
	var zero workspaceapi.URI
	r.focusURI.Store(&zero)
	return r
}

func (r *eventRouter) setFocus(uri workspaceapi.URI) {
	r.focusURI.Store(&uri)
}

func (r *eventRouter) globalPublisher() func(term.Event) bool {
	return r.publish
}

func (r *eventRouter) newPublisher(uri workspaceapi.URI) func(term.Event) bool {
	return func(ev term.Event) bool {
		if ev.Type == term.EventInterrupt &&
			ev.UserFunc == nil && ev.Raw == nil && ev.Context == nil {
			focus := r.focusURI.Load()
			if focus != nil && uri != *focus {
				// Pretend success so the caller does not retry a
				// redraw we intentionally dropped.
				return true
			}
		}
		return r.publish(ev)
	}
}

func (r *eventRouter) globalInterrupter() term.Interrupter {
	return routerInterrupter{publish: r.globalPublisher()}
}

func (r *eventRouter) newInterrupter(uri workspaceapi.URI) term.Interrupter {
	return routerInterrupter{publish: r.newPublisher(uri)}
}

type routerInterrupter struct {
	publish func(term.Event) bool
}

func (r routerInterrupter) Interrupt(ctx context.Context) error {
	payload, _ := term.PayloadFromContext(ctx)
	ev := term.Event{Type: term.EventInterrupt, Raw: payload, Context: ctx}
	if !r.publish(ev) {
		return errEventStreamNotReady
	}
	return nil
}
