package test_all

import (
	"testing"

	"github.com/ernestrc/blue/encoding/json"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
	storework "unstable.build/go-tui/storage/workspace"
	"unstable.build/go-tui/workspace"
	workdoc "unstable.build/go-tui/workspace/document"
)

func TestMemoryWorkspaceSchemeBackedByWorkspaceSchemeService(t *testing.T) {
	testWorkspaceSchemeSuite(t, func(t *testing.T) workspace.Scheme {
		workspaceURI, err := workspace.ParseURI("memory:///")
		require.NoError(t, err)

		scheme, err := workspace.NewMemoryScheme(config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		svc, err := storework.NewWorkspaceService(scheme, json.Marshaler())
		require.NoError(t, err)

		s, err := workdoc.WorkspaceScheme[testStruct](workspaceURI, svc, json.Marshaler(),
			errMissingID)(config.NopConfig(), workspaceURI)
		require.NoError(t, err)
		return s
	})
}
