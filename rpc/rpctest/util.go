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

//revive:disable:exported
package rpctest

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"unstable.build/go-tui/rpc"
)

type testMuxServer struct {
	addr string
}

func (t testMuxServer) Stop() {
}

func (t testMuxServer) GracefulStop() {
}

func (t testMuxServer) Serve(context.Context) error {
	return errors.New("nope")
}

func (t testMuxServer) Registrar() rpc.ServiceRegistrar {
	return testRegistrar{}
}

func (t testMuxServer) Addr() net.Addr {
	return t
}

func (t testMuxServer) Network() string {
	return "test"
}

func (t testMuxServer) String() string {
	return t.addr
}

type testRegistrar struct {
}

func (r testRegistrar) GetServiceInfo() map[string]grpc.ServiceInfo {
	return nil
}

func (r testRegistrar) RegisterService(desc *grpc.ServiceDesc, impl interface{}) {
}

func ExpectBrokerServe(t *testing.T, brokerID string, mockBroker *rpc.MockMuxBroker) {
	mockBroker.EXPECT().NewChannel(gomock.Any()).
		DoAndReturn(func() (rpc.MuxServer, error) {
			return testMuxServer{addr: brokerID}, nil
		}).
		Times(1)
}

func ExpectMonitorConn(ret *rpc.MockMuxConn) chan struct{} {
	quitCh := make(chan struct{})
	ret.EXPECT().GetState().Return(connectivity.Ready).Times(1)
	ret.EXPECT().WaitForStateChange(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, sourceState connectivity.State) bool {
			select {
			case <-ctx.Done():
				return false
			case _, ok := <-quitCh:
				return ok
			}
		}).Times(1)

	return quitCh
}

func ExpectBrokerDial(
	t *testing.T, ctrl *gomock.Controller,
	mockBroker *rpc.MockMuxBroker, expectedBrokerID string,
) *rpc.MockMuxConn {
	ret := rpc.NewMockMuxConn(ctrl)

	mockBroker.EXPECT().DialChannel(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, brokerId string, tags ...string) (rpc.MuxConn, error) {
			assert.Equal(t, expectedBrokerID, brokerId)
			return ret, nil
		}).
		Times(1)

	return ret
}

func ExpectBrokerDialError(
	t *testing.T, ctrl *gomock.Controller,
	mockBroker *rpc.MockMuxBroker, expectedBrokerID string,
) {
	mockBroker.EXPECT().DialChannel(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("whoopsie")).
		Times(1)
}
func ExpectSignalExit(
	mockConn *rpc.MockMuxConn, quitCh chan struct{},
	returnErr error,
) func() error {
	return func() error {
		mockConn.EXPECT().GetState().Return(connectivity.Shutdown).AnyTimes()
		close(quitCh)
		return returnErr
	}
}

func ExpectBrokerDialChannel(
	t *testing.T, ctrl *gomock.Controller,
	mockBroker *rpc.MockMuxBroker, expectedChannelID string,
) *rpc.MockMuxConn {
	ret := rpc.NewMockMuxConn(ctrl)

	mockBroker.EXPECT().DialChannel(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, channelID string, tags ...string) (rpc.MuxConn, error) {
			assert.Equal(t, expectedChannelID, channelID)
			return ret, nil
		}).
		Times(1)

	return ret
}

func ExpectBrokerDialChannelError(
	t *testing.T, ctrl *gomock.Controller,
	mockBroker *rpc.MockMuxBroker, expectedBrokerID string,
) {
	mockBroker.EXPECT().DialChannel(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("whoopsie")).
		Times(1)
}

func ExpectBrokerNewChannel(t *testing.T, channelID string, mockBroker *rpc.MockMuxBroker) {
	mockBroker.EXPECT().NewChannel(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(tags ...string) (rpc.MuxServer, error) {
			return testMuxServer{addr: channelID}, nil
		}).
		Times(1)
}
