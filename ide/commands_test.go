package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWindowControlCommands(t *testing.T) {
	for cmd := range windowControlCommands {
		if cmd == cmdSwitchToWorkspace {
			continue // workspace handler command
		}
		_, ok := exCommands[cmd]
		assert.True(t, ok, cmd)
	}
}
