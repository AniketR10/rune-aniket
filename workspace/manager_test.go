package workspace

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cell"
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
		err := m.RegisterScheme("test", NewNopScheme("test"))
		require.NoError(t, err)

		w, ok, err := m.Workspace(parseURI(t, "test:///tmp/file.txt"))
		require.NoError(t, err)
		assert.Nil(t, w)
		require.False(t, ok)

		w, err = m.AddWorkspace(parseURI(t, "test:///tmp/"))
		assert.NotNil(t, w)
		require.NoError(t, err)

		w1, ok, err := m.Workspace(parseURI(t, "test:///tmp/file.txt"))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, w, w1)

		require.NoError(t, m.Close())
	})

	t.Run("removes Workspace upon call to workspace.Close", func(*testing.T) {
		m := NewManager(config.NopConfig())
		err := m.RegisterScheme("test", NewNopScheme("test"))
		require.NoError(t, err)

		w, err := m.AddWorkspace(parseURI(t, "test:///tmp/"))
		assert.NotNil(t, w)
		require.NoError(t, err)

		w1, ok, err := m.Workspace(parseURI(t, "test:///tmp/file.txt"))
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, w, w1)

		require.NoError(t, w1.Close())

		w1, ok, err = m.Workspace(parseURI(t, "test:///tmp/file.txt"))
		require.NoError(t, err)
		require.False(t, ok)
		assert.Nil(t, w1)

		require.NoError(t, m.Close())
	})

	t.Run("AddWorkspace creates a new Workspace if URI is different", func(*testing.T) {
		m := NewManager(config.NopConfig())
		err := m.RegisterScheme("test", NewNopScheme("test"))
		require.NoError(t, err)

		w0, err := m.AddWorkspace(parseURI(t, "test:///tmp/"))
		assert.NotNil(t, w0)
		require.NoError(t, err)

		w1, err := m.AddWorkspace(parseURI(t, "test:///tmp/blah"))
		assert.NotNil(t, w1)
		require.NoError(t, err)

		assert.NotEqual(t, w0, w1)

		w2, err := m.AddWorkspace(parseURI(t, "test:///tmp/blah/hello"))
		assert.NotNil(t, w2)
		require.NoError(t, err)

		assert.NotEqual(t, w1, w2)

		// same as w2
		w3, err := m.AddWorkspace(parseURI(t, "test:///tmp/blah/hello"))
		assert.NotNil(t, w2)
		require.NoError(t, err)

		assert.Equal(t, w2, w3)

		require.NoError(t, m.Close())
	})

	t.Run("Workspace returns ANY workspace capable "+
		"of handling a uri, as defined by IsWorkspaceURI", func(*testing.T) {
		m := NewManager(config.NopConfig())
		err := m.RegisterScheme("test", NewNopScheme("test"))
		require.NoError(t, err)

		w0, err := m.AddWorkspace(parseURI(t, "test:///var/"))
		assert.NotNil(t, w0)
		require.NoError(t, err)

		w1, err := m.AddWorkspace(parseURI(t, "test:///tmp/blah"))
		assert.NotNil(t, w1)
		require.NoError(t, err)

		assert.NotEqual(t, w0, w1)

		w2, ok, err := m.Workspace(parseURI(t, "test:///tmp/blah/hello"))
		require.NoError(t, err)
		assert.True(t, ok)
		assert.True(t, w2 == w0 || w2 == w1)

		require.NoError(t, m.Close())
	})

	t.Run("register same scheme twice returns error", func(t *testing.T) {
		m := NewManager(config.NopConfig())
		err := m.RegisterScheme("test", NewNopScheme("test"))
		require.NoError(t, err)
		err = m.RegisterScheme("test", NewNopScheme("test"))
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

func TestIntegrationManagerWithWorkspaceLoad(t *testing.T) {
	finnWorkspaceURI, err := ParseURI("finn:///tmp/hello")
	require.NoError(t, err)

	jakeFileURI, err := ParseURI("jake:///tmp/hello")
	require.NoError(t, err)

	jakeSwapDirURI, err := ParseURI("jake:///tmp/hello/.hallo.txt.swp")
	require.NoError(t, err)

	t.Run("default workspace is NOT able to load files from other schemes", func(t *testing.T) {
		manager := NewManager(config.NopConfig())
		require.NoError(t, manager.RegisterScheme("finn", NewNopScheme("finn")))
		require.NoError(t, manager.RegisterScheme("jake", NewNopScheme("jake")))

		finnWorkspace, err := manager.AddWorkspace(finnWorkspaceURI)
		require.NoError(t, err)

		_, err = finnWorkspace.Load(jakeFileURI, cell.NewBuffer(), jakeSwapDirURI, false)
		require.Error(t, err)

		_, err = finnWorkspace.Recover(jakeFileURI, jakeSwapDirURI, cell.NewBuffer(), false)
		require.Error(t, err)
	})

	t.Run("default workspace wrapped with multi is able to load files from other schemes", func(t *testing.T) {
		manager := NewManager(config.NopConfig())
		require.NoError(t, manager.RegisterScheme("finn", LoggingScheme("finn", NewNopScheme("finn"))))
		require.NoError(t, manager.RegisterScheme("jake", LoggingScheme("jake", NewNopScheme("jake"))))

		finnWorkspace, err := manager.AddWorkspace(finnWorkspaceURI)
		require.NoError(t, err)

		finnWorkspace = Multi(manager, finnWorkspace, finnWorkspaceURI)

		ret, err := finnWorkspace.Load(jakeFileURI, cell.NewBuffer(), jakeSwapDirURI, false)
		require.NoError(t, err)
		require.NoError(t, ret.Close())

		ret, err = finnWorkspace.Recover(jakeFileURI, jakeSwapDirURI, cell.NewBuffer(), false)
		require.NoError(t, err)
		require.NoError(t, ret.Close())
	})
}
