package ssh

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/blue/retry"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace"
	workspacetest "unstable.build/go-tui/workspace/test"
)

func expectSchemeAPISuccess(t *testing.T, mu *sync.Mutex, mock *workspacetest.MockScheme, scheme workspace.Scheme) {
	mu.Lock()
	defer mu.Unlock()

	mock.EXPECT().StartCommand(gomock.Any(), gomock.Any()).Return(workspaceapi.Pid(0), nil).Times(1)
	_, err := scheme.StartCommand(context.Background(), workspaceapi.Cmd{})
	require.NoError(t, err)

	mock.EXPECT().Signal(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	err = scheme.Signal(0, 0)
	require.NoError(t, err)

	mock.EXPECT().Open(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
	_, osErr := scheme.Open("", 0, 0)
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

	mock.EXPECT().ReadLink(gomock.Any()).Return("", nil).Times(1)
	_, err = scheme.ReadLink("")
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

func expectSchemeClose(t *testing.T, mock *workspacetest.MockScheme, scheme workspace.Scheme) {
	mock.EXPECT().Close().Return(nil)
	err := scheme.Close()
	require.NoError(t, err)
}

func TestRemoteScheme(t *testing.T) {
	uri, err := workspaceapi.ParseURI("ssh://unsable.build/home/ernie")
	require.NoError(t, err)

	t.Run("establish initial connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// emulate event loop synchronization
		var mu sync.Mutex
		ctx := workspace.ContextWithLocker(context.Background(), &mu)

		mock := workspacetest.NewMockScheme(ctrl)
		mu.Lock()
		scheme := newRemoteScheme(ctx,
			func(_ context.Context, uri workspaceapi.URI, closehook func(error)) (workspace.Scheme, error) {
				return mock, nil
			}, uri)
		mu.Unlock()

		expectSchemeAPISuccess(t, &mu, mock, scheme)
		expectSchemeClose(t, mock, scheme)
	})

	t.Run("eventually succeed upon initial connect errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		// emulate event loop synchronization
		var mu sync.Mutex
		ctx := workspace.ContextWithLocker(context.Background(), &mu)

		mock := workspacetest.NewMockScheme(ctrl)
		var i int

		mu.Lock()
		scheme := newRemoteScheme(ctx, func(
			_ context.Context, uri workspaceapi.URI, closehook func(error),
		) (workspace.Scheme, error) {
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

		// emulate event loop synchronization
		var mu sync.Mutex
		ctx := workspace.ContextWithLocker(context.Background(), &mu)

		mock := workspacetest.NewMockScheme(ctrl)
		var closeHook func(error)
		var wg sync.WaitGroup
		var reconnect bool
		mu.Lock()
		scheme := newRemoteScheme(ctx, func(
			_ context.Context, uri workspaceapi.URI, _closehook func(error),
		) (workspace.Scheme, error) {
			closeHook = _closehook
			if reconnect {
				wg.Done()
			}
			return mock, nil
		}, uri)
		mu.Unlock()
		expectSchemeAPISuccess(t, &mu, mock, scheme)

		wg.Add(1)
		reconnect = true
		mock.EXPECT().Close().Return(errors.New("already closed but should be fine"))
		closeHook(errors.New("kaboom"))
		wg.Wait()
		expectSchemeAPISuccess(t, &mu, mock, scheme)

		expectSchemeClose(t, mock, scheme)
	})
}
