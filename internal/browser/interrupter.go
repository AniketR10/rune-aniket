// Copyright (C) 2017-2026 The Rune Authors
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

package browser

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// EventPublisherInterrupter wraps an EventPublisher to satisfy term.Interrupter.
func EventPublisherInterrupter(e EventPublisher) term.Interrupter {
	return pubInterrupt{e: e}
}

type pubInterrupt struct {
	e EventPublisher
}

func (p pubInterrupt) Interrupt(ctx context.Context) error {
	payload, _ := term.PayloadFromContext(ctx)
	return p.e.PublishEvent(term.Event{Type: term.EventInterrupt, Raw: payload})
}
