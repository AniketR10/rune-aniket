package proto

import grpc "google.golang.org/grpc"

// MuxBroker allows a client or server to multiplex over connections.
type MuxBroker interface {
	NextId() uint32
	AcceptAndServe(ID uint32, srv func(opts []grpc.ServerOption) *grpc.Server)
	Dial(ID uint32) (conn *grpc.ClientConn, err error)
}
