package plugin

import (
	"context"
	_ "net/http/pprof"
	"sync"
	"testing"

	"github.com/ernestrc/blue/datastore/document"
	"github.com/ernestrc/go-tui/browser"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/editor"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"github.com/golang/mock/gomock"
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
	require.NoError(t, err)
}

func TestIntegrationRace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockWin := browser.NopWindow()
	h := browser.NewTestHandler()
	evKeyCtrlA := term.Event{Type: term.EventKey, Key: term.KeyCtrlA}
	keymap := map[term.Event]term.Event{
		evKeyCtrlA: term.Event{Type: term.EventKey, Key: term.KeyCtrlB},
	}
	nopHandler := browser.FuncEventHandler(func(term.Event) bool { return false })

	cache := document.NewInMemoryCache()
	broker := proto.NewDatastoreBroker(cache, nil)
	defer broker.Close()
	mock := browser.NewMockBrowser(ctrl)
	edMock := editor.NewMockEditor(ctrl)
	edMock.EXPECT().SubscribeEditor(gomock.Any(), gomock.Any()).Times(2)
	resources := MergeResourceMap(BrowserResources(mock), EditorResources(edMock))

	// plugin manager serves each permission on a different grantID
	perms := map[Permission]uint32{
		PermissionBrowserWindowManager:   broker.NextId(),
		PermissionBrowserKeyMapper:       broker.NextId(),
		PermissionBrowserResourceOpener:  broker.NextId(),
		PermissionBrowserMessenger:       broker.NextId(),
		PermissionBrowserEventSubscriber: broker.NextId(),
		PermissionBrowserEventPublisher:  broker.NextId(),
		PermissionEditor:                 broker.NextId(),
		PermissionBrowserStorage:         broker.NextId(),
	}
	for perm, brokerID := range perms {
		resources[perm].Serve("caliu-plugins-ltd", brokerID,
			broker, nil, new(sync.Mutex))
	}

	tsuite := []struct {
		perm           Permission
		createResource func(uint32, proto.MuxBroker) (interface{}, error)
		expect         func(*editor.MockEditorMockRecorder, *browser.MockBrowserMockRecorder) *gomock.Call
		method         func(ifc interface{}) error
	}{
		{PermissionBrowserWindowManager, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Focus().Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).Focus()
			return err
		}},
		{PermissionBrowserWindowManager, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SplitHorizontalAbove(gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).SplitHorizontalAbove(h)
			return err
		}},
		{PermissionBrowserWindowManager, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SplitHorizontalBelow(gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).SplitHorizontalBelow(h)
			return err
		}},
		{PermissionBrowserWindowManager, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SplitVerticalLeft(gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).SplitVerticalLeft(h)
			return err
		}},
		{PermissionBrowserWindowManager, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return WindowManager(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SplitVerticalRight(gomock.Any()).Return(mockWin, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.WindowManager).SplitVerticalRight(h)
			return err
		}},
		{PermissionBrowserKeyMapper, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return KeyMapper(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.MergeKeyMap(gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.KeyMapper).MergeKeyMap(keymap)
		}},
		{PermissionBrowserResourceOpener, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return ResourceOpener(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Open(gomock.Any()).Return(h, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.ResourceOpener).Open("")
			return err
		}},
		{PermissionBrowserMessenger, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Messenger(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.SetMessage(gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.Messenger).SetMessage("")
		}},
		{PermissionBrowserEventSubscriber, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return EventSubscriber(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Subscribe(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.EventSubscriber).Subscribe(evKeyCtrlA, nopHandler)
		}},
		{PermissionBrowserEventPublisher, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return EventPublisher(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.PublishInterrupt().Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.EventPublisher).PublishInterrupt()
		}},
		{PermissionEditor, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return ed.Edit(gomock.Any(), gomock.Any()).Return(nil, nil)
		}, func(ifc interface{}) error {
			_, err := ifc.(editor.Editor).Edit("", cell.NewBuffer())
			return err
		}},
		{PermissionEditor, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return ed.SubscribeEditor(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			h := editor.FuncEventHandler(func(editor.Event) bool { return false })
			return ifc.(editor.Editor).SubscribeEditor(editor.EventTypeFlush, h)
		}},
		{PermissionEditor, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			ed.Edit(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			return ed.SetCursor(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			// force cache token
			h, err := ifc.(editor.Editor).Edit("", cell.NewBuffer())
			if err != nil {
				return err
			}
			return ifc.(editor.Editor).SetCursor(h, term.Coordinates{})
		}},
		{PermissionEditor, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Editor(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			ed.Edit(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
			return ed.Cursor(gomock.Any()).Return(term.Coordinates{}, nil)
		}, func(ifc interface{}) error {
			// force cache token
			h, err := ifc.(editor.Editor).Edit("", cell.NewBuffer())
			if err != nil {
				return err
			}
			_, err = ifc.(editor.Editor).Cursor(h)
			return err
		}},
		{PermissionBrowserStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Create(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.Storage).Create(context.Background(), "", map[string]interface{}{"a": "b"})
		}},
		{PermissionBrowserStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			var recv map[string]interface{}
			return ifc.(browser.Storage).Get(context.Background(), "", &recv)
		}},
		{PermissionBrowserStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Update(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.Storage).Update(context.Background(), "", []document.Update{{FieldPath: []string{"a"}, Value: "b"}})
		}},
		{PermissionBrowserStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			return mock.Delete(gomock.Any(), gomock.Any()).Return(nil)
		}, func(ifc interface{}) error {
			return ifc.(browser.Storage).Delete(context.Background(), "")
		}},
		{PermissionBrowserStorage, func(token uint32, broker proto.MuxBroker) (interface{}, error) {
			return Storage(token, broker)
		}, func(ed *editor.MockEditorMockRecorder, mock *browser.MockBrowserMockRecorder) *gomock.Call {
			m1, m2 := make(map[string]interface{}), make(map[string]interface{})
			return mock.List(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, filters []document.Filter) (document.Iterator, error) {
					// instantiate for every invokation
					// or else we run into data race. as mock expect data
					// is shared across different invokations
					return document.ListIterator(m1, m2), nil
				})
		}, func(ifc interface{}) error {
			_, err := ifc.(browser.Storage).List(context.Background(), nil)
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
		tcase.expect(edMock.EXPECT(), mock.EXPECT()).Times(n * 2)
		for i := 0; i < n; i++ {
			wg.Add(2)
			go assertClientMethodNoError(t, res1, &wg, &mu, tcase.method)
			go assertClientMethodNoError(t, res2, &wg, &mu, tcase.method)
		}

	}
	wg.Wait()
}
