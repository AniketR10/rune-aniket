package proto

import (
	context "context"
	"net"

	grpc "google.golang.org/grpc"
)

// wrapper around grpc.Server to satisfy MuxBroker
type grpcServer struct {
	*grpc.Server
	lis net.Listener
}

// GRPCServer returns a grpc.Server based MuxServer.
func GRPCServer(lis net.Listener, opts ...grpc.ServerOption) MuxServer {
	srv := grpc.NewServer(opts...)
	ret := &grpcServer{Server: srv, lis: lis}
	return ret
}

func (s *grpcServer) Registrar() ServiceRegistrar {
	return s.Server
}

func (s *grpcServer) Serve(ctx context.Context) error {
	return s.Server.Serve(s.lis)
}

func (s *grpcServer) Addr() net.Addr {
	return s.lis.Addr()
}
