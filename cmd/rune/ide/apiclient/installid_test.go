// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package apiclient

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
)

// testInstallIDBackup returns a per-test backup directory and the path
// of the backup file inside it, so tests never touch the real OS temp
// directory.
func testInstallIDBackup(t *testing.T) (dir, path string) {
	t.Helper()
	dir = t.TempDir()
	return dir, filepath.Join(dir, installIDTempFileName())
}

func readInstallIDDoc(t *testing.T, store storageapi.Service) (installIDDoc, bool) {
	t.Helper()
	var doc installIDDoc
	err := store.Get(context.Background(), installIDDocID, &doc)
	if err != nil {
		return installIDDoc{}, false
	}
	return doc, true
}

// errSetFailed is the sentinel a setFailingStore returns from Set so tests can
// assert the failure is threaded into the reported install-ID error.
var errSetFailed = errors.New("set boom")

// setFailingStore is a storageapi.Service whose Set always fails, used to
// exercise the self-heal write error path.
type setFailingStore struct {
	storageapi.Service
}

func (setFailingStore) Set(context.Context, string, any) error {
	return errSetFailed
}

func TestGetInstallIDBothPresent(t *testing.T) {
	dir, tempPath := testInstallIDBackup(t)
	store := storagestub.NewInMemoryService()
	require.NoError(t, store.Set(context.Background(), installIDDocID, installIDDoc{ID: "shared-id"}))
	require.NoError(t, os.WriteFile(tempPath, []byte("shared-id"), 0o600))

	id, tampered, errStr := getInstallID(context.Background(), store, dir)

	assert.Equal(t, "shared-id", id)
	assert.False(t, tampered, "identical present copies must not flag tampering")
	assert.Empty(t, errStr)
}

func TestGetInstallIDStorageAbsentTempPresentFlagsTampered(t *testing.T) {
	dir, tempPath := testInstallIDBackup(t)
	store := storagestub.NewInMemoryService()
	require.NoError(t, os.WriteFile(tempPath, []byte("backup-id"), 0o600))

	id, tampered, errStr := getInstallID(context.Background(), store, dir)

	assert.Equal(t, "backup-id", id)
	assert.True(t, tampered, "wiped store with surviving backup must flag tampering")
	assert.Empty(t, errStr)

	doc, ok := readInstallIDDoc(t, store)
	require.True(t, ok, "storage must be rewritten from the backup")
	assert.Equal(t, "backup-id", doc.ID)
}

func TestGetInstallIDStoragePresentTempAbsentNoFlag(t *testing.T) {
	dir, tempPath := testInstallIDBackup(t)
	store := storagestub.NewInMemoryService()
	require.NoError(t, store.Set(context.Background(), installIDDocID, installIDDoc{ID: "stored-id"}))

	id, tampered, errStr := getInstallID(context.Background(), store, dir)

	assert.Equal(t, "stored-id", id)
	assert.False(t, tampered, "missing backup (routine temp reaping) must not flag tampering")
	assert.Empty(t, errStr)

	raw, err := os.ReadFile(tempPath)
	require.NoError(t, err, "backup must be recovered from storage")
	assert.Equal(t, "stored-id", string(raw))
}

func TestGetInstallIDBothAbsentGeneratesNew(t *testing.T) {
	dir, tempPath := testInstallIDBackup(t)
	store := storagestub.NewInMemoryService()

	id, tampered, errStr := getInstallID(context.Background(), store, dir)

	require.NotEmpty(t, id, "fresh install must generate an identifier")
	assert.False(t, tampered, "fresh install must not flag tampering")
	assert.Empty(t, errStr)

	doc, ok := readInstallIDDoc(t, store)
	require.True(t, ok, "storage must persist the new identifier")
	assert.Equal(t, id, doc.ID)

	raw, err := os.ReadFile(tempPath)
	require.NoError(t, err, "backup must persist the new identifier")
	assert.Equal(t, id, string(raw))
}

func TestGetInstallIDStableAcrossCalls(t *testing.T) {
	dir, _ := testInstallIDBackup(t)
	store := storagestub.NewInMemoryService()

	first, tampered, errStr := getInstallID(context.Background(), store, dir)
	require.NotEmpty(t, first)
	require.False(t, tampered)
	require.Empty(t, errStr)

	second, tampered, errStr := getInstallID(context.Background(), store, dir)
	assert.Equal(t, first, second, "identifier must be stable once persisted")
	assert.False(t, tampered)
	assert.Empty(t, errStr)
}

func TestGetInstallIDReportsStoreWriteError(t *testing.T) {
	dir, _ := testInstallIDBackup(t)
	store := setFailingStore{Service: storagestub.NewInMemoryService()}

	id, tampered, errStr := getInstallID(context.Background(), store, dir)

	require.NotEmpty(t, id, "a store write failure must not stop id generation")
	assert.False(t, tampered)
	assert.Contains(t, errStr, errSetFailed.Error(),
		"store write failure must be reported instead of silently dropped")
}
