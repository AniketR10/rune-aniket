package ssh

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ernestrc/blue/retry"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/workspace"
	workspacetest "unstable.build/go-tui/workspace/test"
)

func expectSchemeAPISuccess(t *testing.T, mock *workspacetest.MockScheme, scheme workspace.Scheme) {
	mock.EXPECT().Start(gomock.Any()).Return(nil).Times(1)
	err := scheme.Start(0)
	require.NoError(t, err)

	mock.EXPECT().Signal(gomock.Any(), gomock.Any()).Return(nil).Times(1)
	err = scheme.Signal(0, 0)
	require.NoError(t, err)

	mock.EXPECT().StderrPipe(gomock.Any()).Return(io.NopCloser(strings.NewReader("")), nil).Times(1)
	_, err = scheme.StderrPipe(0)
	require.NoError(t, err)

	mock.EXPECT().StdinPipe(gomock.Any()).Return(nopWriteCloser{Writer: new(bytes.Buffer)}, nil).Times(1)
	_, err = scheme.StdinPipe(0)
	require.NoError(t, err)

	mock.EXPECT().StdoutPipe(gomock.Any()).Return(io.NopCloser(strings.NewReader("")), nil).Times(1)
	_, err = scheme.StdoutPipe(0)
	require.NoError(t, err)

	mock.EXPECT().Wait(gomock.Any()).Return(nil).Times(1)
	err = scheme.Wait(0)
	require.NoError(t, err)

	mock.EXPECT().Open(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
	_, err = scheme.Open("", 0, 0)
	require.Nil(t, err)

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

	mock.EXPECT().Command(gomock.Any()).Return(workspace.Pid(0), nil).Times(1)
	_, err = scheme.Command("blah")
	require.NoError(t, err)
}

func expectSchemeClose(t *testing.T, mock *workspacetest.MockScheme, scheme workspace.Scheme) {
	mock.EXPECT().Close().Return(nil)
	err := scheme.Close()
	require.NoError(t, err)
}

func TestRemoteScheme(t *testing.T) {
	uri, err := workspace.ParseURI("ssh://unsable.build/home/ernie")
	require.NoError(t, err)

	t.Run("establish initial connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := workspacetest.NewMockScheme(ctrl)
		scheme := newRemoteScheme(func(uri workspace.URI, closehook func(error)) (workspace.Scheme, error) {
			return mock, nil
		}, uri)

		expectSchemeAPISuccess(t, mock, scheme)
		expectSchemeClose(t, mock, scheme)
	})

	t.Run("eventually succeed upon initial connect errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := workspacetest.NewMockScheme(ctrl)
		var i int
		scheme := newRemoteScheme(func(uri workspace.URI, closehook func(error)) (workspace.Scheme, error) {
			i++
			if i <= 5 {
				return nil, errors.New("unable to connect")
			}
			return mock, nil
		}, uri)

		mock.EXPECT().Command(gomock.Any()).Return(workspace.Pid(0), nil).Times(1)
		retry.Retry(context.Background(), retry.ExponentialStrategy(1*time.Millisecond, 10*time.Millisecond), func(context.Context) (bool, error) {
			_, err = scheme.Command("blah")
			return true, err
		})
		require.NoError(t, err)
		expectSchemeClose(t, mock, scheme)
	})

	t.Run("re-establish connection", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		mock := workspacetest.NewMockScheme(ctrl)
		var closeHook func(error)
		var wg sync.WaitGroup
		var reconnect bool
		scheme := newRemoteScheme(func(uri workspace.URI, _closehook func(error)) (workspace.Scheme, error) {
			closeHook = _closehook
			if reconnect {
				wg.Done()
			}
			return mock, nil
		}, uri)
		expectSchemeAPISuccess(t, mock, scheme)

		wg.Add(1)
		reconnect = true
		mock.EXPECT().Close().Return(errors.New("already closed but should be fine"))
		closeHook(errors.New("kaboom"))
		wg.Wait()
		expectSchemeAPISuccess(t, mock, scheme)

		expectSchemeClose(t, mock, scheme)
	})
}
