// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

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
