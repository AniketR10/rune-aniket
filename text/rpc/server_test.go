package rpc

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler"
	"unstable.build/go-tui/proto"
	prototest "unstable.build/go-tui/proto/test"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

const asyncResultsSleepDuration = 300 * time.Millisecond
const locID = "errors"

type nopLocker struct{}

func (l nopLocker) Lock()   {}
func (l nopLocker) Unlock() {}

func newTestServer(t *testing.T, ctrl *gomock.Controller) (*proto.MockMuxBroker, *text.MockEditor, *Server) {
	broker := proto.NewMockMuxBroker(ctrl)
	ed := text.NewMockEditor(ctrl)
	expectInitialServerSubscribe(t, ed)
	s := NewServer(broker, ed, new(sync.Mutex), testBrowserServer{})
	broker.EXPECT().Cleanup(gomock.Any()).AnyTimes()
	return broker, ed, s
}

func expectEdit(t *testing.T, mock *text.MockEditor, resource workspace.URI, content string) {
	mock.EXPECT().Edit(gomock.Any(), gomock.Any()).Times(1).
		DoAndReturn(func(_uri workspace.URI, buf *cell.Buffer) (tui.Handler, error) {
			assert.Equal(t, resource, _uri)
			assert.Equal(t, content, buf.String())
			return handler.NewTestHandler(), nil
		})
}

func callServerEdit(
	t *testing.T, ctx context.Context, broker *proto.MockMuxBroker,
	s *Server, nextID uint32, uri workspace.URI, content string,
) {
	broker.EXPECT().NextId().Return(nextID).Times(1)

	buf := cell.NewBuffer()
	buf.WriteString(content)
	req := NewEditRequest(uri, buf)

	res, err := s.Edit(ctx, &req)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, nextID, res.GetHandlerId())
}

func TestServerEdit(t *testing.T) {
	nextID := uint32(99)
	ctx := context.Background()
	resource, err := workspace.ParseURI("file:///ULaptopNotLinux:@")
	require.NoError(t, err)
	bufContent1 := "ULaptopWillLinux:)"

	t.Run("Edit is propagated to underlying Editor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		broker, mock, s := newTestServer(t, ctrl)
		expectEdit(t, mock, resource, bufContent1)
		callServerEdit(t, ctx, broker, s, nextID, resource, bufContent1)
	})

	t.Run("relative path is converted to absolute", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		broker, mock, s := newTestServer(t, ctrl)
		require.NoError(t, err)
		expectEdit(t, mock, resource, bufContent1)
		callServerEdit(t, ctx, broker, s, nextID, resource, bufContent1)
	})

	t.Run("bubbles up underlying's Editor Edit errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		_, mock, s := newTestServer(t, ctrl)

		mock.EXPECT().Edit(gomock.Any(), gomock.Any()).
			Return(nil, errors.New("NOLINUX")).
			Times(1)

		req := NewEditRequest(resource, cell.NewBuffer())

		res, err := s.Edit(ctx, &req)
		require.Error(t, err)
		require.Nil(t, res)
	})
}

func waitForMonitoringExit(quitCh chan struct{}) {
	<-quitCh
	time.Sleep(asyncResultsSleepDuration)
}

func expectHandlerInvokeExit(t *testing.T, handlerConn *proto.MockMuxConn) {
	handlerConn.EXPECT().
		Invoke(gomock.Any(), gomock.Eq("/text.EditorEventHandler/Handle"),
			gomock.Any(),
			gomock.Any()).
		DoAndReturn(func(ctx context.Context,
			method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			res, ok := reply.(*EditorEventHandleResponse)
			require.True(t, ok)

			res.Quit = true
			return nil
		}).
		Times(1)
}

func assertServerHandlerExitClose(
	t *testing.T, handlerConn *proto.MockMuxConn,
	h text.EventHandler, s *Server, quitCh chan struct{},
	broker *proto.MockMuxBroker,
) {
	resource := &text.TestHandler{}
	expectHandlerInvokeExit(t, handlerConn)

	handlerConn.EXPECT().Close().Times(1).
		DoAndReturn(prototest.ExpectSignalExit(handlerConn, quitCh, nil))

	broker.EXPECT().NextId().Return(uint32(88)).Times(1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// force server to store resource name and make an ID
	s.Handle(ctx, text.Event{Type: text.EventTypeOpen, URI: uri, Resource: resource})

	s.editor.Lock()
	h.Handle(ctx, text.Event{Type: text.EventTypeClose, URI: uri, Resource: resource})
	s.editor.Unlock()

	waitForMonitoringExit(quitCh)

	s.editor.Lock()
	defer s.editor.Unlock()
	assert.Equal(t, 0, len(s.clients))
}

func TestServerSubscribe(t *testing.T) {
	ctx := context.Background()
	t.Run("propagates subscribe with multiple event types", func(t *testing.T) {
		nextID := uint32(12)
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)
		expectedEvTypes := []text.EventType{text.EventTypeFlush, text.EventTypeFocus}

		var actualEvTypes []text.EventType
		var wg sync.WaitGroup
		mock.EXPECT().SubscribeEditorEvents(gomock.Any(), gomock.Any()).
			DoAndReturn(func(evs []text.EventType, _h text.EventHandler) error {
				defer wg.Done()
				actualEvTypes = evs
				return nil
			})
		req := EditorSubscribeRequest{
			HandlerId: nextID,
			Type: []EditorEvent_Type{
				EditorEvent_TypeFlush,
				EditorEvent_TypeFocus,
			},
		}
		conn := prototest.ExpectBrokerDial(t, ctrl, broker, nextID)
		quitCh := prototest.ExpectMonitorConn(conn)

		wg.Add(1)
		res, err := s.Subscribe(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)

		wg.Wait()

		conn.EXPECT().Close().Times(1).
			DoAndReturn(prototest.ExpectSignalExit(conn, quitCh, nil))
		conn.EXPECT().Invoke(gomock.Any(), gomock.Any(),
			gomock.Any(), gomock.Any()).AnyTimes()

		assert.Equal(t, expectedEvTypes, actualEvTypes)
		s.editor.Lock()
		assert.NoError(t, s.Close())
		s.editor.Unlock()
		waitForMonitoringExit(quitCh)
	})

	t.Run("cleans resources when handler subscriber returns exit=true", func(t *testing.T) {
		nextID := uint32(12222)
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)
		expectedEvTypes := []text.EventType{text.EventTypeFlush}

		var actualEvTypes []text.EventType
		var h text.EventHandler
		var wg sync.WaitGroup
		mock.EXPECT().SubscribeEditorEvents(gomock.Any(), gomock.Any()).
			DoAndReturn(func(evs []text.EventType, _h text.EventHandler) error {
				defer wg.Done()
				actualEvTypes = evs
				h = _h
				return nil
			})
		conn := prototest.ExpectBrokerDial(t, ctrl, broker, nextID)
		quitCh := prototest.ExpectMonitorConn(conn)

		req := EditorSubscribeRequest{
			HandlerId: nextID,
			Type:      []EditorEvent_Type{EditorEvent_TypeFlush},
		}

		wg.Add(1)
		res, err := s.Subscribe(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)

		wg.Wait()

		require.NotNil(t, h)
		assert.Equal(t, expectedEvTypes, actualEvTypes)
		assertServerHandlerExitClose(t, conn, h, s, quitCh, broker)
	})
}

func assertEqualLocations(t *testing.T, loc, expected text.LocationList) {
	var locations, expectedLocations []text.Location
	for ok := true; ok; _, ok = loc.Prev() {

	}
	for ok := true; ok; _, ok = expected.Prev() {

	}
	for n, ok := loc.Current(); ok; n, ok = loc.Next() {
		locations = append(locations, n)
	}
	for n, ok := expected.Current(); ok; n, ok = expected.Next() {
		expectedLocations = append(expectedLocations, n)
	}
	assert.EqualValues(t, expectedLocations, locations)
}

func TestServerRegister(t *testing.T) {
	t.Run("calls underlying editor SubscribeCommand", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)

		conn := prototest.ExpectBrokerDial(t, ctrl, broker, 1234)
		quitCh := prototest.ExpectMonitorConn(conn)

		var wg sync.WaitGroup
		mock.EXPECT().SubscribeCommand(gomock.Any(), gomock.Any()).
			DoAndReturn(func(string, text.CommandHandler) error {
				wg.Done()
				return nil
			})

		wg.Add(1)
		req := RegisterCommandRequest{Command: "bla", HandlerId: 1234}
		res, err := s.Register(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)
		wg.Wait()

		conn.EXPECT().Close().Times(1).
			DoAndReturn(prototest.ExpectSignalExit(conn, quitCh, nil))

		s.editor.Lock()
		assert.NoError(t, s.Close())
		s.editor.Unlock()
		waitForMonitoringExit(quitCh)
	})

	t.Run("handles SubscribeCommand errors", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)

		conn := prototest.ExpectBrokerDial(t, ctrl, broker, 1234)
		quitCh := prototest.ExpectMonitorConn(conn)

		var wg sync.WaitGroup
		mock.EXPECT().SubscribeCommand(gomock.Any(), gomock.Any()).
			DoAndReturn(func(string, text.CommandHandler) error {
				wg.Done()
				return errors.New("boom")
			})

		conn.EXPECT().Close().Times(1).
			DoAndReturn(prototest.ExpectSignalExit(conn, quitCh, nil))

		wg.Add(1)
		req := RegisterCommandRequest{Command: "bla", HandlerId: 1234}
		res, err := s.Register(ctx, &req)
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "boom")
		wg.Wait()

		s.editor.Lock()
		assert.NoError(t, s.Close())
		s.editor.Unlock()
		waitForMonitoringExit(quitCh)
	})
}

func TestServerSetLocationList(t *testing.T) {
	resource, err := workspace.ParseURI("file:///go-tui")
	require.NoError(t, err)

	t.Run("calls underlying editor SetLocationList", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)

		content := "main"
		nextID := uint32(232)
		expectEdit(t, mock, resource, content)
		callServerEdit(t, ctx, broker, s, nextID, resource, content)

		locs := text.LocationSlice([]text.Location{{Message: "wsb: hold AMC", To: term.Coordinates{X: 3}}})
		mock.EXPECT().SetLocationList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(h text.Handler, pri text.LocationPriority, ID string, l text.LocationList) error {
				assert.Equal(t, text.LocationPriorityInfo, pri)
				assert.Equal(t, locID, ID)
				return nil
			})

		req := makeLocationListRequest(nextID, text.LocationPriorityInfo, locID, locs)
		res, err := s.SetLocationList(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("is threadsafe", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker := proto.NewMockMuxBroker(ctrl)
		ed := text.NopEditor()
		c, err := text.NewComponent(ed, &testLoader{}, text.Config{})
		require.NoError(t, err)
		s := NewServer(broker, c, new(sync.Mutex), testBrowserServer{})

		content := "main"
		nextID := uint32(232)
		callServerEdit(t, ctx, broker, s, nextID, resource, content)

		locs := []text.Location{
			{From: term.Coordinates{X: 0, Y: 0}, To: term.Coordinates{X: 3, Y: 0}},
			{From: term.Coordinates{X: 1, Y: 4}, To: term.Coordinates{X: 2, Y: 4}},
			{From: term.Coordinates{X: 0, Y: 5}, To: term.Coordinates{X: 0, Y: 6}},
		}

		var wg sync.WaitGroup
		n := 100
		wg.Add(n)
		for i := 0; i < n; i++ {
			go func() {
				defer wg.Done()
				l := text.LocationSlice(locs)

				req := makeLocationListRequest(nextID, text.LocationPriorityWarning, locID, l)
				res, err := s.SetLocationList(ctx, &req)
				if !assert.NoError(t, err) {
					return
				}
				if !assert.NotNil(t, res) {
					return
				}
			}()
		}

		wg.Wait()

		h, ok := s.idToHandler[nextID]
		if !assert.True(t, ok) {
			return
		}

		l, ok := h.(*text.TestEditorHandler)
		if !assert.True(t, ok) {
			return
		}
		assertEqualLocations(t, text.LocationSlice(locs), l.LocationList)
	})
}

func TestServerSetCursor(t *testing.T) {
	t.Run("calls underlying editor SetCursor", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)

		resource, err := workspace.ParseURI("file:///SetCursorer")
		require.NoError(t, err)
		content := "Oh my"
		nextID := uint32(12888)
		expectEdit(t, mock, resource, content)
		callServerEdit(t, ctx, broker, s, nextID, resource, content)

		pos := term.Coordinates{X: 4, Y: 5}
		mock.EXPECT().SetCursor(gomock.Any(), gomock.Eq(pos)).Return(nil).Times(1)

		var protoPos termpb.Coordinates
		protoPos.FromModel(pos)

		req := SetCursorRequest{HandlerId: nextID, Pos: &protoPos}
		res, err := s.SetCursor(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)
	})
}

func TestServerCursor(t *testing.T) {
	t.Run("calls underlying editor Cursor", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)

		resource, err := workspace.ParseURI("file:///Cursorer")
		require.NoError(t, err)
		nextID := uint32(12888)
		expectEdit(t, mock, resource, "")
		callServerEdit(t, ctx, broker, s, nextID, resource, "")

		pos := term.Coordinates{X: 4, Y: 5}
		mock.EXPECT().Cursor(gomock.Any()).Return(pos, nil).Times(1)

		req := CursorRequest{HandlerId: nextID}
		res, err := s.Cursor(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, pos, res.GetPos().ToModel())
	})
}
