package test_all

import (
	"context"
	"testing"

	"github.com/unstablebuild/blue/encoding/json"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	storework "unstable.build/go-tui/storage/workspace"
	"unstable.build/go-tui/workspace"
	workdoc "unstable.build/go-tui/workspace/document"
)

func TestMemoryWorkspaceSchemeBackedByWorkspaceSchemeService(t *testing.T) {
	testWorkspaceSchemeSuite(t, func(t *testing.T) schemeapi.Scheme {
		workspaceURI, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)

		ctx := context.Background()
		scheme, err := workspace.NewMemoryScheme(
			ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		svc, err := storework.NewWorkspaceService(scheme, json.Marshaler())
		require.NoError(t, err)

		s, err := workdoc.WorkspaceScheme[testStruct](workspaceURI, svc, json.Marshaler(),
			errMissingID)(ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)
		return s
	})
}
