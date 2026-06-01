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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

type recordingPublisher struct {
	events []term.Event
}

func (r *recordingPublisher) PublishEvent(ev term.Event) error {
	r.events = append(r.events, ev)
	return nil
}

func TestEventPublisherRefreshesOnInterruptOnly(t *testing.T) {
	t.Parallel()
	pub := &recordingPublisher{}
	wrapper := newEventPublisher(pub)
	var refreshes int
	wrapper.setRefresh(func() { refreshes++ })

	keyEv := term.Event{Type: term.EventKey, Ch: 'x'}
	intrEv := term.Event{Type: term.EventInterrupt}

	require := assert.New(t)
	require.NoError(wrapper.PublishEvent(keyEv))
	require.NoError(wrapper.PublishEvent(intrEv))
	require.NoError(wrapper.PublishEvent(keyEv))

	assert.Equal(t, []term.Event{keyEv, intrEv, keyEv}, pub.events,
		"every event must be forwarded to the wrapped publisher")
	assert.Equal(t, 1, refreshes,
		"refresh must run exactly once per EventInterrupt")
}

func TestEventPublisherNilRefreshIsNop(t *testing.T) {
	t.Parallel()
	pub := &recordingPublisher{}
	wrapper := newEventPublisher(pub)
	assert.NoError(t, wrapper.PublishEvent(term.Event{Type: term.EventInterrupt}))
	assert.Len(t, pub.events, 1)
}
