package rpc

import (
	"context"
	"errors"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/tcell/v3"
	"google.golang.org/grpc"
	textapi "unstable.build/go-tui/api/text"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
	termpb "unstable.build/go-tui/term/rpc"
	"unstable.build/go-tui/text"
)

var (
	loc1 = textapi.Location{
		To:   term.Coordinates{X: 1, Y: 3},
		Attr: term.Attributes{Attrs: tcell.AttrBold},
	}
	loc2 = textapi.Location{
		From:    term.Coordinates{X: 1, Y: 3},
		Attr:    term.Attributes{Fg: tcell.ColorBlack, Bg: tcell.ColorGreen},
		Message: "wsb: hold BBBY",
	}
	loc3 = textapi.Location{}
)

func newTestClient(ctrl *gomock.Controller) (
	*rpc.MockMuxBroker, *rpc.MockMuxConn, *Client,
) {
	broker := rpc.NewMockMuxBroker(ctrl)
	cc := rpc.NewMockMuxConn(ctrl)
	c := NewClient(context.Background(), broker, cc)
	// runtime finalizer calls close after test is done
	cc.EXPECT().Close().AnyTimes()
	return broker, cc, c
}

func expectClientEdit(
	t *testing.T, mockCC *rpc.MockMuxConn,
	expectedContent string, uri workspaceapi.URI,
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
		uri, err := workspaceapi.ParseURI("file:///tmp/hello")
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

		h, err := c.Edit(workspaceapi.URI{}, cell.NewBuffer())
		require.Error(t, err)
		require.Nil(t, h)
	})
}

func TestSetLocationListRequest(t *testing.T) {
	uri, err := workspaceapi.ParseURI("test:///")
	require.NoError(t, err)
	t.Run("non-nil zero slice", func(t *testing.T) {
		l := text.LocationSlice([]textapi.Location{})
		expected := SetLocationListRequest{
			ResourceName: NewURI(uri),
			ListId:       locID,
			Locations:    nil,
			Priority:     2,
		}
		assert.Equal(t, expected, makeLocationListRequest(uri, textapi.LocationPriorityError, locID, l))
	})

	t.Run("nil zero slice", func(t *testing.T) {
		l := text.LocationSlice(nil)
		expected := SetLocationListRequest{
			Priority:     2,
			ResourceName: NewURI(uri),
			ListId:       locID,
			Locations:    nil,
		}
		assert.Equal(t, expected, makeLocationListRequest(uri, textapi.LocationPriorityError, locID, l))
	})
	t.Run("non-zero slice", func(t *testing.T) {
		l := text.LocationSlice([]textapi.Location{
			loc1,
			loc2,
			loc3,
		})
		expected := SetLocationListRequest{
			ResourceName: NewURI(uri),
			ListId:       locID,
			Priority:     2,
			Locations: []*SetLocationListRequest_Location{
				{
					From: &termpb.Coordinates{},
					To:   &termpb.Coordinates{X: 1, Y: 3},
					Attr: &termpb.Attributes{Attrs: int64(tcell.AttrBold)},
				},
				{
					To:   &termpb.Coordinates{},
					From: &termpb.Coordinates{X: 1, Y: 3},
					Attr: &termpb.Attributes{
						Foreground: uint64(tcell.ColorBlack),
						Background: uint64(tcell.ColorGreen),
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

		assert.Equal(t, expected, makeLocationListRequest(uri, textapi.LocationPriorityError, locID, l))
	})
	t.Run("with message", func(t *testing.T) {
		myMsg := "wsb: HOLD GME"
		l := text.LocationSlice([]textapi.Location{
			{
				To:      term.Coordinates{X: 1, Y: 3},
				Attr:    term.Attributes{Attrs: tcell.AttrBold},
				Message: myMsg,
			},
		})
		expected := SetLocationListRequest{
			Priority:     2,
			ResourceName: NewURI(uri),
			ListId:       locID,
			Locations: []*SetLocationListRequest_Location{
				{
					From: &termpb.Coordinates{},
					To:   &termpb.Coordinates{X: 1, Y: 3},
					Attr: &termpb.Attributes{Attrs: int64(tcell.AttrBold)},
					Msg:  myMsg,
				},
			},
		}

		assert.Equal(t, expected, makeLocationListRequest(uri, textapi.LocationPriorityError, locID, l))
	})
}

func benchmarkSetLocationListRequest(b *testing.B, n int) {
	l := make([]textapi.Location, n)
	for i := 0; i < n; i++ {
		l[i] = textapi.Location{
			From: term.Coordinates{X: i, Y: n},
			To:   term.Coordinates{X: n, Y: n},
			Attr: term.Attributes{Attrs: tcell.AttrBold},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ll := text.LocationSlice(l)
		_ = makeLocationListRequest(uri, textapi.LocationPriorityError, locID, ll)
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
