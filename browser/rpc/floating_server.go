package rpc

import (
	"context"

	"unstable.build/go-tui/browser"
)

var _ FloatingServer = floatingServer{}

type floatingServer struct {
	UnimplementedFloatingServer
	f browser.Floating
}

func newFloatingServer(f browser.Floating) floatingServer {
	return floatingServer{f: f}
}

func (f floatingServer) Dimensions(ctx context.Context, req *DimensionsRequest) (
	*DimensionsResponse, error,
) {
	width, height := f.f.Dimensions()
	res := new(DimensionsResponse)
	res.Width = uint32(width)
	res.Height = uint32(height)
	return res, nil
}
