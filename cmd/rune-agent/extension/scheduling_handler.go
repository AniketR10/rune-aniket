// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package extension

import (
	"context"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// schedulingHandler wraps a repl.Handler to bridge its scheduleNextTick
// callback into the extension context, where there is no local termbox
// event loop. Callbacks are queued via schedule and drained at the start
// of every Draw call, with term.Interrupter used to trigger redraws.
type schedulingHandler struct {
	inner       *repl.Handler
	interrupter term.Interrupter
	ctx         context.Context

	mu      sync.Mutex
	pending []func()
}

// schedule is passed to repl.New as the scheduleNextTick parameter.
// It queues fn and interrupts the event loop to force a redraw.
func (s *schedulingHandler) schedule(fn func()) bool {
	s.mu.Lock()
	s.pending = append(s.pending, fn)
	s.mu.Unlock()

	_ = s.interrupter.Interrupt(s.ctx)
	return true
}

// drainPending executes all queued callbacks. It is called at the
// start of Draw so that any state mutations are visible before rendering.
func (s *schedulingHandler) drainPending() {
	s.mu.Lock()
	batch := s.pending
	s.pending = nil
	s.mu.Unlock()

	for _, fn := range batch {
		fn()
	}
}

func (s *schedulingHandler) Draw(w term.Writer) {
	s.drainPending()
	s.inner.Draw(w)
}

func (s *schedulingHandler) Handle(ev term.Event) (exit, handled bool) {
	return s.inner.Handle(ev)
}

func (s *schedulingHandler) Resize(width, height int) {
	s.inner.Resize(width, height)
}

func (s *schedulingHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return s.inner.Cursor()
}

func (s *schedulingHandler) Selection() (string, bool) {
	return s.inner.Selection()
}
