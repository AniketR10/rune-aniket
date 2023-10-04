package extension

import (
	"context"
	_ "net/http/pprof"
	"sync"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	browserapi "unstable.build/go-tui/api/browser"
	browserapitest "unstable.build/go-tui/api/browser/test"
	workspaceapi "unstable.build/go-tui/api/workspace"
	browsertest "unstable.build/go-tui/browser/test"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/component/notifications"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
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

	mockWin := browserapitest.NopWindow()
	h := browsertest.NewTestHandler()
	broker := proto.NewUnixGRPCBroker("")
	defer broker.Close()
	mock := browserapitest.NewMockBrowser(ctrl)
	resources := extension.BrowserResources(browsertest.BrowserFromAPIBrowser(mock))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ctx = proto.ContextWithWaitGroup(ctx, new(sync.WaitGroup))

	uri, err := workspaceapi.ParseURI("file:///tmp/test")
	require.NoError(t, err)

	grantor := extension.GrantAll(resources)
	srv, err := broker.NewChannel()
	require.NoError(t, err)
	grantID := extension.Grant{Token: srv.Addr().String(), Context: ctx}
	perms := map[extension.Permission]extension.Grant{
		extension.PermissionBrowserWindowManager:  grantID,
		extension.PermissionBrowserResourceOpener: grantID,
		extension.PermissionBrowserNotifications:  grantID,
		extension.PermissionBrowserEventPublisher: grantID,
	}

	for perm := range perms {
		resources[perm].Register("caliu-extensions-ltd", grantor,
			srv.Registrar(), broker, new(sync.Mutex))
	}

	go srv.Serve(ctx)

	tsuite := []struct {
		perm           extension.Permission
		createResource func(extension.Grant, proto.MuxBroker) (interface{}, error)
		expect         func(
			*browserapitest.MockBrowserMockRecorder) *gomock.Call
		method func(ifc interface{}) error
	}{
		{extension.PermissionBrowserWindowManager, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Focus().Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Focus()
			return err
		}},
		{extension.PermissionBrowserWindowManager, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
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
		{extension.PermissionBrowserWindowManager, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Bar(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.WindowManager).Bar(browserapi.OrientationBottom, h)
		}},
		{extension.PermissionBrowserWindowManager, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Tab(gomock.Any(), gomock.Any(), gomock.Any()).Return(h, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Tab(uri, "", h)
			return err
		}},
		{extension.PermissionBrowserWindowManager, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Floating(gomock.Any(), gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Floating(browserapi.StaticFloating(h, 4, 4), component.FloatingConfig{})
			return err
		}},
		{extension.PermissionBrowserResourceOpener, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return ResourceOpener(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Open(gomock.Any()).Return(h, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.ResourceOpener).Open(uri)
			return err
		}},
		{extension.PermissionBrowserNotifications, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return Notifications(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Notify(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.Notifications).Notify(notifications.LevelSuccess, "")
		}},
		{extension.PermissionBrowserEventPublisher, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return EventPublisher(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Interrupt().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.EventPublisher).Interrupt()
		}},
		{extension.PermissionBrowserEventPublisher, func(token extension.Grant, broker proto.MuxBroker) (interface{}, error) {
			return EventPublisher(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.PublishEventNone().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.EventPublisher).PublishEventNone()
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
			tcase.expect(mock.EXPECT()).Times(n * 2)
		}
		for i := 0; i < n; i++ {
			wg.Add(2)
			go assertClientMethodNoError(t, res1, &wg, &mu, tcase.method)
			go assertClientMethodNoError(t, res2, &wg, &mu, tcase.method)
		}

	}
	wg.Wait()
}
