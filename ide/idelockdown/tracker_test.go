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
	assert.Empty(t, doc.Usage)
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
	require.NoError(t, tr.recordUsage(context.Background(), monday))

	tr2 := newTracker(store)
	require.NoError(t, tr2.load(context.Background()))
	assert.Equal(t, []time.Time{monday}, tr2.usage())
}

func TestTrackerRecordUsageDedupesAndSorts(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	ctx := context.Background()
	require.NoError(t, tr.recordUsage(ctx, monday.AddDate(0, 0, 2)))
	require.NoError(t, tr.recordUsage(ctx, monday))
	require.NoError(t, tr.recordUsage(ctx, monday.AddDate(0, 0, 1)))
	require.NoError(t, tr.recordUsage(ctx, monday),
		"duplicate sample must be a no-op")

	assert.Equal(t, []time.Time{
		monday, monday.AddDate(0, 0, 1), monday.AddDate(0, 0, 2),
	}, tr.usage())

	var doc usageDoc
	require.NoError(t, store.Get(ctx, usageDocID, &doc))
	assert.Equal(t, []string{
		"2026-01-05T00:00:00Z", "2026-01-06T00:00:00Z", "2026-01-07T00:00:00Z",
	}, doc.Usage)
}

func TestTrackerRecordSampleNormalizesToUTC(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	loc := time.FixedZone("minus10", -10*3600)
	// 2026-01-05 20:00 -10:00 is 2026-01-06 06:00 UTC.
	require.NoError(t, tr.recordUsage(context.Background(),
		time.Date(2026, 1, 5, 20, 0, 0, 0, loc)))
	assert.Equal(t, []time.Time{
		time.Date(2026, 1, 6, 6, 0, 0, 0, time.UTC),
	}, tr.usage())
}

func TestTrackerRecordSampleCapsHistory(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	ctx := context.Background()
	for i := range maxSamples + 5 {
		require.NoError(t, tr.recordUsage(ctx, monday.AddDate(0, 0, i)))
	}
	samples := tr.usage()
	require.Len(t, samples, maxSamples)
	assert.Equal(t, monday.AddDate(0, 0, 5), samples[0],
		"oldest samples must be evicted first")
	assert.Equal(t, monday.AddDate(0, 0, maxSamples+4), samples[len(samples)-1])
}

func TestTrackerMarkNagged(t *testing.T) {
	store := storagestub.NewInMemoryService()
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	ctx := context.Background()
	marked, err := tr.markNagged(ctx, monday, nagCooldown)
	require.NoError(t, err)
	assert.True(t, marked, "first nag must mark")

	marked, err = tr.markNagged(ctx, monday, nagCooldown)
	require.NoError(t, err)
	assert.False(t, marked, "second nag within the cooldown must not mark")

	marked, err = tr.markNagged(ctx, monday.Add(nagCooldown-time.Second), nagCooldown)
	require.NoError(t, err)
	assert.False(t, marked, "a nag just inside the cooldown must not mark")

	// A fresh tracker over the same store (a restart) must also see
	// the persisted mark.
	tr2 := newTracker(store)
	require.NoError(t, tr2.load(ctx))
	marked, err = tr2.markNagged(ctx, monday, nagCooldown)
	require.NoError(t, err)
	assert.False(t, marked, "nag mark must persist across restarts")

	marked, err = tr2.markNagged(ctx, monday.Add(nagCooldown), nagCooldown)
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
			{FieldPath: []string{"Usage"},
				Value: insertSample(doc.Usage, "2020-06-01T00:00:00Z")},
			{FieldPath: []string{"Version"}, Value: doc.Version + 1},
		}
		if err := s.Service.Update(ctx, id, concurrent); err != nil {
			return err
		}
		return storageapi.ErrPreconditionFailed
	}
	return s.Service.Update(ctx, id, updates, preconditions...)
}

func TestTrackerRecordSampleRetriesOnVersionConflict(t *testing.T) {
	store := &conflictOnceStore{Service: storagestub.NewInMemoryService()}
	tr := newTracker(store)
	require.NoError(t, tr.load(context.Background()))

	require.NoError(t, tr.recordUsage(context.Background(), monday))

	// Both the concurrent write and ours must survive the retry.
	assert.Equal(t, []time.Time{
		time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC), monday,
	}, tr.usage())
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
