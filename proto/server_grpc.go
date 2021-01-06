package proto

import grpc "google.golang.org/grpc"

// wrapper around grpc.Server to satisfy MuxBroker
type grpcServer struct {
	*grpc.Server
}

// GRPCServer returns a grpc.Server based MuxServer.
func GRPCServer(opts ...grpc.ServerOption) MuxServer {
	srv := grpc.NewServer(opts...)
	return grpcServer{srv}
}

func (s grpcServer) GRPC() *grpc.Server {
	return s.Server
}
