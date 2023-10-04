package test

import (
	"io"
	"sync"

	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/proto"
)

var _ extension.ResourceRegistrar = (*MockResourceServer)(nil)

// MockResourceServer satisfies extension.ResourceRegistrar for testing.
type MockResourceServer struct {
	mu    sync.Mutex
	muxes []proto.MuxBroker
}

func (s *MockResourceServer) Register(
	extensionID string, g extension.Grantor, grantor proto.ServiceRegistrar,
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

// MockGrantor satisfies extension.Grantor for testing.
type MockGrantor struct {
	mu   sync.Mutex
	srvs []*MockResourceServer
}

func (g *MockGrantor) Servers() []*MockResourceServer {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.srvs
}

func (g *MockGrantor) Grant(extension string, perm extension.Permission) (extension.ResourceRegistrar, bool) {
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
