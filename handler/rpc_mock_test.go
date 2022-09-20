package handler

import (
	"context"
	"io"

	"github.com/ernestrc/go-tui"
	handlerpb "github.com/ernestrc/go-tui/handler/rpc"
	"github.com/ernestrc/go-tui/term"
	termpb "github.com/ernestrc/go-tui/term/rpc"
	"google.golang.org/grpc"
)

type mockHandlerClient struct {
	handledCh chan term.Event
	quitCh    chan struct{}
	rpcError  error
	remote    tui.Handler
}

func (c *mockHandlerClient) Handle(
	ctx context.Context, in *handlerpb.HandleRequest, opts ...grpc.CallOption,
) (*handlerpb.HandleResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}

	resp := new(handlerpb.HandleResponse)
	ev, err := in.GetEvent().ToModel()
	if err != nil {
		return nil, err
	}

	width, height := int(in.GetDraw().GetWidth()), int(in.GetDraw().GetHeight())
	c.remote.Resize(width, height)
	pos, show := c.remote.Cursor()
	resp.Draw = handlerpb.NewDrawResponse(c.remote, width, height)
	resp.Draw.Cursor = &handlerpb.DrawResponse_Cursor{
		Position: &termpb.Coordinates{
			X: int32(pos.X),
			Y: int32(pos.Y),
		},
		Show: show,
	}

	resp.Quit, resp.Handled = c.remote.Handle(ev)
	if c.handledCh != nil {
		select {
		case c.handledCh <- ev:
		case <-c.quitCh:
		}
	}

	return resp, nil
}

func (c *mockHandlerClient) Man(
	ctx context.Context, in *handlerpb.ManRequest, opts ...grpc.CallOption,
) (*handlerpb.ManResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}

	man := c.remote.Man()
	protoMan := termpb.Manual{}
	protoMan.FromModel(man)

	return &handlerpb.ManResponse{Man: &protoMan}, nil
}

func (c *mockHandlerClient) Close(
	ctx context.Context, in *handlerpb.CloseRequest, opts ...grpc.CallOption,
) (*handlerpb.CloseResponse, error) {
	if c.rpcError != nil {
		return nil, c.rpcError
	}
	if closer, ok := c.remote.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return nil, err
		}
	}
	return &handlerpb.CloseResponse{}, nil
}
