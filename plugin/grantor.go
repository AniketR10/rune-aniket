package plugin

import (
	"fmt"
	"io"
	"sync"

	log "github.com/sirupsen/logrus"
	"unstable.build/go-tui/proto"
)

// ResourceRegistrar wraps the basic Serve method, to serve resources over a mux broker.
type ResourceRegistrar interface {
	Register(
		pluginID string, grantor Grantor,
		registar proto.ServiceRegistrar, broker proto.MuxBroker,
		locker sync.Locker) (io.Closer, error)
}

// enables functions matching signature of Serve to
// satisfy ResourceRegistrar
type resourceServerFn func(string, uint32,
	proto.MuxBroker, *log.Logger, sync.Locker)

func (fn resourceServerFn) Serve(
	pluginID string, grantID uint32,
	broker proto.MuxBroker, l *log.Logger, mu sync.Locker,
) {
	fn(pluginID, grantID, broker, l, mu)
}

// Grantor encapsulates the ability grant or deny access to resources.
type Grantor interface {
	Grant(plugin string, perm Permission) (ResourceRegistrar, bool)
}

type inmemoryGrantor struct {
	mu     sync.Mutex
	grants map[string]Permissions
	res    map[Permission]ResourceRegistrar
}

// NewInmemoryGrantor returns a Grantor that Grants according to the given
// grants and resources maps. This function panics if there's a Permission in
// grants that does not have a ResourceServe in res.
func NewInmemoryGrantor(
	grants map[string]Permissions,
	res map[Permission]ResourceRegistrar,
) Grantor {
	ret := new(inmemoryGrantor)
	ret.init(grants, res)
	return ret
}

func (m *inmemoryGrantor) init(
	grants map[string]Permissions,
	res map[Permission]ResourceRegistrar,
) {
	m.grants = grants
	m.res = res

	for _, pluginGrants := range m.grants {
		for grant := range pluginGrants {
			if _, ok := m.res[grant]; !ok {
				panic(fmt.Sprintf("grantor: resource registrar not found for grant: %s", grant))
			}
		}
	}
}

func (m *inmemoryGrantor) Grant(plugin string, perm Permission) (ResourceRegistrar, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	grant, ok := m.grants[plugin]
	if !ok {
		return nil, false
	}

	_, ok = grant[perm]
	if !ok {
		return nil, false
	}

	return m.res[perm], true
}

type grantAll struct {
	caps map[Permission]ResourceRegistrar
}

func (g *grantAll) Grant(plugin string, perm Permission) (ResourceRegistrar, bool) {
	srv, ok := g.caps[perm]
	return srv, ok
}

// GrantAll returns Grantor that Grants permission to all the given capabilities.
func GrantAll(capabilities map[Permission]ResourceRegistrar) Grantor {
	return &grantAll{caps: capabilities}
}
