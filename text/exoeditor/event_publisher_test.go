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
