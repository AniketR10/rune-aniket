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

package vteparser

import "time"

// Timeout provides an interface for creating timeouts and checking their expiry.
type Timeout interface {
	// SetTimeout sets the timeout for the next synchronized update.
	//
	// The duration parameter specifies the duration of the timeout. Once the
	// specified duration has elapsed, the synchronized update routine can be
	// performed.
	SetTimeout(duration time.Duration)

	// ClearTimeout clears the current timeout.
	ClearTimeout()

	// PendingTimeout returns whether a timeout is currently active and has not yet expired.
	PendingTimeout() bool
}

// StdTimeout represents a std library's Timeout implementation.
// A zero value is ready to be used.
type StdTimeout struct {
	timeout time.Time
}

// SyncTimeout returns the synchronized update expiration time.
func (h *StdTimeout) SyncTimeout() time.Time {
	return h.timeout
}

// SetTimeout sets the timeout for the next synchronized update.
func (h *StdTimeout) SetTimeout(duration time.Duration) {
	h.timeout = time.Now().Add(duration)
}

// ClearTimeout clears the current timeout.
func (h *StdTimeout) ClearTimeout() {
	h.timeout = time.Time{}
}

// PendingTimeout returns whether a timeout is currently active and has not yet expired.
func (h *StdTimeout) PendingTimeout() bool {
	// The zero check keeps the common no-sync-update path free of a
	// clock read; PendingTimeout is polled once per parsed batch.
	return !h.timeout.IsZero() && time.Now().Before(h.timeout)
}
