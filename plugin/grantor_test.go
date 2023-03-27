package plugin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	grants = map[string]Permissions{
		"pluginA": {
			Permission("read"):  struct{}{},
			Permission("write"): struct{}{},
		},
		"pluginB": {
			Permission("read"): struct{}{},
		},
		"pluginC": {},
	}
	res = map[Permission]ResourceRegistrar{
		Permission("read"):  new(mockResourceServer),
		Permission("write"): new(mockResourceServer),
	}
)

func testGrantor(t *testing.T, g Grantor) {
	t.Run("should deny if plugin not in grant list", func(t *testing.T) {
		_, ok := g.Grant("shits", Permission("write"))
		assert.False(t, ok)
	})

	t.Run("should deny if permission not in list", func(t *testing.T) {
		_, ok := g.Grant("pluginA", Permission("append"))
		assert.False(t, ok)
	})

	t.Run("should deny if permission not in plugin's grant list", func(t *testing.T) {
		_, ok := g.Grant("pluginB", Permission("write"))
		assert.False(t, ok)

		_, ok = g.Grant("pluginC", Permission("read"))
		assert.False(t, ok)
	})

	t.Run("should grant if permission in list", func(t *testing.T) {
		res, ok := g.Grant("pluginA", Permission("read"))
		assert.True(t, ok)
		assert.NotNil(t, res)

		res, ok = g.Grant("pluginB", Permission("read"))
		assert.True(t, ok)
		assert.NotNil(t, res)

	})
}

func TestGrantor(t *testing.T) {
	g := NewInmemoryGrantor(grants, res)

	testGrantor(t, g)

	t.Run("should panic if trying to make a Grantor with mismatch of capabilities/grants", func(t *testing.T) {
		assert.Panics(t, func() {
			res := map[Permission]ResourceRegistrar{
				Permission("read"): new(mockResourceServer),
			}
			_ = NewInmemoryGrantor(grants, res)
		})
	})

}
