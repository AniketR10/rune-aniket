package test

import (
	"io"
	"sync"

	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/proto"
)

var _ plugin.ResourceRegistrar = (*MockResourceServer)(nil)

// MockResourceServer satisfies plugin.ResourceRegistrar for testing.
type MockResourceServer struct {
	mu    sync.Mutex
	muxes []proto.MuxBroker
}

func (s *MockResourceServer) Register(
	pluginID string, g plugin.Grantor, grantor proto.ServiceRegistrar,
	mux proto.MuxBroker, lock sync.Locker,
) (io.Closer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.muxes = append(s.muxes, mux)
	return nopCloser{}, nil
}

func (s *MockResourceServer) Muxes() []proto.MuxBroker {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.muxes
}

func (s *MockResourceServer) Close() error {
	return nil
}

// MockGrantor satisfies plugin.Grantor for testing.
type MockGrantor struct {
	mu   sync.Mutex
	srvs []*MockResourceServer
}

func (g *MockGrantor) Servers() []*MockResourceServer {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.srvs
}

func (g *MockGrantor) Grant(plugin string, perm plugin.Permission) (plugin.ResourceRegistrar, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	server := &MockResourceServer{}
	g.srvs = append(g.srvs, server)

	return server, true
}

type nopCloser struct {
}

func (c nopCloser) Close() error {
	return nil
}
