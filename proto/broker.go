package proto

import (
	context "context"
	"io"
	"net"

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

type MuxServer interface {
	Serve(context.Context, net.Listener) error
	Registrar() ServiceRegistrar
	Addr() net.Addr
	Stop()
}

// MuxBroker allows a client or server to multiplex over connections.
type MuxBroker interface {
	NewChannel(tags ...string) (net.Listener, error)
	DialChannel(string, ...string) (MuxConn, error)

	Close() error
}

// ServiceRegistrar adds GetServiceInfo to grpc.ServiceRegistrar.
type ServiceRegistrar interface {
	GetServiceInfo() map[string]grpc.ServiceInfo
	grpc.ServiceRegistrar
}

// IsRegistered returns whether the given service is registered already
// with the given ServiceRegistrar. This is to avoid RegisterService panicking.
func IsRegistered(srv ServiceRegistrar, desc grpc.ServiceDesc) bool {
	_, ok := srv.GetServiceInfo()[desc.ServiceName]
	return ok
}
