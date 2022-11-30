package browser

import (
	"context"

	browserpb "unstable.build/go-tui/browser/rpc"
	handlerpb "unstable.build/go-tui/handler/rpc"
)

var _ Floating = (*floatingClient)(nil)

type floatingClient struct {
	*handlerpb.Client
	fc            browserpb.FloatingClient
	errorCh       chan error
	width, height int
}

func newFloatingClient(handler *handlerpb.Client, fc browserpb.FloatingClient) *floatingClient {
	return &floatingClient{Client: handler, fc: fc, errorCh: make(chan error)}
}

func (f *floatingClient) Resize(width, height int) {
	f.width = width
	f.height = width
	f.Client.Resize(width, height)
}

func (f *floatingClient) Dimensions() (width, height int) {
	req := browserpb.DimensionsRequest{}
	ctx := context.Background()
	resp, err := f.fc.Dimensions(ctx, &req)
	if err != nil {
		select {
		case f.errorCh <- err:
		default:
		}
		// best effort
		return f.width, f.height
	}
	return int(resp.GetWidth()), int(resp.GetHeight())
}
