package plugin

import (
	"context"
	"io/ioutil"
	_ "net/http/pprof"
	"strings"
	"sync"
	"testing"

	"github.com/ernestrc/blue/document"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	browserapi "unstable.build/go-tui/api/browser"
	browserplugin "unstable.build/go-tui/api/browser/plugin"
	browserapitest "unstable.build/go-tui/api/browser/test"
	browsertest "unstable.build/go-tui/browser/test"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
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
	edMock := text.NewMockEditor(ctrl)
	edMock.EXPECT().SubscribeEditorEvents(gomock.Any(), gomock.Any()).Times(1)
	resources := MergeResourceMap(
		BrowserResources(browsertest.BrowserFromAPIBrowser(mock)),
		EditorResources(edMock),
	)
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	resources = MergeResourceMap(resources, StorageResources(dir))
	resources[PermissionClipboard] = NewClipboardManager()

	wpMock := workspacetest.NewMockWorkspace(ctrl)
	resources = MergeResourceMap(resources, WorkspaceResources(wpMock))
	interrupt = func() {}
	defer func() {
		interrupt = term.Interrupt
	}()

	uri, err := workspace.ParseURI("file:///tmp/test")
	require.NoError(t, err)

	grantor := cachingGrantor(GrantAll(resources))
	grantID := broker.NextId()
	perms := map[Permission]uint32{
		Permission(browserplugin.PermissionBrowserWindowManager):  grantID,
		Permission(browserplugin.PermissionBrowserResourceOpener): grantID,
		Permission(browserplugin.PermissionBrowserMessenger):      grantID,
		Permission(browserplugin.PermissionBrowserEventPublisher): grantID,
		PermissionEditor:    grantID,
		PermissionStorage:   grantID,
		PermissionClipboard: grantID,
		PermissionWorkspace: grantID,
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
		expect         func(*workspacetest.MockWorkspaceMockRecorder, *text.MockEditorMockRecorder, *browserapitest.MockBrowserMockRecorder) *gomock.Call
		method         func(ifc interface{}) error
	}{
		{Permission(browserplugin.PermissionBrowserWindowManager), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.WindowManager(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return mock.Focus().Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Focus()
			return err
		}},
		{Permission(browserplugin.PermissionBrowserWindowManager), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.WindowManager(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
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
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return mock.Bar(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.WindowManager).Bar(browserapi.OrientationBottom, h)
		}},
		{Permission(browserplugin.PermissionBrowserWindowManager), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.WindowManager(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return mock.Tab(gomock.Any(), gomock.Any(), gomock.Any()).Return(h, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Tab(uri, "", h)
			return err
		}},
		{Permission(browserplugin.PermissionBrowserWindowManager), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.WindowManager(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return mock.Floating(gomock.Any(), gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Floating(browserapi.StaticFloating(h, 4, 4), component.FloatingConfig{})
			return err
		}},
		{Permission(browserplugin.PermissionBrowserResourceOpener), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.ResourceOpener(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return mock.Open(gomock.Any()).Return(h, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.ResourceOpener).Open(uri)
			return err
		}},
		{Permission(browserplugin.PermissionBrowserMessenger), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.Messenger(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return mock.SetMessage(gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.Messenger).SetMessage("")
		}},
		{Permission(browserplugin.PermissionBrowserEventPublisher), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.EventPublisher(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return mock.Interrupt().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.EventPublisher).Interrupt()
		}},
		{Permission(browserplugin.PermissionBrowserEventPublisher), func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return browserplugin.EventPublisher(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return mock.PublishEventNone().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.EventPublisher).PublishEventNone()
		}},
		{PermissionEditor, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return ed.Edit(gomock.Any(), gomock.Any()).Return(nil, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(text.Editor).Edit(uri, cell.NewBuffer())
			return err
		}},
		{PermissionEditor, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return ed.SubscribeEditorEvents(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			h := text.FuncEventHandler(func(context.Context, text.Event) bool { return false })
			ev := []text.EventType{text.EventTypeFlush}
			return ifc.(text.Editor).SubscribeEditorEvents(ev, h)
		}},
		{PermissionEditor, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			ed.Edit(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			return ed.SetCursor(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			// force cache token
			h, err := ifc.(text.Editor).Edit(uri, cell.NewBuffer())
			if err != nil {
				return err
			}
			return ifc.(text.Editor).SetCursor(h, term.Coordinates{})
		}},
		{PermissionEditor, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(_ *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			ed.Edit(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			return ed.Cursor(gomock.Any()).Return(term.Coordinates{}, nil)
		}, func(ifc interface{}) error {
			// force cache token
			h, err := ifc.(text.Editor).Edit(uri, cell.NewBuffer())
			if err != nil {
				return err
			}
			_, err = ifc.(text.Editor).Cursor(h)
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
		{PermissionClipboard, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return GetClipboard(token, broker)
			// we test ClipboardManager directly
		}, nil, func(ifc interface{}) error {
			return ifc.(ClipboardSetter).SetRegister(text.DefaultRegisterID, nil)
		}},
		{PermissionWorkspace, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Workspace(token, broker)
		}, func(exec *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return exec.Command(gomock.Any(), gomock.Any()).Return(workspace.Pid(1), nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(workspace.Executor).Command("", "")
			return err
		}},
		{PermissionWorkspace, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Workspace(token, broker)
		}, func(exec *workspacetest.MockWorkspaceMockRecorder, ed *text.MockEditorMockRecorder, mock *browserapitest.MockBrowserMockRecorder) *gomock.Call {
			return exec.StdoutPipe(gomock.Any()).Return(ioutil.NopCloser(strings.NewReader(":")), nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(workspace.Executor).StdoutPipe(workspace.Pid(0))
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
			tcase.expect(wpMock.EXPECT(), edMock.EXPECT(), mock.EXPECT()).Times(n * 2)
		}
		for i := 0; i < n; i++ {
			wg.Add(2)
			go assertClientMethodNoError(t, res1, &wg, &mu, tcase.method)
			go assertClientMethodNoError(t, res2, &wg, &mu, tcase.method)
		}

	}
	wg.Wait()
}
