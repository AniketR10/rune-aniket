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

package ideplan

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

type stubSource struct {
	mu            sync.Mutex
	decisions     []Decision
	index         int
	refreshes     atomic.Int32
	decisionCalls atomic.Int32
}

func (s *stubSource) Decision(context.Context) (Decision, error) {
	s.decisionCalls.Add(1)
	return s.next(), nil
}

func (s *stubSource) Refresh(context.Context) (Decision, error) {
	s.refreshes.Add(1)
	return s.next(), nil
}

func (s *stubSource) next() Decision {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.index >= len(s.decisions) {
		return Decision{Status: StatusExpired}
	}
	d := s.decisions[s.index]
	s.index++
	return d
}

type recordingLocker struct {
	mu     sync.Mutex
	calls  []bool
	locked bool
}

func (l *recordingLocker) SetLocked(locked bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, locked)
	l.locked = locked
}

func TestMonitorActiveUnlocks(t *testing.T) {
	src := &stubSource{decisions: []Decision{{Status: StatusActive}}}
	lock := &recordingLocker{locked: true}
	m := NewMonitor(MonitorConfig{
		Source: src,
		Locker: lock,
	})
	m.Tick(context.Background())
	assert.False(t, lock.locked)
}

// TestMonitorReevaluateUsesDecisionNotRefresh pins the post-login
// re-evaluation contract: Reevaluate must read the cached token via
// Decision and never drive the refresh_token grant, so a from-scratch
// re-signin reacts to the freshly minted token instead of rotating
// the prior session.
func TestMonitorReevaluateUsesDecisionNotRefresh(t *testing.T) {
	src := &stubSource{decisions: []Decision{{Status: StatusActive}}}
	lock := &recordingLocker{locked: true}
	m := NewMonitor(MonitorConfig{
		Source: src,
		Locker: lock,
	})
	m.Reevaluate(context.Background())
	assert.False(t, lock.locked)
	assert.Equal(t, int32(0), src.refreshes.Load(),
		"Reevaluate must not use the refresh_token grant")
	assert.Equal(t, int32(1), src.decisionCalls.Load())
}

func TestMonitorExpiredLocks(t *testing.T) {
	src := &stubSource{decisions: []Decision{{Status: StatusExpired}}}
	lock := &recordingLocker{}
	m := NewMonitor(MonitorConfig{
		Source: src,
		Locker: lock,
	})
	m.Tick(context.Background())
	assert.True(t, lock.locked)
}

func TestMonitorNeverSubscribedLocks(t *testing.T) {
	src := &stubSource{decisions: []Decision{
		{Status: StatusNeverSubscribed, SignedIn: SignedIn},
	}}
	lock := &recordingLocker{}
	m := NewMonitor(MonitorConfig{
		Source: src,
		Locker: lock,
	})
	m.Tick(context.Background())
	assert.True(t, lock.locked,
		"never-subscribed must request enforcement like expired")
}

func TestMonitorUpgradeExpiredSignalsEnforcement(t *testing.T) {
	src := &stubSource{decisions: []Decision{
		{Status: StatusUpgradeExpired, SignedIn: SignedIn},
	}}
	lock := &recordingLocker{}
	m := NewMonitor(MonitorConfig{
		Source: src,
		Locker: lock,
	})
	m.Tick(context.Background())
	assert.True(t, lock.locked,
		"upgrade-expired must request enforcement so the planner "+
			"can apply the usage runway")
}

func TestNewMonitorPanicsOnMissingDependencies(t *testing.T) {
	valid := func() MonitorConfig {
		return MonitorConfig{
			Source: &stubSource{},
			Locker: &recordingLocker{},
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*MonitorConfig)
	}{
		{name: "nil source", mutate: func(c *MonitorConfig) { c.Source = nil }},
		{name: "nil locker", mutate: func(c *MonitorConfig) { c.Locker = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid()
			tc.mutate(&cfg)
			assert.Panics(t, func() { NewMonitor(cfg) })
		})
	}
	t.Run("interval is defaulted", func(t *testing.T) {
		assert.NotPanics(t, func() { NewMonitor(valid()) })
	})
}
