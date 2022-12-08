package prototest

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/connectivity"
	"unstable.build/go-tui/proto"
)

type testListener struct {
	addr string
}

func (t testListener) Accept() (net.Conn, error) {
	return nil, errors.New("nope")
}

func (t testListener) Close() error {
	return nil
}

func (t testListener) Addr() net.Addr {
	return t
}

func (t testListener) Network() string {
	return "test"
}
func (t testListener) String() string {
	return t.addr
}

func ExpectBrokerServe(t *testing.T, brokerID uint32, mockBroker *proto.MockMuxBroker) {
	mockBroker.EXPECT().NextId().Return(uint32(brokerID))
	mockBroker.EXPECT().Accept(gomock.Any()).
		DoAndReturn(func(brokerId uint32) (net.Listener, error) {
			assert.Equal(t, uint32(brokerID), brokerId)
			return testListener{}, nil
		}).
		Times(1)
}

func ExpectMonitorConn(ret *proto.MockMuxConn) chan struct{} {
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
	mockBroker *proto.MockMuxBroker, expectedBrokerID uint32,
) *proto.MockMuxConn {
	ret := proto.NewMockMuxConn(ctrl)

	mockBroker.EXPECT().Dial(gomock.Any()).
		DoAndReturn(func(brokerId uint32) (proto.MuxConn, error) {
			assert.Equal(t, expectedBrokerID, brokerId)
			return ret, nil
		}).
		Times(1)

	return ret
}

func ExpectBrokerDialError(
	t *testing.T, ctrl *gomock.Controller,
	mockBroker *proto.MockMuxBroker, expectedBrokerID uint32,
) {
	mockBroker.EXPECT().Dial(gomock.Any()).
		Return(nil, errors.New("whoopsie")).
		Times(1)
}
func ExpectSignalExit(
	mockConn *proto.MockMuxConn, quitCh chan struct{},
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
	mockBroker *proto.MockMuxBroker, expectedChannelID string,
) *proto.MockMuxConn {
	ret := proto.NewMockMuxConn(ctrl)

	mockBroker.EXPECT().DialChannel(gomock.Any()).
		DoAndReturn(func(channelID string) (proto.MuxConn, error) {
			assert.Equal(t, expectedChannelID, channelID)
			return ret, nil
		}).
		Times(1)

	return ret
}

func ExpectBrokerDialChannelError(
	t *testing.T, ctrl *gomock.Controller,
	mockBroker *proto.MockMuxBroker, expectedBrokerID uint32,
) {
	mockBroker.EXPECT().DialChannel(gomock.Any()).
		Return(nil, errors.New("whoopsie")).
		Times(1)
}

func ExpectBrokerNewChannel(t *testing.T, channelID string, mockBroker *proto.MockMuxBroker) {
	mockBroker.EXPECT().NewChannel(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(tags ...string) (net.Listener, error) {
			return testListener{addr: channelID}, nil
		}).
		Times(1)
}
