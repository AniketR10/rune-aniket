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
	retry.Retry(ctx, b.s, func(ctx context.Context) bool {
		lis, err = b.b.Accept(id)
		if err != nil {
			return true
		}
		return false
	})
	return
}

func (b retryBroker) Dial(ID uint32) (conn MuxConn, err error) {
	ctx := context.Background()
	retry.Retry(ctx, b.s, func(ctx context.Context) bool {
		conn, err = b.b.Dial(ID)
		if err != nil {
			return true
		}
		return false
	})
	return
}

func (b retryBroker) Close() (err error) {
	return b.b.Close()
}
