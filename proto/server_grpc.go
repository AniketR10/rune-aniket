package proto

import (
	context "context"
	"net"
	"sync"

	grpc "google.golang.org/grpc"
)

// wrapper around grpc.Server to satisfy MuxBroker
type grpcServer struct {
	wg   sync.WaitGroup
	addr net.Addr
	*grpc.Server
}

// GRPCServer returns a grpc.Server based MuxServer.
func GRPCServer(opts ...grpc.ServerOption) MuxServer {
	srv := grpc.NewServer(opts...)
	ret := &grpcServer{Server: srv}
	ret.wg.Add(1)
	return ret
}

func (s *grpcServer) Registrar() ServiceRegistrar {
	return s.Server
}

func (s *grpcServer) Serve(ctx context.Context, lis net.Listener) error {
	s.addr = lis.Addr()
	s.wg.Done()
	return s.Server.Serve(lis)
}

func (s *grpcServer) Addr() net.Addr {
	s.wg.Wait()
	return s.addr
}
