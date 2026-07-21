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
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/retry"
)

const (
	// Partition is the storage partition the planner persists usage
	// under.
	Partition = "idelockdown"

	usageDocID   = "usage"
	usageDocKind = "usage-samples"
	// dayLayout is fixed-width for UTC days, so lexicographic order
	// is chronological order.
	dayLayout = "2006-01-02"
	// sampleLayout is fixed-width for UTC times, so lexicographic
	// order is chronological order.
	sampleLayout = time.RFC3339
)

var retryStrategy = retry.DefaultStrategy

// dayUsage is one UTC day of activity evidence.
type dayUsage struct {
	// Day is the UTC day formatted as dayLayout.
	Day string
	// Slots is a hex bitmask of the day's slotsPerDay activity
	// slots (slotMaskLen characters, 4 slots per character).
	Slots string
}

type usageDoc struct {
	Kind    string
	Version int64
	// Days are per-day activity bitmasks, ascending by Day, pruned
	// past the retention window.
	Days []dayUsage
	// LastNag is the UTC time the nag prompt was last shown.
	LastNag string
	// Tampered marks the install as having wiped the data directory
	// while gated: install-ID resolution found the obscure backup but
	// no authoritative store. The mark persists here because the
	// detection signal self-heals after its first observation.
	Tampered bool
}

// tracker persists the usage-sample history through a
// storageapi.Service partition.
type tracker struct {
	store storageapi.Service

	mu  sync.Mutex
	doc usageDoc
}

func newTracker(store storageapi.Service) *tracker {
	return &tracker{store: store, doc: usageDoc{Kind: usageDocKind, Version: 1}}
}

func (t *tracker) load(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	err := t.store.Get(ctx, usageDocID, &t.doc)
	if err == nil {
		return nil
	}
	if !errors.Is(err, storageapi.ErrNotFound) {
		return err
	}
	// A retried Create can race with the original (now-succeeded)
	// Create, in which case the second attempt sees
	// ErrAlreadyExists. The doc is ours either way, so treat
	// ErrAlreadyExists as success and re-read the persisted state.
	if err := t.store.Create(ctx, usageDocID, &t.doc); err != nil {
		if !errors.Is(err, storageapi.ErrAlreadyExists) {
			return err
		}
		return t.store.Get(ctx, usageDocID, &t.doc)
	}
	return nil
}

// recordActivity sets the activity-slot bit containing now and
// prunes days older than the retention window. It is a no-op when
// the slot is already recorded, so at most one write lands per slot.
func (t *tracker) recordActivity(ctx context.Context, now time.Time) error {
	utc := now.UTC()
	day := utc.Format(dayLayout)
	slot := (utc.Hour()*60 + utc.Minute()) / int(slotDuration/time.Minute)
	t.mu.Lock()
	defer t.mu.Unlock()
	if slotRecorded(t.doc.Days, day, slot) {
		return nil
	}
	return storageapi.ConsistentUpdate(ctx, t.store, usageDocID, &t.doc, retryStrategy,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			if slotRecorded(t.doc.Days, day, slot) {
				return nil, nil
			}
			days := pruneDays(setSlot(t.doc.Days, day, slot), utc)
			return []storageapi.Update{
					{FieldPath: []string{"Days"}, Value: days},
					{FieldPath: []string{"Version"}, Value: t.doc.Version + 1},
				}, []storageapi.Precondition{
					{FieldPath: []string{"Version"}, Value: t.doc.Version},
				}
		})
}

// markNagged persists now as the last nag time. It reports false
// when the persisted mark is younger than cooldown (e.g. another
// session nagged first) so the caller can skip showing the prompt.
func (t *tracker) markNagged(
	ctx context.Context, now time.Time, cooldown time.Duration,
) (bool, error) {
	key := now.UTC().Format(sampleLayout)
	t.mu.Lock()
	defer t.mu.Unlock()
	if !nagDue(t.doc.LastNag, now, cooldown) {
		return false, nil
	}
	marked := false
	err := storageapi.ConsistentUpdate(ctx, t.store, usageDocID, &t.doc, retryStrategy,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			if !nagDue(t.doc.LastNag, now, cooldown) {
				marked = false
				return nil, nil
			}
			marked = true
			return []storageapi.Update{
					{FieldPath: []string{"LastNag"}, Value: key},
					{FieldPath: []string{"Version"}, Value: t.doc.Version + 1},
				}, []storageapi.Precondition{
					{FieldPath: []string{"Version"}, Value: t.doc.Version},
				}
		})
	if err != nil {
		return false, err
	}
	return marked, nil
}

// setTampered persists the tamper mark. It is a no-op when the mark
// already has the requested value.
func (t *tracker) setTampered(ctx context.Context, tampered bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.doc.Tampered == tampered {
		return nil
	}
	return storageapi.ConsistentUpdate(ctx, t.store, usageDocID, &t.doc, retryStrategy,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			if t.doc.Tampered == tampered {
				return nil, nil
			}
			return []storageapi.Update{
					{FieldPath: []string{"Tampered"}, Value: tampered},
					{FieldPath: []string{"Version"}, Value: t.doc.Version + 1},
				}, []storageapi.Precondition{
					{FieldPath: []string{"Version"}, Value: t.doc.Version},
				}
		})
}

// tampered reports whether the tamper mark is set.
func (t *tracker) tampered() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.doc.Tampered
}

// nagDue reports whether the persisted last-nag mark is at least
// cooldown old. Missing or malformed marks count as due.
func nagDue(lastNag string, now time.Time, cooldown time.Duration) bool {
	if lastNag == "" {
		return true
	}
	last, err := time.Parse(sampleLayout, lastNag)
	if err != nil {
		return true
	}
	return now.Sub(last) >= cooldown
}

// DayActivity is one UTC day of decoded activity evidence.
type DayActivity struct {
	// Day is the UTC midnight of the day.
	Day time.Time
	// ActiveSlots is the number of activity slots with at least one
	// recorded event.
	ActiveSlots int
}

// days returns the recorded per-day activity, ascending by day.
// Malformed entries are skipped.
func (t *tracker) days() []DayActivity {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]DayActivity, 0, len(t.doc.Days))
	for _, d := range t.doc.Days {
		day, err := time.Parse(dayLayout, d.Day)
		if err != nil {
			continue
		}
		out = append(out, DayActivity{Day: day.UTC(), ActiveSlots: countSlots(d.Slots)})
	}
	return out
}

// slotRecorded reports whether the slot bit for day is already set.
func slotRecorded(days []dayUsage, day string, slot int) bool {
	i := searchDay(days, day)
	if i >= len(days) || days[i].Day != day {
		return false
	}
	mask := days[i].Slots
	if len(mask) != slotMaskLen {
		return false
	}
	return hexNibble(mask[slot/4])&(1<<(slot%4)) != 0
}

// setSlot returns days with the slot bit for day set, inserting the
// day in ascending order when absent. The input slice is not
// modified. A malformed persisted mask is replaced by an empty one
// rather than propagated.
func setSlot(days []dayUsage, day string, slot int) []dayUsage {
	i := searchDay(days, day)
	mask := emptySlotMask()
	present := i < len(days) && days[i].Day == day
	if present && len(days[i].Slots) == slotMaskLen {
		mask = days[i].Slots
	}
	nibble := hexNibble(mask[slot/4]) | 1<<(slot%4)
	mask = mask[:slot/4] + string(hexDigits[nibble]) + mask[slot/4+1:]
	out := make([]dayUsage, 0, len(days)+1)
	out = append(out, days[:i]...)
	out = append(out, dayUsage{Day: day, Slots: mask})
	if present {
		out = append(out, days[i+1:]...)
	} else {
		out = append(out, days[i:]...)
	}
	return out
}

// pruneDays drops days older than the retention window ending at
// now.
func pruneDays(days []dayUsage, now time.Time) []dayUsage {
	cutoff := now.UTC().Add(-retention).Format(dayLayout)
	i := sort.Search(len(days), func(i int) bool { return days[i].Day >= cutoff })
	return days[i:]
}

func searchDay(days []dayUsage, day string) int {
	return sort.Search(len(days), func(i int) bool { return days[i].Day >= day })
}

const hexDigits = "0123456789abcdef"

// hexNibble decodes one lowercase hex character; malformed input
// counts as zero so a corrupt mask degrades to "no activity".
func hexNibble(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	default:
		return 0
	}
}

func emptySlotMask() string {
	return strings.Repeat("0", slotMaskLen)
}

// countSlots counts the set bits in a slot mask.
func countSlots(mask string) int {
	n := 0
	for i := 0; i < len(mask); i++ {
		nibble := hexNibble(mask[i])
		for ; nibble != 0; nibble &= nibble - 1 {
			n++
		}
	}
	return n
}
