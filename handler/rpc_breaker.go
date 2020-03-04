package handler

import (
	"context"
	"fmt"
	"time"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/component"
	"github.com/ernestrc/go-tui/proto"
	"google.golang.org/grpc"
)

const connectionLostCopy = `
          ___
         /___/\_               
        _\   \/_/\__           
      __\       \/_/\          
      \   __    __ \ \         
     __\  \_\   \_\ \ \   __   
    /_/\\   __   __  \ \_/_/\  
    \_\/_\__\/\__\/\__\/_\_\/  
       \_\/_/\       /_\_\/    
          \_\/       \_\/      
    

Connection to plugin was lost:
%.30s...
`

type clientBreaker struct {
	width, height int
	other         proto.HandlerClient
	rpcTimeout    time.Duration
}

// clientBreaker wraps a HandlerClient to provide an RPC
// clientBreaker circuit-breaking mechanism.
func withClientBreaker(
	client proto.HandlerClient, d time.Duration,
) proto.HandlerClient {
	ret := new(clientBreaker)
	ret.other = client
	ret.rpcTimeout = d
	return ret
}

func (b *clientBreaker) rpcWithTimeout(
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

func (b *clientBreaker) Resize(
	ctx context.Context, in *proto.ResizeRequest, opts ...grpc.CallOption,
) (*proto.ResizeResponse, error) {
	b.width = int(in.Width)
	b.height = int(in.Height)
	resIfc, err := b.rpcWithTimeout(ctx, func(ctx context.Context) (interface{}, error) {
		return b.other.Resize(ctx, in, opts...)
	})
	if err != nil {
		return nil, err
	}
	return resIfc.(*proto.ResizeResponse), err
}

func (b *clientBreaker) makeSadFaceComponent(err error) tui.Component {
	comp := component.String(fmt.Sprintf(connectionLostCopy, err))
	comp.Resize(b.width, b.height)
	return comp
}

func (b *clientBreaker) Draw(
	ctx context.Context, in *proto.DrawRequest, opts ...grpc.CallOption,
) (*proto.DrawResponse, error) {
	resIfc, err := b.rpcWithTimeout(ctx, func(ctx context.Context) (interface{}, error) {
		return b.other.Draw(ctx, in, opts...)
	})
	if err != nil {
		sadFace := b.makeSadFaceComponent(err)
		return proto.NewDrawResponse(sadFace, b.width, b.height), nil
	}
	return resIfc.(*proto.DrawResponse), nil
}

func (b *clientBreaker) Handle(
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

func (b *clientBreaker) Cursor(
	ctx context.Context, in *proto.CursorRequest, opts ...grpc.CallOption,
) (*proto.CursorResponse, error) {
	resIfc, err := b.rpcWithTimeout(ctx, func(ctx context.Context) (interface{}, error) {
		return b.other.Cursor(ctx, in, opts...)
	})
	if err != nil {
		return nil, err
	}
	return resIfc.(*proto.CursorResponse), err
}

func (b *clientBreaker) Man(
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
