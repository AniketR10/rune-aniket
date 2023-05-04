package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/plugin"
)

var (
	grants = map[string]plugin.Permissions{
		"pluginA": {
			plugin.Permission("read"):  struct{}{},
			plugin.Permission("write"): struct{}{},
		},
		"pluginB": {
			plugin.Permission("read"): struct{}{},
		},
		"pluginC": {},
	}
	res = map[plugin.Permission]plugin.ResourceRegistrar{
		plugin.Permission("read"):  new(MockResourceServer),
		plugin.Permission("write"): new(MockResourceServer),
	}
)

func testGrantor(t *testing.T, g plugin.Grantor) {
	t.Run("should deny if plugin not in grant list", func(t *testing.T) {
		_, ok := g.Grant("shits", plugin.Permission("write"))
		assert.False(t, ok)
	})

	t.Run("should deny if permission not in list", func(t *testing.T) {
		_, ok := g.Grant("pluginA", plugin.Permission("append"))
		assert.False(t, ok)
	})

	t.Run("should deny if permission not in plugin's grant list", func(t *testing.T) {
		_, ok := g.Grant("pluginB", plugin.Permission("write"))
		assert.False(t, ok)

		_, ok = g.Grant("pluginC", plugin.Permission("read"))
		assert.False(t, ok)
	})

	t.Run("should grant if permission in list", func(t *testing.T) {
		res, ok := g.Grant("pluginA", plugin.Permission("read"))
		assert.True(t, ok)
		assert.NotNil(t, res)

		res, ok = g.Grant("pluginB", plugin.Permission("read"))
		assert.True(t, ok)
		assert.NotNil(t, res)

	})
}

func TestGrantor(t *testing.T) {
	g := plugin.NewInmemoryGrantor(grants, res)

	testGrantor(t, g)

	t.Run("should panic if trying to make a Grantor with mismatch of capabilities/grants", func(t *testing.T) {
		assert.Panics(t, func() {
			res := map[plugin.Permission]plugin.ResourceRegistrar{
				plugin.Permission("read"): new(MockResourceServer),
			}
			_ = plugin.NewInmemoryGrantor(grants, res)
		})
	})

}
