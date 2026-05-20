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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// newTestPublishOverride lets a test install a custom publish closure
// for the next call to newTestWorkspaceManagerHandlerWithManagerMu
// (and its callers). Tests must reset it to nil in t.Cleanup so other
// tests fall back to the default "always succeed" publisher.
var newTestPublishOverride func(term.Event) bool

// recordingPublisher captures every term.Event passed through it.
// Callers obtain the snapshot via events and reset state with clear.
type recordingPublisher struct {
	mu     sync.Mutex
	events []term.Event
}

func (r *recordingPublisher) publish(ev term.Event) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, ev)
	return true
}

func (r *recordingPublisher) snapshot() []term.Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]term.Event, len(r.events))
	copy(out, r.events)
	return out
}

func (r *recordingPublisher) clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = r.events[:0]
}

func TestEventRouterDropsNonFocusPureInterrupts(t *testing.T) {
	rec := &recordingPublisher{}
	newTestPublishOverride = rec.publish
	t.Cleanup(func() { newTestPublishOverride = nil })

	uriA, err := workspaceapi.ParseURI("memory:///a")
	require.NoError(t, err)
	uriB, err := workspaceapi.ParseURI("memory:///b")
	require.NoError(t, err)

	m := newTestWorkspaceManagerHandler(t, defaultCfg(), nil,
		nopShutdownShaderConfig())

	m.mu.Lock()
	require.NoError(t, m.addOrCreateWorkspace(uriA))
	m.mu.Unlock()
	m.waitForWorkspace(t, uriA)

	m.mu.Lock()
	require.NoError(t, m.addOrCreateWorkspace(uriB))
	m.mu.Unlock()
	m.waitForWorkspace(t, uriB)

	// Focus workspace A.
	m.mu.Lock()
	slotA, _ := m.findInstalledSlot(uriA)
	require.True(t, m.switchToWorkspace(slotA))
	m.mu.Unlock()

	pureInterrupt := term.Event{Type: term.EventInterrupt}
	withUserFunc := term.Event{Type: term.EventInterrupt, UserFunc: func() {}}
	withRaw := term.Event{Type: term.EventInterrupt, Raw: []byte("x")}
	withCtx := term.Event{Type: term.EventInterrupt, Context: context.Background()}
	keyEv := term.Event{Type: term.EventKey, Key: term.KeyEnter}

	cases := []struct {
		name      string
		ev        term.Event
		recorded  bool
		returnVal bool
	}{
		{"pure-redraw", pureInterrupt, false, true},
		{"user-func", withUserFunc, true, true},
		{"raw-payload", withRaw, true, true},
		{"context", withCtx, true, true},
		{"non-interrupt", keyEv, true, true},
	}

	// Workspace B is bound but not focused — pure interrupts must drop.
	for _, tc := range cases {
		t.Run("B/"+tc.name, func(t *testing.T) {
			rec.clear()
			pub := m.events.newPublisher(uriB)
			got := pub(tc.ev)
			assert.Equal(t, tc.returnVal, got)
			snap := rec.snapshot()
			if tc.recorded {
				require.Len(t, snap, 1)
				assert.Equal(t, tc.ev.Type, snap[0].Type)
			} else {
				assert.Empty(t, snap)
			}
		})
	}

	// Workspace A is focused — every event must reach the publisher.
	for _, tc := range cases {
		t.Run("A/"+tc.name, func(t *testing.T) {
			rec.clear()
			pub := m.events.newPublisher(uriA)
			got := pub(tc.ev)
			assert.Equal(t, tc.returnVal, got)
			snap := rec.snapshot()
			require.Len(t, snap, 1)
			assert.Equal(t, tc.ev.Type, snap[0].Type)
		})
	}

	// Switching focus to B should reverse the routing without
	// rebuilding the publisher closure.
	m.mu.Lock()
	slotB, _ := m.findInstalledSlot(uriB)
	require.True(t, m.switchToWorkspace(slotB))
	m.mu.Unlock()

	rec.clear()
	pubA := m.events.newPublisher(uriA)
	assert.True(t, pubA(pureInterrupt))
	assert.Empty(t, rec.snapshot(),
		"A is no longer focused: pure interrupt must drop")

	rec.clear()
	pubB := m.events.newPublisher(uriB)
	assert.True(t, pubB(pureInterrupt))
	require.Len(t, rec.snapshot(), 1,
		"B is now focused: pure interrupt must pass through")

	rec.clear()
	publish := m.events.globalPublisher()
	assert.True(t, publish(pureInterrupt))
	require.Len(t, rec.snapshot(), 1,
		"global publisher must not filter pure interrupts")
}
