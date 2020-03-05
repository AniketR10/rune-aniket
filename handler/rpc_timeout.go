package handler

import (
	"context"
	"time"

	"github.com/ernestrc/go-tui/proto"
	"google.golang.org/grpc"
)

type clientTimeout struct {
	other      proto.HandlerClient
	rpcTimeout time.Duration
}

// withClientTimeout wraps a HandlerClient to provide an RPC
// clientTimeout circuit-breaking mechanism.
func withClientTimeout(
	client proto.HandlerClient, d time.Duration,
) proto.HandlerClient {
	ret := new(clientTimeout)
	ret.other = client
	ret.rpcTimeout = d
	return ret
}

func (b *clientTimeout) rpcWithTimeout(
	ctx context.Context, rpc func(ctx context.Context) (interface{}, error),
) (interface{}, error) {
	ctx, cancel := context.WithTimeout(ctx, b.rpcTimeout)
	defer cancel()

	var res interface{}
	var err error
	ch := make(chan struct{})
	defer close(ch)

	go func() {
		res, err = rpc(ctx)
		<-ch
	}()

	select {
	case ch <- struct{}{}:
		return res, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *clientTimeout) Draw(
	ctx context.Context, in *proto.DrawRequest, opts ...grpc.CallOption,
) (*proto.DrawResponse, error) {
	resIfc, err := b.rpcWithTimeout(ctx, func(ctx context.Context) (interface{}, error) {
		return b.other.Draw(ctx, in, opts...)
	})
	if err != nil {
		return nil, err
	}
	return resIfc.(*proto.DrawResponse), nil
}

func (b *clientTimeout) Handle(
	ctx context.Context, in *proto.HandleRequest, opts ...grpc.CallOption,
) (*proto.HandleResponse, error) {
	resIfc, err := b.rpcWithTimeout(ctx, func(ctx context.Context) (interface{}, error) {
		return b.other.Handle(ctx, in, opts...)
	})
	if err != nil {
		return nil, err
	}
	return resIfc.(*proto.HandleResponse), err
}

func (b *clientTimeout) Man(
	ctx context.Context, in *proto.ManRequest, opts ...grpc.CallOption,
) (*proto.ManResponse, error) {
	resIfc, err := b.rpcWithTimeout(ctx, func(ctx context.Context) (interface{}, error) {
		return b.other.Man(ctx, in, opts...)
	})
	if err != nil {
		return nil, err
	}
	return resIfc.(*proto.ManResponse), err
}
