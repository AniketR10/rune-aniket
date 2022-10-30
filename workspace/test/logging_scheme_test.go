package test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

func TestLoggingScheme(t *testing.T) {
	TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
		uri, err := workspace.ParseURI("memory:///")
		require.NoError(t, err)
		log := workspace.LoggingScheme("memory", workspace.NewMemoryScheme)
		scheme, err := log(config.NopConfig(), uri)
		require.NoError(t, err)
		return scheme
	})
}
