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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/ide/ideplan"
)

// monday is a known UTC reference day (2026-01-05) used for activity
// seeding.
var monday = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

// weekBase is the start of the weekWindow containing monday.
var weekBase = bucketStart(monday, weekWindow)

const day = 24 * time.Hour

// weekDays returns n days of activity starting at start, each with
// slots active slots.
func weekDays(start time.Time, n, slots int) []DayActivity {
	days := make([]DayActivity, 0, n)
	for i := range n {
		days = append(days, DayActivity{
			Day: start.Add(time.Duration(i) * day), ActiveSlots: slots,
		})
	}
	return days
}

// weeks returns qualifyingDays days with the given slots in each of
// n consecutive weeks, the first at weekBase+start*weekWindow.
func weeks(start, n, slots int) []DayActivity {
	var days []DayActivity
	for w := range n {
		days = append(days, weekDays(
			weekBase.Add(time.Duration(start+w)*weekWindow),
			qualifyingDays, slots)...)
	}
	return days
}

// professionalWeeks returns n consecutive qualifying professional
// weeks starting at week offset start.
func professionalWeeks(start, n int) []DayActivity {
	return weeks(start, n, professionalDaySlots)
}

// heavyWeeks returns n consecutive qualifying heavy weeks starting
// at week offset start.
func heavyWeeks(start, n int) []DayActivity {
	return weeks(start, n, heavyDaySlots)
}

// afterWeeks is an evaluation time right after n weeks from
// weekBase.
func afterWeeks(n int) time.Time {
	return weekBase.Add(time.Duration(n) * weekWindow)
}

func TestBucketIndex(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	for _, tc := range []struct {
		name string
		t    time.Time
		d    time.Duration
		want int64
	}{
		{name: "epoch is bucket zero", t: epoch, d: 24 * time.Hour, want: 0},
		{name: "just before epoch floors to minus one",
			t: epoch.Add(-time.Nanosecond), d: 24 * time.Hour, want: -1},
		{name: "end of first bucket",
			t: epoch.Add(24*time.Hour - time.Nanosecond), d: 24 * time.Hour, want: 0},
		{name: "exact pre-epoch multiple",
			t: epoch.Add(-24 * time.Hour), d: 24 * time.Hour, want: -1},
		{name: "pre-epoch non-multiple floors down",
			t: epoch.Add(-24*time.Hour - time.Nanosecond), d: 24 * time.Hour, want: -2},
		{name: "sub-second buckets",
			t: epoch.Add(45 * time.Millisecond), d: 20 * time.Millisecond, want: 2},
		{name: "non-UTC input normalizes",
			t:    monday.Add(10 * time.Hour).In(time.FixedZone("plus14", 14*3600)),
			d:    24 * time.Hour,
			want: bucketIndex(monday, 24*time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, bucketIndex(tc.t, tc.d))
		})
	}
}

func TestBucketStart(t *testing.T) {
	for _, tc := range []struct {
		name string
		t    time.Time
		d    time.Duration
		want time.Time
	}{
		{name: "24h buckets are UTC days",
			t: monday.Add(10 * time.Hour), d: 24 * time.Hour, want: monday},
		{name: "exact bucket start is identity",
			t: monday, d: 24 * time.Hour, want: monday},
		{name: "pre-epoch aligns down",
			t:    time.Unix(0, 0).Add(-time.Hour),
			d:    24 * time.Hour,
			want: time.Unix(0, 0).Add(-24 * time.Hour).UTC()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := bucketStart(tc.t, tc.d)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, time.UTC, got.Location())
		})
	}
}

func TestQualifyingWeeks(t *testing.T) {
	baseWeek := bucketIndex(weekBase, weekWindow)
	for _, tc := range []struct {
		name     string
		days     []DayActivity
		minSlots int
		want     []int64
	}{
		{name: "no days", minSlots: professionalDaySlots, want: nil},
		{
			name:     "too few qualifying days per week",
			days:     weekDays(weekBase, qualifyingDays-1, professionalDaySlots),
			minSlots: professionalDaySlots,
			want:     nil,
		},
		{
			name:     "days below the slot threshold do not count",
			days:     weekDays(weekBase, qualifyingDays, professionalDaySlots-1),
			minSlots: professionalDaySlots,
			want:     nil,
		},
		{
			name:     "enough professional days qualify the week",
			days:     weekDays(weekBase, qualifyingDays, professionalDaySlots),
			minSlots: professionalDaySlots,
			want:     []int64{baseWeek},
		},
		{
			name:     "professional days are not heavy days",
			days:     weekDays(weekBase, qualifyingDays, professionalDaySlots),
			minSlots: heavyDaySlots,
			want:     nil,
		},
		{
			name:     "heavy days also qualify professionally",
			days:     weekDays(weekBase, qualifyingDays, heavyDaySlots),
			minSlots: professionalDaySlots,
			want:     []int64{baseWeek},
		},
		{
			name: "qualifying days split across weeks do not merge",
			days: append(
				weekDays(weekBase.Add(weekWindow-2*day), 2, professionalDaySlots),
				weekDays(weekBase.Add(weekWindow), 2, professionalDaySlots)...),
			minSlots: professionalDaySlots,
			want:     nil,
		},
		{
			name:     "multiple consecutive weeks",
			days:     professionalWeeks(0, 3),
			minSlots: professionalDaySlots,
			want:     []int64{baseWeek, baseWeek + 1, baseWeek + 2},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := qualifyingWeeks(tc.days, tc.minSlots, qualifyingDays)
			if len(tc.want) == 0 {
				assert.Empty(t, got)
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestLongestRun(t *testing.T) {
	for _, tc := range []struct {
		name  string
		weeks []int64
		want  int
	}{
		{name: "no weeks", want: 0},
		{name: "single week", weeks: []int64{10}, want: 1},
		{name: "consecutive weeks", weeks: []int64{10, 11, 12}, want: 3},
		{name: "one light week is tolerated and does not count",
			weeks: []int64{10, 11, 13, 14}, want: 4},
		{name: "two consecutive light weeks reset",
			weeks: []int64{10, 11, 14, 15, 16}, want: 3},
		{name: "gap tolerance does not chain across long gaps",
			weeks: []int64{10, 20, 30}, want: 1},
		{name: "earlier run can be the longest",
			weeks: []int64{1, 2, 3, 4, 10, 11}, want: 4},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, longestRun(tc.weeks, gapWeeks))
		})
	}
}

func TestCurrentRun(t *testing.T) {
	for _, tc := range []struct {
		name    string
		weeks   []int64
		nowWeek int64
		want    int
	}{
		{name: "no weeks", nowWeek: 10, want: 0},
		{name: "run including the current week", weeks: []int64{8, 9, 10}, nowWeek: 10, want: 3},
		{name: "run ending the previous week", weeks: []int64{8, 9}, nowWeek: 10, want: 2},
		{name: "one complete light week keeps the run current",
			weeks: []int64{7, 8}, nowWeek: 10, want: 2},
		{name: "two complete light weeks end the run",
			weeks: []int64{6, 7}, nowWeek: 10, want: 0},
		{name: "tolerated light week inside the run does not count",
			weeks: []int64{6, 7, 9, 10}, nowWeek: 10, want: 4},
		{name: "two light weeks inside history restart the count",
			weeks: []int64{4, 5, 9, 10}, nowWeek: 10, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, currentRun(tc.weeks, tc.nowWeek, gapWeeks))
		})
	}
}

func TestPoliciesEvaluate(t *testing.T) {
	expired := ideplan.Decision{Status: ideplan.StatusExpired}
	neverSubscribed := ideplan.Decision{Status: ideplan.StatusNeverSubscribed}
	signedOut := ideplan.Decision{Status: ideplan.StatusExpired, SignedIn: ideplan.SignedOut}
	parseError := ideplan.Decision{Status: ideplan.StatusExpired, SignedIn: ideplan.ParseClaimsError}
	upgradeExpired := ideplan.Decision{Status: ideplan.StatusUpgradeExpired}
	for _, tc := range []struct {
		name            string
		days            []DayActivity
		now             time.Time
		plan            ideplan.Decision
		wantNag         Action
		wantAskMoreTime Action
		wantLockdown    Action
	}{
		{
			name:            "expired with no activity",
			now:             afterWeeks(1),
			plan:            expired,
			wantNag:         ActionNone,
			wantAskMoreTime: ActionNone,
			wantLockdown:    ActionNone,
		},
		{
			name:            "expired below the nag run",
			days:            professionalWeeks(0, nagRun-1),
			now:             afterWeeks(nagRun - 1),
			plan:            expired,
			wantNag:         ActionNone,
			wantAskMoreTime: ActionNone,
			wantLockdown:    ActionNone,
		},
		{
			name:            "expired at the nag run nags",
			days:            professionalWeeks(0, nagRun),
			now:             afterWeeks(nagRun),
			plan:            expired,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionNone,
			wantLockdown:    ActionNone,
		},
		{
			name:            "professional-only usage nags forever but never locks",
			days:            professionalWeeks(0, lockdownRun+4),
			now:             afterWeeks(lockdownRun + 4),
			plan:            expired,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionNone,
			wantLockdown:    ActionNone,
		},
		{
			name:            "a tolerated light week keeps the nag run alive",
			days:            append(professionalWeeks(0, 2), professionalWeeks(3, 1)...),
			now:             afterWeeks(4),
			plan:            expired,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionNone,
			wantLockdown:    ActionNone,
		},
		{
			name:            "two light weeks stop the nag",
			days:            professionalWeeks(0, nagRun),
			now:             afterWeeks(nagRun + 3),
			plan:            expired,
			wantNag:         ActionNone,
			wantAskMoreTime: ActionNone,
			wantLockdown:    ActionNone,
		},
		{
			name:            "heavy below the ask-more-time run nags only",
			days:            heavyWeeks(0, askMoreTimeRun-1),
			now:             afterWeeks(askMoreTimeRun - 1),
			plan:            expired,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionNone,
			wantLockdown:    ActionNone,
		},
		{
			name:            "heavy at the ask-more-time run asks for more time",
			days:            heavyWeeks(0, askMoreTimeRun),
			now:             afterWeeks(askMoreTimeRun),
			plan:            expired,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionAskMoreTime,
			wantLockdown:    ActionNone,
		},
		{
			name:            "heavy at the lockdown run locks",
			days:            heavyWeeks(0, lockdownRun),
			now:             afterWeeks(lockdownRun),
			plan:            expired,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionAskMoreTime,
			wantLockdown:    ActionLockdown,
		},
		{
			name:            "the lock does not self-heal after two idle weeks",
			days:            heavyWeeks(0, lockdownRun),
			now:             afterWeeks(lockdownRun + 3),
			plan:            expired,
			wantNag:         ActionNone,
			wantAskMoreTime: ActionAskMoreTime,
			wantLockdown:    ActionLockdown,
		},
		{
			name:            "never-subscribed at the lockdown run locks",
			days:            heavyWeeks(0, lockdownRun),
			now:             afterWeeks(lockdownRun),
			plan:            neverSubscribed,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionAskMoreTime,
			wantLockdown:    ActionLockdown,
		},
		{
			name:            "signed-out at the lockdown run locks",
			days:            heavyWeeks(0, lockdownRun),
			now:             afterWeeks(lockdownRun),
			plan:            signedOut,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionAskMoreTime,
			wantLockdown:    ActionLockdown,
		},
		{
			name:            "parse-error at the lockdown run locks",
			days:            heavyWeeks(0, lockdownRun),
			now:             afterWeeks(lockdownRun),
			plan:            parseError,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionAskMoreTime,
			wantLockdown:    ActionLockdown,
		},
		{
			name:            "upgrade-expired without activity does not lock",
			now:             afterWeeks(1),
			plan:            upgradeExpired,
			wantNag:         ActionNone,
			wantAskMoreTime: ActionNone,
			wantLockdown:    ActionNone,
		},
		{
			name:            "upgrade-expired at the lockdown run locks",
			days:            heavyWeeks(0, lockdownRun),
			now:             afterWeeks(lockdownRun),
			plan:            upgradeExpired,
			wantNag:         ActionPrompt,
			wantAskMoreTime: ActionAskMoreTime,
			wantLockdown:    ActionLockdown,
		},
		{
			name:            "active user is never nagged or locked",
			days:            heavyWeeks(0, lockdownRun+4),
			now:             afterWeeks(lockdownRun + 4),
			plan:            ideplan.Decision{Status: ideplan.StatusActive},
			wantNag:         ActionNone,
			wantAskMoreTime: ActionNone,
			wantLockdown:    ActionNone,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Snapshot{Now: tc.now, Days: tc.days, Plan: tc.plan}
			nagNext, nag := NagPolicy{}.Evaluate(s)
			assert.Equal(t, tc.wantNag, nag, "nag policy")
			assert.Equal(t, s.Now.Add(evaluateInterval), nagNext,
				"nag policy must re-evaluate one interval later")
			askNext, ask := AskMoreTimePolicy{}.Evaluate(s)
			assert.Equal(t, tc.wantAskMoreTime, ask, "ask-more-time policy")
			assert.Equal(t, s.Now.Add(evaluateInterval), askNext,
				"ask-more-time policy must re-evaluate one interval later")
			lockNext, lock := LockdownPolicy{}.Evaluate(s)
			assert.Equal(t, tc.wantLockdown, lock, "lockdown policy")
			assert.Equal(t, s.Now.Add(evaluateInterval), lockNext,
				"lockdown policy must re-evaluate one interval later")
		})
	}
}

func TestNagCadence(t *testing.T) {
	for _, tc := range []struct {
		name string
		days []DayActivity
		now  time.Time
		want time.Duration
	}{
		{
			name: "weekly below the daily-nag run",
			days: professionalWeeks(0, dailyNagRun-1),
			now:  afterWeeks(dailyNagRun - 1),
			want: weeklyNagCooldown,
		},
		{
			name: "daily at the daily-nag professional run",
			days: professionalWeeks(0, dailyNagRun),
			now:  afterWeeks(dailyNagRun),
			want: dailyNagCooldown,
		},
		{
			name: "daily at the ask-more-time heavy run",
			days: heavyWeeks(0, askMoreTimeRun),
			now:  afterWeeks(askMoreTimeRun),
			want: dailyNagCooldown,
		},
		{
			name: "heavy run stays daily even after usage stops",
			days: heavyWeeks(0, askMoreTimeRun),
			now:  afterWeeks(askMoreTimeRun + 4),
			want: dailyNagCooldown,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Snapshot{Now: tc.now, Days: tc.days}
			assert.Equal(t, tc.want, nagCadence(s))
		})
	}
}

func TestDefaultPolicies(t *testing.T) {
	names := make([]string, 0)
	for _, p := range DefaultPolicies() {
		names = append(names, p.Name())
	}
	assert.Equal(t, []string{
		"nag", "ask-more-time", "lockdown", "tamper",
	}, names)
}

// TestTamperPolicyEvaluate pins the ~/.rune wipe defense: a tampered
// install locks gated users immediately, regardless of accrued
// usage, while entitled users are never affected.
func TestTamperPolicyEvaluate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		tampered bool
		plan     ideplan.Decision
		want     Action
	}{
		{
			name:     "expired and tampered locks without usage",
			tampered: true,
			plan:     ideplan.Decision{Status: ideplan.StatusExpired},
			want:     ActionLockdown,
		},
		{
			name:     "never-subscribed and tampered locks",
			tampered: true,
			plan:     ideplan.Decision{Status: ideplan.StatusNeverSubscribed},
			want:     ActionLockdown,
		},
		{
			name:     "signed-out and tampered locks",
			tampered: true,
			plan: ideplan.Decision{
				Status: ideplan.StatusExpired, SignedIn: ideplan.SignedOut,
			},
			want: ActionLockdown,
		},
		{
			name:     "expired without the tamper mark does nothing",
			tampered: false,
			plan:     ideplan.Decision{Status: ideplan.StatusExpired},
			want:     ActionNone,
		},
		{
			name:     "active and tampered does nothing",
			tampered: true,
			plan:     ideplan.Decision{Status: ideplan.StatusActive},
			want:     ActionNone,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Snapshot{Now: monday, Plan: tc.plan, Tampered: tc.tampered}
			next, a := TamperPolicy{}.Evaluate(s)
			assert.Equal(t, tc.want, a)
			assert.Equal(t, s.Now.Add(evaluateInterval), next,
				"tamper policy must re-evaluate one interval later")
		})
	}
}

func TestGated(t *testing.T) {
	for _, tc := range []struct {
		name       string
		dec        ideplan.Decision
		wantGated  bool
		wantReason LockReason
	}{
		{
			name:       "active is not gated",
			dec:        ideplan.Decision{Status: ideplan.StatusActive},
			wantGated:  false,
			wantReason: LockNone,
		},
		{
			name:       "expired is gated as expired",
			dec:        ideplan.Decision{Status: ideplan.StatusExpired},
			wantGated:  true,
			wantReason: LockExpired,
		},
		{
			name:       "never subscribed is gated as never subscribed",
			dec:        ideplan.Decision{Status: ideplan.StatusNeverSubscribed},
			wantGated:  true,
			wantReason: LockNeverSubscribed,
		},
		{
			name:       "upgrade expired is gated as upgrade expired",
			dec:        ideplan.Decision{Status: ideplan.StatusUpgradeExpired},
			wantGated:  true,
			wantReason: LockUpgradeExpired,
		},
		{
			name: "signed out takes precedence over status",
			dec: ideplan.Decision{
				Status: ideplan.StatusExpired, SignedIn: ideplan.SignedOut,
			},
			wantGated:  true,
			wantReason: LockSignedOut,
		},
		{
			name: "parse error takes precedence over status",
			dec: ideplan.Decision{
				Status: ideplan.StatusNeverSubscribed, SignedIn: ideplan.ParseClaimsError,
			},
			wantGated:  true,
			wantReason: LockParseError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, reason := gated(tc.dec)
			assert.Equal(t, tc.wantGated, g, "gated")
			assert.Equal(t, tc.wantReason, reason, "reason")
		})
	}
}
