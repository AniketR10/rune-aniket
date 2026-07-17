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

package vtereservoir

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser/browsertest"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/workspace/workspacetest"
)

func TestFacility(t *testing.T) {
	t.Parallel()
	t.Run("pre-allocates initial capacity", func(t *testing.T) {
		t.Parallel()
		var called atomic.Int64
		f := newTestFacility(5, func(f *Facility) (VTE, error) {
			called.Add(1)
			return newTestVte(f), nil
		})
		assert.Equal(t, 5, int(called.Load()))

		_, err := f.Get()
		require.NoError(t, err)

		assert.Equal(t, 5, int(called.Load()))
	})

	t.Run("pre-allocates if pool is empty", func(t *testing.T) {
		t.Parallel()
		var called atomic.Int64
		f := newTestFacility(1, func(f *Facility) (VTE, error) {
			called.Add(1)
			return newTestVte(f), nil
		})
		assert.Equal(t, 1, int(called.Load()))

		_, err := f.Get()
		require.NoError(t, err)

		assert.Equal(t, 2, int(called.Load()))
	})

	t.Run("refill failure does not cost the caller the warm VTE", func(t *testing.T) {
		t.Parallel()
		var called atomic.Int64
		f := newTestFacility(2, func(f *Facility) (VTE, error) {
			// Warm-up (calls 1-2) succeeds; the opportunistic
			// refill triggered by draining the pool (call 3) fails.
			if called.Add(1) > 2 {
				return nil, errors.New("stalled transport")
			}
			return newTestVte(f), nil
		})

		_, err := f.Get()
		require.NoError(t, err)

		// Drains the pool: the refill fails, but the caller must
		// still receive the warm VTE it already had.
		v, err := f.Get()
		require.NoError(t, err)
		require.NotNil(t, v)
		assert.Equal(t, 3, int(called.Load()))
	})

	t.Run("facility.Close closes all free vtes", func(t *testing.T) {
		t.Parallel()
		var mu sync.Mutex
		var created []*testVte
		f := newTestFacility(5, func(f *Facility) (VTE, error) {
			tvte := newTestVte(f)
			mu.Lock()
			defer mu.Unlock()
			created = append(created, tvte)
			return tvte, nil
		})

		// free and returned
		vte1, err := f.Get()
		require.NoError(t, err)

		vte2, err := f.Get()
		require.NoError(t, err)

		require.NoError(t, vte1.Close())
		require.NoError(t, f.Close())
		require.NoError(t, vte2.Close())

		for _, vte := range created {
			assert.True(t, vte.calledClose)
		}
	})

	t.Run("VTE.Close puts vte back into the pool", func(t *testing.T) {
		t.Parallel()
		var called atomic.Int32
		f := newTestFacility(2, func(f *Facility) (VTE, error) {
			tvte := newTestVte(f)
			called.Add(1)
			return tvte, nil
		})

		vte, err := f.Get()
		require.NoError(t, err)

		require.NoError(t, vte.Close())

		vte, err = f.Get()
		require.NoError(t, err)

		assert.Equal(t, 2, int(called.Load()))
	})

	t.Run("VTE.Close does not put vte back into the pool, if used alternate buffer", func(t *testing.T) {
		t.Parallel()
		var called atomic.Int32
		f := newTestFacility(2, func(f *Facility) (VTE, error) {
			tvte := newTestVte(f)
			tvte.usedAlt = true
			called.Add(1)
			return tvte, nil
		})

		vte, err := f.Get()
		require.NoError(t, err)

		require.NoError(t, vte.Close())

		vte, err = f.Get()
		require.NoError(t, err)

		assert.Equal(t, 3, int(called.Load()))
	})

	t.Run("VTE.Close clears the primary buffer when put back into pool", func(t *testing.T) {
		t.Parallel()
		var tvte *testVte
		var c atomic.Int32
		var mu sync.Mutex
		f := newTestFacility(1, func(f *Facility) (VTE, error) {
			if c.CompareAndSwap(0, 1) {
				mu.Lock()
				defer mu.Unlock()
				tvte = newTestVte(f)
				return tvte, nil
			}
			return newTestVte(f), nil
		})

		vte, err := f.Get()
		require.NoError(t, err)

		require.NoError(t, vte.Close())

		mu.Lock()
		defer mu.Unlock()

		require.NotNil(t, 2, tvte)
		assert.True(t, tvte.clearedPrimary)
	})

	t.Run("Resize after new doesn't block waiting for all vtes to be created", func(t *testing.T) {
		t.Parallel()
		b := nopBrowser{}
		var wg sync.WaitGroup
		const capacity = 5
		wg.Add(capacity)
		uri, err := workspaceapi.ParseURI("file:///tmp")
		require.NoError(t, err)
		scheme, err := workspacetest.NewNopScheme("file:///tmp")(
			context.Background(), config.NopConfig(), uri)
		scheme.(*workspacetest.NopScheme).NewPtyFunc = func(ctx context.Context) (workspaceapi.Pty, error) {
			defer wg.Done()
			return workspaceapi.Pty{
				Master: scheme.NewFile(0, ""),
				Slave:  scheme.NewFile(1, ""),
			}, nil
		}
		scheme.(*workspacetest.NopScheme).StartCommandFunc = func(context.Context, workspaceapi.Cmd) (workspaceapi.Pid, error) {
			time.Sleep(100 * time.Minute)
			return 0, nil
		}
		require.NoError(t, err)
		f := New(b, b, scheme, scheme, b, vte.DefaultConfig(), capacity)
		// this would block for 100 minutes if not implemented correctly
		f.Resize(10, 10)
		wg.Wait()
	})

	t.Run("put does not resurrect the pool after Close", func(t *testing.T) {
		t.Parallel()
		f := newTestFacility(1, func(f *Facility) (VTE, error) {
			return newTestVte(f), nil
		})

		require.NoError(t, f.Close())

		// Simulate an async vteAdapter.Close() callback that fires
		// after the facility has been closed. It must NOT re-populate
		// the pool (which would leak the underlying VTE and its
		// associated pty + scrollback memory).
		stray := newTestVte(f)
		assert.False(t, f.put(stray))
		assert.Equal(t, 0, f.Capacity())
	})

	t.Run("Reset drains the warm pool and lets Get re-allocate", func(t *testing.T) {
		t.Parallel()
		// Track every VTE the factory builds so we can assert that
		// the ones warm at Reset time were closed.
		var mu sync.Mutex
		var all []*testVte
		f := newTestFacility(2, func(f *Facility) (VTE, error) {
			v := newTestVte(f)
			mu.Lock()
			all = append(all, v)
			mu.Unlock()
			return v, nil
		})
		require.Equal(t, 2, f.Capacity())

		mu.Lock()
		preReset := append([]*testVte(nil), all...)
		mu.Unlock()

		require.NoError(t, f.Reset())

		assert.Equal(t, 0, f.Capacity(),
			"Reset must empty the pool so the next Get re-allocates")
		for _, v := range preReset {
			assert.True(t, v.calledClose,
				"Reset must Close every warm VTE so it is not "+
					"handed out bound to the dead transport")
		}

		v, err := f.Get()
		require.NoError(t, err)
		require.NotNil(t, v)
	})

	t.Run("Reset is a no-op after Close", func(t *testing.T) {
		t.Parallel()
		f := newTestFacility(1, func(f *Facility) (VTE, error) {
			return newTestVte(f), nil
		})
		require.NoError(t, f.Close())
		require.NoError(t, f.Reset())
	})

	t.Run("remote scheme OnDisconnect drains the pool and exits", func(t *testing.T) {
		t.Parallel()
		// Drive watchRemoteScheme directly with an in-memory fake
		// to keep the test independent of the workspacessh package.
		// Use recordingVTE rather than testVte so Close does not
		// peek at f.pool without locking (which would race against
		// the concurrent Reset below).
		f := newTestFacility(1, func(f *Facility) (VTE, error) {
			return newRecordingVTE(), nil
		})
		f.ctx, f.cancelCtx = context.WithCancel(context.Background())
		require.Equal(t, 1, f.Capacity())

		rs := newFakeRemoteScheme()
		watcherDone := make(chan struct{})
		go func() {
			defer close(watcherDone)
			f.watchRemoteScheme(rs.OnDisconnect())
		}()

		rs.broadcastDisconnect()

		// One-shot contract: after the single disconnect signal,
		// the watcher resets the pool and exits. Re-arming would
		// busy-loop on a permanently-closed channel.
		select {
		case <-watcherDone:
		case <-time.After(2 * time.Second):
			t.Fatal("watchRemoteScheme must exit after the " +
				"one-shot disconnect signal fires")
		}
		require.Equal(t, 0, f.Capacity(),
			"watchRemoteScheme must Reset the pool when "+
				"OnDisconnect fires")

		require.NoError(t, f.Close())
	})

	t.Run("watchRemoteScheme exits when facility ctx is cancelled", func(t *testing.T) {
		t.Parallel()
		f := newTestFacility(1, func(f *Facility) (VTE, error) {
			return newRecordingVTE(), nil
		})
		f.ctx, f.cancelCtx = context.WithCancel(context.Background())

		rs := newFakeRemoteScheme()
		done := make(chan struct{})
		go func() {
			defer close(done)
			f.watchRemoteScheme(rs.OnDisconnect())
		}()

		// No disconnect: the watcher must still exit when the
		// facility shuts down, otherwise it leaks past Close.
		f.cancelCtx()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("watchRemoteScheme must exit when the facility " +
				"context is cancelled")
		}
		require.NoError(t, f.Close())
	})

	t.Run("Get is served from the pool when warm", func(t *testing.T) {
		t.Parallel()
		var called atomic.Int64
		f := newTestFacility(2, func(f *Facility) (VTE, error) {
			called.Add(1)
			return newTestVte(f), nil
		})
		// initCap synchronously fires off two news.
		assert.Equal(t, int64(2), called.Load())

		// First Get: pool has 2, take one, pool now has 1 (no
		// refill because len(pool) > 0 after pop).
		_, err := f.Get()
		require.NoError(t, err)
		assert.Equal(t, int64(2), called.Load())

		// Second Get: pool has 1, take it, pool empty so refill -> 1 new call.
		_, err = f.Get()
		require.NoError(t, err)
		assert.Equal(t, int64(3), called.Load())
	})

	t.Run("Get blocks until initCap delivers a pre-warmed VTE", func(t *testing.T) {
		t.Parallel()
		// Use New(...) so that initCap runs asynchronously - this is
		// the exact race that defeats the warm pool in production.
		release := make(chan struct{})
		var newCalls atomic.Int64
		b := nopBrowser{}
		uri, err := workspaceapi.ParseURI("file:///tmp")
		require.NoError(t, err)
		scheme, err := workspacetest.NewNopScheme("file:///tmp")(
			context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)
		nop := scheme.(*workspacetest.NopScheme)
		nop.NewPtyFunc = func(ctx context.Context) (workspaceapi.Pty, error) {
			// Block initCap until the test releases it.
			<-release
			newCalls.Add(1)
			return workspaceapi.Pty{
				Master: scheme.NewFile(0, ""),
				Slave:  scheme.NewFile(1, ""),
			}, nil
		}
		nop.StartCommandFunc = func(context.Context, workspaceapi.Cmd) (workspaceapi.Pid, error) {
			return 0, nil
		}
		f := New(b, b, scheme, scheme, b, vte.DefaultConfig(), 2)
		t.Cleanup(func() { _ = f.Close() })

		// Issue a Get before initCap can deliver any VTE. It must not
		// fall through to the from-scratch path; it must wait for the
		// pool to be populated.
		got := make(chan VTE, 1)
		errCh := make(chan error, 1)
		go func() {
			v, err := f.Get()
			if err != nil {
				errCh <- err
				return
			}
			got <- v
		}()

		// Give the goroutine a chance to enter Get and observe the
		// empty pool. With the bug, Get would immediately call
		// f.new(false) and return.
		select {
		case <-got:
			t.Fatal("Get returned before initCap delivered a VTE")
		case err := <-errCh:
			t.Fatalf("Get returned an unexpected error: %v", err)
		case <-time.After(50 * time.Millisecond):
		}

		// Release initCap so it can populate the pool.
		close(release)

		select {
		case <-got:
		case err := <-errCh:
			t.Fatalf("Get returned an unexpected error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Get blocked indefinitely waiting for initCap")
		}
	})

	t.Run("initCap does not resurrect a closed pool", func(t *testing.T) {
		t.Parallel()
		release := make(chan struct{})
		f := new(Facility)
		f.cond = sync.NewCond(&f.mu)
		f.initialCapacity = 3
		f.pendingInit = 3
		f.new = func(bool) (VTE, error) {
			<-release
			return newTestVte(f), nil
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			f.initCap(3)
		}()

		// Close while initCap is mid-flight.
		require.NoError(t, f.Close())
		close(release)
		<-done

		// The pool must remain empty - initCap must not re-populate
		// it after Close.
		assert.Equal(t, 0, f.Capacity())
	})

	// Pins the regression where a stalled remote workspace transport
	// blocked the warm-up's NewPty forever: pendingInit never drained,
	// so Get (called on the host event loop during session restore)
	// waited on the cond indefinitely and froze the UI. With the
	// SpawnTimeout plumbed through New's factory, both the warm-up
	// and Get's fallback allocation fail after the bound.
	t.Run("stalled spawn does not wedge Get indefinitely", func(t *testing.T) {
		t.Parallel()
		b := nopBrowser{}
		uri, err := workspaceapi.ParseURI("file:///tmp")
		require.NoError(t, err)
		scheme, err := workspacetest.NewNopScheme("file:///tmp")(
			context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)
		nop := scheme.(*workspacetest.NopScheme)
		nop.NewPtyFunc = func(ctx context.Context) (workspaceapi.Pty, error) {
			// Simulate a wedged transport: the RPC never returns
			// until its context is cancelled.
			<-ctx.Done()
			return workspaceapi.Pty{}, ctx.Err()
		}
		cfg := vte.DefaultConfig()
		cfg.SpawnTimeout = 20 * time.Millisecond
		f := New(b, b, scheme, scheme, b, cfg, 2)
		t.Cleanup(func() { _ = f.Close() })

		errCh := make(chan error, 1)
		go func() {
			_, err := f.Get()
			errCh <- err
		}()

		select {
		case err := <-errCh:
			require.Error(t, err,
				"Get must surface the spawn failure, not hand out a VTE")
		case <-time.After(10 * time.Second):
			t.Fatal("Get wedged behind a stalled spawn; SpawnTimeout " +
				"must bound the warm-up and the fallback allocation")
		}
	})

	// First-connect provisioning is unbounded (it may install
	// packages on the remote host) and remote scheme calls block
	// until the first connection attempt settles. The spawn budget
	// must not race that wait: a spurious warm-up failure would
	// fail session restore on freshly provisioned workspaces right
	// as they become usable.
	t.Run("spawn budget starts only after the remote transport resolves", func(t *testing.T) {
		t.Parallel()
		b := nopBrowser{}
		uri, err := workspaceapi.ParseURI("file:///tmp")
		require.NoError(t, err)
		scheme, err := workspacetest.NewNopScheme("file:///tmp")(
			context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)
		nop := scheme.(*workspacetest.NopScheme)
		var ptyCalls atomic.Int64
		nop.NewPtyFunc = func(ctx context.Context) (workspaceapi.Pty, error) {
			ptyCalls.Add(1)
			return workspaceapi.Pty{
				Master: scheme.NewFile(0, ""),
				Slave:  scheme.NewFile(1, ""),
			}, nil
		}
		nop.StartCommandFunc = func(context.Context, workspaceapi.Cmd) (workspaceapi.Pid, error) {
			return 0, nil
		}
		gated := &connectGatedTerminal{
			Terminal:     nop,
			connected:    make(chan struct{}),
			disconnectCh: make(chan struct{}),
		}
		cfg := vte.DefaultConfig()
		cfg.SpawnTimeout = 20 * time.Millisecond
		f := New(b, b, gated, scheme, b, cfg, 1)
		t.Cleanup(func() { _ = f.Close() })

		// Let several spawn-timeout windows elapse while the
		// transport is still "provisioning".
		time.Sleep(5 * cfg.SpawnTimeout)
		assert.EqualValues(t, 0, ptyCalls.Load(),
			"no spawn RPC may be issued (and no budget consumed) "+
				"while the transport is unresolved")

		close(gated.connected)
		f.WaitForInitialFill()

		v, err := f.Get()
		require.NoError(t, err,
			"Get must succeed once the transport resolves; the "+
				"connect wait must not have failed the warm-up")
		require.NotNil(t, v)
	})
}

type nopBrowser struct {
}

func (n nopBrowser) PublishEvent(term.Event) error {
	return nil
}
func (n nopBrowser) Notify(level browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	return "", nil
}
func (n nopBrowser) NotifyOnce(level browserapi.NotificationLevel, msg string, args ...any) (string, error) {
	return "", nil
}
func (n nopBrowser) UpdateNotificationProgress(id, message string, progress, total int64) error {
	return nil
}
func (nopBrowser) Tab(uri workspaceapi.URI, icon rune, name string, h browserapi.Handler) (
	browserapi.Handler, error,
) {
	return browsertest.NewTestHandler(), nil
}

func (n nopBrowser) SetTabName(workspaceapi.URI, string, term.Attributes) error {
	return nil
}

func newTestFacility(
	initCap int, newFn func(*Facility) (VTE, error),
) *Facility {
	// Build the facility via the same internal entrypoint
	// production [New] uses, injecting the test factory so
	// every test exercises the real lifecycle wiring (cond,
	// ctx/cancelCtx, pendingInit, remote-scheme watcher) — no
	// hand-rolled construction or private-field poking.
	f := newWithFactory(
		func(f *Facility, _ bool) (VTE, error) { return newFn(f) },
		nopTerminal{}, initCap)
	// initCap runs asynchronously in production; wait for it to
	// settle so callers observing Capacity / called counters
	// immediately after construction see a fully resolved pool,
	// mirroring the synchronous semantics the previous helper
	// provided.
	f.WaitForInitialFill()
	return f
}

// nopTerminal satisfies the schemeapi.Terminal slot required by
// [newWithFactory]. The test factory replaces vte.NewHandler so
// these methods are never actually called; the only reason they
// exist is to type-check.
type nopTerminal struct{}

func (nopTerminal) NewPty(context.Context) (workspaceapi.Pty, error) {
	return workspaceapi.Pty{}, nil
}
func (nopTerminal) SetPtySize(workspaceapi.Pty, int, int) error { return nil }

var _ VTE = (*testVte)(nil)

type testVte struct {
	component.String
	initialCmd     string
	f              *Facility
	usedAlt        bool
	clearedPrimary bool

	calledClose bool
}

func newTestVte(f *Facility) *testVte {
	return newTestVteWithConfig(f, "")
}

func newTestVteWithConfig(f *Facility, initialCmd string) *testVte {
	ret := new(testVte)
	ret.initialCmd = initialCmd
	ret.String = component.NewString(initialCmd)
	ret.f = f
	return ret
}

func (t *testVte) Handle(ev term.Event) (bool, bool) {
	return false, false
}

func (t *testVte) SeekUp() bool {
	return false
}

func (t *testVte) SeekDown() bool {
	return false
}

func (t *testVte) SeekOffset() int {
	return 0
}

func (t *testVte) MaxSeekOffset() int {
	return 0
}

func (t *testVte) Cursor() (ret term.Coordinates, style term.CursorStyle, show bool) {
	show = true
	ret = term.Coordinates{X: len(t.initialCmd)}
	return
}

func (t *testVte) Selection() (string, bool) {
	return "", false
}

func (t *testVte) Dimensions() (int, int) {
	return 0, 0
}

func (t *testVte) Close() error {
	if t.calledClose {
		return errors.New("called close twice")
	}
	if t.f.pool == nil || t.UsedAlternateBuffer() {
		t.calledClose = true
		return nil
	}
	t.f.put(t)
	return nil
}

func (t *testVte) OnFocusChange(inFocus bool) {
}

func (t *testVte) SetDefaultAttributes(attr term.Attributes) {
}

func (t *testVte) Snapshot() (vte.Snapshot, error) {
	return vte.Snapshot{}, nil
}

func (t *testVte) RestoreFromSnapshot(vte.Snapshot) error {
	return nil
}

func (t *testVte) IsComplete() bool {
	return false
}

func (t *testVte) URI() workspaceapi.URI {
	return workspaceapi.URI{}
}

func (t *testVte) Title() string {
	return t.initialCmd
}

func (v *testVte) UsedAlternateBuffer() bool {
	return v.usedAlt
}

func (v *testVte) ClearPrimaryBuffer() bool {
	v.clearedPrimary = true
	return true
}

// fakeRemoteScheme mirrors workspace.RemoteScheme for tests that
// exercise Facility.watchRemoteScheme without spinning up a real
// SSH transport. The disconnect channel is allocated once at
// construction and closed once by broadcastDisconnect — matching
// the production contract (see workspace.RemoteScheme).
type fakeRemoteScheme struct {
	ch              chan struct{}
	closeOnce       sync.Once
	nilOnDisconnect bool
}

func newFakeRemoteScheme() *fakeRemoteScheme {
	return &fakeRemoteScheme{ch: make(chan struct{})}
}

func (r *fakeRemoteScheme) OnDisconnect() <-chan struct{} {
	if r.nilOnDisconnect {
		return nil
	}
	return r.ch
}

func (r *fakeRemoteScheme) WaitConnected(context.Context) error {
	return nil
}

// connectGatedTerminal simulates a remote scheme whose transport is
// still being established (e.g. first-connect provisioning):
// WaitConnected blocks until the test closes connected. The embedded
// Terminal (a NopScheme) serves the pty RPCs once resolved.
type connectGatedTerminal struct {
	schemeapi.Terminal
	connected    chan struct{}
	disconnectCh chan struct{}
}

func (g *connectGatedTerminal) OnDisconnect() <-chan struct{} {
	return g.disconnectCh
}

func (g *connectGatedTerminal) WaitConnected(ctx context.Context) error {
	select {
	case <-g.connected:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *fakeRemoteScheme) broadcastDisconnect() {
	r.closeOnce.Do(func() { close(r.ch) })
}

// recordingVTE is a minimal VTE used by the remote-scheme tests.
// Unlike testVte it does not peek at Facility internals from
// Close, so it is safe to drive across goroutines while another
// goroutine mutates the pool.
type recordingVTE struct {
	component.String
	closed atomic.Bool
}

func newRecordingVTE() *recordingVTE {
	r := new(recordingVTE)
	r.String = component.NewString("")
	return r
}

func (r *recordingVTE) Handle(term.Event) (bool, bool) { return false, false }
func (r *recordingVTE) SeekUp() bool                   { return false }
func (r *recordingVTE) SeekDown() bool                 { return false }
func (r *recordingVTE) SeekOffset() int                { return 0 }
func (r *recordingVTE) MaxSeekOffset() int             { return 0 }
func (r *recordingVTE) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}
func (r *recordingVTE) Selection() (string, bool)              { return "", false }
func (r *recordingVTE) Dimensions() (int, int)                 { return 0, 0 }
func (r *recordingVTE) OnFocusChange(bool)                     {}
func (r *recordingVTE) SetDefaultAttributes(term.Attributes)   {}
func (r *recordingVTE) Snapshot() (vte.Snapshot, error)        { return vte.Snapshot{}, nil }
func (r *recordingVTE) RestoreFromSnapshot(vte.Snapshot) error { return nil }
func (r *recordingVTE) IsComplete() bool                       { return false }
func (r *recordingVTE) URI() workspaceapi.URI                  { return workspaceapi.URI{} }
func (r *recordingVTE) Title() string                          { return "" }
func (r *recordingVTE) UsedAlternateBuffer() bool              { return false }
func (r *recordingVTE) ClearPrimaryBuffer() bool               { return true }
func (r *recordingVTE) Close() error                           { r.closed.Store(true); return nil }
