package plugin

import (
	"sync"

	"github.com/ernestrc/go-tui/proto"
)

type mockResourceServer struct {
	mu    sync.Mutex
	srvd  []uint32
	muxes []proto.MuxBroker
}

func (s *mockResourceServer) Serve(uid uint32, mux proto.MuxBroker) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.muxes = append(s.muxes, mux)
	s.srvd = append(s.srvd, uid)
}

func (s *mockResourceServer) served() []uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.srvd
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

func (g *mockGrantor) Grant(plugin string, perm Permission) (ResourceServer, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	server := &mockResourceServer{}
	g.srvs = append(g.srvs, server)

	return server, true
}
