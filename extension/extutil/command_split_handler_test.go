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

package extutil

import (
	"context"
	"sync"
	"testing"

	"go.uber.org/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"unstable.build/go-tui"
	browserapi "unstable.build/go-tui/api/browser"
	browserapitest "unstable.build/go-tui/api/browser/browsertest"
	"unstable.build/go-tui/api/config"
	textapi "unstable.build/go-tui/api/text"
	"unstable.build/go-tui/browser/browsertest"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/rpc"
	"unstable.build/go-tui/rpc/rpctest"
	"unstable.build/go-tui/text/textrpc"
)

var emptyConfig = config.MapConfig(make(map[string]interface{}))

type handlerCloser struct {
	tui.Handler
	calledClosed int
}

func (h *handlerCloser) Close() error {
	h.calledClosed++
	return nil
}

func expectInitialization(
	t *testing.T, ctrl *gomock.Controller, h *cmdSplitHandler,
) *rpc.MockMuxBroker {
	broker := rpc.NewMockMuxBroker(ctrl)
	h.Connected(context.Background(), broker, emptyConfig)
	h.Health(context.Background())
	return broker
}

func expectSubscribe(
	t *testing.T, ctrl *gomock.Controller,
	broker *rpc.MockMuxBroker, cmd string, token string,
) *rpc.MockMuxConn {
	conn := rpctest.ExpectBrokerDial(t, ctrl, broker, token)
	rpctest.ExpectBrokerNewChannel(t, "1234", broker)
	cmdRpc := textrpc.CommandManual{Name: cmd}
	expected := textrpc.RegisterCommandRequest{ChannelId: "1234", Command: &cmdRpc}

	conn.EXPECT().
		Invoke(gomock.Any(),
			gomock.Eq("/text.Editor/Register"),
			gomock.Eq(&expected),
			gomock.Any()).
		Times(1)

	return conn
}

type nopConn struct {
}

func (n nopConn) GetState() connectivity.State {
	return connectivity.Ready
}

func (n nopConn) WaitForStateChange(ctx context.Context, sourceState connectivity.State) bool {
	return true
}

func (n nopConn) Invoke(ctx context.Context, method string, args interface{}, reply interface{}, opts ...grpc.CallOption) error {
	return nil
}

func (n nopConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

func (n nopConn) Close() error {
	return nil
}

func TestCommandSplitHandlerEmpty(t *testing.T) {

	t.Run("panics if cmd configuration is missing", func(t *testing.T) {
		assert.Panics(t, func() {
			NewCommandSplitHandler(CommandSplitHandlerConfig{})
		})
	})

	t.Run("grantee initialization runs correctly", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		config := CommandSplitHandlerConfig{}
		h := &cmdSplitHandler{config: config}

		expectInitialization(t, ctrl, h)
	})

	t.Run("subscribe to cmd event when subscriber permission is received", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		config := CommandSplitHandlerConfig{Command: testCommand("blah")}
		h := &cmdSplitHandler{config: config}
		broker := expectInitialization(t, ctrl, h)

		token := "123"
		conn := expectSubscribe(t, ctrl, broker, "blah", token)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		ctx = rpc.ContextWithWaitGroup(ctx, new(sync.WaitGroup))

		// called async waiting for ctx to be done
		conn.EXPECT().Close().AnyTimes()

		grants := []extension.Grant{
			{
				Token:      token,
				Permission: extension.Permission(extension.PermissionEditor),
				Context:    ctx,
			},
		}
		h.PermissionGranted(context.Background(), grants)
	})

	t.Run("does nothing if shutdown is called when window not active", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		config := CommandSplitHandlerConfig{Command: testCommand("blah")}
		h := &cmdSplitHandler{config: config}
		expectInitialization(t, ctrl, h)
		h.Shutdown(context.Background(), "you are being naughty")
	})
}

func TestCommandSplitHandlerOpenWindow(t *testing.T) {
	t.Run("open a split window if command received", func(t *testing.T) {
		cmdName := "blah"
		config := CommandSplitHandlerConfig{
			Command:          testCommand(cmdName),
			SplitOrientation: browserapi.OrientationLeft,
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		grants := extension.Grant{
			Token:      "1555",
			Permission: extension.Permission(extension.PermissionEditor),
			Context:    ctx,
		}
		testSplitWindow(t, config, grants, func(h *cmdSplitHandler) {
			err := h.HandleCommand(context.Background(), textapi.Command{Name: cmdName})
			require.NoError(t, err)
		})
	})
}

func testSplitWindow(
	t *testing.T, cfg CommandSplitHandlerConfig,
	grant extension.Grant, action func(*cmdSplitHandler),
) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg.Handler = func(_ context.Context, _ textapi.Command, grants []extension.Grant, broker rpc.MuxBroker,
		focus browserapi.Window, c config.Config) (browserapi.Handler, error) {
		return browsertest.NewTestHandler(), nil
	}
	mockWm := browserapitest.NewMockWindowManager(ctrl)
	h := &cmdSplitHandler{config: cfg, wm: mockWm}
	mockWm.EXPECT().
		Split(gomock.Eq(cfg.SplitOrientation), gomock.Any(), gomock.Any()).
		Return(nil, nil)
	action(h)
}

func testCommand(cmd string) textapi.CommandManual {
	return textapi.CommandManual{
		Name: cmd,
	}
}
