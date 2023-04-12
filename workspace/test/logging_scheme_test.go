package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/api/config"
	"unstable.build/go-tui/workspace"
)

func TestLoggingScheme(t *testing.T) {
	TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
		uri, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)
		log := workspace.LoggingScheme("memory", workspace.NewMemoryScheme)
		scheme, err := log(context.Background(), config.NopConfig(), uri)
		require.NoError(t, err)
		return scheme
	})
}
