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

// Package idelockdown implements usage-based licensing enforcement.
// Rune is free for personal use; professional use requires a Rune
// Pro license. A background planner records editor activity in
// 15-minute slots, and a set of policies classify that history into
// professional/heavy usage runs which, together with the ideplan
// gating decision, decide whether to show a closable upgrade prompt
// or lock the IDE behind the lockdown overlay.
package idelockdown

import (
	"slices"
	"time"

	"unstable.build/go-tui/ide/ideplan"
)

// Action is the enforcement outcome of a Policy evaluation.
type Action int

const (
	// ActionNone requests no enforcement.
	ActionNone Action = iota
	// ActionPrompt requests a closable pay/sign-in prompt.
	ActionPrompt
	// ActionAskMoreTime requests a closable prompt that also offers
	// the user a way to ask for more time before lockdown.
	ActionAskMoreTime
	// ActionLockdown requests locking the IDE behind the lockdown
	// overlay.
	ActionLockdown
)

// LockReason explains why the IDE is being locked so the lockdown
// prompt can pick the matching button label and copy. It is derived
// from the plan Decision by gated.
type LockReason int

const (
	// LockNone means the IDE should not be locked.
	LockNone LockReason = iota
	// LockSignedOut means no token is present; the user must sign in.
	LockSignedOut
	// LockParseError means the token could not be verified; the user
	// must sign in again.
	LockParseError
	// LockNeverSubscribed means the user has never held a paid plan.
	LockNeverSubscribed
	// LockExpired means a previously paid plan has lapsed.
	LockExpired
	// LockUpgradeExpired means a one-off buyer is running a build
	// newer than their upgrade entitlement.
	LockUpgradeExpired
)

func (r LockReason) String() string {
	switch r {
	case LockNone:
		return "none"
	case LockSignedOut:
		return "signed_out"
	case LockParseError:
		return "parse_error"
	case LockNeverSubscribed:
		return "never_subscribed"
	case LockExpired:
		return "expired"
	case LockUpgradeExpired:
		return "upgrade_expired"
	default:
		return "unknown"
	}
}

// gated reports whether the plan Decision warrants locking the IDE
// and, if so, why. The authentication axis takes precedence: a
// signed-out or unverifiable session is a sign-in problem regardless
// of the plan Status. Otherwise the plan Status selects the reason.
func gated(d ideplan.Decision) (bool, LockReason) {
	switch d.SignedIn {
	case ideplan.SignedOut:
		return true, LockSignedOut
	case ideplan.ParseClaimsError:
		return true, LockParseError
	}
	switch d.Status {
	case ideplan.StatusNeverSubscribed:
		return true, LockNeverSubscribed
	case ideplan.StatusExpired:
		return true, LockExpired
	case ideplan.StatusUpgradeExpired:
		return true, LockUpgradeExpired
	default:
		return false, LockNone
	}
}

// Snapshot is the evidence a Policy inspects.
type Snapshot struct {
	// Now is the evaluation time (UTC).
	Now time.Time
	// Days are the persisted per-day activity records, ascending.
	Days []DayActivity
	// Plan is the login/plan state from ideplan.
	Plan ideplan.Decision
	// Tampered reports whether the usage ledger carries the tamper
	// mark: the data directory was wiped while the obscure install-ID
	// backup survived.
	Tampered bool
}

// Policy decides an enforcement Action from a usage Snapshot.
type Policy interface {
	Name() string
	// Evaluate returns the time at which the policy should next be
	// evaluated together with the enforcement action. A zero time
	// means the policy needs no further evaluation.
	Evaluate(s Snapshot) (time.Time, Action)
}

// bucketIndex maps t to the index of its bucket of size d, with
// buckets anchored at the Unix epoch. Division floors (not
// truncates) so pre-epoch times stay aligned.
func bucketIndex(t time.Time, d time.Duration) int64 {
	n := t.UnixNano()
	size := d.Nanoseconds()
	q := n / size
	if n%size != 0 && n < 0 {
		q--
	}
	return q
}

// bucketStart returns the start of the bucket of size d containing
// t, in UTC.
func bucketStart(t time.Time, d time.Duration) time.Time {
	return time.Unix(0, bucketIndex(t, d)*d.Nanoseconds()).UTC()
}

// qualifyingWeeks returns the epoch-anchored week indices whose days
// include at least minDays days with at least minSlots active slots,
// sorted ascending. A partial current week counts once it reaches
// the threshold.
func qualifyingWeeks(days []DayActivity, minSlots, minDays int) []int64 {
	perWeek := make(map[int64]int)
	for _, d := range days {
		if d.ActiveSlots >= minSlots {
			perWeek[bucketIndex(d.Day, weekWindow)]++
		}
	}
	weeks := make([]int64, 0, len(perWeek))
	for w, n := range perWeek {
		if n >= minDays {
			weeks = append(weeks, w)
		}
	}
	slices.Sort(weeks)
	return weeks
}

// longestRun returns the length in qualifying weeks of the longest
// run in weeks (sorted ascending), where up to gap consecutive
// non-qualifying weeks between qualifying weeks are tolerated
// (tolerated weeks do not count toward the length).
func longestRun(weeks []int64, gap int64) int {
	var best, run int
	for i, w := range weeks {
		if i > 0 && w-weeks[i-1] <= gap+1 {
			run++
		} else {
			run = 1
		}
		if run > best {
			best = run
		}
	}
	return best
}

// currentRun returns the length of the run that is still live at
// nowWeek: its last qualifying week is recent enough that qualifying
// in the current week would extend it. Once more than gap complete
// weeks have passed without qualifying, the run is no longer current
// and the result is zero.
func currentRun(weeks []int64, nowWeek int64, gap int64) int {
	n := len(weeks)
	if n == 0 || nowWeek-weeks[n-1] > gap+1 {
		return 0
	}
	run := 1
	for i := n - 1; i > 0; i-- {
		if weeks[i]-weeks[i-1] > gap+1 {
			break
		}
		run++
	}
	return run
}
