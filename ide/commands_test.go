package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommands(t *testing.T) {
	t.Run("all commands have a handler defined", func(t *testing.T) {
		for cmd, handler := range exCommands {
			assert.NotNil(t, handler.handler, cmd)
		}
	})
}
