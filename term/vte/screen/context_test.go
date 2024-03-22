package screen

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsScreenContext(t *testing.T) {
	t.Run("returns false if context is not screen", func(t *testing.T) {
		assert.False(t, IsScreenContext(context.Background()))
	})
	t.Run("returns true if context is screen", func(t *testing.T) {
		ctx := screenContext(context.Background())
		assert.True(t, IsScreenContext(ctx))
	})
	t.Run("returns true if context double set screen", func(t *testing.T) {
		ctx := screenContext(screenContext(context.Background()))
		assert.True(t, IsScreenContext(ctx))
	})
}
