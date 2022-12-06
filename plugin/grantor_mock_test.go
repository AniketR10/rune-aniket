package plugin

import (
	"io"
	"sync"

	"google.golang.org/grpc"
	"unstable.build/go-tui/proto"
)

type mockResourceServer struct {
	mu    sync.Mutex
	muxes []proto.MuxBroker
}

func (s *mockResourceServer) Register(
	pluginID string, g Grantor, grantor grpc.ServiceRegistrar,
	mux proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.muxes = append(s.muxes, mux)
	return nopCloser{}, nil
}

func (s *mockResourceServer) Close() error {
	return nil
}

type mockGrantor struct {
	mu   sync.Mutex
	srvs []*mockResourceServer
}

func (g *mockGrantor) servers() []*mockResourceServer {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.srvs
}

func (g *mockGrantor) Grant(plugin string, perm Permission) (ResourceRegistrar, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	server := &mockResourceServer{}
	g.srvs = append(g.srvs, server)

	return server, true
}
