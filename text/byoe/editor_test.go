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

package byoe

import (
	"context"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/workspacetest"
)

// stubWorkspace wraps workspacetest.NopScheme to satisfy
// workspace.Workspace (which adds the Loader methods).
type stubWorkspace struct {
	*workspacetest.NopScheme
}

func newStubWorkspace() workspace.Workspace {
	return stubWorkspace{NopScheme: &workspacetest.NopScheme{}}
}

func (stubWorkspace) Load(
	workspaceapi.URI, *cell.Buffer, workspaceapi.URI, bool,
) (workspace.FlusherCloser, error) {
	return nil, nil
}

func (stubWorkspace) Recover(
	workspaceapi.URI, workspaceapi.URI, *cell.Buffer, bool,
) (workspace.FlusherCloser, error) {
	return nil, nil
}

func (stubWorkspace) Remove(string) error { return nil }

// newTestEditor builds a byoe.Editor with stub dependencies so other
// tests can exercise byoe-specific behaviour without restating the
// constructor's argument list.
func newTestEditor() *Editor {
	return New(
		"vim {file}",
		"",
		"<esc>:qa<enter>",
		func(fn func()) bool { fn(); return true },
		newStubWorkspace(),
		workspaceapi.URI{},
		stubNotifications{},
		stubPublisher{},
		stubTerminal{},
		stubExecutor{},
		stubTabManager{},
		vte.DefaultConfig(),
		stubReloader{},
		nil,
		true,
		nil, nil, nil,
	)
}

type stubPublisher struct{}

func (stubPublisher) PublishEvent(term.Event) error { return nil }

type stubNotifications struct{}

func (stubNotifications) Notify(browserapi.NotificationLevel, string, ...any) (string, error) {
	return "", nil
}
func (stubNotifications) NotifyOnce(browserapi.NotificationLevel, string, ...any) (string, error) {
	return "", nil
}
func (stubNotifications) UpdateNotificationProgress(string, string, int64, int64) error {
	return nil
}

type stubTerminal struct{}

func (stubTerminal) NewPty(context.Context) (workspaceapi.Pty, error) {
	return workspaceapi.Pty{}, nil
}
func (stubTerminal) SetPtySize(workspaceapi.Pty, int, int) error {
	return nil
}

type stubExecutor struct{}

func (stubExecutor) StartCommand(context.Context, workspaceapi.Cmd) (workspaceapi.Pid, error) {
	return 0, nil
}
func (stubExecutor) Signal(workspaceapi.Pid, syscall.Signal) error { return nil }
func (stubExecutor) Close() error                                  { return nil }

type stubTabManager struct{}

func (stubTabManager) Tab(workspaceapi.URI, rune, string, browserapi.Handler) (
	browserapi.Handler, error,
) {
	return nil, nil
}
func (stubTabManager) SetTabName(workspaceapi.URI, string, term.Attributes) error {
	return nil
}

// stubReloader satisfies byoe.Reloader for tests that only need a
// non-nil value to satisfy byoe.New's invariants.
type stubReloader struct{}

func (stubReloader) Reload(workspaceapi.URI) error { return nil }

var _ browser.EventPublisher = stubPublisher{}
var _ browserapi.Notifications = stubNotifications{}
var _ schemeapi.Terminal = stubTerminal{}
var _ schemeapi.Executor = stubExecutor{}
var _ browser.TabManager = stubTabManager{}

// TestEditorIsExternal verifies the byoe editor reports
// IsExternal()==true.
func TestEditorIsExternal(t *testing.T) {
	assert.True(t, newTestEditor().IsExternal())
}

// TestNewPanicsOnMissingArgument exercises every required-argument
// panic path so a misconfigured call site fails at construction
// rather than at first Edit. Each case clones the valid-argument set
// from goodArgs() and zeroes one dependency.
func TestNewPanicsOnMissingArgument(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*newArgs)
	}{
		{"no command", func(a *newArgs) { a.command = "" }},
		{"no schedule", func(a *newArgs) { a.schedule = nil }},
		{"no cwd", func(a *newArgs) { a.cwd = nil }},
		{"no notifications", func(a *newArgs) { a.notifications = nil }},
		{"no publisher", func(a *newArgs) { a.publisher = nil }},
		{"no terminal", func(a *newArgs) { a.terminal = nil }},
		{"no executor", func(a *newArgs) { a.executor = nil }},
		{"no tab manager", func(a *newArgs) { a.tabManager = nil }},
		{"no reloader", func(a *newArgs) { a.reloader = nil }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := goodArgs()
			tc.mutate(&a)
			assert.Panics(t, func() { a.call() })
		})
	}
}

// TestNewPanicsOnInvalidGotoTemplate verifies that a malformed goto
// template causes a constructor panic instead of a silent fallback.
// validateBYOE rewrites invalid templates back to modal mode before
// reaching New; a panic here indicates the IDE skipped validation.
func TestNewPanicsOnInvalidGotoTemplate(t *testing.T) {
	a := goodArgs()
	a.gotoTemplate = "<bogus-key>"
	assert.Panics(t, func() { a.call() })
}

func TestNewPanicsOnInvalidQuitTemplate(t *testing.T) {
	a := goodArgs()
	a.quit = "<bogus-key>"
	assert.Panics(t, func() { a.call() })
}

// newArgs collects byoe.New's positional arguments so test cases can
// clone a valid baseline and mutate one field at a time without
// repeating the full signature.
type newArgs struct {
	command       string
	gotoTemplate  string
	quit          string
	schedule      func(func()) bool
	cwd           workspace.Workspace
	uri           workspaceapi.URI
	notifications browserapi.Notifications
	publisher     browser.EventPublisher
	terminal      schemeapi.Terminal
	executor      schemeapi.Executor
	tabManager    browser.TabManager
	vteCfg        vte.Config
	reloader      Reloader
}

func goodArgs() newArgs {
	return newArgs{
		command:       "vim {file}",
		gotoTemplate:  "",
		quit:          "<esc>:qa<enter>",
		schedule:      func(fn func()) bool { fn(); return true },
		cwd:           newStubWorkspace(),
		uri:           workspaceapi.URI{},
		notifications: stubNotifications{},
		publisher:     stubPublisher{},
		terminal:      stubTerminal{},
		executor:      stubExecutor{},
		tabManager:    stubTabManager{},
		vteCfg:        vte.DefaultConfig(),
		reloader:      stubReloader{},
	}
}

func (a newArgs) call() *Editor {
	return New(
		a.command, a.gotoTemplate, a.quit, a.schedule,
		a.cwd, a.uri, a.notifications, a.publisher,
		a.terminal, a.executor, a.tabManager, a.vteCfg,
		a.reloader, nil, true,
		nil, nil, nil)
}

// TestPublisherFuncSurfaceClosedError verifies the PublisherFunc
// adapter returns a non-nil error when the underlying
// `func(term.Event) bool` reports the publisher is closed. The
// embedded vte uses this signal to drop events instead of looping.
func TestPublisherFuncSurfaceClosedError(t *testing.T) {
	closed := PublisherFunc(func(term.Event) bool { return false })
	open := PublisherFunc(func(term.Event) bool { return true })
	assert.Error(t, closed.PublishEvent(term.Event{}))
	assert.NoError(t, open.PublishEvent(term.Event{}))
}

// TestSubstituteCommand verifies the {file}/{line}/{col} placeholders
// are replaced in the argv template before shell tokenisation.
func TestSubstituteCommand(t *testing.T) {
	got := substituteCommand("vim +call cursor({line}, {col}) {file}",
		"/tmp/x.go", 10, 4)
	assert.Equal(t, "vim +call cursor(10, 4) /tmp/x.go", got)
}

// recordingReloader captures every URI it is asked to reload so
// scheduleReload tests can assert exactly what got routed into the
// IDE pipeline.
type recordingReloader struct {
	calls []workspaceapi.URI
	err   error
}

func (r *recordingReloader) Reload(uri workspaceapi.URI) error {
	r.calls = append(r.calls, uri)
	return r.err
}

// TestScheduleReloadRoutesThroughReloaderOnUIGoroutine verifies that
// the FS-watcher-driven reload is handed off to the Reloader, and that
// the hand-off goes through scheduleNextTick — the watcher goroutine
// must not call Reloader directly because Reload touches UI-owned tab
// state (open-tab map, FlusherCloser swap state, cell.Buffer
// subscribers).
func TestScheduleReloadRoutesThroughReloaderOnUIGoroutine(t *testing.T) {
	rel := &recordingReloader{}
	var scheduled []func()
	sched := func(fn func()) bool {
		scheduled = append(scheduled, fn)
		return true
	}
	uri, err := workspaceapi.ParseURI("file:///x")
	require.NoError(t, err)
	h := &editorHandler{
		resource:         uri,
		notifications:    stubNotifications{},
		scheduleNextTick: sched,
		reloader:         rel,
	}

	h.scheduleReload(uri)

	// Reloader must NOT have been called inline: the watcher
	// goroutine handed the work to the UI scheduler instead.
	require.Empty(t, rel.calls,
		"scheduleReload must defer to scheduleNextTick, not call Reload inline")
	require.Len(t, scheduled, 1)

	// Drain the scheduled callback to simulate the UI goroutine
	// picking it up.
	scheduled[0]()
	assert.Equal(t, []workspaceapi.URI{uri}, rel.calls,
		"scheduled callback must invoke Reloader with the watched URI")
}
