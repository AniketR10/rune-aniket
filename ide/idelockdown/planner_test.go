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

// seedDays persists the given per-day activity through the tracker,
// one recordActivity call per active slot.
func seedDays(t *testing.T, store storageapi.Service, days []DayActivity) {
	t.Helper()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))
	for _, d := range days {
		for slot := range d.ActiveSlots {
			ts := d.Day.Add(time.Duration(slot) * slotDuration)
			require.NoError(t, tr.recordActivity(context.Background(), ts))
		}
	}
}

// TestPlannerStartRecordsNoActivity pins that merely launching Rune
// is not usage evidence: only RecordActivity accrues slots.
func TestPlannerStartRecordsNoActivity(t *testing.T) {
	store := storagestub.NewInMemoryService()
	now := monday.Add(10 * time.Hour)
	newTestPlanner(t, store, expiredSource(), &recordingLocker{}, func() {},
		func() time.Time { return now })

	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))
	assert.Empty(t, tr.days(),
		"start and evaluation must not record activity")
}

func TestPlannerRecordActivityPersistsSlots(t *testing.T) {
	store := storagestub.NewInMemoryService()
	clock := &fakeClock{now: monday.Add(10 * time.Hour)}
	p := newTestPlanner(t, store, expiredSource(), &recordingLocker{},
		func() {}, clock.Now)

	ctx := context.Background()
	p.RecordActivity(ctx)
	p.RecordActivity(ctx)
	clock.Advance(time.Minute)
	p.RecordActivity(ctx)

	tr := newTracker(store)
	require.NoError(t, tr.load(ctx))
	assert.Equal(t, []DayActivity{{Day: monday, ActiveSlots: 1}}, tr.days(),
		"repeat events within one slot must record a single slot")

	clock.Advance(slotDuration)
	p.RecordActivity(ctx)
	tr2 := newTracker(store)
	require.NoError(t, tr2.load(ctx))
	assert.Equal(t, []DayActivity{{Day: monday, ActiveSlots: 2}}, tr2.days(),
		"a new slot must record once the clock advances past the boundary")
}

// TestPlannerRecordActivityBeforeLoadIsNoop pins that early events
// cannot race the storage doc bootstrap.
func TestPlannerRecordActivityBeforeLoadIsNoop(t *testing.T) {
	store := storagestub.NewInMemoryService()
	p := New(Config{
		Storage:               store,
		Source:                expiredSource(),
		Policies:              DefaultPolicies(),
		Locker:                &recordingLocker{},
		ShowNagPrompt:         func() {},
		ShowAskMoreTimePrompt: func() {},
		Now:                   func() time.Time { return monday },
	})
	t.Cleanup(p.Stop)
	p.RecordActivity(context.Background())

	require.NoError(t, p.Load(context.Background()))
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))
	assert.Empty(t, tr.days())
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
	seedDays(t, store, heavyWeeks(0, lockdownRun))
	locker := &recordingLocker{}
	p := newTestPlanner(t, store, expiredSource(), locker, func() {},
		func() time.Time { return afterWeeks(lockdownRun) })

	p.SetLocked(true)
	last, ok := locker.last()
	require.True(t, ok)
	assert.True(t, last, "a heavy lockdown run while expired must lock")
	reason, ok := locker.lastReason()
	require.True(t, ok)
	assert.Equal(t, LockExpired, reason,
		"an expired lockdown must carry the expired reason")
}

func TestPlannerNeverSubscribedWithLockdownRunLocksWithReason(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedDays(t, store, heavyWeeks(0, lockdownRun))
	locker := &recordingLocker{}
	p := newTestPlanner(t, store,
		staticSource{dec: ideplan.Decision{Status: ideplan.StatusNeverSubscribed}},
		locker, func() {},
		func() time.Time { return afterWeeks(lockdownRun) })

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
	seedDays(t, store, heavyWeeks(0, lockdownRun))
	locker := &recordingLocker{}
	p := newTestPlanner(t, store,
		staticSource{dec: ideplan.Decision{Status: ideplan.StatusActive}},
		locker, func() {},
		func() time.Time { return afterWeeks(lockdownRun) })

	p.SetLocked(true)
	last, ok := locker.last()
	require.True(t, ok)
	assert.False(t, last, "active users must never lock regardless of usage")
}

// TestPlannerUpgradeExpiredWithoutUsageDoesNotLock pins that an
// upgrade-expired one-off buyer gets the normal usage runway instead
// of an immediate lock.
func TestPlannerUpgradeExpiredWithoutUsageDoesNotLock(t *testing.T) {
	store := storagestub.NewInMemoryService()
	locker := &recordingLocker{}
	p := newTestPlanner(t, store,
		staticSource{dec: ideplan.Decision{Status: ideplan.StatusUpgradeExpired}},
		locker, func() {},
		func() time.Time { return monday })

	p.SetLocked(true)
	last, ok := locker.last()
	require.True(t, ok)
	assert.False(t, last,
		"upgrade-expired without qualifying usage must not lock")
}

func TestPlannerUpgradeExpiredWithLockdownRunLocksWithReason(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedDays(t, store, heavyWeeks(0, lockdownRun))
	locker := &recordingLocker{}
	p := newTestPlanner(t, store,
		staticSource{dec: ideplan.Decision{Status: ideplan.StatusUpgradeExpired}},
		locker, func() {},
		func() time.Time { return afterWeeks(lockdownRun) })

	p.SetLocked(true)
	last, ok := locker.last()
	require.True(t, ok)
	assert.True(t, last, "a lockdown run while upgrade-expired must lock")
	reason, ok := locker.lastReason()
	require.True(t, ok)
	assert.Equal(t, LockUpgradeExpired, reason,
		"an upgrade-expired lockdown must carry the upgrade-expired reason")
}

func TestPlannerNagsOncePerWeeklyCooldown(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedDays(t, store, professionalWeeks(0, nagRun))
	var prompts int
	var mu sync.Mutex
	now := afterWeeks(nagRun)
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
	now = now.Add(dailyNagCooldown)
	mu.Unlock()
	p.SetLocked(true)
	assert.Equal(t, 1, prompts,
		"a day within the weekly cadence must not nag again")

	mu.Lock()
	now = now.Add(weeklyNagCooldown)
	mu.Unlock()
	p.SetLocked(true)
	assert.Equal(t, 2, prompts, "an elapsed weekly cooldown must nag again")
}

// TestPlannerDailyNagAfterEscalation pins the cadence escalation: at
// dailyNagRun professional weeks the nag fires daily.
func TestPlannerDailyNagAfterEscalation(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedDays(t, store, professionalWeeks(0, dailyNagRun))
	var prompts int
	var mu sync.Mutex
	now := afterWeeks(dailyNagRun)
	getNow := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return now
	}
	p := newTestPlanner(t, store, expiredSource(), &recordingLocker{},
		func() { prompts++ }, getNow)
	assert.Equal(t, 1, prompts, "startup evaluation must nag once")

	mu.Lock()
	now = now.Add(dailyNagCooldown)
	mu.Unlock()
	p.SetLocked(true)
	assert.Equal(t, 2, prompts,
		"past the escalation run the nag must fire daily")
}

func TestPlannerNagsOncePerCooldownAcrossRestarts(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedDays(t, store, professionalWeeks(0, nagRun))
	now := func() time.Time { return afterWeeks(nagRun) }

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
// heavy week before lockdown the ask-more-time prompt fires in place
// of the nag, on the shared (daily, since the heavy run escalates the
// cadence) nag cooldown.
func TestPlannerAskMoreTimeReplacesLastNag(t *testing.T) {
	store := storagestub.NewInMemoryService()
	seedDays(t, store, heavyWeeks(0, askMoreTimeRun))
	var nags, asks int
	var mu sync.Mutex
	now := afterWeeks(askMoreTimeRun)
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
	now = now.Add(dailyNagCooldown)
	mu.Unlock()
	p.SetLocked(true)
	assert.Equal(t, 2, asks, "an elapsed cooldown must prompt again")
	assert.Equal(t, 0, nags,
		"the nag must stay replaced past the ask-more-time threshold")
}

// TestPlannerSnapshotReflectsPersistedActivity pins that recorded
// editor activity is the usage evidence: the snapshot carries
// exactly what the tracker persisted.
func TestPlannerSnapshotReflectsPersistedActivity(t *testing.T) {
	store := storagestub.NewInMemoryService()
	now := monday.Add(15 * time.Hour)
	p := newTestPlanner(t, store, expiredSource(), &recordingLocker{},
		func() {}, func() time.Time { return now })

	p.RecordActivity(context.Background())
	s := p.snapshot(context.Background())
	assert.Equal(t, []DayActivity{{Day: monday, ActiveSlots: 1}}, s.Days,
		"the recorded slot must surface in the snapshot")
	assert.Equal(t, ideplan.StatusExpired, s.Plan.Status)
}

func newTamperTestPlanner(
	t *testing.T, store storageapi.Service, src ideplan.Source,
	locker Locker, tampered bool,
) *Planner {
	t.Helper()
	p := New(Config{
		Storage:               store,
		Source:                src,
		Policies:              DefaultPolicies(),
		Locker:                locker,
		ShowNagPrompt:         func() {},
		ShowAskMoreTimePrompt: func() {},
		Now:                   func() time.Time { return monday },
		Tampered:              tampered,
	})
	require.NoError(t, p.Start(context.Background()))
	t.Cleanup(p.Stop)
	return p
}

// TestPlannerTamperedLocksImmediately pins the ~/.rune wipe defense:
// when install-ID resolution flags a tampered install, a gated user
// is locked at startup with no usage runway at all.
func TestPlannerTamperedLocksImmediately(t *testing.T) {
	store := storagestub.NewInMemoryService()
	locker := &recordingLocker{}
	newTamperTestPlanner(t, store, expiredSource(), locker, true)

	last, ok := locker.last()
	require.True(t, ok)
	assert.True(t, last, "a tampered gated install must lock immediately")
	reason, ok := locker.lastReason()
	require.True(t, ok)
	assert.Equal(t, LockExpired, reason,
		"the tamper lock must reuse the plan-derived reason")
}

// TestPlannerTamperMarkPersistsAcrossRestarts pins that the tamper
// mark lives in the usage doc: once persisted, later sessions lock
// even though their install-ID resolution reports no tampering (the
// backup self-heals on first detection).
func TestPlannerTamperMarkPersistsAcrossRestarts(t *testing.T) {
	store := storagestub.NewInMemoryService()
	newTamperTestPlanner(t, store, expiredSource(), &recordingLocker{}, true)

	locker := &recordingLocker{}
	newTamperTestPlanner(t, store, expiredSource(), locker, false)
	last, ok := locker.last()
	require.True(t, ok)
	assert.True(t, last, "the persisted tamper mark must keep locking")
}

// TestPlannerEntitledClearsTamperMark pins the recovery path: an
// entitled session stays unlocked and durably clears the tamper mark,
// so a later lapse gets the normal usage runway instead of an
// immediate lock.
func TestPlannerEntitledClearsTamperMark(t *testing.T) {
	store := storagestub.NewInMemoryService()
	active := staticSource{dec: ideplan.Decision{Status: ideplan.StatusActive}}
	locker := &recordingLocker{}
	newTamperTestPlanner(t, store, active, locker, true)
	last, ok := locker.last()
	require.True(t, ok)
	assert.False(t, last, "an entitled tampered install must stay unlocked")

	relocker := &recordingLocker{}
	newTamperTestPlanner(t, store, expiredSource(), relocker, false)
	last, ok = relocker.last()
	require.True(t, ok)
	assert.False(t, last,
		"after an entitled session cleared the mark, a lapse without "+
			"qualifying usage must not lock")
}
