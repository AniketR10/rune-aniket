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
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/browser"
)

// PublisherFunc adapts the IDE's `func(term.Event) bool` event-loop
// publisher to browser.EventPublisher. ok==false from the underlying
// function is surfaced as a non-nil error so the caller can decide
// whether to drop or retry the event.
type PublisherFunc func(term.Event) bool

// PublishEvent satisfies browser.EventPublisher.
func (p PublisherFunc) PublishEvent(ev term.Event) error {
	if p(ev) {
		return nil
	}
	return errPublisherClosed
}

var _ browser.EventPublisher = PublisherFunc(nil)

var errPublisherClosed = errPublisherClosedT("exoeditor: event publisher closed")

type errPublisherClosedT string

func (e errPublisherClosedT) Error() string { return string(e) }
