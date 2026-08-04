// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"golang.org/x/oauth2"
)

// TestCloseFlushesFinalUsage verifies that closing telemetry posts the
// counts accumulated since the last periodic flush, so the final window
// of usage is not lost on shutdown.
func TestCloseFlushesFinalUsage(t *testing.T) {
	backupDir := t.TempDir()
	usage := make(chan telemetryUsagePayload, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p telemetryUsagePayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p.Type == "ClientUsage" {
			usage <- p
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	// A long period keeps the periodic flush from firing so the only
	// ClientUsage post we observe is the one Close emits.
	tel := newTelemetry(oauth2.StaticTokenSource(&oauth2.Token{}), u, time.Hour, "test", "exo",
		storagestub.NewInMemoryService(), backupDir)

	tel.Handle(context.Background(), textapi.Event{Type: textapi.EventTypeEdit})
	tel.Handle(context.Background(), textapi.Event{Type: textapi.EventTypeEdit})
	tel.Handle(context.Background(), textapi.Event{Type: textapi.EventTypeOpen})

	if err := tel.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case p := <-usage:
		if p.Edited != 2 || p.Opened != 1 {
			t.Fatalf("final usage = %+v, want Edited=2 Opened=1", p)
		}
		assert.Equal(t, "exo", p.EditorMode)
	case <-time.After(time.Second):
		t.Fatal("Close did not flush a final ClientUsage event")
	}
}

// TestCloseFlushesWithNoEvents verifies Close still posts a final usage
// event when no editor events were recorded (e.g. the user opened no
// files), so a session is not dropped just because its counters are zero.
func TestCloseFlushesWithNoEvents(t *testing.T) {
	backupDir := t.TempDir()
	usage := make(chan telemetryUsagePayload, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p telemetryUsagePayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p.Type == "ClientUsage" {
			usage <- p
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	tel := newTelemetry(oauth2.StaticTokenSource(&oauth2.Token{}), u, time.Hour, "test", "modal",
		storagestub.NewInMemoryService(), backupDir)

	if err := tel.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case p := <-usage:
		if p.Opened != 0 || p.Closed != 0 || p.Flushed != 0 || p.Edited != 0 {
			t.Fatalf("final usage = %+v, want all zero counters", p)
		}
	case <-time.After(time.Second):
		t.Fatal("Close did not flush a final ClientUsage event with no prior events")
	}
}

// TestCloseFlushBoundedWhenOffline verifies Close returns within a small
// multiple of the flush timeout even when the server never responds, so
// an offline user is not blocked on shutdown.
func TestCloseFlushBoundedWhenOffline(t *testing.T) {
	backupDir := t.TempDir()
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	defer srv.Close()
	defer close(block)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}

	tel := newTelemetry(oauth2.StaticTokenSource(&oauth2.Token{}), u, time.Hour, "test", "modal",
		storagestub.NewInMemoryService(), backupDir)
	tel.Handle(context.Background(), textapi.Event{Type: textapi.EventTypeEdit})

	done := make(chan error, 1)
	go func() { done <- tel.Close() }()

	select {
	case <-done:
	case <-time.After(flushTimeout + 2*time.Second):
		t.Fatalf("Close blocked longer than flushTimeout %v when offline", flushTimeout)
	}
}

// TestRecordWatchedFilesChangeFlushed verifies agent-driven
// workspace/didChangeWatchedFiles calls are reported as a usage counter and
// that the posted count is cleared afterwards.
func TestRecordWatchedFilesChangeFlushed(t *testing.T) {
	usage := make(chan telemetryUsagePayload, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p telemetryUsagePayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p.Type == "ClientUsage" {
			usage <- p
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	tel := newTelemetry(oauth2.StaticTokenSource(&oauth2.Token{}), u, time.Hour, "test", "modal",
		storagestub.NewInMemoryService(), t.TempDir())
	client := &Client{telemetry: tel}

	client.RecordWatchedFilesChange(2)
	client.RecordWatchedFilesChange(1)

	require.NoError(t, tel.Close())

	select {
	case p := <-usage:
		assert.Equal(t, 3, p.WatchedChanges)
		tel.resetUsage(p)
		assert.Equal(t, 0, tel.getUsage().WatchedChanges)
	case <-time.After(time.Second):
		t.Fatal("Close did not flush a final ClientUsage event")
	}
}

// TestRecordWatchedFilesChangeDisabled verifies the counter is a safe no-op
// when telemetry is disabled, so the hook can be wired unconditionally.
func TestRecordWatchedFilesChangeDisabled(t *testing.T) {
	client := &Client{}
	assert.False(t, client.TelemetryEnabled())
	assert.NotPanics(t, func() { client.RecordWatchedFilesChange(3) })
}

// captureSystemPayload starts telemetry against a test server and returns the
// first ClientSystem payload it posts. The system event is emitted eagerly on
// start, so a long period keeps any usage post from racing the assertion.
func captureSystemPayload(
	t *testing.T, store storageapi.Service, backupDir, editorMode string,
) telemetrySystemPayload {
	t.Helper()

	system := make(chan telemetrySystemPayload, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var p telemetrySystemPayload
		_ = json.NewDecoder(r.Body).Decode(&p)
		if p.Type == "ClientSystem" {
			select {
			case system <- p:
			default:
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)

	tel := newTelemetry(oauth2.StaticTokenSource(&oauth2.Token{}), u, time.Hour, "test",
		editorMode, store, backupDir)
	tel.start()
	defer func() { _ = tel.Close() }()

	select {
	case p := <-system:
		return p
	case <-time.After(2 * time.Second):
		t.Fatal("telemetry did not post a ClientSystem event")
		return telemetrySystemPayload{}
	}
}

func TestTelemetrySystemPayloadEditorMode(t *testing.T) {
	p := captureSystemPayload(t, storagestub.NewInMemoryService(), t.TempDir(), "exo")

	assert.Equal(t, "exo", p.EditorMode)
}

// TestTelemetrySystemPayloadInstallID exercises the two-location install ID
// through the real telemetry post path, asserting the InstallID and Tampered
// fields the server receives for every persistence scenario.
func TestTelemetrySystemPayloadInstallID(t *testing.T) {
	t.Run("both present and equal", func(t *testing.T) {
		dir, tempPath := testInstallIDBackup(t)
		store := storagestub.NewInMemoryService()
		require.NoError(t, store.Set(context.Background(), installIDDocID, installIDDoc{ID: "shared-id"}))
		require.NoError(t, os.WriteFile(tempPath, []byte("shared-id"), 0o600))

		p := captureSystemPayload(t, store, dir, "modal")

		assert.Equal(t, "shared-id", p.InstallID)
		assert.False(t, p.Tampered)
	})

	t.Run("storage absent temp present flags tampered", func(t *testing.T) {
		dir, tempPath := testInstallIDBackup(t)
		store := storagestub.NewInMemoryService()
		require.NoError(t, os.WriteFile(tempPath, []byte("backup-id"), 0o600))

		p := captureSystemPayload(t, store, dir, "modal")

		assert.Equal(t, "backup-id", p.InstallID)
		assert.True(t, p.Tampered, "wiped store with surviving backup must post tampered=true")

		var doc installIDDoc
		require.NoError(t, store.Get(context.Background(), installIDDocID, &doc))
		assert.Equal(t, "backup-id", doc.ID, "storage must be rewritten from the backup")
	})

	t.Run("storage present temp absent no flag", func(t *testing.T) {
		dir, tempPath := testInstallIDBackup(t)
		store := storagestub.NewInMemoryService()
		require.NoError(t, store.Set(context.Background(), installIDDocID, installIDDoc{ID: "stored-id"}))

		p := captureSystemPayload(t, store, dir, "modal")

		assert.Equal(t, "stored-id", p.InstallID)
		assert.False(t, p.Tampered, "missing backup (routine temp reaping) must not flag tampering")

		raw, err := os.ReadFile(tempPath)
		require.NoError(t, err, "backup must be recovered from storage")
		assert.Equal(t, "stored-id", string(raw))
	})

	t.Run("both absent generates new", func(t *testing.T) {
		dir, tempPath := testInstallIDBackup(t)
		store := storagestub.NewInMemoryService()

		p := captureSystemPayload(t, store, dir, "modal")

		require.NotEmpty(t, p.InstallID, "fresh install must post a generated identifier")
		assert.False(t, p.Tampered)

		var doc installIDDoc
		require.NoError(t, store.Get(context.Background(), installIDDocID, &doc))
		assert.Equal(t, p.InstallID, doc.ID, "storage must persist the posted identifier")

		raw, err := os.ReadFile(tempPath)
		require.NoError(t, err, "backup must persist the posted identifier")
		assert.Equal(t, p.InstallID, string(raw))
	})

	t.Run("store write failure reported in payload", func(t *testing.T) {
		dir, _ := testInstallIDBackup(t)
		store := setFailingStore{Service: storagestub.NewInMemoryService()}

		p := captureSystemPayload(t, store, dir, "modal")

		require.NotEmpty(t, p.InstallID, "a store write failure must not stop id generation")
		assert.False(t, p.Tampered)
		assert.Contains(t, p.InstallIDErr, errSetFailed.Error(),
			"store write failure must be posted, not silently dropped")
	})
}
