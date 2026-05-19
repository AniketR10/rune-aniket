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

package ide

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/workspace"
)

// flusherTarget is the subset of text.Component that the flusher
// depends on. It is an interface so tests can stub the underlying
// disk/RPC work without spinning up a real workspace.
type flusherTarget interface {
	FlushTab(ctx context.Context, h browserapi.Handler) (<-chan error, error)
	ForceFlushTab(ctx context.Context, h browserapi.Handler) (<-chan error, error)
	OverwriteTab(ctx context.Context, h browserapi.Handler) (<-chan error, error)
	ReloadTab(ctx context.Context, h browserapi.Handler) (<-chan error, error)
	// Resource returns the tab handler open for uri, or false if no
	// tab with that URI is open. byoe's Reloader implementation
	// uses this to translate the FS-watcher URI into the handler
	// the flusher's reloadAsync expects.
	Resource(uri workspaceapi.URI) (browserapi.Handler, bool)
}

// flusher coordinates async save / reload operations for an ex.
//
// Each public method starts work via the embedded
// text.Component, then tracks the awaiter goroutine so tests /
// shutdown can drain pending notifications. The underlying
// workspace.FlusherCloser (workspace/file.go) is the single
// source of truth for the per-URI "one async op at a time"
// invariant — it returns workspace.ErrFlushInProgress directly
// when a second op is started while one is in flight.
//
// Cancellation is intentionally absent: blocking file I/O cannot
// be aborted in a cross-platform way, and even if we plumbed ctx
// through the scheme layer the underlying os/fs syscall would
// still need to return before the file's `flushing` flag clears.
// The flusher therefore does not store cancel funcs.
type flusher struct {
	comp          flusherTarget
	notifications browserapi.Notifications
	sched         func(func()) bool

	// inflight counts awaiter goroutines that have not yet
	// dispatched their completion callback. Reads from :q / :wq
	// use this to refuse exit while saves are pending; tests use
	// `wait` to drain it.
	inflight atomic.Int64

	// wg tracks awaiter goroutines so tests / shutdown can drain
	// outstanding work.
	wg sync.WaitGroup
}

// flusherOp tags the kind of async op for notification formatting in
// onDone.
type flusherOp int

const (
	opSave flusherOp = iota
	opOverwrite
	opReload
)

func newFlusher(
	comp flusherTarget,
	notifications browserapi.Notifications,
	sched func(func()) bool,
) *flusher {
	if sched == nil {
		panic("ide.flusher: sched must not be nil")
	}
	return &flusher{
		comp:          comp,
		notifications: notifications,
		sched:         sched,
	}
}

// flush starts a non-force save for the tab at uri/h.
func (f *flusher) flush(
	uri workspaceapi.URI, h browserapi.Handler,
) error {
	return f.flushAndThen(uri, h, false, nil)
}

// forceFlush starts a force save (overwrites changes from other
// processes) for the tab at uri/h.
func (f *flusher) forceFlush(
	uri workspaceapi.URI, h browserapi.Handler,
) error {
	return f.flushAndThen(uri, h, true, nil)
}

// flushAndThen starts an async save and invokes onSuccess on the
// sched goroutine after a successful completion. :wq uses this hook
// to schedule editor exit only when the save succeeded.
func (f *flusher) flushAndThen(
	uri workspaceapi.URI, h browserapi.Handler,
	force bool, onSuccess func(),
) error {
	method := f.comp.FlushTab
	if force {
		method = f.comp.ForceFlushTab
	}
	return f.startAsync(uri, opSave, f.startWith(method, h), onSuccess)
}

// overwrite starts an async overwrite for the tab at uri/h. The
// resulting error (if any) is surfaced as an error notification.
func (f *flusher) overwrite(
	uri workspaceapi.URI, h browserapi.Handler,
) error {
	return f.startAsync(uri, opOverwrite, f.startWith(f.comp.OverwriteTab, h), nil)
}

// reloadAsync starts an async reload for the tab at uri/h. Used by
// prompt-driven reloads and filesystem-watcher reloads where the
// user shouldn't have to wait for the underlying scheme (or a
// scheduler-driven reparse) to respond before the host event loop
// makes progress.
func (f *flusher) reloadAsync(
	uri workspaceapi.URI, h browserapi.Handler, onSuccess func(),
) error {
	return f.startAsync(uri, opReload, f.startWith(f.comp.ReloadTab, h), onSuccess)
}

// Reload satisfies byoe.Reloader. The byoe handler's FS watcher
// calls this from the host UI goroutine (scheduled via
// scheduleNextTick) on every Write/Rename/Create/Remove event for
// the open file. We resolve uri to the open tab handler through the
// embedded text.Component, then hand off to the same reloadAsync
// pipeline :reloadfile uses — disk read, buffer reset, lastFlush
// advance, and dirty-tab clear all happen on the IDE's awaiter
// goroutine + UI scheduler.
//
// Returns nil when the reload was started successfully (the buffered
// completion channel is owned by reloadAsync's awaiter and surfaces
// errors as notifications). Returns workspace.ErrFlushInProgress
// when a previous reload is still in flight; callers can treat that
// as benign because the in-flight op will catch up the mirror.
// Returns workspaceapi.ErrInvalidReload when uri has no open tab —
// for example a Remove event arriving after the tab was closed.
func (f *flusher) Reload(uri workspaceapi.URI) error {
	h, ok := f.comp.Resource(uri)
	if !ok {
		return textapi.ErrInvalidReload
	}
	return f.reloadAsync(uri, h, nil)
}

// inFlightCount reports how many awaiter goroutines are still
// pending. Used by :q / :wq to refuse exit while saves are pending.
// Used by :q / :wq to refuse exit while saves are pending.
func (f *flusher) inFlightCount() int {
	return int(f.inflight.Load())
}

// wait blocks until every awaiter goroutine has delivered its
// completion callback through sched. Intended for tests and graceful
// shutdown.
func (f *flusher) wait() {
	f.wg.Wait()
}

// startWith adapts a (ctx, handler) -> chan method to the
// (ctx) -> chan signature startAsync expects, partially applying h.
func (f *flusher) startWith(
	m func(context.Context, browserapi.Handler) (<-chan error, error),
	h browserapi.Handler,
) func(context.Context) (<-chan error, error) {
	return func(ctx context.Context) (<-chan error, error) {
		return m(ctx, h)
	}
}

func (f *flusher) startAsync(
	uri workspaceapi.URI,
	kind flusherOp,
	start func(ctx context.Context) (<-chan error, error),
	onSuccess func(),
) error {
	// The underlying workspace.FlusherCloser is the single owner of
	// the per-URI "one in-flight op at a time" invariant; if a
	// second op is started while one is in flight, start() returns
	// workspace.ErrFlushInProgress directly.
	ch, err := start(context.Background())
	if err != nil {
		return err
	}
	f.inflight.Add(1)
	f.wg.Add(1)
	go debug.CapturePanicReport(func() {
		defer f.wg.Done()
		defer f.inflight.Add(-1)
		ferr := <-ch
		// sched must dispatch onto the UI goroutine so
		// notifications and onSuccess hooks happen on a single
		// thread.
		f.sched(func() { f.onDone(uri, kind, ferr, onSuccess) })
	})
	return nil
}

func (f *flusher) onDone(
	uri workspaceapi.URI, kind flusherOp, err error, onSuccess func(),
) {
	if err == nil {
		if onSuccess != nil {
			onSuccess()
		}
		return
	}
	name := uri.Name()
	switch kind {
	case opSave:
		switch {
		case errors.Is(err, workspaceapi.ErrStaleData),
			errors.Is(err, workspaceapi.ErrFileIsNotWritable):
			_, _ = f.notifications.Notify(browserapi.LevelWarn,
				"save '%s': %v", name, err)
		case errors.Is(err, workspace.ErrFlushInProgress):
			// Concurrent save kicked while one was in flight;
			// the in-flight op will surface its own result.
			return
		default:
			_, _ = f.notifications.Notify(browserapi.LevelError,
				"save '%s': %v", name, err)
		}
	case opOverwrite:
		if errors.Is(err, workspace.ErrFlushInProgress) {
			return
		}
		_, _ = f.notifications.Notify(browserapi.LevelError,
			"failed to overwrite tab: %v", err)
	case opReload:
		if errors.Is(err, workspace.ErrFlushInProgress) {
			return
		}
		_, _ = f.notifications.Notify(browserapi.LevelError,
			"failed to reload tab: %v", err)
	}
}
