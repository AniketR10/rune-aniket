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

package exoeditor

import (
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/browser"
)

// eventPublisher decorates browser.EventPublisher with a refresh
// callback that fires on EventInterrupt. refresh is assigned
// concurrently with the vte's reader goroutine (which is started
// inside vte.NewHandler before Edit can wire the callback), so the
// field is stored as an atomic.Pointer to avoid a data race; an
// unset refresh is simply skipped.
type eventPublisher struct {
	publisher browser.EventPublisher
	refresh   atomic.Pointer[func()]
}

func newEventPublisher(publisher browser.EventPublisher) *eventPublisher {
	return &eventPublisher{publisher: publisher}
}

// setRefresh installs fn as the refresh callback. Safe to call from
// any goroutine.
func (p *eventPublisher) setRefresh(fn func()) {
	p.refresh.Store(&fn)
}

func (p *eventPublisher) PublishEvent(ev term.Event) error {
	if ev.Type == term.EventInterrupt {
		if fn := p.refresh.Load(); fn != nil {
			(*fn)()
		}
	}
	return p.publisher.PublishEvent(ev)
}
