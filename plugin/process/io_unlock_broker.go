package process

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"unstable.build/go-tui/proto"
)

// wraps a proto.MuxConn to provide unlocking a
// resource mutex while waiting for a I/O
type ioUnlockBroker struct {
	locker sync.Locker
	proto.MuxBroker
}

func newIOUnlockBroker(
	lock sync.Locker, broker proto.MuxBroker,
) proto.MuxBroker {
	ret := new(ioUnlockBroker)
	ret.init(lock, broker)
	return ret
}

func (b *ioUnlockBroker) init(lock sync.Locker, other proto.MuxBroker) {
	b.locker = lock
	b.MuxBroker = other
}

func (b *ioUnlockBroker) DialChannel(id string, tags ...string) (proto.MuxConn, error) {
	conn, err := b.MuxBroker.DialChannel(id, tags...)
	if err != nil {
		return nil, err
	}
	return ioUnlockConn{locker: b.locker, MuxConn: conn}, nil
}

type ioUnlockConn struct {
	locker sync.Locker
	proto.MuxConn
}

func (c ioUnlockConn) Invoke(
	ctx context.Context, method string, args interface{},
	reply interface{}, opts ...grpc.CallOption,
) error {
	c.locker.Unlock()
	defer c.locker.Lock()
	return c.MuxConn.Invoke(ctx, method, args, reply, opts...)
}

func (c ioUnlockConn) NewStream(
	ctx context.Context, desc *grpc.StreamDesc,
	method string, opts ...grpc.CallOption,
) (grpc.ClientStream, error) {
	c.locker.Unlock()
	defer c.locker.Lock()
	// NOTE: stream is not wrapped further, because there's
	// no guarantee that Send/Recv methods are called within
	// the main event loop.
	return c.MuxConn.NewStream(ctx, desc, method, opts...)
}
