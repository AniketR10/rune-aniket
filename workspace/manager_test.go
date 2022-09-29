package workspace

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/config"
)

func parseURI(t *testing.T, uriStr string) URI {
	u, err := ParseURI(uriStr)
	require.NoError(t, err)
	return u
}

func TestManager(t *testing.T) {

	t.Run("registers scheme to be used by AddWorkspace", func(*testing.T) {
		m := NewManager(config.NopConfig())
		err := m.RegisterScheme("test", NewNopScheme)
		require.NoError(t, err)

		w, ok, err := m.WorkspaceFile(parseURI(t, "test:///tmp/file.txt"))
		require.NoError(t, err)
		assert.Nil(t, w)
		require.False(t, ok)

		w, err = m.AddWorkspace(parseURI(t, "test:///tmp/"))
		assert.NotNil(t, w)
		require.NoError(t, err)

		w1, ok, err := m.WorkspaceFile(parseURI(t, "test:///tmp/file.txt"))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, w, w1)

		require.NoError(t, m.Close())
	})

	t.Run("removes Workspace upon call to workspace.Close", func(*testing.T) {
		m := NewManager(config.NopConfig())
		err := m.RegisterScheme("test", NewNopScheme)
		require.NoError(t, err)

		w, err := m.AddWorkspace(parseURI(t, "test:///tmp/"))
		assert.NotNil(t, w)
		require.NoError(t, err)

		w1, ok, err := m.WorkspaceFile(parseURI(t, "test:///tmp/file.txt"))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, w, w1)

		require.NoError(t, w1.Close())

		w1, ok, err = m.WorkspaceFile(parseURI(t, "test:///tmp/file.txt"))
		require.NoError(t, err)
		require.False(t, ok)
		assert.Nil(t, w1)

		require.NoError(t, m.Close())
	})

	t.Run("register same scheme twice returns error", func(t *testing.T) {
		m := NewManager(config.NopConfig())
		err := m.RegisterScheme("test", NewNopScheme)
		require.NoError(t, err)
		err = m.RegisterScheme("test", NewNopScheme)
		require.Error(t, err)
	})

	t.Run("buubles up scheme constructor errors", func(t *testing.T) {
		m := NewManager(config.NopConfig())
		err := m.RegisterScheme("test", func(cfg config.Config, uri URI) (Scheme, error) {
			return nil, errors.New("boom")
		})
		require.NoError(t, err)

		w, err := m.AddWorkspace(parseURI(t, "test:///tmp/"))
		assert.Nil(t, w)
		require.Error(t, err)

		require.NoError(t, m.Close())
	})

	t.Run("passes scheme config to scheme constructor", func(*testing.T) {
		m := NewManager(config.MapConfig(map[string]interface{}{
			"test": map[string]interface{}{
				"key": "value",
			},
			"file": map[string]interface{}{
				"kk": "vv",
			},
		}))

		var called bool
		err := m.RegisterScheme("test", func(cfg config.Config, uri URI) (Scheme, error) {

			value, err := cfg.GetString("key")
			assert.NoError(t, err)
			assert.Equal(t, "value", value)

			value, err = cfg.GetString("kk")
			assert.Equal(t, config.ErrNotFound, err)
			assert.Zero(t, value)

			called = true
			return &testScheme{}, nil
		})
		require.NoError(t, err)

		w, err := m.AddWorkspace(parseURI(t, "test:///tmp/"))
		require.NoError(t, err)
		assert.NotNil(t, w)
		assert.True(t, called)

		require.NoError(t, m.Close())
	})
}
