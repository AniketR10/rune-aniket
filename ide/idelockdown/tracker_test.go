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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

func TestTrackerLoadCreatesDoc(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	var doc usageDoc
	require.NoError(t, store.Get(context.Background(), usageDocID, &doc))
	assert.Equal(t, usageDocKind, doc.Kind)
	assert.Equal(t, int64(1), doc.Version)
	assert.Empty(t, doc.Days)
}

// retryCreateOnceStore simulates the firstmover retry behaviour that
// can produce ErrAlreadyExists on the second Create attempt: the
// underlying write succeeded on the first call, but the caller saw a
// transport error and the retry now races with itself.
type retryCreateOnceStore struct {
	storageapi.Service
	createAlreadyExistsOnce bool
}

func (s *retryCreateOnceStore) Create(ctx context.Context, id string, doc any) error {
	if !s.createAlreadyExistsOnce {
		s.createAlreadyExistsOnce = true
		if err := s.Service.Create(ctx, id, doc); err != nil {
			return err
		}
		return storageapi.ErrAlreadyExists
	}
	return s.Service.Create(ctx, id, doc)
}

func TestTrackerLoadToleratesAlreadyExists(t *testing.T) {
	store := &retryCreateOnceStore{Service: storagestub.NewInMemoryService()}
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()),
		"ErrAlreadyExists on retried Create must not propagate")
}

func TestTrackerLoadReadsExistingDoc(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))
	require.NoError(t, tr.recordActivity(context.Background(), monday))

	tr2 := newTracker(store)
	require.NoError(t, tr2.load(context.Background()))
	assert.Equal(t, []DayActivity{{Day: monday, ActiveSlots: 1}}, tr2.days())
}

func TestTrackerRecordActivityDedupesSlots(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	ctx := context.Background()
	require.NoError(t, tr.recordActivity(ctx, monday.Add(10*time.Hour)))
	require.NoError(t, tr.recordActivity(ctx, monday.Add(10*time.Hour+5*time.Minute)),
		"a second event within the slot must be a no-op")
	require.NoError(t, tr.recordActivity(ctx, monday.Add(10*time.Hour+20*time.Minute)))

	assert.Equal(t, []DayActivity{{Day: monday, ActiveSlots: 2}}, tr.days())

	var doc usageDoc
	require.NoError(t, store.Get(ctx, usageDocID, &doc))
	require.Len(t, doc.Days, 1)
	assert.Equal(t, "2026-01-05", doc.Days[0].Day)
	assert.Len(t, doc.Days[0].Slots, slotMaskLen,
		"the mask must stay fixed-width")
	assert.Equal(t, int64(3), doc.Version,
		"only distinct slots must bump the version")
}

func TestTrackerRecordActivitySortsDays(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	ctx := context.Background()
	require.NoError(t, tr.recordActivity(ctx, monday.AddDate(0, 0, 2)))
	require.NoError(t, tr.recordActivity(ctx, monday))
	require.NoError(t, tr.recordActivity(ctx, monday.AddDate(0, 0, 1)))

	assert.Equal(t, []DayActivity{
		{Day: monday, ActiveSlots: 1},
		{Day: monday.AddDate(0, 0, 1), ActiveSlots: 1},
		{Day: monday.AddDate(0, 0, 2), ActiveSlots: 1},
	}, tr.days())
}

func TestTrackerRecordActivityNormalizesToUTC(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	loc := time.FixedZone("minus10", -10*3600)
	// 2026-01-05 20:00 -10:00 is 2026-01-06 06:00 UTC.
	require.NoError(t, tr.recordActivity(context.Background(),
		time.Date(2026, 1, 5, 20, 0, 0, 0, loc)))
	assert.Equal(t, []DayActivity{
		{Day: time.Date(2026, 1, 6, 0, 0, 0, 0, time.UTC), ActiveSlots: 1},
	}, tr.days())
}

// TestTrackerRecordActivityAllSlots pins the mask encoding: all
// slotsPerDay slots of a day round-trip through the hex bitmask.
func TestTrackerRecordActivityAllSlots(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	ctx := context.Background()
	for slot := range slotsPerDay {
		require.NoError(t, tr.recordActivity(ctx,
			monday.Add(time.Duration(slot)*slotDuration)))
	}
	assert.Equal(t, []DayActivity{{Day: monday, ActiveSlots: slotsPerDay}},
		tr.days())
}

func TestTrackerRecordActivityPrunesOldDays(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	ctx := context.Background()
	old := monday.Add(-retention - 24*time.Hour)
	kept := monday.Add(-retention + 24*time.Hour)
	require.NoError(t, tr.recordActivity(ctx, old))
	require.NoError(t, tr.recordActivity(ctx, kept))
	require.NoError(t, tr.recordActivity(ctx, monday))

	assert.Equal(t, []DayActivity{
		{Day: bucketStart(kept, 24 * time.Hour), ActiveSlots: 1},
		{Day: monday, ActiveSlots: 1},
	}, tr.days(), "days past the retention window must be pruned")
}

func TestTrackerMarkNagged(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	ctx := context.Background()
	marked, err := tr.markNagged(ctx, monday, dailyNagCooldown)
	require.NoError(t, err)
	assert.True(t, marked, "first nag must mark")

	marked, err = tr.markNagged(ctx, monday, dailyNagCooldown)
	require.NoError(t, err)
	assert.False(t, marked, "second nag within the cooldown must not mark")

	marked, err = tr.markNagged(ctx, monday.Add(dailyNagCooldown-time.Second), dailyNagCooldown)
	require.NoError(t, err)
	assert.False(t, marked, "a nag just inside the cooldown must not mark")

	// A fresh tracker over the same store (a restart) must also see
	// the persisted mark.
	tr2 := newTracker(store)
	require.NoError(t, tr2.load(ctx))
	marked, err = tr2.markNagged(ctx, monday, dailyNagCooldown)
	require.NoError(t, err)
	assert.False(t, marked, "nag mark must persist across restarts")

	marked, err = tr2.markNagged(ctx, monday.Add(dailyNagCooldown), dailyNagCooldown)
	require.NoError(t, err)
	assert.True(t, marked, "an elapsed cooldown must mark again")
}

// conflictOnceStore fails the first Update with ErrPreconditionFailed
// after applying a concurrent write, simulating a CAS conflict with
// another session.
type conflictOnceStore struct {
	storageapi.Service
	conflicted bool
}

func (s *conflictOnceStore) Update(
	ctx context.Context, id string, updates []storageapi.Update,
	preconditions ...storageapi.Precondition,
) error {
	if !s.conflicted {
		s.conflicted = true
		var doc usageDoc
		if err := s.Service.Get(ctx, id, &doc); err != nil {
			return err
		}
		concurrent := []storageapi.Update{
			{FieldPath: []string{"Days"},
				Value: setSlot(doc.Days, "2026-01-01", 0)},
			{FieldPath: []string{"Version"}, Value: doc.Version + 1},
		}
		if err := s.Service.Update(ctx, id, concurrent); err != nil {
			return err
		}
		return storageapi.ErrPreconditionFailed
	}
	return s.Service.Update(ctx, id, updates, preconditions...)
}

func TestTrackerRecordActivityRetriesOnVersionConflict(t *testing.T) {
	store := &conflictOnceStore{Service: storagestub.NewInMemoryService()}
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	require.NoError(t, tr.recordActivity(context.Background(), monday))

	// Both the concurrent write and ours must survive the retry.
	assert.Equal(t, []DayActivity{
		{Day: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ActiveSlots: 1},
		{Day: monday, ActiveSlots: 1},
	}, tr.days())
}

// TestTrackerSetTampered pins that the tamper mark round-trips
// through the store in both directions, so a wiped-and-restored
// install stays flagged across restarts and an entitled sign-in
// durably clears it.
func TestTrackerSetTampered(t *testing.T) {
	ctx := context.Background()
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(ctx))
	assert.False(t, tr.tampered(), "a fresh doc must not carry the tamper mark")

	require.NoError(t, tr.setTampered(ctx, true))
	assert.True(t, tr.tampered())

	reloaded := newTracker(store)
	require.NoError(t, reloaded.load(ctx))
	assert.True(t, reloaded.tampered(), "the tamper mark must persist")

	require.NoError(t, reloaded.setTampered(ctx, false))
	assert.False(t, reloaded.tampered())

	cleared := newTracker(store)
	require.NoError(t, cleared.load(ctx))
	assert.False(t, cleared.tampered(), "the cleared mark must persist")
}

// TestTrackerMalformedMaskDegradesToEmpty pins the corrupt-doc
// behaviour: a malformed persisted mask counts as no activity and is
// replaced wholesale on the next write instead of poisoning the doc.
func TestTrackerMalformedMaskDegradesToEmpty(t *testing.T) {
	ctx := context.Background()
	store := storagestub.NewInMemoryService()
	doc := usageDoc{Kind: usageDocKind, Version: 1, Days: []dayUsage{
		{Day: "2026-01-05", Slots: "zz"},
	}}
	require.NoError(t, store.Create(ctx, usageDocID, &doc))

	tr := newTracker(store)
	require.NoError(t, tr.load(ctx))
	assert.Equal(t, []DayActivity{{Day: monday, ActiveSlots: 0}}, tr.days())

	require.NoError(t, tr.recordActivity(ctx, monday.Add(10*time.Hour)))
	assert.Equal(t, []DayActivity{{Day: monday, ActiveSlots: 1}}, tr.days())
}
