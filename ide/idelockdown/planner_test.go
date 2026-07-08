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

package idelockdown

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"unstable.build/go-tui/ide/ideplan"
)

type recordingLocker struct {
	mu      sync.Mutex
	calls   []bool
	reasons []LockReason
}

func (l *recordingLocker) SetLocked(locked bool, reason LockReason) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, locked)
	l.reasons = append(l.reasons, reason)
}

func (l *recordingLocker) last() (bool, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.calls) == 0 {
		return false, false
	}
	return l.calls[len(l.calls)-1], true
}

func (l *recordingLocker) lastReason() (LockReason, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.reasons) == 0 {
		return LockNone, false
	}
	return l.reasons[len(l.reasons)-1], true
}

type staticSource struct {
	dec ideplan.Decision
}

func (s staticSource) Decision(context.Context) (ideplan.Decision, error) { return s.dec, nil }
func (s staticSource) Refresh(context.Context) (ideplan.Decision, error)  { return s.dec, nil }

func expiredSource() ideplan.Source {
	return staticSource{dec: ideplan.Decision{Status: ideplan.StatusExpired}}
}

// stubPolicy lets tests drive the evaluation chain at arbitrary
// cadences.
type stubPolicy struct {
	next  func(s Snapshot) time.Time
	calls atomic.Int32
}

func (p *stubPolicy) Name() string { return "stub" }

func (p *stubPolicy) Evaluate(s Snapshot) (time.Time, Action) {
	p.calls.Add(1)
	return p.next(s), ActionNone
}

type fakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func newTestPlanner(
	t *testing.T, store storageapi.Service, src ideplan.Source,
	locker Locker, showPrompt func(), now func() time.Time,
) *Planner {
	t.Helper()
	p := New(Config{
		Storage:               store,
		Source:                src,
		Policies:              DefaultPolicies(),
		Locker:                locker,
		ShowNagPrompt:         showPrompt,
		ShowAskMoreTimePrompt: func() {},
		Now:                   now,
	})
	require.NoError(t, p.Start(context.Background()))
	t.Cleanup(p.Stop)
	return p
}

// TestNewPanicsOnMissingDependencies pins the construction contract:
// every dependency is wired unconditionally in production, so a nil
// must fail loudly at construction rather than silently disabling
// enforcement at some call site.
func TestNewPanicsOnMissingDependencies(t *testing.T) {
	valid := func() Config {
		return Config{
			Storage:               storagestub.NewInMemoryService(),
			Source:                expiredSource(),
			Policies:              DefaultPolicies(),
			Locker:                &recordingLocker{},
			ShowNagPrompt:         func() {},
			ShowAskMoreTimePrompt: func() {},
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Config)
	}{
		{name: "nil storage", mutate: func(c *Config) { c.Storage = nil }},
		{name: "nil source", mutate: func(c *Config) { c.Source = nil }},
		{name: "empty policies", mutate: func(c *Config) { c.Policies = nil }},
		{name: "nil locker", mutate: func(c *Config) { c.Locker = nil }},
		{name: "nil show nag prompt", mutate: func(c *Config) { c.ShowNagPrompt = nil }},
		{name: "nil show ask-more-time prompt",
			mutate: func(c *Config) { c.ShowAskMoreTimePrompt = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid()
			tc.mutate(&cfg)
			assert.Panics(t, func() { New(cfg) })
		})
	}

	t.Run("nil now is defaulted", func(t *testing.T) {
		assert.NotPanics(t, func() { New(valid()) })
	})
}

func seedUsage(t *testing.T, store storageapi.Service, samples []time.Time) {
	t.Helper()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))
	for _, s := range samples {
		require.NoError(t, tr.recordUsage(context.Background(), s))
	}
}

func TestPlannerStartRecordsLaunchSample(t *testing.T) {
	store := storagestub.NewInMemoryService()
	now := monday.Add(10 * time.Hour)
	newTestPlanner(t, store, expiredSource(), &recordingLocker{}, func() {},
		func() time.Time { return now })

	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))
	assert.Equal(t, []time.Time{bucketStart(now, usageBucket)}, tr.usage(),
		"the launch sample must be bucket-aligned")
}

// TestPlannerScheduledEvaluationRecordsNewSample drives the real
// scheduling path at a fast cadence: the stub policy re-evaluates
// every 20ms of fake time (20ms of real time on the timer), and the
// usage-bucket advance is picked up by a scheduled evaluation, not
// by a direct call.
func TestPlannerScheduledEvaluationRecordsNewSample(t *testing.T) {
	store := storagestub.NewInMemoryService()
	clock := &fakeClock{now: monday.Add(10 * time.Hour)}
	pol := &stubPolicy{next: func(s Snapshot) time.Time {
		return s.Now.Add(20 * time.Millisecond)
	}}
	p := New(Config{
		Storage:               store,
		Source:                expiredSource(),
		Policies:              []Policy{pol},
		Locker:                &recordingLocker{},
		ShowNagPrompt:         func() {},
		ShowAskMoreTimePrompt: func() {},
		Now:                   clock.Now,
	})
	require.NoError(t, p.Start(context.Background()))
	t.Cleanup(p.Stop)

	clock.Advance(usageBucket)
	require.Eventually(t, func() bool {
		tr := newTracker(store)
		require.NoError(t, tr.load(context.Background()))
		return len(tr.usage()) == 2
	}, 5*time.Second, 10*time.Millisecond,
		"a scheduled evaluation must record the new usage sample")
}

// TestPlannerSchedulesEarliestPolicyTime pins the chain cadence
// contract: with one policy asking for an hour and another for a few
// milliseconds, evaluations must recur at the earlier time.
func TestPlannerSchedulesEarliestPolicyTime(t *testing.T) {
	slow := &stubPolicy{next: func(s Snapshot) time.Time {
		return s.Now.Add(time.Hour)
	}}
	fast := &stubPolicy{next: func(s Snapshot) time.Time {
		return s.Now.Add(5 * time.Millisecond)
	}}
	p := New(Config{
		Storage:               storagestub.NewInMemoryService(),
		Source:                expiredSource(),
		Policies:              []Policy{slow, fast},
		Locker:                &recordingLocker{},
		ShowNagPrompt:         func() {},
		ShowAskMoreTimePrompt: func() {},
		Now:                   time.Now,
	})
	require.NoError(t, p.Start(context.Background()))
	t.Cleanup(p.Stop)

	require.Eventually(t, func() bool { return slow.calls.Load() >= 3 },
		5*time.Second, time.Millisecond,
		"the chain must recur at the earliest requested time and evaluate all policies")
}

// TestPlannerZeroNextTimeEndsChain pins the zero-time contract: the
// chain stops, and an out-of-band enforcement signal restarts it.
func TestPlannerZeroNextTimeEndsChain(t *testing.T) {
	pol := &stubPolicy{next: func(Snapshot) time.Time { return time.Time{} }}
	p := New(Config{
		Storage:               storagestub.NewInMemoryService(),
		Source:                expiredSource(),
		Policies:              []Policy{pol},
		Locker:                &recordingLocker{},
		ShowNagPrompt:         func() {},
		ShowAskMoreTimePrompt: func() {},
		Now:                   time.Now,
	})
	require.NoError(t, p.Start(context.Background()))
	t.Cleanup(p.Stop)
	require.Equal(t, int32(1), pol.calls.Load())

	time.Sleep(50 * time.Millisecond)
	assert.Equal(t, int32(1), pol.calls.Load(),
		"a zero next time must not schedule another evaluation")

	p.SetLocked(true)
	assert.Equal(t, int32(2), pol.calls.Load(),
		"an enforcement signal must still evaluate after the chain ended")
}

func TestPlannerSetLockedFalseUnlocks(t *testing.T) {
	store := storagestub.NewInMemoryService()
	locker := &recordingLocker{}
	p := newTestPlanner(t, store, expiredSource(), locker, func() {},
		func() time.Time { return monday })

	p.SetLocked(false)
	last, ok := locker.last()
	require.True(t, ok)
	assert.False(t, last, "unlock must pass through to the inner locker")
	reason, ok := locker.lastReason()
	require.True(t, ok)
	assert.Equal(t, LockNone, reason, "unlock must carry LockNone")
}

func TestPlannerExpiredWithoutUsageDoesNotLock(t *testing.T) {
	store := storagestub.NewInMemoryService()
	locker := &recordingLocker{}
	p := newTestPlanner(t, store, expiredSource(), locker, func() {},
		func() time.Time { return monday })

	p.SetLocked(true)
	last, ok := locker.last()
	require.True(t, ok)
	assert.False(t, last,
		"expired without qualifying usage must resolve to unlocked")
}

func TestPlannerExpiredWithLockdownRunLocks(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedUsage(t, store, qualifyingWindows(0, int(lockdownRun/qualifyWindow)))
	locker := &recordingLocker{}
	p := newTestPlanner(t, store, expiredSource(), locker, func() {},
		func() time.Time { return windowBase.Add(lockdownRun) })

	p.SetLocked(true)
	last, ok := locker.last()
	require.True(t, ok)
	assert.True(t, last, "a lockdown run while expired must lock")
	reason, ok := locker.lastReason()
	require.True(t, ok)
	assert.Equal(t, LockExpired, reason,
		"an expired lockdown must carry the expired reason")
}

func TestPlannerNeverSubscribedWithLockdownRunLocksWithReason(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedUsage(t, store, qualifyingWindows(0, int(lockdownRun/qualifyWindow)))
	locker := &recordingLocker{}
	p := newTestPlanner(t, store,
		staticSource{dec: ideplan.Decision{Status: ideplan.StatusNeverSubscribed}},
		locker, func() {},
		func() time.Time { return windowBase.Add(lockdownRun) })

	p.SetLocked(true)
	last, ok := locker.last()
	require.True(t, ok)
	assert.True(t, last, "a lockdown run while never-subscribed must lock")
	reason, ok := locker.lastReason()
	require.True(t, ok)
	assert.Equal(t, LockNeverSubscribed, reason,
		"a never-subscribed lockdown must carry the never-subscribed reason")
}

func TestPlannerActiveWithLockdownRunStaysUnlocked(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedUsage(t, store, qualifyingWindows(0, int(lockdownRun/qualifyWindow)))
	locker := &recordingLocker{}
	p := newTestPlanner(t, store,
		staticSource{dec: ideplan.Decision{Status: ideplan.StatusActive}},
		locker, func() {},
		func() time.Time { return windowBase.Add(lockdownRun) })

	p.SetLocked(true)
	last, ok := locker.last()
	require.True(t, ok)
	assert.False(t, last, "active users must never lock regardless of usage")
}

func TestPlannerNagsOncePerCooldown(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedUsage(t, store, qualifyingWindows(0, int(nagRun/qualifyWindow)))
	var prompts int
	var mu sync.Mutex
	now := windowBase.Add(nagRun)
	getNow := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	p := newTestPlanner(t, store, expiredSource(), &recordingLocker{},
		func() { prompts++ }, getNow)
	assert.Equal(t, 1, prompts, "startup evaluation must nag once")

	p.SetLocked(true)
	p.SetLocked(true)
	assert.Equal(t, 1, prompts,
		"repeat evaluations within the cooldown must not nag")

	mu.Lock()
	now = now.Add(nagCooldown)
	mu.Unlock()
	p.SetLocked(true)
	assert.Equal(t, 2, prompts, "an elapsed cooldown must nag again")
}

func TestPlannerNagsOncePerCooldownAcrossRestarts(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedUsage(t, store, qualifyingWindows(0, int(nagRun/qualifyWindow)))
	now := func() time.Time { return windowBase.Add(nagRun) }

	var prompts int
	newTestPlanner(t, store, expiredSource(), &recordingLocker{},
		func() { prompts++ }, now)
	assert.Equal(t, 1, prompts)

	// A second planner over the same store simulates an IDE restart
	// within the cooldown: the persisted LastNag must suppress the
	// nag.
	newTestPlanner(t, store, expiredSource(), &recordingLocker{},
		func() { prompts++ }, now)
	assert.Equal(t, 1, prompts,
		"restart within the cooldown must not nag again")
}

// TestPlannerAskMoreTimeReplacesLastNag pins that during the final
// qualifying window before lockdown the ask-more-time prompt fires in
// place of the nag, on the shared nag cooldown.
func TestPlannerAskMoreTimeReplacesLastNag(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedUsage(t, store, qualifyingWindows(0, int(askMoreTimeRun/qualifyWindow)))
	var nags, asks int
	var mu sync.Mutex
	now := windowBase.Add(askMoreTimeRun)
	getNow := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	locker := &recordingLocker{}
	p := New(Config{
		Storage:               store,
		Source:                expiredSource(),
		Policies:              DefaultPolicies(),
		Locker:                locker,
		ShowNagPrompt:         func() { nags++ },
		ShowAskMoreTimePrompt: func() { asks++ },
		Now:                   getNow,
	})
	require.NoError(t, p.Start(context.Background()))
	t.Cleanup(p.Stop)
	assert.Equal(t, 1, asks,
		"startup evaluation must show the ask-more-time prompt once")
	assert.Equal(t, 0, nags,
		"the ask-more-time prompt must replace the nag")

	p.SetLocked(true)
	assert.Equal(t, 1, asks,
		"repeat evaluations within the cooldown must not re-prompt")

	mu.Lock()
	now = now.Add(nagCooldown)
	mu.Unlock()
	p.SetLocked(true)
	assert.Equal(t, 2, asks, "an elapsed cooldown must prompt again")
	assert.Equal(t, 0, nags,
		"the nag must stay replaced past the ask-more-time threshold")
}

// TestPlannerSnapshotReflectsPersistedUsage pins that open-IDE time
// is the usage evidence: the snapshot carries exactly what the
// tracker persisted.
func TestPlannerSnapshotReflectsPersistedUsage(t *testing.T) {
	store := storagestub.NewInMemoryService()
	now := monday.Add(15 * time.Hour)
	p := newTestPlanner(t, store, expiredSource(), &recordingLocker{},
		func() {}, func() time.Time { return now })

	s := p.snapshot(context.Background())
	assert.Equal(t, []time.Time{bucketStart(now, usageBucket)}, s.Usage,
		"the launch sample must be bucket-aligned")
	assert.Equal(t, ideplan.StatusExpired, s.Plan.Status)
}
