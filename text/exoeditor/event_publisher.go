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

package exoeditor

import (
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
)

// eventPublisher decorates browser.EventPublisher with a refresh
// callback that fires on EventInterrupt. refresh is assigned
// concurrently with the vte's reader goroutine (which is started
// inside vte.NewHandler before Edit can wire the callback), so the
// field is stored as an atomic.Pointer to avoid a data race; an
// unset refresh is simply skipped.
type eventPublisher struct {
	publisher browser.EventPublisher
	refresh   atomic.Pointer[func()]
}

func newEventPublisher(publisher browser.EventPublisher) *eventPublisher {
	return &eventPublisher{publisher: publisher}
}

// setRefresh installs fn as the refresh callback. Safe to call from
// any goroutine.
func (p *eventPublisher) setRefresh(fn func()) {
	p.refresh.Store(&fn)
}

func (p *eventPublisher) PublishEvent(ev term.Event) error {
	if ev.Type == term.EventInterrupt {
		if fn := p.refresh.Load(); fn != nil {
			(*fn)()
		}
	}
	return p.publisher.PublishEvent(ev)
}
