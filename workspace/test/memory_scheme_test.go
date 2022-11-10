package test

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
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

// TODO add to scheme suite
func TestMemoryFile(t *testing.T) {
	t.Run("Write overwrites data", func(t *testing.T) {
		f := workspace.NewMemoryFile("bla", 0, []byte("12345"))
		n, err := f.Write([]byte("ZZ"))
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		nn, err := f.Seek(0, 0)
		require.NoError(t, err)
		assert.Equal(t, int64(0), nn)

		data, err := ioutil.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "ZZ345", string(data))
	})

	t.Run("Read uses write offset", func(t *testing.T) {
		f := workspace.NewMemoryFile("bla", 0, []byte("12345"))
		n, err := f.Write([]byte("ZZ"))
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		data, err := ioutil.ReadAll(f)
		require.NoError(t, err)
		assert.Equal(t, "345", string(data))
	})
}
