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

// Enforcement tuning knobs. Everything is expressed as a duration so
// a manual test build can shrink the whole timeline (e.g. minutes
// instead of days) by editing only these constants.
//
// MANUAL TEST VALUES — DO NOT COMMIT. Production values:
//
//	evaluateInterval  = 24 * time.Hour
//	usageBucket       = 24 * time.Hour
//	qualifyWindow     = 7 * usageBucket
//	qualifyingBuckets = 3
//	nagRun            = 2 * qualifyWindow
//	lockdownRun       = 6 * qualifyWindow
//	nagCooldown       = 24 * time.Hour
//	askMoreTimeRun    = lockdownRun - qualifyWindow
const (
	// evaluateInterval is how far ahead the policies schedule their
	// next evaluation.
	evaluateInterval = 5 * time.Second
	// usageBucket is the resolution at which samples count as
	// distinct usage evidence; the planner persists at most one
	// usage sample per bucket.
	usageBucket = 10 * time.Second
	// qualifyWindow counts toward an enforcement run when it
	// contains at least qualifyingBuckets distinct usage buckets.
	qualifyWindow = 3 * usageBucket
	// qualifyingBuckets is the minimum distinct usage buckets a
	// window needs to qualify.
	qualifyingBuckets = 3
	// nagRun is the consecutive qualifying usage run after which
	// expired users see the upgrade prompt.
	nagRun = 2 * qualifyWindow
	// lockdownRun is the consecutive qualifying usage run after
	// which expired users are locked out.
	lockdownRun = 6 * qualifyWindow
	// nagCooldown is the minimum interval between upgrade prompts.
	nagCooldown = 30 * time.Second
	// askMoreTimeRun is the consecutive qualifying usage run after
	// which the nag prompt is replaced by the ask-more-time prompt.
	// It covers the last qualifying window before lockdownRun so the
	// final prompt(s) before lockdown offer a way to ask for more
	// time.
	askMoreTimeRun = lockdownRun - qualifyWindow
)

// usageRun is the longest consecutive qualifying usage run in s.
func usageRun(s Snapshot) time.Duration {
	return maxConsecutiveQualifyingRun(
		s.Usage, qualifyWindow, usageBucket, qualifyingBuckets)
}

// NagPolicy shows a closable upgrade prompt to expired users after
// a nagRun of qualifying usage.
type NagPolicy struct{}

// Name identifies the policy in logs.
func (NagPolicy) Name() string { return "nag" }

// Evaluate returns ActionPrompt once the usage run reaches the nag
// threshold. Only gated users (expired, never-subscribed, or a
// sign-in problem) are nagged: active and grace-period users keep the
// existing daily warning.
func (NagPolicy) Evaluate(s Snapshot) (time.Time, Action) {
	next := s.Now.Add(evaluateInterval)
	if g, _ := gated(s.Plan); !g {
		return next, ActionNone
	}
	if usageRun(s) >= nagRun {
		return next, ActionPrompt
	}
	return next, ActionNone
}

// AskMoreTimePolicy replaces the nag prompt with the ask-more-time
// prompt for the last qualifying window before lockdown. Its action
// outranks ActionPrompt, so while both policies fire the planner
// shows the ask-more-time copy instead of the nag.
type AskMoreTimePolicy struct{}

// Name identifies the policy in logs.
func (AskMoreTimePolicy) Name() string { return "ask-more-time" }

// Evaluate returns ActionAskMoreTime once the usage run reaches the
// ask-more-time threshold. Only gated users (expired,
// never-subscribed, or a sign-in problem) are prompted.
func (AskMoreTimePolicy) Evaluate(s Snapshot) (time.Time, Action) {
	next := s.Now.Add(evaluateInterval)
	if g, _ := gated(s.Plan); !g {
		return next, ActionNone
	}
	if usageRun(s) >= askMoreTimeRun {
		return next, ActionAskMoreTime
	}
	return next, ActionNone
}

// LockdownPolicy locks the IDE for expired users after a lockdownRun
// of qualifying usage.
type LockdownPolicy struct{}

// Name identifies the policy in logs.
func (LockdownPolicy) Name() string { return "lockdown" }

// Evaluate returns ActionLockdown once the usage run reaches the
// lockdown threshold. Only gated users (expired, never-subscribed, or
// a sign-in problem) are locked.
func (LockdownPolicy) Evaluate(s Snapshot) (time.Time, Action) {
	next := s.Now.Add(evaluateInterval)
	if g, _ := gated(s.Plan); !g {
		return next, ActionNone
	}
	if usageRun(s) >= lockdownRun {
		return next, ActionLockdown
	}
	return next, ActionNone
}

// DefaultPolicies returns the enforcement policies Rune ships with.
func DefaultPolicies() []Policy {
	return []Policy{NagPolicy{}, AskMoreTimePolicy{}, LockdownPolicy{}}
}
