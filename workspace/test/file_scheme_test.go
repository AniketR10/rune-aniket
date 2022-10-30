package test

import (
	"io/ioutil"
	os "os"
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/workspace"
)

func TestFileSchemeScheme(t *testing.T) {
	var dirs []string

	TestWorkspaceSchemeFiles(t, func(t *testing.T) workspace.Scheme {
		dir, err := ioutil.TempDir("", "file_scheme_suite")
		require.NoError(t, err)

		dirs = append(dirs, dir)

		workspaceURI, err := workspace.ParseURI("file://" + dir)
		require.NoError(t, err)

		fileScheme, err := workspace.NewFileScheme(config.NopConfig(), workspaceURI)
		require.NoError(t, err)
		return fileScheme
	})

	for _, dir := range dirs {
		_ = os.RemoveAll(dir)
	}
}
