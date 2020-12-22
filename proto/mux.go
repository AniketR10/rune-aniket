package proto

//go:generate mockgen -destination=./mux_gomock.go -package proto -self_package proto -source mux.go

import (
	context "context"
	"io"

	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
)

// MuxConn is a MuxBroker connection
type MuxConn interface {
	grpc.ClientConnInterface
	GetState() connectivity.State
	WaitForStateChange(ctx context.Context, sourceState connectivity.State) bool
	io.Closer
}

// MuxBroker allows a client or server to multiplex over connections.
type MuxBroker interface {
	NextId() uint32
	AcceptAndServe(ID uint32, srv func(opts []grpc.ServerOption) *grpc.Server)
	Dial(ID uint32) (conn MuxConn, err error)
	Close() error
}
