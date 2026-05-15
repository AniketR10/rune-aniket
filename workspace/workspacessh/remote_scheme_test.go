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

package workspacessh

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/retry"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"go.uber.org/mock/gomock"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/schemetest"
	"unstable.build/go-tui/workspace/workspaceapitest"
)

var _ workspace.RemoteScheme = (*remoteScheme)(nil)

func TestRemoteScheme(t *testing.T) {
	uri, err := workspaceapi.ParseURI("ssh://unsable.build/home/ernie")
	require.NoError(t, err)

	t.Run("establish initial connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var mu sync.Mutex
		ctx := context.Background()

		mock := schemetest.NewMockScheme(ctrl)
		mu.Lock()
		scheme := newRemoteScheme(ctx,
			func(_ context.Context, uri workspaceapi.URI, closehook func(error)) (schemeapi.Scheme, error) {
				return mock, nil
			}, uri)
		mu.Unlock()

		expectSchemeAPISuccess(t, ctrl, &mu, mock, scheme)
		expectSchemeClose(t, mock, scheme)
	})

	t.Run("eventually succeed upon initial connect errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var mu sync.Mutex
		ctx := context.Background()

		mock := schemetest.NewMockScheme(ctrl)
		var i int

		mu.Lock()
		scheme := newRemoteScheme(ctx, func(
			_ context.Context, uri workspaceapi.URI, closehook func(error),
		) (schemeapi.Scheme, error) {
			i++
			if i <= 5 {
				return nil, errors.New("unable to connect")
			}
			return mock, nil
		}, uri)
		mu.Unlock()

		mock.EXPECT().StartCommand(gomock.Any(), gomock.Any()).Return(workspaceapi.Pid(0), nil).Times(1)
		retry.Retry(context.Background(), retry.ExponentialStrategy(1*time.Millisecond, 10*time.Millisecond), func(context.Context) (bool, error) {
			mu.Lock()
			defer mu.Unlock()

			_, err = scheme.StartCommand(context.Background(),
				workspaceapi.Cmd{Path: "blah"})
			return true, err
		})
		require.NoError(t, err)
		expectSchemeClose(t, mock, scheme)
	})

	t.Run("re-establish connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var mu sync.Mutex
		ctx := context.Background()

		mock := schemetest.NewMockScheme(ctrl)
		var closeHook func(error)
		var wg sync.WaitGroup
		var reconnect bool
		mu.Lock()
		scheme := newRemoteScheme(ctx, func(
			_ context.Context, uri workspaceapi.URI, _closehook func(error),
		) (schemeapi.Scheme, error) {
			closeHook = _closehook
			if reconnect {
				wg.Done()
			}
			return mock, nil
		}, uri)
		mu.Unlock()
		expectSchemeAPISuccess(t, ctrl, &mu, mock, scheme)

		wg.Add(1)
		reconnect = true
		mock.EXPECT().Close().Return(errors.New("already closed but should be fine"))
		closeHook(errors.New("kaboom"))
		wg.Wait()
		expectSchemeAPISuccess(t, ctrl, &mu, mock, scheme)

		expectSchemeClose(t, mock, scheme)
	})

	t.Run("NewFile on un-opened file returns nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var mu sync.Mutex
		ctx := context.Background()

		mock := schemetest.NewMockScheme(ctrl)
		mu.Lock()
		scheme := newRemoteScheme(ctx,
			func(_ context.Context, uri workspaceapi.URI, closehook func(error)) (schemeapi.Scheme, error) {
				return mock, nil
			}, uri)
		mu.Unlock()

		expectSchemeAPISuccess(t, ctrl, &mu, mock, scheme)
		expectSchemeClose(t, mock, scheme)
		f := scheme.NewFile(1299, "blabla")
		require.Nil(t, f)
	})

	// Reproducer: closeHook(nil) — which fires when the remote
	// process exits cleanly or s.ctx is cancelled before any
	// transport error — used to store (scheme=nil, err=nil) into
	// currState. state() then returned (nil, nil) and the next
	// scheme call (e.g. StopWatch from the FS-watcher defer in
	// ide.installPendingWorkspace) nil-derefed the embedded
	// schemeapi.Scheme.
	t.Run("closeHook(nil) does not poison currState", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		mock := schemetest.NewMockScheme(ctrl)
		hookCh := make(chan func(error), 1)
		scheme := newRemoteScheme(ctx, func(
			_ context.Context, _ workspaceapi.URI, hook func(error),
		) (schemeapi.Scheme, error) {
			select {
			case hookCh <- hook:
			default:
			}
			return mock, nil
		}, uri)
		t.Cleanup(func() {
			mock.EXPECT().Close().Return(nil).AnyTimes()
			_ = scheme.Close()
		})

		// Wait for connect to land.
		mock.EXPECT().Close().Return(nil).AnyTimes()
		closeHook := <-hookCh

		// Fire the close hook with a nil error (mirrors a clean
		// remote process exit). State must still surface a typed
		// disconnect error so callers short-circuit instead of
		// dereferencing a nil scheme.
		closeHook(nil)

		// StopWatch is the call site that crashed in production
		// (see ide.installPendingWorkspace FS-watcher defer). Any
		// other scheme method exercises the same state() path.
		err := scheme.StopWatch(42)
		require.Error(t, err,
			"after a closeHook(nil) disconnect, scheme methods must "+
				"return a typed error rather than nil-deref the "+
				"underlying scheme")
		require.ErrorIs(t, err, ErrLostConnection)
	})

	t.Run("OnDisconnect is a stable channel closed once on transport drop", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		var mu sync.Mutex
		ctx := context.Background()

		mock := schemetest.NewMockScheme(ctrl)
		hookCh := make(chan func(error), 1)
		mu.Lock()
		scheme := newRemoteScheme(ctx, func(
			_ context.Context, uri workspaceapi.URI, hook func(error),
		) (schemeapi.Scheme, error) {
			select {
			case hookCh <- hook:
			default:
			}
			return mock, nil
		}, uri)
		mu.Unlock()

		rs, ok := scheme.(*remoteScheme)
		require.True(t, ok)

		// OnDisconnect must hand out the same stable channel
		// across calls so observers can subscribe before any
		// transport drop without coordinating with each other.
		ch := rs.OnDisconnect()
		require.NotNil(t, ch)
		require.Equal(t, ch, rs.OnDisconnect(),
			"OnDisconnect must return a stable channel: "+
				"creating a fresh one per call would force "+
				"callers to coordinate or miss signals")

		// The channel closes on the first transport drop.
		// Semantic: every FD this scheme handed out so far is
		// invalid; observers drop their caches.
		mock.EXPECT().Close().Return(nil).AnyTimes()
		closeHook := <-hookCh
		closeHook(errors.New("kaboom"))

		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatal("OnDisconnect channel must close on transport drop")
		}

		// After the drop the same channel is still returned so
		// late subscribers wake up immediately, and the runtime
		// does not need to track per-observer state.
		require.Equal(t, ch, rs.OnDisconnect(),
			"OnDisconnect must keep returning the same "+
				"(now-closed) channel after a drop")

		// A second close hook firing on the same remoteScheme
		// (e.g. SSH transport flapping) must not panic via
		// double-close of the disconnect channel.
		closeHook(errors.New("kaboom again"))

		require.NoError(t, scheme.Close())
	})

	t.Run("Close fires OnDisconnect for waiting observers", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ctx := context.Background()
		mock := schemetest.NewMockScheme(ctrl)
		scheme := newRemoteScheme(ctx, func(
			_ context.Context, _ workspaceapi.URI, _ func(error),
		) (schemeapi.Scheme, error) {
			return mock, nil
		}, uri)
		mock.EXPECT().Close().Return(nil).AnyTimes()

		rs, ok := scheme.(*remoteScheme)
		require.True(t, ok)
		ch := rs.OnDisconnect()

		// Closing the scheme must wake observers that were
		// blocked on OnDisconnect; otherwise watcher goroutines
		// would leak past shutdown.
		require.NoError(t, scheme.Close())
		select {
		case <-ch:
		case <-time.After(2 * time.Second):
			t.Fatal("OnDisconnect channel must close on Close")
		}
	})
}

func expectSchemeAPISuccess(
	t *testing.T, ctrl *gomock.Controller, mu *sync.Mutex,
	mock *schemetest.MockScheme, scheme schemeapi.Scheme,
) {
	mu.Lock()
	defer mu.Unlock()

	mock.EXPECT().StartCommand(gomock.Any(), gomock.Any()).Return(workspaceapi.Pid(0), nil).Times(1)
	_, err := scheme.StartCommand(context.Background(), workspaceapi.Cmd{})
	require.NoError(t, err)

	mock.EXPECT().Signal(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	err = scheme.Signal(0, 0)
	require.NoError(t, err)

	f := workspaceapitest.NewMockFile(ctrl)
	f.EXPECT().Fd().AnyTimes()
	f.EXPECT().Name().AnyTimes()
	mock.EXPECT().OpenFile(gomock.Any(), gomock.Any(), gomock.Any()).Return(f, nil).Times(1)
	_, osErr := scheme.OpenFile("", 0, 0)
	require.Nil(t, osErr)

	mock.EXPECT().Remove(gomock.Any()).Return(nil).Times(1)
	err = scheme.Remove("")
	require.NoError(t, err)

	mock.EXPECT().Rename(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	err = scheme.Rename("", "")
	require.NoError(t, err)

	mock.EXPECT().Stat(gomock.Any()).Return(nil, nil).Times(1)
	_, err = scheme.Stat("")
	require.NoError(t, err)

	mock.EXPECT().Lstat(gomock.Any()).Return(nil, nil).Times(1)
	_, err = scheme.Lstat("")
	require.NoError(t, err)

	mock.EXPECT().Readlink(gomock.Any()).Return("", nil).Times(1)
	_, err = scheme.Readlink("")
	require.NoError(t, err)

	mock.EXPECT().StartCommand(gomock.Any(), gomock.Any()).
		Return(workspaceapi.Pid(0), nil).Times(1)
	_, err = scheme.StartCommand(context.Background(),
		workspaceapi.Cmd{Path: "blah"})
	require.NoError(t, err)

	mock.EXPECT().ReadDir(gomock.Any()).Return([]os.DirEntry{}, nil).Times(1)
	_, err = scheme.ReadDir("")
	require.NoError(t, err)
}

func expectSchemeClose(t *testing.T, mock *schemetest.MockScheme, scheme schemeapi.Scheme) {
	mock.EXPECT().Close().Return(nil)
	err := scheme.Close()
	require.NoError(t, err)
}
