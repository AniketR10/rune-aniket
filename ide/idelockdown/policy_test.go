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
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/ide/ideplan"
)

// monday is a known UTC reference day (2026-01-05) used for sample
// seeding.
var monday = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

// windowBase is the start of the qualifyWindow containing monday.
var windowBase = bucketStart(monday, qualifyWindow)

// windowSamples returns n samples in consecutive usage buckets
// starting at start.
func windowSamples(start time.Time, n int) []time.Time {
	samples := make([]time.Time, 0, n)
	for i := range n {
		samples = append(samples, start.Add(time.Duration(i)*usageBucket))
	}
	return samples
}

// qualifyingWindows returns qualifyingBuckets samples in each of n
// consecutive windows, the first at windowBase+start*qualifyWindow.
func qualifyingWindows(start, n int) []time.Time {
	var samples []time.Time
	for w := range n {
		samples = append(samples, windowSamples(
			windowBase.Add(time.Duration(start+w)*qualifyWindow),
			qualifyingBuckets)...)
	}
	return samples
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

func TestMaxConsecutiveQualifyingRun(t *testing.T) {
	preEpochWindow := bucketStart(
		time.Date(1969, 11, 1, 0, 0, 0, 0, time.UTC), qualifyWindow)
	for _, tc := range []struct {
		name    string
		samples []time.Time
		want    time.Duration
	}{
		{name: "no samples", want: 0},
		{
			name:    "below bucket threshold does not qualify",
			samples: windowSamples(windowBase, qualifyingBuckets-1),
			want:    0,
		},
		{
			name:    "threshold buckets in one window qualify",
			samples: windowSamples(windowBase, qualifyingBuckets),
			want:    qualifyWindow,
		},
		{
			name:    "two consecutive qualifying windows",
			samples: qualifyingWindows(0, 2),
			want:    2 * qualifyWindow,
		},
		{
			name:    "gap resets the run",
			samples: append(qualifyingWindows(0, 2), qualifyingWindows(3, 3)...),
			want:    3 * qualifyWindow,
		},
		{
			name: "non-qualifying window between runs resets",
			samples: append(qualifyingWindows(0, 4),
				append(windowSamples(windowBase.Add(4*qualifyWindow), qualifyingBuckets-1),
					qualifyingWindows(5, 2)...)...),
			want: 4 * qualifyWindow,
		},
		{
			name:    "five consecutive qualifying windows",
			samples: qualifyingWindows(0, 5),
			want:    5 * qualifyWindow,
		},
		{
			name:    "six consecutive qualifying windows",
			samples: qualifyingWindows(0, 6),
			want:    6 * qualifyWindow,
		},
		{
			name: "samples in the same bucket count once",
			samples: []time.Time{
				windowBase,
				windowBase.Add(time.Hour),
				windowBase.Add(2 * time.Hour),
			},
			want: 0,
		},
		{
			name: "buckets straddling a window boundary do not merge",
			samples: append(
				windowSamples(windowBase.Add(qualifyWindow-usageBucket), 1),
				windowSamples(windowBase.Add(qualifyWindow), qualifyingBuckets-1)...),
			want: 0,
		},
		{
			name: "unsorted samples still count",
			samples: func() []time.Time {
				samples := qualifyingWindows(0, 3)
				slices.Reverse(samples)
				return samples
			}(),
			want: 3 * qualifyWindow,
		},
		{
			name: "distant qualifying windows do not join",
			samples: append(qualifyingWindows(0, 1),
				qualifyingWindows(1000000, 1)...),
			want: qualifyWindow,
		},
		{
			name: "pre-epoch windows stay aligned",
			samples: append(windowSamples(preEpochWindow, qualifyingBuckets),
				windowSamples(preEpochWindow.Add(qualifyWindow), qualifyingBuckets)...),
			want: 2 * qualifyWindow,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, maxConsecutiveQualifyingRun(
				tc.samples, qualifyWindow, usageBucket, qualifyingBuckets))
		})
	}
}

// TestMaxConsecutiveQualifyingRunArbitraryDurations pins the point of
// the duration knobs: the run computation works at any timescale, so
// a manual test build can shrink the knobs to milliseconds.
func TestMaxConsecutiveQualifyingRunArbitraryDurations(t *testing.T) {
	const (
		window = 50 * time.Millisecond
		bucket = 10 * time.Millisecond
	)
	base := time.Unix(0, 0).UTC()
	samples := []time.Time{
		base, base.Add(10 * time.Millisecond), base.Add(20 * time.Millisecond),
		base.Add(50 * time.Millisecond), base.Add(60 * time.Millisecond),
		base.Add(70 * time.Millisecond),
	}
	assert.Equal(t, 2*window,
		maxConsecutiveQualifyingRun(samples, window, bucket, 3))
}

func TestPoliciesEvaluate(t *testing.T) {
	expired := ideplan.Decision{Status: ideplan.StatusExpired}
	neverSubscribed := ideplan.Decision{Status: ideplan.StatusNeverSubscribed}
	signedOut := ideplan.Decision{Status: ideplan.StatusExpired, SignedIn: ideplan.SignedOut}
	parseError := ideplan.Decision{Status: ideplan.StatusExpired, SignedIn: ideplan.ParseClaimsError}
	nagWindows := int(nagRun / qualifyWindow)
	lockdownWindows := int(lockdownRun / qualifyWindow)
	for _, tc := range []struct {
		name         string
		usage        []time.Time
		plan         ideplan.Decision
		wantNag      Action
		wantLockdown Action
	}{
		{
			name:         "expired with no usage",
			plan:         expired,
			wantNag:      ActionNone,
			wantLockdown: ActionNone,
		},
		{
			name:         "expired below the nag run",
			usage:        qualifyingWindows(0, nagWindows-1),
			plan:         expired,
			wantNag:      ActionNone,
			wantLockdown: ActionNone,
		},
		{
			name:         "expired at the nag run nags",
			usage:        qualifyingWindows(0, nagWindows),
			plan:         expired,
			wantNag:      ActionPrompt,
			wantLockdown: ActionNone,
		},
		{
			name:         "expired below the lockdown run nags only",
			usage:        qualifyingWindows(0, lockdownWindows-1),
			plan:         expired,
			wantNag:      ActionPrompt,
			wantLockdown: ActionNone,
		},
		{
			name:         "expired at the lockdown run locks",
			usage:        qualifyingWindows(0, lockdownWindows),
			plan:         expired,
			wantNag:      ActionPrompt,
			wantLockdown: ActionLockdown,
		},
		{
			name:         "never-subscribed at the lockdown run locks",
			usage:        qualifyingWindows(0, lockdownWindows),
			plan:         neverSubscribed,
			wantNag:      ActionPrompt,
			wantLockdown: ActionLockdown,
		},
		{
			name:         "signed-out at the lockdown run locks",
			usage:        qualifyingWindows(0, lockdownWindows),
			plan:         signedOut,
			wantNag:      ActionPrompt,
			wantLockdown: ActionLockdown,
		},
		{
			name:         "parse-error at the lockdown run locks",
			usage:        qualifyingWindows(0, lockdownWindows),
			plan:         parseError,
			wantNag:      ActionPrompt,
			wantLockdown: ActionLockdown,
		},
		{
			name: "partial current window counts once it qualifies",
			usage: append(qualifyingWindows(0, lockdownWindows-1),
				windowSamples(
					windowBase.Add(time.Duration(lockdownWindows-1)*qualifyWindow),
					qualifyingBuckets)...),
			plan:         expired,
			wantNag:      ActionPrompt,
			wantLockdown: ActionLockdown,
		},
		{
			name:         "active user is never nagged or locked",
			usage:        qualifyingWindows(0, lockdownWindows+4),
			plan:         ideplan.Decision{Status: ideplan.StatusActive},
			wantNag:      ActionNone,
			wantLockdown: ActionNone,
		},
		{
			name:         "grace-period user is never nagged or locked",
			usage:        qualifyingWindows(0, lockdownWindows+4),
			plan:         ideplan.Decision{Status: ideplan.StatusGracePeriod},
			wantNag:      ActionNone,
			wantLockdown: ActionNone,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := Snapshot{
				Now:   windowBase.Add(100 * qualifyWindow),
				Usage: tc.usage,
				Plan:  tc.plan,
			}
			nagNext, nag := NagPolicy{}.Evaluate(s)
			assert.Equal(t, tc.wantNag, nag, "nag policy")
			assert.Equal(t, s.Now.Add(evaluateInterval), nagNext,
				"nag policy must re-evaluate one interval later")
			lockNext, lock := LockdownPolicy{}.Evaluate(s)
			assert.Equal(t, tc.wantLockdown, lock, "lockdown policy")
			assert.Equal(t, s.Now.Add(evaluateInterval), lockNext,
				"lockdown policy must re-evaluate one interval later")
		})
	}
}

func TestDefaultPolicies(t *testing.T) {
	names := make([]string, 0)
	for _, p := range DefaultPolicies() {
		names = append(names, p.Name())
	}
	assert.Equal(t, []string{"nag", "lockdown"}, names)
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
			name:       "grace period is not gated",
			dec:        ideplan.Decision{Status: ideplan.StatusGracePeriod},
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
