// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package browserrpc

import (
	context "context"
	"fmt"
	"net"
	"sync"
	"testing"

	gomock "go.uber.org/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
	"google.golang.org/grpc"
	browserapi "unstable.build/go-tui/api/browser"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/browser/browsertest"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/term"
)

func newClientServerIntegration(
	t *testing.T, h browser.Browser,
) (*Client, func()) {
	lis, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	broker := rpc.NewUnixGRPCBroker("", "", "")
	mutex := new(sync.Mutex)

	grpcServer := grpc.NewServer()
	rpcServer := NewServer(broker, h, mutex)
	rpcServer.SetSyncMode()
	RegisterWindowManagerServer(grpcServer, rpcServer)
	RegisterResourceOpenerServer(grpcServer, rpcServer)
	RegisterNotificationsServer(grpcServer, rpcServer)
	RegisterEventPublisherServer(grpcServer, rpcServer)

	go grpcServer.Serve(lis)

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure())
	require.NoError(t, err)

	client := NewClient(context.Background(), broker, conn)

	closeFn := func() {
		client.Close()
		rpcServer.browser.Lock()
		rpcServer.Stop()
		rpcServer.browser.Unlock()
		grpcServer.Stop()
		conn.Close()
	}

	return client, closeFn
}

func TestIntegrationSetFocus(t *testing.T) {
	defer goleak.VerifyNone(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	win1 := browsertest.NopWindow()
	win2 := browsertest.NopWindow()

	mock := browsertest.NewMockBrowser(ctrl)
	mock.EXPECT().Window(gomock.Any()).DoAndReturn(func(id uint64) (browser.Window, bool) {
		if id == 1 {
			return win1, true
		}
		return win2, true
	}).AnyTimes()
	client, cleanup := newClientServerIntegration(t, mock)
	defer cleanup()

	mock.EXPECT().Focus().Return(win1, nil)
	resWin1, err := client.Focus()
	require.NoError(t, err)

	mock.EXPECT().Split(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(o browserapi.Orientation,
			win browser.Window, h browserapi.Handler) (browser.Window, error) {
			return win2, win2.SetContent(h)
		})
	resWin2, err := client.Split(browserapi.OrientationDefault, resWin1, browsertest.NewTestHandler())
	require.NoError(t, err)

	mock.EXPECT().SetFocus(gomock.Any()).Return(win2, nil)
	resPrev, err := client.SetFocus(resWin1)
	require.NoError(t, err)
	assert.Equal(t, resWin2.(interface{ ID() uint64 }).ID(), resPrev.(interface{ ID() uint64 }).ID())

	mock.EXPECT().SetFocus(gomock.Any()).Return(win1, nil)
	resPrev, err = client.SetFocus(resWin2)
	require.NoError(t, err)
	assert.Equal(t, resWin1.(interface{ ID() uint64 }).ID(), resPrev.(interface{ ID() uint64 }).ID())
	require.NoError(t, resWin1.Close())
	require.NoError(t, resWin2.Close())
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

	for _, _tcase := range tsuite {
		tcase := _tcase
		t.Run(fmt.Sprintf("%v", tcase.Alignment), func(t *testing.T) {
			defer goleak.VerifyNone(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mock := browsertest.NewMockBrowser(ctrl)
			client, cleanup := newClientServerIntegration(t, mock)
			defer cleanup()

			win1 := browsertest.NopWindow()

			var wg sync.WaitGroup
			wg.Add(1)
			mock.EXPECT().Floating(gomock.Any(), gomock.Any()).
				DoAndReturn(func(h browser.Floating, cfg component.FloatingConfig) (browser.Window, error) {
					defer wg.Done()
					actualWidth, actualHeight := h.Dimensions()
					assert.Equal(t, 2, actualWidth)
					assert.Equal(t, 2, actualHeight)
					assert.Equal(t, tcase.Offset, cfg.Offset)
					assert.Equal(t, tcase.Alignment, cfg.Alignment)
					return win1, win1.SetContent(h)
				})
			resWin1, err := client.Floating(browsertest.NewTestFloating(2, 2), tcase)
			require.NoError(t, err)

			wg.Wait()
			mock.EXPECT().Window(gomock.Any()).Return(win1, true).AnyTimes()
			require.NoError(t, resWin1.Close())
		})
	}
}
