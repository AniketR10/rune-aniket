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
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/proto"
	prototest "unstable.build/go-tui/proto/test"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
	texttest "unstable.build/go-tui/text/test"
)

const asyncResultsSleepDuration = 300 * time.Millisecond
const locID = "errors"

type nopLocker struct{}

func (l nopLocker) Lock()   {}
func (l nopLocker) Unlock() {}

func newTestServer(t *testing.T, ctrl *gomock.Controller) (*proto.MockMuxBroker, *texttest.MockEditor, *Server) {
	broker := proto.NewMockMuxBroker(ctrl)
	ed := texttest.NewMockEditor(ctrl)
	s := NewServer(broker, ed, new(sync.Mutex), testBrowserServer{})
	return broker, ed, s
}

func expectEdit(t *testing.T, mock *texttest.MockEditor, resource workspaceapi.URI, content string) {
	mock.EXPECT().Edit(gomock.Any(), gomock.Any()).Times(1).
		DoAndReturn(func(_uri workspaceapi.URI, buf *cell.Buffer) (text.Handler, error) {
			assert.Equal(t, resource, _uri)
			assert.Equal(t, content, buf.String())
			return texttest.NewTestHandler(), nil
		})
}

func expectEditor(t *testing.T, mock *texttest.MockEditor, resource workspaceapi.URI) {
	mock.EXPECT().Editor(gomock.Any()).AnyTimes().
		DoAndReturn(func(_uri workspaceapi.URI) (text.Handler, error) {
			assert.Equal(t, resource, _uri)
			return texttest.NewTestHandler(), nil
		})
}

func callServerEdit(
	t *testing.T, ctx context.Context, broker *proto.MockMuxBroker,
	s *Server, uri workspaceapi.URI, content string,
) {
	buf := cell.NewBuffer()
	buf.WriteString(content)
	req := NewEditRequest(uri, buf)

	res, err := s.Edit(ctx, &req)
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestServerEdit(t *testing.T) {
	ctx := context.Background()
	resource, err := workspaceapi.ParseURI("file:///ULaptopNotLinux:@")
	require.NoError(t, err)
	bufContent1 := "ULaptopWillLinux:)"

	t.Run("Edit is propagated to underlying Editor", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		broker, mock, s := newTestServer(t, ctrl)
		expectEdit(t, mock, resource, bufContent1)
		callServerEdit(t, ctx, broker, s, resource, bufContent1)
	})

	t.Run("relative path is converted to absolute", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		broker, mock, s := newTestServer(t, ctrl)
		require.NoError(t, err)
		expectEdit(t, mock, resource, bufContent1)
		callServerEdit(t, ctx, broker, s, resource, bufContent1)
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
	h textapi.EventHandler, s *Server,
	broker *proto.MockMuxBroker,
) {
	resource := &texttest.TestHandler{}
	expectHandlerInvokeExit(t, handlerConn)

	handlerConn.EXPECT().Close().AnyTimes()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.editor.Lock()
	h.Handle(ctx, textapi.Event{Type: textapi.EventTypeClose, URI: uri, Resource: resource})
	s.editor.Unlock()
}

func TestServerSubscribe(t *testing.T) {
	ctx := context.Background()
	t.Run("propagates subscribe with multiple event types", func(t *testing.T) {
		channelID := "12"
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)
		expectedEvTypes := []textapi.EventType{textapi.EventTypeFlush, textapi.EventTypeFocus}

		var actualEvTypes []textapi.EventType
		var wg sync.WaitGroup
		mock.EXPECT().SubscribeEditorEvents(gomock.Any(), gomock.Any()).
			DoAndReturn(func(evs []textapi.EventType, _h textapi.EventHandler) error {
				defer wg.Done()
				actualEvTypes = evs
				return nil
			})
		req := EditorSubscribeRequest{
			ChannelId: channelID,
			Type: []EditorEvent_Type{
				EditorEvent_TypeFlush,
				EditorEvent_TypeFocus,
			},
		}
		conn := prototest.ExpectBrokerDialChannel(t, ctrl, broker, channelID)
		wg.Add(1)
		res, err := s.Subscribe(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)

		wg.Wait()

		// set to anytimes so finalizers can run after test is done
		conn.EXPECT().Close().AnyTimes()
		conn.EXPECT().Invoke(gomock.Any(), gomock.Any(),
			gomock.Any(), gomock.Any()).AnyTimes()

		assert.Equal(t, expectedEvTypes, actualEvTypes)
		s.editor.Lock()
		assert.NoError(t, s.Close())
		s.editor.Unlock()
	})
}

func assertEqualLocations(t *testing.T, loc, expected text.LocationList) {
	var locations, expectedLocations []textapi.Location
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

		channelID := "1234"
		conn := prototest.ExpectBrokerDialChannel(t, ctrl, broker, channelID)

		var wg sync.WaitGroup
		mock.EXPECT().SubscribeCommand(gomock.Any(), gomock.Any()).
			DoAndReturn(func(string, text.CommandHandler) error {
				wg.Done()
				return nil
			})

		wg.Add(1)
		req := RegisterCommandRequest{Command: "bla", ChannelId: channelID}
		res, err := s.Register(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)
		wg.Wait()

		conn.EXPECT().Close().AnyTimes()

		s.editor.Lock()
		assert.NoError(t, s.Close())
		s.editor.Unlock()
	})

	t.Run("handles SubscribeCommand errors", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)

		channelID := "1234"
		conn := prototest.ExpectBrokerDialChannel(t, ctrl, broker, channelID)

		var wg sync.WaitGroup
		mock.EXPECT().SubscribeCommand(gomock.Any(), gomock.Any()).
			DoAndReturn(func(string, text.CommandHandler) error {
				wg.Done()
				return errors.New("boom")
			})

		conn.EXPECT().Close().AnyTimes()

		wg.Add(1)
		req := RegisterCommandRequest{Command: "bla", ChannelId: channelID}
		res, err := s.Register(ctx, &req)
		require.Error(t, err)
		require.Nil(t, res)
		assert.Contains(t, err.Error(), "boom")
		wg.Wait()

		s.editor.Lock()
		assert.NoError(t, s.Close())
		s.editor.Unlock()
	})
}

func TestServerSetLocationList(t *testing.T) {
	resource, err := workspaceapi.ParseURI("file:///go-tui")
	require.NoError(t, err)

	t.Run("calls underlying editor SetLocationList", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)

		content := "main"
		expectEdit(t, mock, resource, content)
		callServerEdit(t, ctx, broker, s, resource, content)

		locs := text.LocationSlice([]textapi.Location{{Message: "wsb: hold AMC", To: term.Coordinates{X: 3}}})
		expectEditor(t, mock, resource)
		mock.EXPECT().SetLocationList(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1).
			DoAndReturn(func(h text.Handler, pri textapi.LocationPriority, ID string, l text.LocationList) error {
				assert.Equal(t, textapi.LocationPriorityInfo, pri)
				assert.Equal(t, locID, ID)
				return nil
			})

		req := makeLocationListRequest(resource, textapi.LocationPriorityInfo, locID, locs)
		res, err := s.SetLocationList(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)
	})

	t.Run("is threadsafe", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker := proto.NewMockMuxBroker(ctrl)
		ed := texttest.NopEditor()
		c, err := text.NewComponent(ed, &testLoader{}, text.Config{})
		require.NoError(t, err)
		s := NewServer(broker, c, new(sync.Mutex), testBrowserServer{})

		content := "main"
		callServerEdit(t, ctx, broker, s, resource, content)

		locs := []textapi.Location{
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

				req := makeLocationListRequest(resource, textapi.LocationPriorityWarning, locID, l)
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
	})
}

func TestServerSetCursor(t *testing.T) {
	t.Run("calls underlying editor SetCursor", func(t *testing.T) {
		ctx := context.Background()
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)

		resource, err := workspaceapi.ParseURI("file:///SetCursorer")
		require.NoError(t, err)
		content := "Oh my"
		expectEdit(t, mock, resource, content)
		callServerEdit(t, ctx, broker, s, resource, content)

		pos := term.Coordinates{X: 4, Y: 5}
		expectEditor(t, mock, resource)
		mock.EXPECT().SetCursor(gomock.Any(), gomock.Eq(pos)).Return(nil).Times(1)

		var protoPos termpb.Coordinates
		protoPos.FromModel(pos)

		req := SetCursorRequest{ResourceName: NewURI(resource), Pos: &protoPos}
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

		resource, err := workspaceapi.ParseURI("file:///Cursorer")
		require.NoError(t, err)
		expectEdit(t, mock, resource, "")
		callServerEdit(t, ctx, broker, s, resource, "")

		pos := term.Coordinates{X: 4, Y: 5}
		expectEditor(t, mock, resource)
		mock.EXPECT().Cursor(gomock.Any()).Return(pos, nil).Times(1)

		req := CursorRequest{ResourceName: NewURI(resource)}
		res, err := s.Cursor(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, pos, res.GetPos().ToModel())
	})
}
