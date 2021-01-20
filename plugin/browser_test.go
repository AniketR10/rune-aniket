package plugin

import (
	_ "net/http/pprof"
	"sync"
	"testing"

	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.NoError(t, err)
}

func TestIntegrationBrowserRace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWin := browser.NopWindow()
	h := handler.NewTestHandler()
	evKeyCtrlA := term.Event{Type: term.EventKey, Key: term.KeyCtrlA}
	keymap := map[term.Event]term.Event{
		evKeyCtrlA: term.Event{Type: term.EventKey, Key: term.KeyCtrlB},
	}
	nopHandler := browser.FuncEventHandler(func(term.Event) bool { return false })

	broker := proto.NewDialBroker()
	brokerIDs := []uint32{broker.NextId(), broker.NextId()}
	defer broker.Close()
	mock := browser.NewMockBrowser(ctrl)
	resources := BrowserResources(mock)

	perms := []Permission{
		PermissionBrowserWindowManager, PermissionBrowserKeyMapper,
		PermissionBrowserResourceOpener, PermissionBrowserMessenger,
		PermissionBrowserEventSubscriber, PermissionBrowserEventPublisher,
	}
	for _, perm := range perms {
		for _, brokerID := range brokerIDs {
			resources[perm].Serve("caliu-plugins-ltd", brokerID,
				broker, nil, new(sync.Mutex))
		}
	}

	tsuite := []struct {
		createResource func(uint32, proto.MuxBroker) (interface{}, error)
		expect         func(*browser.MockBrowserMockRecorder) *gomock.Call
		method         func(interface{}) error
	}{
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Focus().Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).Focus()
			return err
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SplitHorizontalAbove(gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).SplitHorizontalAbove(h)
			return err
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SplitHorizontalBelow(gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).SplitHorizontalBelow(h)
			return err
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SplitVerticalLeft(gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).SplitVerticalLeft(h)
			return err
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SplitVerticalRight(gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).SplitVerticalRight(h)
			return err
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return KeyMapper(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.MergeKeyMap(gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.KeyMapper).MergeKeyMap(keymap)
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return ResourceOpener(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Open(gomock.Any()).Return(nil, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.ResourceOpener).Open("")
			return err
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Messenger(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SetMessage(gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.Messenger).SetMessage("")
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return EventSubscriber(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Subscribe(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.EventSubscriber).Subscribe(evKeyCtrlA, nopHandler)
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return EventPublisher(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.PublishInterrupt().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.EventPublisher).PublishInterrupt()
		}},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, brokerID := range brokerIDs {
		for _, tcase := range tsuite {
			res1, err := tcase.createResource(brokerID, broker)
			require.NoError(t, err)

			res2, err := tcase.createResource(brokerID, broker)
			require.NoError(t, err)

			n := 5
			tcase.expect(mock.EXPECT()).Times(n * 2)
			for i := 0; i < n; i++ {
				wg.Add(2)
				go assertClientMethodNoError(t, res1, &wg, &mu, tcase.method)
				go assertClientMethodNoError(t, res2, &wg, &mu, tcase.method)
			}

		}
	}
	wg.Wait()
}
