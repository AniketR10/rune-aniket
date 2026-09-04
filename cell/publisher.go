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

package cell

import (
	"context"

	"slices"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// A cell.Editor that synchronously
// publishes updated to a list of Subscribers
type syncPublisher struct {
	w           Editor
	subscribers []Subscriber
}

func newPublisher(w Editor) *syncPublisher {
	p := new(syncPublisher)
	p.w = w
	return p
}

func (p *syncPublisher) Edit(ctx context.Context, start, end term.Coordinates, str string) (
	from, to term.Coordinates, old string,
) {
	for _, sub := range p.subscribers {
		sub.OnWillEdit(ctx, start, end, str)
	}
	// Pair every OnWillEdit with an OnDidEdit even if Edit panics:
	// subscribers release in-flight edit state in OnDidEdit, which
	// workspace.file waits on while closing, so a missed call wedges
	// teardown instead of surfacing the panic as a crash report.
	notified := false
	defer func() {
		if !notified {
			p.notifyDidEdit(ctx, start, start, "")
		}
	}()

	from, to, old = p.w.Edit(ctx, start, end, str)
	notified = true
	p.notifyDidEdit(ctx, from, to, old)
	return
}

func (p *syncPublisher) notifyDidEdit(
	ctx context.Context, from, to term.Coordinates, old string,
) {
	for _, sub := range p.subscribers {
		sub.OnDidEdit(ctx, from, to, old)
	}
}

func (p *syncPublisher) Subscribe(s Subscriber) {
	p.subscribers = append(p.subscribers, s)
}

func (p *syncPublisher) Unsubscribe(s Subscriber) {
	unsubs := -1
	for i, sub := range p.subscribers {
		if sub == s {
			unsubs = i
			break
		}
	}
	if unsubs < 0 {
		panic("Subscriber is not subscribed")
	}
	// slices.Delete clears the tail slot so the removed Subscriber (often an
	// editorFlusherCloser holding *cell.Buffer) is not pinned via the
	// backing array past len.
	p.subscribers = slices.Delete(p.subscribers, unsubs, unsubs+1)
}
