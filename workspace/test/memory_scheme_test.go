package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

func TestMemoryScheme(t *testing.T) {
	t.Run("at root path", func(t *testing.T) {
		TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
			uri, err := workspace.ParseURI("memory:///")
			require.NoError(t, err)
			mem, err := workspace.NewMemoryScheme(config.NopConfig(), uri)
			require.NoError(t, err)
			return mem
		})
	})
	t.Run("at nested path", func(t *testing.T) {
		TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
			uri, err := workspace.ParseURI("memory:///var/log")
			require.NoError(t, err)
			mem, err := workspace.NewMemoryScheme(config.NopConfig(), uri)
			require.NoError(t, err)
			return mem
		})
	})
	t.Run("at nested path with end-slash", func(t *testing.T) {
		TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
			uri, err := workspace.ParseURI("memory:///var/log/")
			require.NoError(t, err)
			mem, err := workspace.NewMemoryScheme(config.NopConfig(), uri)
			require.NoError(t, err)
			return mem
		})
	})
}
