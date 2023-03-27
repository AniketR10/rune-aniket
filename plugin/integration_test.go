package plugin

import (
	"context"
	"io/ioutil"
	_ "net/http/pprof"
	"sync"
	"testing"

	"github.com/ernestrc/blue/document"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	browserapi "unstable.build/go-tui/api/browser"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	browserapitest "unstable.build/go-tui/api/browser/test"
	textapi "unstable.build/go-tui/api/text"
	textplugin "unstable.build/go-tui/api/text/plugin"
	workspaceapi "unstable.build/go-tui/api/workspace"
	workspaceplugin "unstable.build/go-tui/api/workspace/plugin"
	workspaceapitest "unstable.build/go-tui/api/workspace/test"
	browsertest "unstable.build/go-tui/browser/test"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	textpb "unstable.build/go-tui/text/rpc"
	texttest "unstable.build/go-tui/text/test"
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

	mockWin := browserapitest.NopWindow()
	h := browsertest.NewTestHandler()
	cache := document.NewInMemoryService()
	broker := proto.NewDatastoreBroker(cache)
	defer broker.Close()
	mock := browserapitest.NewMockBrowser(ctrl)
	edMock := texttest.NewMockEditor(ctrl)
	mockFile := workspaceapitest.NewMockFile(ctrl)
	mockFile.EXPECT().Fd().AnyTimes()
	mockFile.EXPECT().Name().AnyTimes()
	mockFile.EXPECT().Close().AnyTimes()
	resources := MergeResourceMap(
		BrowserResources(browsertest.BrowserFromAPIBrowser(mock)),
		EditorResources(edMock),
	)
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	resources = MergeResourceMap(resources, StorageResources(dir))

	fsMock := workspaceapitest.NewMockFileSystem(ctrl)
	termMock := workspaceapitest.NewMockTerminal(ctrl)
	execMock := workspaceapitest.NewMockExecutor(ctrl)
	workspace := workspacetest.WorkspaceFromAPIWorkspace(execMock, fsMock, termMock)
	resources = MergeResourceMap(resources, WorkspaceResources(workspace))
	interrupt = func() {}
	defer func() {
		interrupt = term.Interrupt
	}()

	uri, err := workspaceapi.ParseURI("file:///tmp/test")
	require.NoError(t, err)

	th := textpb.Token{URI: uri}

	grantor := cachingGrantor(GrantAll(resources))
	grantID := broker.NextId()
	perms := map[Permission]uint32{
		Permission(browserplugin.PermissionBrowserWindowManager):  grantID,
		Permission(browserplugin.PermissionBrowserResourceOpener): grantID,
		Permission(browserplugin.PermissionBrowserMessenger):      grantID,
		Permission(browserplugin.PermissionBrowserEventPublisher): grantID,
		Permission(textplugin.PermissionEditor):                   grantID,
		PermissionStorage:                                         grantID,
		Permission(workspaceplugin.PermissionFileSystem):          grantID,
		Permission(workspaceplugin.PermissionTerminal):            grantID,
		Permission(workspaceplugin.PermissionExecute):             grantID,
	}
	lis, err := broker.Accept(grantID)
	require.NoError(t, err)
	srv := proto.GRPCServer()

	for perm := range perms {
		resources[perm].Register("caliu-plugins-ltd", grantor,
			srv.Registrar(), broker, new(sync.Mutex))
	}
	go srv.Serve(context.Background(), lis)

	tsuite := []struct {
		perm           Permission
		createResource func(uint32, proto.MuxBroker) (interface{}, error)
		expect         func(
			*workspaceapitest.MockFileSystemMockRecorder,
			*workspaceapitest.MockTerminalMockRecorder,
			*workspaceapitest.MockExecutorMockRecorder,
			*texttest.MockEditorMockRecorder,
			*browserapitest.MockBrowserMockRecorder) *gomock.Call
		method func(ifc interface{}) error
	}{
		{Permission(browserplugin.PermissionBrowserWindowManager), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.WindowManager(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Focus().Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Focus()
			return err
		}},
		{Permission(browserplugin.PermissionBrowserWindowManager), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.WindowManager(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			mock.Focus().Return(mockWin, nil).AnyTimes()
			return mock.Split(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			win, err := ifc.(browserapi.WindowManager).Focus()
			if err != nil {
				return err
			}
			_, err = ifc.(browserapi.WindowManager).Split(browserapi.OrientationBottom, win, h)
			return err
		}},
		{Permission(browserplugin.PermissionBrowserWindowManager), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.WindowManager(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Bar(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.WindowManager).Bar(browserapi.OrientationBottom, h)
		}},
		{Permission(browserplugin.PermissionBrowserWindowManager), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.WindowManager(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Tab(gomock.Any(), gomock.Any(), gomock.Any()).Return(h, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Tab(uri, "", h)
			return err
		}},
		{Permission(browserplugin.PermissionBrowserWindowManager), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.WindowManager(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Floating(gomock.Any(), gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Floating(browserapi.StaticFloating(h, 4, 4), component.FloatingConfig{})
			return err
		}},
		{Permission(browserplugin.PermissionBrowserResourceOpener), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.ResourceOpener(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Open(gomock.Any()).Return(h, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.ResourceOpener).Open(uri)
			return err
		}},
		{Permission(browserplugin.PermissionBrowserMessenger), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.Messenger(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.SetMessage(gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.Messenger).SetMessage("")
		}},
		{Permission(browserplugin.PermissionBrowserEventPublisher), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.EventPublisher(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Interrupt().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.EventPublisher).Interrupt()
		}},
		{Permission(browserplugin.PermissionBrowserEventPublisher), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.EventPublisher(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.PublishEventNone().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.EventPublisher).PublishEventNone()
		}},
		{Permission(textplugin.PermissionEditor), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return textplugin.Editor(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return ed.Edit(gomock.Any(), gomock.Any()).Return(nil, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(textapi.Editor).Edit(uri, cell.NewBuffer())
			return err
		}},
		{Permission(textplugin.PermissionEditor), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return textplugin.Editor(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return ed.SubscribeEditorEvents(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			h := text.FuncEventHandler(func(context.Context, textapi.Event) bool { return false })
			ev := []textapi.EventType{textapi.EventTypeFlush}
			return ifc.(textapi.Editor).SubscribeEditorEvents(ev, h)
		}},
		{Permission(textplugin.PermissionEditor), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return textplugin.Editor(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			ed.Editor(gomock.Any()).Return(th, nil).AnyTimes()
			return ed.SetCursor(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(textapi.Editor).SetCursor(th, term.Coordinates{})
		}},
		{Permission(textplugin.PermissionEditor), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return textplugin.Editor(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			ed.Editor(gomock.Any()).Return(th, nil).AnyTimes()
			return ed.Cursor(gomock.Any()).Return(term.Coordinates{}, nil)
		}, func(ifc interface{}) error {
			_, err = ifc.(textapi.Editor).Cursor(th)
			return err
		}},
		// for document.Service, just do a race test
		{PermissionStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, nil, func(ifc interface{}) error {
			_ = ifc.(document.Service).Create(context.Background(), "", map[string]interface{}{"a": "b"})
			return nil
		}},
		{PermissionStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, nil, func(ifc interface{}) error {
			var recv map[string]interface{}
			_ = ifc.(document.Service).Get(context.Background(), "", &recv)
			return nil
		}},
		{PermissionStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, nil, func(ifc interface{}) error {
			_ = ifc.(document.Service).Update(context.Background(), "", []document.Update{{FieldPath: []string{"a"}, Value: "b"}})
			return nil
		}},
		{PermissionStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, nil, func(ifc interface{}) error {
			_ = ifc.(document.Service).Delete(context.Background(), "")
			return nil
		}},
		{PermissionStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, nil, func(ifc interface{}) error {
			_, _ = ifc.(document.Service).List(context.Background(), nil)
			return nil
		}},
		{Permission(workspaceplugin.PermissionExecute), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return workspaceplugin.Executor(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return exec.Start(gomock.Any()).Return(workspaceapi.Pid(1), nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(workspaceapi.Executor).Start(workspaceapi.Cmd{
				Path: "/bin/zsh",
			})
			return err
		}},
		{Permission(workspaceplugin.PermissionFileSystem), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return workspaceplugin.FileSystem(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return fs.Open(gomock.Any(), gomock.Any(), gomock.Any()).Return(mockFile, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(workspaceapi.FileSystem).Open("bla", 0, 0)
			if err == nil {
				return nil
			}
			return err.ToError()
		}},
		{Permission(workspaceplugin.PermissionTerminal), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return workspaceplugin.Terminal(token, broker)
		}, func(
			fs *workspaceapitest.MockFileSystemMockRecorder,
			t *workspaceapitest.MockTerminalMockRecorder,
			exec *workspaceapitest.MockExecutorMockRecorder,
			ed *texttest.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder,
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
		brokerID := perms[tcase.perm]
		res1, err := tcase.createResource(brokerID, broker)
		require.NoError(t, err)

		res2, err := tcase.createResource(brokerID, broker)
		require.NoError(t, err)

		n := 5
		if tcase.expect != nil {
			tcase.expect(
				fsMock.EXPECT(), termMock.EXPECT(), execMock.EXPECT(),
				edMock.EXPECT(), mock.EXPECT(),
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
