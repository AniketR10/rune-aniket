package plugin

import (
	"fmt"
	"sync"
)

// CachingGrantor wraps g and returns a Grantor that returns the same ResourceRegistrar
// for a given pluginID and permission combination.
func CachingGrantor(g Grantor) Grantor {
	return &cacheGrantor{g: g, grants: make(map[string]ResourceRegistrar)}
}

type cacheGrantor struct {
	mu     sync.Mutex
	g      Grantor
	grants map[string]ResourceRegistrar
}

func (c *cacheGrantor) Grant(
	plugin string, perm Permission,
) (ResourceRegistrar, bool) {
	key := fmt.Sprintf("%s.%s", plugin, perm)

	c.mu.Lock()
	defer c.mu.Unlock()

	if cached, ok := c.grants[key]; ok {
		return cached, true
	}
	ret, ok := c.g.Grant(plugin, perm)
	if ok {
		c.grants[key] = ret
	}
	return ret, ok
}
