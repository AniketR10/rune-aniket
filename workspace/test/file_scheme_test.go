package test

import (
	"context"

	os "os"
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/workspace"
)

func TestFileScheme(t *testing.T) {
	TestWorkspaceSchemeFiles(t, func(t *testing.T) schemeapi.Scheme {
		dir, err := os.MkdirTemp("", "file_scheme_suite")
		require.NoError(t, err)

		workspaceURI, err := workspaceapi.ParseURI("file://" + dir)
		require.NoError(t, err)

		fileScheme, err := workspace.NewFileScheme(
			context.Background(), config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		return fileScheme
	})

	TestWorkspaceSchemeExecutor(t, func(t *testing.T) schemeapi.Scheme {
		dir, err := os.MkdirTemp("", "file_scheme_suite")
		require.NoError(t, err)

		workspaceURI, err := workspaceapi.ParseURI("file://" + dir)
		require.NoError(t, err)

		fileScheme, err := workspace.NewFileScheme(
			context.Background(), config.NopConfig(), workspaceURI)
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})
		return fileScheme
	})
}
