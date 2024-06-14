package test_all

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/encoding/json"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/storage/schemedoc"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/docscheme"
)

func TestMemoryWorkspaceSchemeBackedByWorkspaceSchemeService(t *testing.T) {
	testWorkspaceSchemeSuite(t, func(t *testing.T) schemeapi.Scheme {
		workspaceURI, err := workspaceapi.ParseURI("memory:///")
		require.NoError(t, err)

		ctx := context.Background()
		scheme, err := workspace.NewMemoryScheme(
			ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		svc, err := schemedoc.NewDocumentService(scheme, json.Marshaler())
		require.NoError(t, err)

		s, err := docscheme.Scheme[testStruct](workspaceURI, svc, json.Marshaler(),
			errMissingID, "author")(ctx, config.NopConfig(), workspaceURI)
		require.NoError(t, err)
		return s
	})
}
