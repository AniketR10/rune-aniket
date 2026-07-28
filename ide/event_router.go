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
