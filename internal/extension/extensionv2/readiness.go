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

package extensionv2

import (
	"context"
	"sync"
)

// extensionReadiness is a one-shot signal that the extension protocol
// handshake has completed. The protocol layer publishes the outcome via
// Set; startExtension waits for it via Wait. It is shared memory used to
// communicate, rather than a channel or a callback: producers and
// consumers synchronise through its internal mutex and condition
// variable, so lifecycle code in the runner does not need to juggle
// one-shot channels or nil sentinels when a stop arrives before
// completion.
type extensionReadiness struct {
	mu   sync.Mutex
	cond *sync.Cond
	done bool
	err  error
}

func newExtensionReadiness() *extensionReadiness {
	r := &extensionReadiness{}
	r.cond = sync.NewCond(&r.mu)
	return r
}

// Set marks the readiness as resolved with err. The first call wins;
// subsequent calls are ignored so the successful handshake outcome
// cannot be overwritten by a later stop, and a terminal error cannot
// be overwritten by a spurious success.
func (r *extensionReadiness) Set(err error) {
	r.mu.Lock()
	if r.done {
		r.mu.Unlock()
		return
	}
	r.done = true
	r.err = err
	r.mu.Unlock()
	r.cond.Broadcast()
}

// Wait blocks until Set has been called or ctx is cancelled, whichever
// happens first. Returns the terminal error from Set, or ctx.Err() if
// ctx was cancelled before Set ran.
func (r *extensionReadiness) Wait(ctx context.Context) error {
	stop := context.AfterFunc(ctx, func() {
		r.mu.Lock()
		r.cond.Broadcast()
		r.mu.Unlock()
	})
	defer stop()

	r.mu.Lock()
	defer r.mu.Unlock()
	for !r.done && ctx.Err() == nil {
		r.cond.Wait()
	}
	if !r.done {
		return ctx.Err()
	}
	return r.err
}
