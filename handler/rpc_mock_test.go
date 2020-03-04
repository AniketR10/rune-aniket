package handler

import (
	"context"

	"github.com/ernestrc/go-tui"
	"github.com/ernestrc/go-tui/proto"
	"google.golang.org/grpc"
)

type mockHandlerClient struct {
	cursorError error
	remote      tui.Handler
}

func (c *mockHandlerClient) Draw(
	ctx context.Context, in *proto.DrawRequest, opts ...grpc.CallOption,
) (*proto.DrawResponse, error) {
	c.remote.Resize(int(in.Width), int(in.Height))
	return proto.NewDrawResponse(c.remote, int(in.Width), int(in.Height)), nil
}

func (c *mockHandlerClient) Handle(
	ctx context.Context, in *proto.HandleRequest, opts ...grpc.CallOption,
) (*proto.HandleResponse, error) {
	resp := new(proto.HandleResponse)
	ev, err := in.GetEvent().ToModel()
	if err != nil {
		return nil, err
	}
	resp.Quit, resp.Handled = c.remote.Handle(ev)
	return resp, nil
}

func (c *mockHandlerClient) Cursor(
	ctx context.Context, in *proto.CursorRequest, opts ...grpc.CallOption,
) (*proto.CursorResponse, error) {
	pos, show := c.remote.Cursor()
	if c.cursorError != nil {
		return nil, c.cursorError
	}

	protoPos := proto.Coordinates{}
	protoPos.FromModel(pos)

	return &proto.CursorResponse{Position: &protoPos, Show: show}, nil
}

func (c *mockHandlerClient) Man(
	ctx context.Context, in *proto.ManRequest, opts ...grpc.CallOption,
) (*proto.ManResponse, error) {
	man := c.remote.Man()

	protoMan := proto.Manual{}
	protoMan.FromModel(man)

	return &proto.ManResponse{Man: &protoMan}, nil
}
