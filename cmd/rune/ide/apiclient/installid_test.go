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
