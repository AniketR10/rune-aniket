package editor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	prototest "github.com/ernestrc/go-tui/proto/test"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

const asyncResultsSleepDuration = 300 * time.Millisecond

type nopLocker struct{}

func (l nopLocker) Lock()   {}
func (l nopLocker) Unlock() {}

func newTestServer(t *testing.T, ctrl *gomock.Controller) (*proto.MockMuxBroker, *MockEditor, *Server) {
	broker := proto.NewMockMuxBroker(ctrl)
	ed := NewMockEditor(ctrl)
	expectInitialServerSubscribe(t, ed)
	s := NewServer(broker, ed, nopLocker{})
	return broker, ed, s
}

func expectEdit(t *testing.T, mock *MockEditor, resource, content string) {
	mock.EXPECT().Edit(gomock.Any(), gomock.Any()).Times(1).
		DoAndReturn(func(_name string, buf *cell.Buffer) (tui.Handler, error) {
			assert.Equal(t, resource, _name)
			assert.Equal(t, content, buf.String())
			return handler.NewTestHandler(), nil
		})
}

func TestServerEdit(t *testing.T) {
	nextID := uint32(99)
	ctx := context.Background()
	resourceName1 := "ULaptopNotLinux:@"
	bufContent1 := "ULaptopWillLinux:)"

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		broker, mock, s := newTestServer(t, ctrl)

		expectEdit(t, mock, resourceName1, bufContent1)
		broker.EXPECT().NextId().Return(nextID).Times(1)

		buf := cell.NewBuffer()
		buf.WriteString(bufContent1)
		req := proto.BufferToEditRequest(buf)
		req.ResourceName = resourceName1

		res, err := s.Edit(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)

		assert.Equal(t, nextID, res.GetHandlerId())
	})

	t.Run("bubbles up underlying's Editor Edit errors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		_, mock, s := newTestServer(t, ctrl)

		mock.EXPECT().Edit(gomock.Eq(resourceName1), gomock.Any()).
			Return(nil, errors.New("NOLINUX")).
			Times(1)

		req := proto.BufferToEditRequest(cell.NewBuffer())
		req.ResourceName = resourceName1

		res, err := s.Edit(ctx, &req)
		require.Error(t, err)
		require.Nil(t, res)
	})
}

func expectSubscribe(
	t *testing.T, mock *MockEditor, expectedType EventType, expectedHandler EventHandler,
) {
	mock.EXPECT().SubscribeEditor(gomock.Any(), gomock.Any()).Times(1).
		DoAndReturn(func(evType EventType, h EventHandler) error {
			assert.Equal(t, expectedType, evType)
			return nil
		})
}

func waitForMonitoringExit(quitCh chan struct{}) {
	<-quitCh
	time.Sleep(asyncResultsSleepDuration)
}

func expectHandlerInvokeExit(t *testing.T, handlerConn *proto.MockMuxConn) {
	handlerConn.EXPECT().
		Invoke(gomock.Any(), gomock.Eq("/proto.EditorEventHandler/Handle"),
			gomock.Any(),
			gomock.Any()).
		DoAndReturn(func(ctx context.Context,
			method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			res, ok := reply.(*proto.EditorEventHandleResponse)
			require.True(t, ok)

			res.Quit = true
			return nil
		}).
		Times(1)
}

func assertServerHandlerExitClose(
	t *testing.T, handlerConn *proto.MockMuxConn,
	h EventHandler, s *Server, quitCh chan struct{},
	broker *proto.MockMuxBroker,
) {
	resource := &handler.TestHandler{}
	name := "sup"
	ev := Event{Type: EventTypeClose, ResourceName: name, Resource: resource}
	expectHandlerInvokeExit(t, handlerConn)

	handlerConn.EXPECT().Close().Times(1).
		DoAndReturn(prototest.ExpectSignalExit(handlerConn, quitCh, nil))

	broker.EXPECT().NextId().Return(uint32(88)).Times(1)
	// force server to store resource name and make an ID
	s.Handle(Event{Type: EventTypeOpen, ResourceName: name, Resource: resource})

	s.editor.Lock()
	_ = h.Handle(ev)
	s.editor.Unlock()

	waitForMonitoringExit(quitCh)

	s.editor.Lock()
	defer s.editor.Unlock()
	assert.Equal(t, 0, len(s.clients))
}

func TestServerSubscribe(t *testing.T) {
	nextID := uint32(12)
	ctx := context.Background()

	t.Run("cleans resources when handler subscriber returns exit=true", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, mock, s := newTestServer(t, ctrl)
		evType := EventTypeFlush

		var h EventHandler
		mock.EXPECT().SubscribeEditor(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ev EventType, _h EventHandler) error {
				assert.Equal(t, evType, ev)
				h = _h
				return nil
			})
		conn := prototest.ExpectBrokerDial(t, ctrl, broker, nextID)
		quitCh := prototest.ExpectMonitorConn(conn)

		req := proto.EditorSubscribeRequest{
			HandlerId: nextID,
			Type:      proto.EditorEvent_TypeFlush,
		}

		res, err := s.Subscribe(ctx, &req)
		require.NoError(t, err)
		require.NotNil(t, res)

		assertServerHandlerExitClose(t, conn, h, s, quitCh, broker)
	})
}
