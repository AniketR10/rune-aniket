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
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/doctoml"

	"unstable.build/go-tui/localstorage/boltdoc"
)

type legacyDoc struct {
	Name  string
	Value int
}

func writeLegacy(t *testing.T, dir, id string, doc any, marshaler interface {
	Marshal(any) ([]byte, error)
}) {
	t.Helper()
	data, err := marshaler.Marshal(doc)
	require.NoError(t, err)
	path := filepath.Join(dir, url.PathEscape(id))
	require.NoError(t, os.MkdirAll(dir, 0o777))
	require.NoError(t, os.WriteFile(path, data, 0o600))
}

// TestMigrateLegacyPromotesRootAndPartitions verifies that records and
// partitions seeded in the schemedoc-style on-disk layout become
// readable from the new bolt database, and that the legacy entries
// are moved aside into .db.legacy/ so the disk does not double-bill
// the user for the same data.
func TestMigrateLegacyPromotesRootAndPartitions(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageDir := filepath.Join(root, ".db")
	marshaler := doctoml.Marshaler()

	writeLegacy(t, storageDir, "alpha", &legacyDoc{Name: "alpha", Value: 1}, marshaler)
	writeLegacy(t, storageDir, "beta", &legacyDoc{Name: "beta", Value: 2}, marshaler)
	partitionDir := filepath.Join(storageDir, url.PathEscape("memories"))
	writeLegacy(t, partitionDir, "m1", &legacyDoc{Name: "m1", Value: 10}, marshaler)

	svc, handle, err := boltdoc.New(filepath.Join(storageDir, "rune.db"), marshaler)
	require.NoError(t, err)
	t.Cleanup(func() { _ = handle.Close() })
	require.NoError(t, boltdoc.MigrateLegacy(ctx, svc, storageDir, "rune.db", marshaler))

	var got legacyDoc
	require.NoError(t, svc.Get(ctx, "alpha", &got))
	assert.Equal(t, "alpha", got.Name)
	require.NoError(t, svc.Get(ctx, "beta", &got))
	assert.Equal(t, "beta", got.Name)

	part, err := svc.Partition("memories")
	require.NoError(t, err)
	require.NoError(t, part.Get(ctx, "m1", &got))
	assert.Equal(t, "m1", got.Name)

	legacyAside := filepath.Join(root, ".db.legacy")
	_, err = os.Stat(filepath.Join(legacyAside, url.PathEscape("alpha")))
	require.NoError(t, err, ".db.legacy/alpha should exist")
	_, err = os.Stat(filepath.Join(storageDir, url.PathEscape("alpha")))
	require.True(t, os.IsNotExist(err), "legacy file should have been moved out")
}

// TestMigrateLegacyNoopWhenBoltExists confirms that MigrateLegacy is
// idempotent once the bolt file is present: re-running it must not
// re-migrate or touch any legacy entries.
func TestMigrateLegacyNoopWhenBoltExists(t *testing.T) {
	root := t.TempDir()
	storageDir := filepath.Join(root, ".db")
	marshaler := doctoml.Marshaler()
	require.NoError(t, os.MkdirAll(storageDir, 0o777))
	require.NoError(t, os.WriteFile(filepath.Join(storageDir, "rune.db"), []byte("existing"), 0o600))
	writeLegacy(t, storageDir, "should-stay", &legacyDoc{Name: "stay"}, marshaler)

	pending, err := boltdoc.LegacyPending(storageDir, "rune.db")
	require.NoError(t, err)
	assert.False(t, pending, "LegacyPending must report false when bolt file is present")
	_, err = os.Stat(filepath.Join(storageDir, url.PathEscape("should-stay")))
	require.NoError(t, err, "legacy file must remain when bolt already exists")
}

// TestMigrateLegacyEmptyDirectoryIsNoop ensures that a fresh install
// (storageDir exists, no legacy entries, no bolt file yet) leaves the
// directory untouched and returns without error.
func TestMigrateLegacyEmptyDirectoryIsNoop(t *testing.T) {
	root := t.TempDir()
	storageDir := filepath.Join(root, ".db")
	require.NoError(t, os.MkdirAll(storageDir, 0o777))

	pending, err := boltdoc.LegacyPending(storageDir, "rune.db")
	require.NoError(t, err)
	assert.False(t, pending, "empty directory must not have pending legacy entries")
	entries, err := os.ReadDir(storageDir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}
