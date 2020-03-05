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

func (c *mockHandlerClient) Draw(
	ctx context.Context, in *proto.DrawRequest, opts ...grpc.CallOption,
) (*proto.DrawResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}

	c.remote.Resize(int(in.Width), int(in.Height))
	pos, show := c.remote.Cursor()
	res := proto.NewDrawResponse(c.remote, int(in.Width), int(in.Height))
	res.Cursor = &proto.DrawResponse_Cursor{
		Position: &proto.Coordinates{
			X: int32(pos.X),
			Y: int32(pos.Y),
		},
		Show: show,
	}
	return res, nil
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

func (c *mockHandlerClient) Close() error {
	return nil
}
