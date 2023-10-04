package test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/extension"
)

var (
	grants = map[string]extension.Permissions{
		"extensionA": {
			extension.Permission("read"):  struct{}{},
			extension.Permission("write"): struct{}{},
		},
		"extensionB": {
			extension.Permission("read"): struct{}{},
		},
		"extensionC": {},
	}
	res = map[extension.Permission]extension.ResourceRegistrar{
		extension.Permission("read"):  new(MockResourceServer),
		extension.Permission("write"): new(MockResourceServer),
	}
)

func testGrantor(t *testing.T, g extension.Grantor) {
	t.Run("should deny if extension not in grant list", func(t *testing.T) {
		_, ok := g.Grant("shits", extension.Permission("write"))
		assert.False(t, ok)
	})

	t.Run("should deny if permission not in list", func(t *testing.T) {
		_, ok := g.Grant("extensionA", extension.Permission("append"))
		assert.False(t, ok)
	})

	t.Run("should deny if permission not in extension's grant list", func(t *testing.T) {
		_, ok := g.Grant("extensionB", extension.Permission("write"))
		assert.False(t, ok)

		_, ok = g.Grant("extensionC", extension.Permission("read"))
		assert.False(t, ok)
	})

	t.Run("should grant if permission in list", func(t *testing.T) {
		res, ok := g.Grant("extensionA", extension.Permission("read"))
		assert.True(t, ok)
		assert.NotNil(t, res)

		res, ok = g.Grant("extensionB", extension.Permission("read"))
		assert.True(t, ok)
		assert.NotNil(t, res)

	})
}

func TestGrantor(t *testing.T) {
	g := extension.NewInmemoryGrantor(grants, res)

	testGrantor(t, g)

	t.Run("should panic if trying to make a Grantor with mismatch of capabilities/grants", func(t *testing.T) {
		assert.Panics(t, func() {
			res := map[extension.Permission]extension.ResourceRegistrar{
				extension.Permission("read"): new(MockResourceServer),
			}
			_ = extension.NewInmemoryGrantor(grants, res)
		})
	})

}
