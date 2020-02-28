package plugin

import (
	"fmt"
	"sync"

	"github.com/ernestrc/go-tui/proto"
)

// ResourceServer wraps the basic Serve method, to serve resources over a mux broker.
type ResourceServer interface {
	Serve(uint32, proto.MuxBroker)
}

// Grantor encapsulates the ability grant or deny access to resources.
type Grantor interface {
	Grant(plugin string, perm Permission) (ResourceServer, bool)
}

type inmemoryGrantor struct {
	mu     sync.Mutex
	grants map[string]Permissions
	res    map[Permission]ResourceServer
}

func NewInmemoryGrantor(
	grants map[string]Permissions,
	res map[Permission]ResourceServer,
) Grantor {
	ret := new(inmemoryGrantor)
	ret.init(grants, res)
	return ret
}

func (m *inmemoryGrantor) init(
	grants map[string]Permissions,
	res map[Permission]ResourceServer,
) {
	m.grants = grants
	m.res = res

	for _, pluginGrants := range m.grants {
		for grant := range pluginGrants {
			if _, ok := m.res[grant]; !ok {
				panic(fmt.Sprintf("inmemoryGrantor: ResourceServer not found for grant: %s", grant))
			}
		}
	}
}

func (m *inmemoryGrantor) Grant(plugin string, perm Permission) (ResourceServer, bool) {
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
