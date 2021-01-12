package editor

import (
	"context"
	"errors"
	"testing"

	"github.com/ernestrc/go-tui/cell"
	"github.com/ernestrc/go-tui/proto"
	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

func newTestClient(ctrl *gomock.Controller) (
	*proto.MockMuxBroker, *proto.MockClientConnInterface, *Client,
) {
	broker := proto.NewMockMuxBroker(ctrl)
	cc := proto.NewMockClientConnInterface(ctrl)
	c := NewClient(broker, cc, nopLocker{})
	return broker, cc, c
}

func expectClientEdit(
	t *testing.T, mockCC *proto.MockClientConnInterface,
	expectedContent string,
) {
	mockCC.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/proto.Editor/Edit"),
			gomock.Any(),
			gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			editReq, ok := args.(*proto.EditRequest)
			require.True(t, ok)

			buf := proto.EditRequestToBuffer(editReq)
			assert.Equal(t, expectedContent, buf.String())

			_, ok = reply.(*proto.EditResponse)
			assert.True(t, ok)
			return nil
		}).
		Times(1)
}

func TestClientEdit(t *testing.T) {
	resourceName1 := "54-46 Was My Number"
	bufContent1 := "The Maytals"

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		_, cc, c := newTestClient(ctrl)

		buf := cell.NewBuffer()
		buf.WriteString(bufContent1)

		expectClientEdit(t, cc, bufContent1)

		h, err := c.Edit(resourceName1, buf)
		require.NoError(t, err)
		require.NotNil(t, h)
	})

	t.Run("handles underlying client error ", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		_, cc, c := newTestClient(ctrl)

		cc.EXPECT().
			Invoke(gomock.Any(),
				gomock.Eq("/proto.Editor/Edit"),
				gomock.Any(),
				gomock.Any()).
			Times(1).
			Return(errors.New("Would be a change of plan"))

		h, err := c.Edit(resourceName1, cell.NewBuffer())
		require.Error(t, err)
		require.Nil(t, h)
	})
}
