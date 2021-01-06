package proto

import (
	context "context"

	log "github.com/sirupsen/logrus"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

type loggingConn struct {
	logger *log.Logger
	MuxConn
}

func (c loggingConn) Invoke(
	ctx context.Context, method string, args interface{},
	reply interface{}, opts ...grpc.CallOption,
) error {
	err := c.MuxConn.Invoke(ctx, method, args, reply, opts...)
	c.logger.Tracef("loggingConn: (%p).Invoke(method=%s): (%v)",
		c.MuxConn, method, err)
	return err
}

func (c loggingConn) NewStream(
	ctx context.Context, desc *grpc.StreamDesc,
	method string, opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	stream, err := c.MuxConn.NewStream(ctx, desc, method, opts...)
	c.logger.Tracef("loggingConn: (%p).NewStream(name=%s, method=%s): (%v, %v)",
		c.MuxConn, desc.StreamName, method, stream, err)
	return stream, err
}

func (c loggingConn) GetState() connectivity.State {
	state := c.MuxConn.GetState()
	c.logger.Tracef("loggingConn: (%p).GetState(): %s", c.MuxConn, state)
	return state
}

func (c loggingConn) WaitForStateChange(
	ctx context.Context, sourceState connectivity.State,
) bool {
	ok := c.MuxConn.WaitForStateChange(ctx, sourceState)
	c.logger.Tracef("loggingConn: (%p).WaitForStateChange(source=%s): %v",
		c.MuxConn, sourceState, ok)
	return ok
}

func (c loggingConn) Close() error {
	err := c.MuxConn.Close()
	c.logger.Tracef("loggingConn: (%p).Close(): %v", c.MuxConn, err)
	return err
}
