package editor

import (
	"context"
	"errors"
	"testing"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/handler"
	"github.com/ernestrc/go-tui/proto"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nopLocker struct{}

func (l nopLocker) Lock()   {}
func (l nopLocker) Unlock() {}

func newTestServer(ctrl *gomock.Controller) (*proto.MockMuxBroker, *MockEditor, *Server) {
	broker := proto.NewMockMuxBroker(ctrl)
	ed := NewMockEditor(ctrl)
	// broker proto.MuxBroker, editor Editor, lock sync.Locker,
	// interruptDraw, interruptHandle func(),
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
		broker, mock, s := newTestServer(ctrl)

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
		_, mock, s := newTestServer(ctrl)

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
