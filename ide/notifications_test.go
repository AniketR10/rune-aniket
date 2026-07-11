// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package ide

import (
	"context"
	"hash/fnv"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/document"
	"github.com/unstablebuild/blue/release/docrelease"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/docmarshal/docbson"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/localstorage"
	"unstable.build/go-tui/workspace"
)

func newTestNotifications(uri workspaceapi.URI, t *testing.T) (*testNotifier, *notis) {
	mock := newTestNotify()
	b := &notis{
		root:    mock,
		storage: storagestub.NewInMemoryService(),
		parent:  &workspaceManagerMock{workspace: new(ex), wantFocusURI: uri},
		uri:     uri,
		cfg: notifications.Config{
			ColorError: term.Attributes{Fg: term.ColorRed},
		},
	}
	return mock, b
}

func TestNotifyAcrossWorkspaces(t *testing.T) {
	t.Run("notifications are paused and attention attrs set", func(t *testing.T) {
		workspace, err := workspaceapi.ParseURI("file:///b")
		require.NoError(t, err)
		mock, b := newTestNotifications(workspace, t)
		workspaceMock := b.parent.(*workspaceManagerMock)
		workspaceMock.wantFocusURI, err = workspaceapi.ParseURI("file:///a")
		require.NoError(t, err)

		_, err = b.Notify(browserapi.LevelError, "abc")
		require.NoError(t, err)

		assert.Equal(t, 1, mock.messages["abc_paused"])
		expectedAttrs := map[workspaceapi.URI]term.Attributes{
			workspace: {Fg: term.ColorRed},
		}
		assert.Equal(t, expectedAttrs, workspaceMock.attrs)
	})
}

// TestSetWorkspaceRequiresAttentionRace reproduces the data race
// observed when many VTE run goroutines deliver notifications
// concurrently: notis.Notify -> setTabAttr ->
// setWorkspaceRequiresAttention mutates the shared tab/layout tree via
// Resize off the event loop. The fix marshals that mutation through
// scheduleNextTick; run under -race, this test fails before the fix
// and passes after.
func TestSetWorkspaceRequiresAttentionRace(t *testing.T) {
	homeURI, err := workspaceapi.ParseURI("memory:///home")
	require.NoError(t, err)

	mu := new(sync.Mutex)
	sched, drain := newTestScheduler(mu)
	cfg := defaultCfg()
	cfg.scheduleNextTick = sched

	manager := workspace.NewManager(cfg.workspace(), inlineSchedule)
	require.NoError(t, manager.RegisterScheme(workspace.MemoryScheme,
		workspace.NewMemoryScheme))

	dir := t.TempDir()
	storage := localstorage.New(context.Background(), dir, docbson.Marshaler())
	releaseManager := docrelease.NewManager(document.NewInMemoryService())

	shRunner := new(shaderRunner)
	shRunner.init(handler.Nop(), term.NopInterrupter(), term.Attributes{},
		nopShutdownShaderConfig(), loadingShaderConfig{}, openShaderConfig{},
		component.FrameCharSetDefault())

	h := new(workspaceManagerHandler)
	h.tutorialsInstalled = func([]string) (bool, error) { return false, nil }
	err = h.init(nil, homeURI, manager,
		notificationsConfig(), cfg, storage, dir,
		func(term.Event) bool { return true },
		FuncExtensionsRunner(testRunnerFn), mu, nil,
		func() (ideConfig, error) { return cfg, nil },
		".sixrc", 0, 0, '1', 0, 0, true, nil, releaseManager,
		shRunner, 0, nil, false, false, newCommandObserverRegistry())
	require.NoError(t, err)

	wsURI, err := workspaceapi.ParseURI("memory:///ws")
	require.NoError(t, err)
	// Slot 1 is not the focused slot (focus defaults to 0), so Resize
	// never dereferences this handler's embedded *ex.
	h.workspaces[1] = &workspaceHandler{uri: wsURI}
	h.width, h.height = 80, 24

	attr := term.Attributes{Fg: term.ColorRed}
	const goroutines = 32
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			h.setWorkspaceRequiresAttention(wsURI, attr)
		}()
	}
	wg.Wait()
	drain()

	assert.Equal(t, attr, h.workspaces[1].attentionAttr)
}

func TestNotifyOnce(t *testing.T) {
	uri, err := workspaceapi.ParseURI("file:///a")
	require.NoError(t, err)
	t.Run("delivers notifications only the first time it's invoked", func(t *testing.T) {
		t.Parallel()

		mock, b := newTestNotifications(uri, t)

		_, err := b.NotifyOnce(browserapi.LevelError, "a")
		require.NoError(t, err)
		assert.Equal(t, 1, mock.messages["a"])

		_, err = b.NotifyOnce(browserapi.LevelError, "a")
		require.NoError(t, err)
		assert.Equal(t, 1, mock.messages["a"])
	})

	t.Run("delivers notifications with different args multiple times, if args are different", func(t *testing.T) {
		t.Parallel()

		mock, b := newTestNotifications(uri, t)

		_, err := b.NotifyOnce(browserapi.LevelError, "a %d", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, mock.messages["a 0"])

		_, err = b.NotifyOnce(browserapi.LevelError, "a %d", 1)
		require.NoError(t, err)
		assert.Equal(t, 1, mock.messages["a 1"])
		assert.Equal(t, 1, mock.messages["a 0"])
	})

	t.Run("delivers notifications with different args only once if args are the same", func(t *testing.T) {
		t.Parallel()

		mock, b := newTestNotifications(uri, t)

		_, err := b.NotifyOnce(browserapi.LevelError, "a %d", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, mock.messages["a 0"])

		_, err = b.NotifyOnce(browserapi.LevelError, "a %d", 0)
		require.NoError(t, err)
		assert.Equal(t, 1, mock.messages["a 0"])
	})

	t.Run("delivers notifications multiple times if subsequent uses Notify rather than NotifyOnce", func(t *testing.T) {
		t.Parallel()

		mock, b := newTestNotifications(uri, t)

		_, err := b.NotifyOnce(browserapi.LevelError, "a")
		require.NoError(t, err)
		assert.Equal(t, 1, mock.messages["a"])

		b.Notify(browserapi.LevelError, "a")
		assert.Equal(t, 2, mock.messages["a"])

		_, err = b.NotifyOnce(browserapi.LevelError, "a")
		require.NoError(t, err)
		assert.Equal(t, 2, mock.messages["a"])
	})
}

type testNotifier struct {
	messages map[string]int
}

func newTestNotify() *testNotifier {
	ret := new(testNotifier)
	ret.messages = make(map[string]int)
	return ret
}

func (t *testNotifier) Notify(
	level notifications.Level, msg string,
) string {
	t.messages[msg] = t.messages[msg] + 1
	return t.ID(level, msg)
}

func (t *testNotifier) ID(
	level notifications.Level, msg string,
) string {
	h := fnv.New64a()
	h.Write([]byte(msg))
	return strconv.FormatUint(h.Sum64(), 10)
}

func (t *testNotifier) UpdateProgress(
	id, message string, progress, total int64,
) bool {
	return false
}

func (t *testNotifier) CloseAll() {
	clear(t.messages)
}

func (t *testNotifier) ResumeAll() {
}

func (t *testNotifier) PauseAll() {
	m := make(map[string]int)
	for k, v := range t.messages {
		m[k+"_paused"] = v
	}
	t.messages = m
}
