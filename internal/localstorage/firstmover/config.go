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

package firstmover

import (
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
)

// Config holds configuration for a firstmover storageapi.Service.
type Config struct {
	Marshaler docmarshal.Marshaler
	// TransientFailureRecoverTimeout is the timeout until a grpc
	// transient connection failure is considered unrecoverable..
	TransientFailureRecoverTimeout time.Duration
	// MethodRetryCadence is the method retry timeout until next attempt.
	MethodRetryCadence time.Duration
	// ReceiveRetryCadence is the receive retry cadence to accomodate
	// new leader/follower assigns. It should be much much longer than
	// MethodRetryCadence as receive is expected to block.
	ReceiveRetryCadence time.Duration
	// ConnectRetryCadence is the connect retry timeout until next attempt.
	ConnectRetryCadence time.Duration
	// TimeToCoup is the time for a follower to take the lead
	// if leader is unresponsive.
	TimeToCoup time.Duration
	// DialTimeout is net.Dial timeout
	DialTimeout time.Duration
	// CloseError can be optionally set to an error value that
	// the underlying storageapi.Service passes when it's been
	// called Close and any other calls to its API will fail.
	// This error will be retried by followers until a new leader
	// is selected.
	CloseError error
	// MaxMessageSize determines the maximum message size
	// of published messages via Service.Publish.
	MaxMessageSize int
}

// DefaultConfig returns a sane Config.
func DefaultConfig() Config {
	return Config{
		Marshaler:                      docbson.Marshaler(),
		TransientFailureRecoverTimeout: 500 * time.Millisecond,
		MethodRetryCadence:             200 * time.Millisecond,
		ReceiveRetryCadence:            5 * time.Second,
		ConnectRetryCadence:            50 * time.Millisecond,
		TimeToCoup:                     1 * time.Second,
		DialTimeout:                    40 * time.Millisecond,
		MaxMessageSize:                 DefaultMaxMessageSize,
	}
}

// methodRetryBudget is how long a method call keeps retrying before it
// gives the transport error back to the caller. A follower only starts
// the coup after TimeToCoup of failed dials and then still has to take
// the lock and bind its listener, so the budget covers two coups:
// a call that races the death of the leader rides the election out
// instead of failing.
func methodRetryBudget(cfg Config) time.Duration {
	return 2 * cfg.TimeToCoup
}
