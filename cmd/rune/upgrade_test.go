// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/go-tui/ide/ideupgrade"
)

// TestUpgradeCommandHandler_DoesNotBlockOnNetwork is the regression
// test for the deadlock reported in pprof goroutine dumps where the
// `:upgrade` command parked the event-loop goroutine on
// fetchManifest's HTTP I/O — preventing the prompt that the same
// path is about to ScheduleNextTick from ever running.
//
// We stand up a manifest endpoint that never responds and assert the
// handler returns well before the request would unblock. The check
// itself completes asynchronously after we release the server.
func TestUpgradeCommandHandler_DoesNotBlockOnNetwork(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseFn := func() { releaseOnce.Do(func() { close(release) }) }
	hit := make(chan struct{}, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/"+runtime.GOOS+"-"+runtime.GOARCH+"/manifest.json",
		func(w http.ResponseWriter, _ *http.Request) {
		select {
		case hit <- struct{}{}:
		default:
		}
		<-release
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	// Cleanups run LIFO; releaseFn must be registered AFTER
	// srv.Close so it fires first and unblocks the in-flight
	// handler before httptest.Server.Close waits for it.
	t.Cleanup(srv.Close)
	t.Cleanup(releaseFn)

	mgr, err := ideupgrade.New(ideupgrade.Config{
		CurrentVersion: "v0.0.0",
		Arch:           runtime.GOOS + "-" + runtime.GOARCH,
		ManifestURL:    srv.URL,
		Storage:        storagestub.NewInMemoryService(),
		HTTPClient:     srv.Client(),
	})
	require.NoError(t, err)

	n := &fakeNotifications{}
	handler := upgradeCommandHandler(mgr, n)

	// Invoke the handler. It must return promptly — well before the
	// manifest server's reply — so the event loop stays responsive.
	done := make(chan error, 1)
	go func() { done <- handler(context.Background(), textapi.Command{}) }()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("upgrade command handler blocked on synchronous manifest fetch")
	}

	// The user gets an in-flight notification *before* the handler
	// returns. By the time the handler is unblocked, we expect:
	//   1. one Notify call ("Checking for updates...")
	//   2. one UpdateNotificationProgress call (1/2) anchoring the
	//      notification in the progress UI
	// The 2/2 close-out happens on the async goroutine after
	// CheckNow returns, which is still blocked on the test server.
	notifs := n.snapshot()
	require.Len(t, notifs.notifies, 1, "expected one Notify call before the handler returns")
	require.Equal(t, browserapi.LevelInfo, notifs.notifies[0].level)
	require.Contains(t, notifs.notifies[0].msg, "Checking for updates")
	require.Len(t, notifs.progress, 1, "expected one progress update before the handler returns")
	require.Equal(t, int64(1), notifs.progress[0].progress)
	require.Equal(t, int64(2), notifs.progress[0].total)

	// Sanity: the work was actually scheduled. The async goroutine
	// should have reached the manifest endpoint by now.
	select {
	case <-hit:
	case <-time.After(5 * time.Second):
		t.Fatal("manifest endpoint was never contacted by the async upgrade check")
	}

	// Let the manifest endpoint return now so the async upgrade
	// goroutine can finish CheckNow and close out the progress
	// notification. releaseFn is sync.Once-guarded so the deferred
	// cleanup that also calls it is a no-op.
	releaseFn()

	// After we let the manifest server respond, the async goroutine
	// must close out the progress notification with 2/2 so it
	// dismisses from the user's progress UI.
	require.Eventually(t, func() bool {
		s := n.snapshot()
		if len(s.progress) < 2 {
			return false
		}
		last := s.progress[len(s.progress)-1]
		return last.progress == 2 && last.total == 2
	}, 5*time.Second, 20*time.Millisecond,
		"expected progress notification to be closed with 2/2 after CheckNow returns")
}

// TestUpgradeCommandHandler_SurfacesErrorAsSeparateNotification is the
// regression test for the silent-failure UX bug where the user
// triggered `:upgrade`, saw "Checking for updates..." flash by, and
// then got no resolution at all because the only failure signal was
// hidden in the log:
//
//	WARN error: manifest fetch ...: status 403, msg: ideupgrade: check now
//
// Encoding the failure message into the same progress notification
// dismissed it (progress==total closes the notification), so the user
// never saw why the check failed. We now close the progress
// notification cleanly AND post a separate LevelError notification
// the user can actually read.
func TestUpgradeCommandHandler_SurfacesErrorAsSeparateNotification(t *testing.T) {
	// The manifest endpoint always returns 500 so CheckNow fails.
	mux := http.NewServeMux()
	mux.HandleFunc("/"+runtime.GOOS+"-"+runtime.GOARCH+"/manifest.json",
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mgr, err := ideupgrade.New(ideupgrade.Config{
		CurrentVersion: "v0.0.0",
		Arch:           runtime.GOOS + "-" + runtime.GOARCH,
		ManifestURL:    srv.URL,
		Storage:        storagestub.NewInMemoryService(),
		HTTPClient:     srv.Client(),
	})
	require.NoError(t, err)

	n := &fakeNotifications{}
	handler := upgradeCommandHandler(mgr, n)
	require.NoError(t, handler(context.Background(), textapi.Command{}))

	// After the async check returns, the handler must have:
	//  1. Closed the progress notification with 2/2 (dismisses it).
	//  2. Posted a fresh LevelError notification with the failure
	//     so the user actually sees a resolution.
	require.Eventually(t, func() bool {
		s := n.snapshot()
		if len(s.progress) < 2 {
			return false
		}
		last := s.progress[len(s.progress)-1]
		if last.progress != 2 || last.total != 2 {
			return false
		}
		// Look for an error-level notification that mentions the
		// failure. There may be more than one Notify call (the
		// initial "Checking for updates..." is LevelInfo).
		for _, nf := range s.notifies {
			if nf.level == browserapi.LevelError &&
				strings.Contains(nf.msg, "Upgrade check failed") {
				return true
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond,
		"expected progress to close 2/2 and a LevelError notification "+
			"surfacing the failure")
}

// TestUpgradeCommandHandler_TreatsForbiddenAsNoManifest covers the
// common rollout case: a fresh public GCS bucket returns 403 (not
// 404) for the not-yet-published manifest object. Clients must not
// surface that as a failure — the user is simply on the latest
// version and there's nothing to do.
func TestUpgradeCommandHandler_TreatsForbiddenAsNoManifest(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/"+runtime.GOOS+"-"+runtime.GOARCH+"/manifest.json",
		func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "Forbidden", http.StatusForbidden)
		})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	mgr, err := ideupgrade.New(ideupgrade.Config{
		CurrentVersion: "v0.0.0",
		Arch:           runtime.GOOS + "-" + runtime.GOARCH,
		ManifestURL:    srv.URL,
		Storage:        storagestub.NewInMemoryService(),
		HTTPClient:     srv.Client(),
		Notifications:  &fakeNotifications{}, // unused but required
	})
	require.NoError(t, err)

	n := &fakeNotifications{}
	handler := upgradeCommandHandler(mgr, n)
	require.NoError(t, handler(context.Background(), textapi.Command{}))

	// Eventually: progress closes 2/2 and the only notifications
	// the user sees are LevelInfo ("Checking..." then "up to
	// date") — no LevelError.
	require.Eventually(t, func() bool {
		s := n.snapshot()
		if len(s.progress) < 2 {
			return false
		}
		last := s.progress[len(s.progress)-1]
		return last.progress == 2 && last.total == 2
	}, 5*time.Second, 20*time.Millisecond,
		"expected progress to close 2/2 on 403")

	s := n.snapshot()
	for _, nf := range s.notifies {
		require.NotEqualf(t, browserapi.LevelError, nf.level,
			"403 should be a silent no-op, got error notification: %q",
			nf.msg)
	}
}

// fakeNotifications is a minimal browserapi.Notifications that
// records Notify and UpdateNotificationProgress calls so tests can
// assert ordering. It is safe for concurrent use.
type fakeNotifications struct {
	mu       sync.Mutex
	notifies []fakeNotify
	progress []fakeProgress
	nextID   int
}

type fakeNotify struct {
	id    string
	level browserapi.NotificationLevel
	msg   string
}

type fakeProgress struct {
	id              string
	message         string
	progress, total int64
}

type fakeNotificationsSnapshot struct {
	notifies []fakeNotify
	progress []fakeProgress
}

func (f *fakeNotifications) snapshot() fakeNotificationsSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return fakeNotificationsSnapshot{
		notifies: append([]fakeNotify(nil), f.notifies...),
		progress: append([]fakeProgress(nil), f.progress...),
	}
}

func (f *fakeNotifications) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	id := fakeNotifyID(f.nextID)
	f.notifies = append(f.notifies, fakeNotify{
		id:    id,
		level: level,
		msg:   formatNotify(msg, args...),
	})
	return id, nil
}

func (f *fakeNotifications) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return f.Notify(level, msg, args...)
}

func (f *fakeNotifications) UpdateNotificationProgress(
	id, message string, progress, total int64,
) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.progress = append(f.progress, fakeProgress{
		id: id, message: message, progress: progress, total: total,
	})
	return nil
}

func fakeNotifyID(n int) string { return fmt.Sprintf("n%d", n) }

func formatNotify(msg string, args ...any) string {
	if len(args) == 0 {
		return msg
	}
	return fmt.Sprintf(msg, args...)
}
