package proto

//go:generate mockgen -destination=./mux_gomock.go -package proto -self_package proto -source mux.go

import (
	"io"

	grpc "google.golang.org/grpc"
)

// MuxConn is a MuxBroker connection
type MuxConn interface {
	grpc.ClientConnInterface
	io.Closer
}

// MuxBroker allows a client or server to multiplex over connections.
type MuxBroker interface {
	NextId() uint32
	AcceptAndServe(ID uint32, srv func(opts []grpc.ServerOption) *grpc.Server)
	Dial(ID uint32) (conn MuxConn, err error)
	Close() error
}
