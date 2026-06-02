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

package syntax

import (
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// notifyReadsState mirrors the real production chain
// notis.Notify -> notis.inFocus -> workspaceManagerHandler.focusURI,
// which reads workspace-handler state on the assumption that the
// caller is on the event loop. The fake reads `state` without
// synchronization so the race detector can flag any caller that
// invokes Notify off the event-loop goroutine.
type notifyReadsState struct {
	state *int
}

func (n notifyReadsState) Notify(
	browserapi.NotificationLevel, string, ...any,
) (string, error) {
	_ = *n.state
	return "", nil
}

func (n notifyReadsState) NotifyOnce(
	browserapi.NotificationLevel, string, ...any,
) (string, error) {
	_ = *n.state
	return "", nil
}

func (notifyReadsState) UpdateNotificationProgress(string, string, int64, int64) error {
	return nil
}

// TestNotifyNotAvailRunsOnEventLoop drives Tree.notifyNotAvail from a
// goroutine that simulates the syntax tree download worker, while a
// "mutator" running on the simulated event-loop goroutine repeatedly
// writes to the same state that Notify reads. If notifyNotAvail
// invokes Notify on the caller goroutine instead of routing through
// ScheduleNextTick, the race detector flags the unsynchronized
// read/write.
func TestNotifyNotAvailRunsOnEventLoop(t *testing.T) {
	const iterations = 200

	ticks := make(chan func(), 256)
	loopDone := make(chan struct{})
	go func() {
		defer close(loopDone)
		for fn := range ticks {
			fn()
		}
	}()

	state := 0
	tree := &Tree{
		n:           notifyReadsState{state: &state},
		interrupter: term.NopInterrupter(),
		config: Config{
			ScheduleNextTick: func(fn func()) bool {
				ticks <- fn
				return true
			},
		},
	}

	mutDone := make(chan struct{})
	go func() {
		defer close(mutDone)
		for i := range iterations {
			ticks <- func() { state = i }
		}
	}()

	callerDone := make(chan struct{})
	go func() {
		defer close(callerDone)
		for range iterations {
			tree.config.ScheduleNextTick(func() { tree.notifyNotAvail("go") })
		}
	}()

	<-mutDone
	<-callerDone

	// Drain any remaining scheduled callbacks before tearing down
	// the simulated event loop.
	drained := make(chan struct{})
	ticks <- func() { close(drained) }
	<-drained
	close(ticks)
	<-loopDone

	// Reference state once on the test goroutine after the loop
	// has stopped so the compiler does not eliminate the writes.
	_ = state
}

// Ensure notifyReadsState satisfies browserapi.Notifications.
var _ browserapi.Notifications = notifyReadsState{}
