package browser

import (
	"fmt"
	"net"
	"sync"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	browserpb "unstable.build/go-tui/browser/rpc"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
)

func newClientServerIntegration(
	t *testing.T, h Browser,
) (*Client, func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	broker := proto.NewDialBroker()
	mutex := new(sync.Mutex)

	grpcServer := grpc.NewServer()
	rpcServer := NewServer(broker, h, mutex)
	browserpb.RegisterWindowManagerServer(grpcServer, rpcServer)
	browserpb.RegisterResourceOpenerServer(grpcServer, rpcServer)
	browserpb.RegisterMessengerServer(grpcServer, rpcServer)
	browserpb.RegisterEventPublisherServer(grpcServer, rpcServer)

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	client := NewClient(broker, conn)

	closeFn := func() {
		client.Close()
		rpcServer.browser.Lock()
		rpcServer.Close()
		rpcServer.browser.Unlock()
		grpcServer.Stop()
		conn.Close()
	}

	return client, closeFn
}

func TestIntegrationSetFocus(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mock := NewMockBrowser(ctrl)
	client, cleanup := newClientServerIntegration(t, mock)
	defer cleanup()

	win1 := NopWindow()
	win2 := NopWindow()

	mock.EXPECT().Focus().Return(win1, nil)
	resWin1, err := client.Focus()
	require.NoError(t, err)

	mock.EXPECT().Split(gomock.Any(), gomock.Any()).Return(win2, nil)
	resWin2, err := client.Split(OrientationDefault, nil)
	require.NoError(t, err)

	mock.EXPECT().SetFocus(gomock.Any()).Return(win2, nil)
	resPrev, err := client.SetFocus(resWin1)
	require.NoError(t, err)
	assert.Equal(t, resWin2, resPrev)

	mock.EXPECT().SetFocus(gomock.Any()).Return(win1, nil)
	resPrev, err = client.SetFocus(resWin2)
	require.NoError(t, err)
	assert.Equal(t, resWin1, resPrev)
}

func TestIntegrationFloating(t *testing.T) {
	tsuite := []component.FloatingConfig{
		{Offset: term.Coordinates{X: 1, Y: 1}},
		{Alignment: component.SpanAlignmentLeft},
		{Alignment: component.SpanAlignmentRight},
		{Alignment: component.SpanAlignmentTop},
		{Alignment: component.SpanAlignmentBottom},
		{Alignment: component.SpanAlignmentTop | component.SpanAlignmentLeft},
		{Alignment: component.SpanAlignmentTop | component.SpanAlignmentRight},
		{Alignment: component.SpanAlignmentBottom | component.SpanAlignmentRight},
		{Alignment: component.SpanAlignmentBottom | component.SpanAlignmentLeft},
		{Alignment: component.SpanAlignmentVerticallyCentered},
		{Alignment: component.SpanAlignmentHorizontallyCentered},
		{Alignment: component.SpanAlignmentCentered},
	}

	for _, tcase := range tsuite {
		t.Run(fmt.Sprintf("%v", tcase.Alignment), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mock := NewMockBrowser(ctrl)
			client, cleanup := newClientServerIntegration(t, mock)
			defer cleanup()

			win1 := NopWindow()

			mock.EXPECT().Floating(gomock.Any(), gomock.Any()).
				DoAndReturn(func(h Floating, cfg component.FloatingConfig) (Window, error) {
					actualWidth, actualHeight := h.Dimensions()
					assert.Equal(t, 2, actualWidth)
					assert.Equal(t, 2, actualHeight)
					assert.Equal(t, tcase.Offset, cfg.Offset)
					assert.Equal(t, tcase.Alignment, cfg.Alignment)
					return win1, nil
				})
			resWin1, err := client.Floating(NewTestFloating(2, 2), tcase)
			require.NoError(t, err)
			require.NoError(t, resWin1.Close())
		})
	}
}
