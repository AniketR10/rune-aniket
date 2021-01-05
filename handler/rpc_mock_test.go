package handler

import (
	"context"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
	"github.com/ernestrc/go-tui/term"
	"google.golang.org/grpc"
)

type mockHandlerClient struct {
	handledCh chan term.Event
	rpcError  error
	remote    tui.Handler
}

func (c *mockHandlerClient) Handle(
	ctx context.Context, in *proto.HandleRequest, opts ...grpc.CallOption,
) (*proto.HandleResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}

	resp := new(proto.HandleResponse)
	ev, err := in.GetEvent().ToModel()
	if err != nil {
		return nil, err
	}

	width, height := int(in.GetDraw().GetWidth()), int(in.GetDraw().GetHeight())
	c.remote.Resize(width, height)
	pos, show := c.remote.Cursor()
	resp.Draw = proto.NewDrawResponse(c.remote, width, height)
	resp.Draw.Cursor = &proto.DrawResponse_Cursor{
		Position: &proto.Coordinates{
			X: int32(pos.X),
			Y: int32(pos.Y),
		},
		Show: show,
	}

	resp.Quit, resp.Handled = c.remote.Handle(ev)
	if c.handledCh != nil {
		c.handledCh <- ev
	}

	return resp, nil
}

func (c *mockHandlerClient) Man(
	ctx context.Context, in *proto.ManRequest, opts ...grpc.CallOption,
) (*proto.ManResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}

	man := c.remote.Man()
	protoMan := proto.Manual{}
	protoMan.FromModel(man)

	return &proto.ManResponse{Man: &protoMan}, nil
}

func (c *mockHandlerClient) OnUnmount(
	ctx context.Context, in *proto.OnUnmountRequest, opts ...grpc.CallOption,
) (*proto.OnUnmountResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}
	if unmounter, ok := c.remote.(interface{ OnUnmount() error }); ok {
		err := unmounter.OnUnmount()
		if err != nil {
			return nil, err
		}
	}
	return &proto.OnUnmountResponse{}, nil
}

func (c *mockHandlerClient) Close() error {
	return nil
}
