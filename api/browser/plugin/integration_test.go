package plugin

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
	"unstable.build/go-tui/plugin"
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
	resources := plugin.BrowserResources(browsertest.BrowserFromAPIBrowser(mock))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	ctx = proto.ContextWithWaitGroup(ctx, new(sync.WaitGroup))

	uri, err := workspaceapi.ParseURI("file:///tmp/test")
	require.NoError(t, err)

	grantor := plugin.GrantAll(resources)
	srv, err := broker.NewChannel()
	require.NoError(t, err)
	grantID := plugin.Grant{Token: srv.Addr().String(), Context: ctx}
	perms := map[plugin.Permission]plugin.Grant{
		plugin.PermissionBrowserWindowManager:  grantID,
		plugin.PermissionBrowserResourceOpener: grantID,
		plugin.PermissionBrowserNotifications:      grantID,
		plugin.PermissionBrowserEventPublisher: grantID,
	}

	for perm := range perms {
		resources[perm].Register("caliu-plugins-ltd", grantor,
			srv.Registrar(), broker, new(sync.Mutex))
	}

	go srv.Serve(ctx)

	tsuite := []struct {
		perm           plugin.Permission
		createResource func(plugin.Grant, proto.MuxBroker) (interface{}, error)
		expect         func(
			*browserapitest.MockBrowserMockRecorder) *gomock.Call
		method func(ifc interface{}) error
	}{
		{plugin.PermissionBrowserWindowManager, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Focus().Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Focus()
			return err
		}},
		{plugin.PermissionBrowserWindowManager, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
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
		{plugin.PermissionBrowserWindowManager, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Bar(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.WindowManager).Bar(browserapi.OrientationBottom, h)
		}},
		{plugin.PermissionBrowserWindowManager, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Tab(gomock.Any(), gomock.Any(), gomock.Any()).Return(h, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Tab(uri, "", h)
			return err
		}},
		{plugin.PermissionBrowserWindowManager, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Floating(gomock.Any(), gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.WindowManager).Floating(browserapi.StaticFloating(h, 4, 4), component.FloatingConfig{})
			return err
		}},
		{plugin.PermissionBrowserResourceOpener, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return ResourceOpener(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Open(gomock.Any()).Return(h, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browserapi.ResourceOpener).Open(uri)
			return err
		}},
		{plugin.PermissionBrowserNotifications, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return Notifications(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Notify(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.Notifications).Notify(notifications.LevelSuccess, "")
		}},
		{plugin.PermissionBrowserEventPublisher, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
			return EventPublisher(token, broker)
		}, func(
			mock *browserapitest.MockBrowserMockRecorder,
		) *gomock.Call {
			return mock.Interrupt().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browserapi.EventPublisher).Interrupt()
		}},
		{plugin.PermissionBrowserEventPublisher, func(token plugin.Grant, broker proto.MuxBroker) (interface{}, error) {
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
