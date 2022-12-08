package proto

import (
	context "context"
	"net"

	"github.com/ernestrc/blue/retry"
)

type retryBroker struct {
	b MuxBroker
	s retry.Strategy
}

// WithRetryBroker wraps a MuxBroker with retry.Strategy retries.
func WithRetryBroker(b MuxBroker, strategy retry.Strategy) MuxBroker {
	return retryBroker{b: b, s: strategy}
}

func (b retryBroker) NextId() uint32 {
	return b.b.NextId()
}

func (b retryBroker) Accept(id uint32) (lis net.Listener, err error) {
	ctx := context.Background()
	retry.Retry(ctx, b.s, func(ctx context.Context) (bool, error) {
		lis, err = b.b.Accept(id)
		return true, err
	})
	return
}

func (b retryBroker) Dial(ID uint32) (conn MuxConn, err error) {
	ctx := context.Background()
	retry.Retry(ctx, b.s, func(ctx context.Context) (bool, error) {
		conn, err = b.b.Dial(ID)
		return true, err
	})
	return
}

func (b retryBroker) Cleanup(ID uint32) error {
	return b.b.Cleanup(ID)
}

func (b retryBroker) NewChannel(tags ...string) (lis net.Listener, err error) {
	ctx := context.Background()
	retry.Retry(ctx, b.s, func(ctx context.Context) (bool, error) {
		lis, err = b.b.NewChannel(tags...)
		return true, err
	})
	return
}

func (b retryBroker) DialChannel(addr string) (conn MuxConn, err error) {
	ctx := context.Background()
	retry.Retry(ctx, b.s, func(ctx context.Context) (bool, error) {
		conn, err = b.b.DialChannel(addr)
		return true, err
	})
	return
}

func (b retryBroker) Close() error {
	return b.b.Close()
}
