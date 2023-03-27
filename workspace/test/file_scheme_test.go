package test

import (
	"context"
	"io/ioutil"
	os "os"
	"testing"

	"github.com/stretchr/testify/require"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

func TestFileScheme(t *testing.T) {
	TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
		dir, err := ioutil.TempDir("", "file_scheme_suite")
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
