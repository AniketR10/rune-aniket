package plugin

import (
	"context"
	_ "net/http/pprof"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceapitest "unstable.build/go-tui/api/workspace/test"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	workspacetest "unstable.build/go-tui/workspace/test"
)

func assertClientMethodNoError(
	t *testing.T, resource interface{},
	wg *sync.WaitGroup, mu sync.Locker,
	method func(ifc interface{}) error,
) {
	defer wg.Done()
	mu.Lock()
	defer mu.Unlock()
	err := method(resource)
	require.NoError(t, err)
}

func TestIntegrationRace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	term.DisableInterruptForTesting()

	broker := proto.NewUnixGRPCBroker("")
	defer broker.Close()
	mockFile := workspaceapitest.NewMockFile(ctrl)
	mockFile.EXPECT().Fd().AnyTimes()
	mockFile.EXPECT().Name().AnyTimes()
	mockFile.EXPECT().Close().AnyTimes()

	fsMock := workspaceapitest.NewMockFileSystem(ctrl)
	termMock := workspaceapitest.NewMockTerminal(ctrl)
	execMock := workspaceapitest.NewMockExecutor(ctrl)
	workspace := workspacetest.WorkspaceFromAPIWorkspace(execMock, fsMock, termMock)
	resources := plugin.WorkspaceResources(workspace)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ctx = proto.ContextWithWaitGroup(ctx, new(sync.WaitGroup))

	grantor := plugin.GrantAll(resources)
	lis, err := broker.NewChannel()
	grantID := lis.Addr().String()
	perms := map[plugin.Permission]plugin.Grant{
		plugin.PermissionFileSystem: plugin.Grant{Token: grantID, Context: ctx},
		plugin.PermissionTerminal:   plugin.Grant{Token: grantID, Context: ctx},
		plugin.PermissionExecute:    plugin.Grant{Token: grantID, Context: ctx},
	}
	require.NoError(t, err)
	srv := proto.GRPCServer()

	for perm := range perms {
		resources[perm].Register("caliu-plugins-ltd", grantor,
			srv.Registrar(), broker, new(sync.Mutex))
	}

	go srv.Serve(ctx, lis)

	tsuite := []struct {
		perm           plugin.Permission
		createResource func(plugin.Grant, proto.MuxBroker) (interface{}, error)
		expect         func(
			*workspaceapitest.MockFileSystemMockRecorder,
			*workspaceapitest.MockTerminalMockRecorder,
			*workspaceapitest.MockExecutorMockRecorder) *gomock.Call
		method func(ifc interface{}) error
	}{
		{plugin.PermissionExecute, func(grant plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return Executor(grant, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
		) *gomock.Call {
			return exec.Start(gomock.Any()).Return(workspaceapi.Pid(1), nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(workspaceapi.Executor).Start(workspaceapi.Cmd{
				Path: "/bin/zsh",
			})
			return err
		}},
		{plugin.PermissionFileSystem, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return FileSystem(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
		) *gomock.Call {
			return fs.Open(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockFile, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(workspaceapi.FileSystem).Open("bla", 0, 0)
			if err == nil {
				return nil
			}
			return err.ToError()
		}},
		{plugin.PermissionTerminal, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return Terminal(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
		) *gomock.Call {
			return t.StartPty().Return(workspaceapi.Pty{Master: mockFile}, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(workspaceapi.Terminal).StartPty()
			return err
		}},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, tcase := range tsuite {
		grant := perms[tcase.perm]
		res1, err := tcase.createResource(grant, broker)
		require.NoError(t, err)

		res2, err := tcase.createResource(grant, broker)
		require.NoError(t, err)

		n := 5
		if tcase.expect != nil {
			tcase.expect(
				fsMock.EXPECT(), termMock.EXPECT(), execMock.EXPECT(),
			).Times(n * 2)
		}
		for i := 0; i < n; i++ {
			wg.Add(2)
			go assertClientMethodNoError(t, res1, &wg, &mu, tcase.method)
			go assertClientMethodNoError(t, res2, &wg, &mu, tcase.method)
		}

	}
	wg.Wait()
}
