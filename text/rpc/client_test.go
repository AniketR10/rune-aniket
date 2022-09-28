package rpc

import (
	"context"
	"errors"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/proto"
	prototest "unstable.build/go-tui/proto/test"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/workspace"
)

var (
	loc1 = text.Location{
		To:   term.Coordinates{X: 1, Y: 3},
		Attr: term.Attributes{Fg: term.AttrBold},
	}
	loc2 = text.Location{
		From:    term.Coordinates{X: 1, Y: 3},
		Attr:    term.Attributes{Fg: term.ColorBlack, Bg: term.ColorGreen},
		Message: "wsb: hold BBBY",
	}
	loc3 = text.Location{}
)

func newTestClient(ctrl *gomock.Controller) (
	*proto.MockMuxBroker, *proto.MockClientConnInterface, *Client,
) {
	broker := proto.NewMockMuxBroker(ctrl)
	cc := proto.NewMockClientConnInterface(ctrl)
	c := NewClient(broker, cc)
	return broker, cc, c
}

func expectClientEdit(
	t *testing.T, mockCC *proto.MockClientConnInterface,
	expectedContent string, uri workspace.URI,
) {
	mockCC.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/text.Editor/Edit"),
			gomock.Any(),
			gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			editReq, ok := args.(*EditRequest)
			require.True(t, ok)

			buf := EditRequestToBuffer(editReq)
			assert.Equal(t, expectedContent, buf.String())
			assert.Equal(t, uri.String(), editReq.ResourceName.GetUri())

			_, ok = reply.(*EditResponse)
			assert.True(t, ok)
			return nil
		}).
		Times(1)
}

func TestClientEdit(t *testing.T) {
	bufContent1 := "The Maytals"

	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		_, cc, c := newTestClient(ctrl)
		uri, err := workspace.ParseURI("file:///tmp/hello")
		require.NoError(t, err)

		buf := cell.NewBuffer()
		buf.WriteString(bufContent1)

		expectClientEdit(t, cc, bufContent1, uri)

		h, err := c.Edit(uri, buf)
		require.NoError(t, err)
		require.NotNil(t, h)
	})

	t.Run("handles underlying client error ", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		_, cc, c := newTestClient(ctrl)

		cc.EXPECT().
			Invoke(gomock.Any(),
				gomock.Eq("/text.Editor/Edit"),
				gomock.Any(),
				gomock.Any()).
			Times(1).
			Return(errors.New("Would be a change of plan"))

		h, err := c.Edit(workspace.URI{}, cell.NewBuffer())
		require.Error(t, err)
		require.Nil(t, h)
	})
}

func expectClientSubscribe(
	t *testing.T, mockCC *proto.MockClientConnInterface,
	expectedHandlerID uint32,
	expectedEventTypes []text.EventType,
) {
	mockCC.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/text.Editor/Subscribe"),
			gomock.Any(),
			gomock.Any()).
		DoAndReturn(func(
			ctx context.Context, method string, args interface{},
			reply interface{}, opts ...grpc.CallOption) error {
			req, ok := args.(*EditorSubscribeRequest)
			require.True(t, ok)

			assert.Equal(t, expectedHandlerID, req.GetHandlerId())

			var expectedProtoTypes []EditorEvent_Type
			for _, ev := range expectedEventTypes {
				expectedProtoTypes = append(expectedProtoTypes, protoType(text.Event{Type: ev}))
			}
			assert.Equal(t, expectedProtoTypes, req.GetType())

			_, ok = reply.(*EditorSubscribeResponse)
			assert.True(t, ok)
			return nil
		}).
		Times(1)
}

func TestClientSubscribe(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		broker, cc, c := newTestClient(ctrl)
		handler := text.NewMockEventHandler(ctrl)

		brokerID := uint32(22)
		evTypes := []text.EventType{text.EventTypeFlush, text.EventTypeClose, text.EventTypeOpen}
		expectClientSubscribe(t, cc, brokerID, evTypes)

		prototest.ExpectBrokerServe(t, brokerID, broker)

		err := c.SubscribeEditorEvents(evTypes, handler)
		require.NoError(t, err)

		assert.NoError(t, c.Close())
	})
}

func TestSetLocationListRequest(t *testing.T) {
	t.Run("non-nil zero slice", func(t *testing.T) {
		l := text.LocationSlice([]text.Location{})
		handlerID := uint32(23)
		expected := SetLocationListRequest{
			HandlerId: handlerID,
			ListId:    locID,
			Locations: nil,
		}
		assert.Equal(t, expected, makeLocationListRequest(handlerID, locID, l))
	})

	t.Run("nil zero slice", func(t *testing.T) {
		l := text.LocationSlice(nil)
		handlerID := uint32(23)
		expected := SetLocationListRequest{
			HandlerId: handlerID,
			ListId:    locID,
			Locations: nil,
		}
		assert.Equal(t, expected, makeLocationListRequest(handlerID, locID, l))
	})
	t.Run("non-zero slice", func(t *testing.T) {
		l := text.LocationSlice([]text.Location{
			loc1,
			loc2,
			loc3,
		})
		handlerID := uint32(23)
		expected := SetLocationListRequest{
			HandlerId: handlerID,
			ListId:    locID,
			Locations: []*SetLocationListRequest_Location{
				{
					From: &termpb.Coordinates{},
					To:   &termpb.Coordinates{X: 1, Y: 3},
					Attr: &termpb.Attributes{Foreground: uint32(term.AttrBold)},
				},
				{
					To:   &termpb.Coordinates{},
					From: &termpb.Coordinates{X: 1, Y: 3},
					Attr: &termpb.Attributes{
						Foreground: uint32(term.ColorBlack),
						Background: uint32(term.ColorGreen),
					},
					Msg: "wsb: hold BBBY",
				},
				{
					From: &termpb.Coordinates{},
					To:   &termpb.Coordinates{},
					Attr: &termpb.Attributes{},
				},
			},
		}

		assert.Equal(t, expected, makeLocationListRequest(handlerID, locID, l))
	})
	t.Run("with message", func(t *testing.T) {
		myMsg := "wsb: HOLD GME"
		l := text.LocationSlice([]text.Location{
			{
				To:      term.Coordinates{X: 1, Y: 3},
				Attr:    term.Attributes{Fg: term.AttrBold},
				Message: myMsg,
			},
		})
		handlerID := uint32(23)
		expected := SetLocationListRequest{
			HandlerId: handlerID,
			ListId:    locID,
			Locations: []*SetLocationListRequest_Location{
				{
					From: &termpb.Coordinates{},
					To:   &termpb.Coordinates{X: 1, Y: 3},
					Attr: &termpb.Attributes{Foreground: uint32(term.AttrBold)},
					Msg:  myMsg,
				},
			},
		}

		assert.Equal(t, expected, makeLocationListRequest(handlerID, locID, l))
	})
}

func benchmarkSetLocationListRequest(b *testing.B, n int) {
	l := make([]text.Location, n)
	for i := 0; i < n; i++ {
		l[i] = text.Location{
			From: term.Coordinates{X: i, Y: n},
			To:   term.Coordinates{X: n, Y: n},
			Attr: term.Attributes{Fg: term.AttrBold},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ll := text.LocationSlice(l)
		_ = makeLocationListRequest(45, locID, ll)
	}
}

func BenchmarkSetLocationListRequest10(b *testing.B) {
	benchmarkSetLocationListRequest(b, 10)
}

func BenchmarkSetLocationListRequest100(b *testing.B) {
	benchmarkSetLocationListRequest(b, 100)
}

func BenchmarkSetLocationListRequest1000(b *testing.B) {
	benchmarkSetLocationListRequest(b, 1000)
}

func BenchmarkSetLocationListRequest10000(b *testing.B) {
	benchmarkSetLocationListRequest(b, 10000)
}
