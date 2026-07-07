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
	maxSamples   = 400
	// sampleLayout is fixed-width for UTC times, so lexicographic
	// order is chronological order.
	sampleLayout = time.RFC3339
)

var retryStrategy = retry.DefaultStrategy

type usageDoc struct {
	Kind    string
	Version int64
	// Usage are unique UTC usage samples formatted as sampleLayout,
	// ascending, capped at maxSamples.
	Usage []string
	// LastNag is the UTC time the nag prompt was last shown.
	LastNag string
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

// recordUsage adds sample to the usage series. It is a no-op when
// the sample is already recorded.
func (t *tracker) recordUsage(ctx context.Context, sample time.Time) error {
	key := sample.UTC().Format(sampleLayout)
	t.mu.Lock()
	defer t.mu.Unlock()
	if containsSample(t.doc.Usage, key) {
		return nil
	}
	return storageapi.ConsistentUpdate(ctx, t.store, usageDocID, &t.doc, retryStrategy,
		func() ([]storageapi.Update, []storageapi.Precondition) {
			if containsSample(t.doc.Usage, key) {
				return nil, nil
			}
			samples := insertSample(t.doc.Usage, key)
			return []storageapi.Update{
					{FieldPath: []string{"Usage"}, Value: samples},
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

// usage returns the recorded usage samples in UTC, ascending.
func (t *tracker) usage() []time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return parseSamples(t.doc.Usage)
}

// parseSamples decodes sampleLayout keys, skipping malformed
// entries.
func parseSamples(keys []string) []time.Time {
	out := make([]time.Time, 0, len(keys))
	for _, k := range keys {
		s, err := time.Parse(sampleLayout, k)
		if err != nil {
			continue
		}
		out = append(out, s.UTC())
	}
	return out
}

func containsSample(samples []string, key string) bool {
	i := sort.SearchStrings(samples, key)
	return i < len(samples) && samples[i] == key
}

// insertSample returns samples with key inserted in ascending order,
// deduplicated and capped at maxSamples (oldest evicted first). The
// input slice is not modified.
func insertSample(samples []string, key string) []string {
	i := sort.SearchStrings(samples, key)
	if i < len(samples) && samples[i] == key {
		return samples
	}
	out := make([]string, 0, len(samples)+1)
	out = append(out, samples[:i]...)
	out = append(out, key)
	out = append(out, samples[i:]...)
	if len(out) > maxSamples {
		out = out[len(out)-maxSamples:]
	}
	return out
}
