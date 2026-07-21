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
	"time"
)

// Enforcement tuning knobs. Days are classified from 15-minute
// activity slots, weeks qualify from days, and runs of qualifying
// weeks drive the nag/ask-more-time/lockdown ladder.
const (
	// evaluateInterval is how far ahead the policies schedule their
	// next evaluation.
	evaluateInterval = 24 * time.Hour
	// slotDuration is the resolution of activity evidence: a slot
	// is active when at least one editor event lands in it, and a
	// day's active time is its active slots times slotDuration.
	slotDuration = 15 * time.Minute
	// slotsPerDay is the number of activity slots in a UTC day.
	slotsPerDay = int(24 * time.Hour / slotDuration)
	// slotMaskLen is the length of the persisted per-day hex slot
	// bitmask (4 slots per character).
	slotMaskLen = slotsPerDay / 4
	// weekWindow is the epoch-anchored window days are grouped into
	// when computing qualifying weeks.
	weekWindow = 7 * 24 * time.Hour
	// professionalDaySlots classifies a day as professional usage
	// (at least 4h of active time).
	professionalDaySlots = 16
	// heavyDaySlots classifies a day as heavy usage (at least 6h of
	// active time).
	heavyDaySlots = 24
	// qualifyingDays is the minimum professional (or heavy) days a
	// week needs to qualify toward a run.
	qualifyingDays = 4
	// gapWeeks is the number of consecutive non-qualifying weeks a
	// run tolerates before it resets.
	gapWeeks = 1
	// nagRun is the current professional run, in qualifying weeks,
	// after which gated users see the weekly upgrade prompt.
	nagRun = 3
	// dailyNagRun is the current professional run after which the
	// upgrade prompt escalates to a daily cadence.
	dailyNagRun = 6
	// askMoreTimeRun is the longest heavy run after which the nag is
	// replaced by the daily ask-more-time prompt, covering the last
	// week before lockdownRun.
	askMoreTimeRun = 7
	// lockdownRun is the longest heavy run after which gated users
	// are locked out.
	lockdownRun = 8
	// weeklyNagCooldown is the minimum interval between upgrade
	// prompts at the weekly cadence.
	weeklyNagCooldown = 7 * 24 * time.Hour
	// dailyNagCooldown is the minimum interval between upgrade
	// prompts at the escalated daily cadence.
	dailyNagCooldown = 24 * time.Hour
	// retention is how long activity evidence is kept. The lockdown
	// axis uses the longest retained run, so a lock ages out with
	// its evidence.
	retention = 180 * 24 * time.Hour
)

// professionalCurrentRun is the still-live run of professional
// weeks. Nagging keys off it so prompts stop once usage drops back
// to personal levels for more than gapWeeks complete weeks.
func professionalCurrentRun(s Snapshot) int {
	weeks := qualifyingWeeks(s.Days, professionalDaySlots, qualifyingDays)
	return currentRun(weeks, bucketIndex(s.Now, weekWindow), gapWeeks)
}

// heavyLongestRun is the longest retained run of heavy weeks. The
// ask-more-time and lockdown axes key off it so the lock does not
// self-heal after a couple of idle weeks; it ages out with evidence
// retention instead.
func heavyLongestRun(s Snapshot) int {
	return longestRun(qualifyingWeeks(s.Days, heavyDaySlots, qualifyingDays), gapWeeks)
}

// nagCadence returns the minimum interval between upgrade prompts
// for the snapshot: daily once the professional run reaches
// dailyNagRun or the heavy run reaches askMoreTimeRun, weekly
// otherwise.
func nagCadence(s Snapshot) time.Duration {
	if professionalCurrentRun(s) >= dailyNagRun || heavyLongestRun(s) >= askMoreTimeRun {
		return dailyNagCooldown
	}
	return weeklyNagCooldown
}

// NagPolicy shows a closable upgrade prompt to gated users after a
// nagRun of professional weeks.
type NagPolicy struct{}

// Name identifies the policy in logs.
func (NagPolicy) Name() string { return "nag" }

// Evaluate returns ActionPrompt once the current professional run
// reaches the nag threshold. Only gated users (expired,
// never-subscribed, upgrade-expired, or a sign-in problem) are
// nagged.
func (NagPolicy) Evaluate(s Snapshot) (time.Time, Action) {
	next := s.Now.Add(evaluateInterval)
	if g, _ := gated(s.Plan); !g {
		return next, ActionNone
	}
	if professionalCurrentRun(s) >= nagRun {
		return next, ActionPrompt
	}
	return next, ActionNone
}

// AskMoreTimePolicy replaces the nag prompt with the ask-more-time
// prompt for the last heavy week before lockdown. Its action
// outranks ActionPrompt, so while both policies fire the planner
// shows the ask-more-time copy instead of the nag.
type AskMoreTimePolicy struct{}

// Name identifies the policy in logs.
func (AskMoreTimePolicy) Name() string { return "ask-more-time" }

// Evaluate returns ActionAskMoreTime once the longest heavy run
// reaches the ask-more-time threshold. Only gated users are
// prompted.
func (AskMoreTimePolicy) Evaluate(s Snapshot) (time.Time, Action) {
	next := s.Now.Add(evaluateInterval)
	if g, _ := gated(s.Plan); !g {
		return next, ActionNone
	}
	if heavyLongestRun(s) >= askMoreTimeRun {
		return next, ActionAskMoreTime
	}
	return next, ActionNone
}

// LockdownPolicy locks the IDE for gated users after a lockdownRun
// of heavy weeks.
type LockdownPolicy struct{}

// Name identifies the policy in logs.
func (LockdownPolicy) Name() string { return "lockdown" }

// Evaluate returns ActionLockdown once the longest heavy run reaches
// the lockdown threshold. Only gated users are locked.
func (LockdownPolicy) Evaluate(s Snapshot) (time.Time, Action) {
	next := s.Now.Add(evaluateInterval)
	if g, _ := gated(s.Plan); !g {
		return next, ActionNone
	}
	if heavyLongestRun(s) >= lockdownRun {
		return next, ActionLockdown
	}
	return next, ActionNone
}

// TamperPolicy locks gated users immediately when the usage ledger
// carries the tamper mark: wiping the data directory to reset the
// usage runway forfeits the runway entirely, forcing a sign-in. An
// entitled session clears the mark (see Planner.evaluate), restoring
// the normal runway for a later lapse.
type TamperPolicy struct{}

// Name identifies the policy in logs.
func (TamperPolicy) Name() string { return "tamper" }

// Evaluate returns ActionLockdown for gated users on a tampered
// install, regardless of accrued usage.
func (TamperPolicy) Evaluate(s Snapshot) (time.Time, Action) {
	next := s.Now.Add(evaluateInterval)
	if g, _ := gated(s.Plan); !g {
		return next, ActionNone
	}
	if s.Tampered {
		return next, ActionLockdown
	}
	return next, ActionNone
}

// DefaultPolicies returns the enforcement policies Rune ships with.
func DefaultPolicies() []Policy {
	return []Policy{
		NagPolicy{}, AskMoreTimePolicy{}, LockdownPolicy{},
		TamperPolicy{},
	}
}
