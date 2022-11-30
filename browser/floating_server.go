package browser

import (
	"context"

	browserpb "unstable.build/go-tui/browser/rpc"
)

var _ browserpb.FloatingServer = floatingServer{}

type floatingServer struct {
	browserpb.UnimplementedFloatingServer
	f Floating
}

func newFloatingServer(f Floating) floatingServer {
	return floatingServer{f: f}
}

func (f floatingServer) Dimensions(ctx context.Context, req *browserpb.DimensionsRequest) (
	*browserpb.DimensionsResponse, error,
) {
	width, height := f.f.Dimensions()
	res := new(browserpb.DimensionsResponse)
	res.Width = uint32(width)
	res.Height = uint32(height)
	return res, nil
}
