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
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
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

type recordingNotifier struct {
	mu       sync.Mutex
	notify   []string
	progress []progressCall
}

type progressCall struct {
	id              string
	message         string
	progress, total int64
}

func (n *recordingNotifier) Notify(_ browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.notify = append(n.notify, msg)
	_ = args
	return "id", nil
}
func (n *recordingNotifier) NotifyOnce(_ browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	return n.Notify(0, msg, args...)
}
func (n *recordingNotifier) UpdateNotificationProgress(id, msg string, p, total int64) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.progress = append(n.progress, progressCall{id, msg, p, total})
	return nil
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
	notif := &recordingNotifier{}
	lock := &recordingLocker{locked: true}
	m := NewMonitor(MonitorConfig{
		Source:        src,
		Notifications: notif,
		Locker:        lock,
		Now:           func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) },
	})
	m.Tick(context.Background())
	assert.False(t, lock.locked)
	assert.Empty(t, notif.notify)
	assert.Empty(t, notif.progress)
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
		Now:    func() time.Time { return time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC) },
	})
	m.Reevaluate(context.Background())
	assert.False(t, lock.locked)
	assert.Equal(t, int32(0), src.refreshes.Load(),
		"Reevaluate must not use the refresh_token grant")
	assert.Equal(t, int32(1), src.decisionCalls.Load())
}

func TestMonitorGracePinsWarningOncePerDay(t *testing.T) {
	now := time.Date(2026, 6, 1, 23, 0, 0, 0, time.FixedZone("minus-two", -2*60*60))
	graceUntil := now.Add(3 * 24 * time.Hour)
	dec := Decision{Status: StatusGracePeriod, PlanEnds: now.Add(-24 * time.Hour), GraceUntil: graceUntil}
	src := &stubSource{decisions: []Decision{dec, dec, dec}}
	notif := &recordingNotifier{}
	lock := &recordingLocker{}
	m := NewMonitor(MonitorConfig{
		Source:        src,
		Notifications: notif,
		Locker:        lock,
		Now:           func() time.Time { return now },
	})
	m.Tick(context.Background())
	m.Tick(context.Background())
	assert.Len(t, notif.notify, 1)
	assert.Len(t, notif.progress, 1)
	assert.Contains(t, notif.progress[0].message, "4 day(s)")
	assert.Equal(t, int64(99), notif.progress[0].progress)
	assert.Equal(t, int64(100), notif.progress[0].total)
	assert.Equal(t, "id", notif.progress[0].id)
}

func TestMonitorActiveClearsPinnedWarning(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	dec := Decision{Status: StatusGracePeriod, PlanEnds: now.Add(-24 * time.Hour), GraceUntil: now.Add(24 * time.Hour)}
	src := &stubSource{decisions: []Decision{dec, {Status: StatusActive}}}
	notif := &recordingNotifier{}
	lock := &recordingLocker{locked: true}
	m := NewMonitor(MonitorConfig{
		Source:        src,
		Notifications: notif,
		Locker:        lock,
		Now:           func() time.Time { return now },
	})
	m.Tick(context.Background())
	m.Tick(context.Background())

	assert.False(t, lock.locked)
	assert.Len(t, notif.progress, 2)
	assert.Equal(t, "id", notif.progress[1].id)
	assert.Equal(t, int64(100), notif.progress[1].progress)
	assert.Equal(t, int64(100), notif.progress[1].total)
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
