package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowControlCommands(t *testing.T) {
	t.Run("all control commands are defined", func(t *testing.T) {
		for cmd := range windowControlCommands {
			if cmd == cmdSwitchToWorkspace {
				continue // workspace handler command
			}
			_, ok := exCommands[cmd]
			assert.True(t, ok, cmd)
		}
	})

	t.Run("all commands have a handler defined", func(t *testing.T) {
		for cmd, handler := range exCommands {
			assert.NotNil(t, handler.handler, cmd)
		}
	})
}
