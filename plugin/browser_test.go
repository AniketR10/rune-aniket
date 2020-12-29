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
	"google.golang.org/grpc"
)

func registerBrowserServer(
	brokerID uint32, mux proto.MuxBroker, bsrv *browser.Server,
) {
	mux.AcceptAndServe(brokerID, func(opts []grpc.ServerOption) *grpc.Server {
		srv := grpc.NewServer(opts...)
		proto.RegisterWindowManagerServer(srv, bsrv)
		proto.RegisterKeyMapperServer(srv, bsrv)
		proto.RegisterResourceOpenerServer(srv, bsrv)
		proto.RegisterMessengerServer(srv, bsrv)
		proto.RegisterEventSubscriberServer(srv, bsrv)
		proto.RegisterEventPublisherServer(srv, bsrv)
		return srv
	})
}

func newMockBrowserServer(
	ctrl *gomock.Controller, mux proto.MuxBroker, lock sync.Locker,
) (*browser.MockBrowser, *browser.Server) {
	ret := browser.NewMockBrowser(ctrl)
	bsrv := browser.NewServer(mux, ret, lock, func() {}, func() {})
	return ret, bsrv
}

func assertClientMethodNoError(
	t *testing.T, resource interface{},
	wg *sync.WaitGroup, mu sync.Locker,
	method func(ifc interface{}) error,
) {
	defer wg.Done()
	err := method(resource)
	mu.Lock()
	defer mu.Unlock()
	assert.NoError(t, err)
}

func TestIntegrationBrowserRace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWin := browser.NewMockWindow(ctrl)
	h := handler.NewTestHandler()
	evNone := term.Event{Type: term.EventNone}
	keymap := map[term.Event]term.Event{
		evNone: term.Event{Type: term.EventInterrupt},
	}
	nopHandler := browser.FuncEventHandler(func(term.Event) bool { return false })

	var mu sync.Mutex
	broker := proto.NewDialBroker()
	brokerIDs := []uint32{broker.NextId(), broker.NextId()}
	defer broker.Close()
	mock, browserServer := newMockBrowserServer(ctrl, broker, &mu)

	for _, brokerID := range brokerIDs {
		registerBrowserServer(brokerID, broker, browserServer)
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
			return mock.Open(gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.ResourceOpener).Open("")
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
			return ifc.(browser.EventSubscriber).Subscribe(evNone, nopHandler)
		}},
		{func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return EventPublisher(token, broker)
		}, func(mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.PublishInterrupt().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.EventPublisher).PublishInterrupt()
		}},
	}

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
