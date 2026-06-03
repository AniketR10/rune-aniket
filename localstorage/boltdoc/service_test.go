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

package boltdoc_test

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/document/doctest"
	"github.com/unstablebuild/ox-api/bluestore"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/retry"

	"unstable.build/go-tui/localstorage/boltdoc"
)

// TestConcurrentServicesShareDBLifecycle reproduces the multi-workspace
// crash where each workspace opens its own boltdoc.Service against the
// same on-disk path (the extension storage server is constructed per
// workspace). The underlying *bolt.DB is shared process-wide, so closing
// one Service must not pull the database out from under the others;
// otherwise the survivors fail with "database not open".
func TestConcurrentServicesShareDBLifecycle(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "rune.db")

	svcA, err := boltdoc.New(dbPath, docbson.Marshaler())
	require.NoError(t, err)
	svcB, err := boltdoc.New(dbPath, docbson.Marshaler())
	require.NoError(t, err)
	t.Cleanup(func() { _ = svcB.Close() })

	type rec struct{ Name string }
	require.NoError(t, svcB.Create(ctx, "shared", &rec{Name: "b"}))

	require.NoError(t, svcA.Close())

	var got rec
	require.NoError(t, svcB.Get(ctx, "shared", &got),
		"closing one service must not close the DB shared with others")
	assert.Equal(t, "b", got.Name)
}

// TestStoreImplementsDocumentService runs the blue/doctest contract suite
// against the bolt-backed storageapi.Service. The suite covers Create,
// Set, Update with preconditions, Get, Delete, List with filters, Drop,
// and partition isolation. We adapt the storageapi.Service back to a
// document.Service so doctest can drive it directly. BSON is the
// production marshaler for local storage.
func TestStoreImplementsDocumentService(t *testing.T) {
	doctest.TestDocumentService(t, func(t *testing.T) document.Service {
		dir := t.TempDir()
		svc, err := boltdoc.New(filepath.Join(dir, "rune.db"), docbson.Marshaler())
		require.NoError(t, err)
		t.Cleanup(func() { _ = svc.Close() })
		return bluestore.AdaptFrom(svc)
	})
}

// TestConsistentUpdate verifies that two goroutines racing to bump a
// Version field via storageapi.ConsistentUpdate both succeed, with the
// final Version reflecting both increments. This is the central
// motivation for the schemedoc → bolt migration: bolt's RW transaction
// makes read-modify-write atomic, whereas schemedoc could lose updates
// under contention.
func TestConsistentUpdate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc, err := boltdoc.New(filepath.Join(dir, "rune.db"), docbson.Marshaler())
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	type counter struct {
		Version int64
		Value   string
	}

	require.NoError(t, svc.Create(ctx, "counter", &counter{Version: 1, Value: "init"}))

	strategy := retry.CombinedStrategy(
		retry.ExponentialStrategy(time.Millisecond, 50*time.Millisecond),
		retry.LimitStrategy(20),
	)
	var wg sync.WaitGroup
	for range 2 {
		wg.Go(func() {
			var c counter
			err := storageapi.ConsistentUpdate(ctx, svc, "counter", &c, strategy,
				func() ([]storageapi.Update, []storageapi.Precondition) {
					return []storageapi.Update{
							{FieldPath: []string{"Version"}, Value: c.Version + 1},
						}, []storageapi.Precondition{
							{FieldPath: []string{"Version"}, Value: c.Version},
						}
				})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	var got counter
	require.NoError(t, svc.Get(ctx, "counter", &got))
	assert.Equal(t, int64(3), got.Version, "both increments should have been applied")
}

// TestPartitionIsolation confirms that Partition returns a sibling Store
// rooted at a different bucket; records written under the partition are
// invisible to the root collection and vice-versa.
func TestPartitionIsolation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	svc, err := boltdoc.New(filepath.Join(dir, "rune.db"), docbson.Marshaler())
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	type rec struct{ Name string }

	require.NoError(t, svc.Create(ctx, "a", &rec{Name: "root-a"}))

	part, err := svc.Partition("memories")
	require.NoError(t, err)
	require.NoError(t, part.Create(ctx, "a", &rec{Name: "partition-a"}))

	var rootGot, partGot rec
	require.NoError(t, svc.Get(ctx, "a", &rootGot))
	require.NoError(t, part.Get(ctx, "a", &partGot))
	assert.Equal(t, "root-a", rootGot.Name)
	assert.Equal(t, "partition-a", partGot.Name)
}
