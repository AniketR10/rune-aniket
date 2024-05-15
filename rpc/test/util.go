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
