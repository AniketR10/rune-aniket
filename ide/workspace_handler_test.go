package ide

import (
	"context"
	"io/ioutil"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/plugin"
	"unstable.build/go-tui/term"
	testutil "unstable.build/go-tui/util/test"
	"unstable.build/go-tui/workspace"
)

func TestWorkspaceConfig(t *testing.T) {
	mockConfig := map[string]interface{}{
		"1": "2",
		"2": map[string]interface{}{
			"dos": "2",
			"two": "2",
		},
	}
	t.Run("passes default scheme config to SchemeFunc", func(t *testing.T) {
		cfg := defaultCfg()
		manager := workspace.NewManager(cfg.workspace())
		workspaceConfig := cfg.cfg["workspace"].(map[string]interface{})
		workspaceConfig[workspace.MemoryScheme] = mockConfig

		passed := make(map[string]interface{})
		manager.RegisterScheme(workspace.MemoryScheme,
			func(ctx context.Context, cfg config.Config, uri workspaceapi.URI) (
				schemeapi.Scheme, error,
			) {
				cfg.Iterate(func(k string, v interface{}) {
					passed[k] = v
				})
				return workspace.NewMemoryScheme(ctx, cfg, uri)
			})

		uri, err := workspaceapi.ParseURI("memory:///tmp")
		require.NoError(t, err)

		m := newTestWorkspaceManagerHandlerWithManager(t, manager, uri, cfg, nil)
		defer m.Close()

		assert.EqualValues(t, mockConfig, passed)
	})
}

func TestWorkspacePlugins(t *testing.T) {
	t.Run("calls plugin runner with user plugins", func(t *testing.T) {
		cfg := ideConfig{cfg: map[string]interface{}{
			"plugins": map[string]interface{}{
				"git": map[string]interface{}{
					"path": "myPath",
					"config": map[string]interface{}{
						"a": "b",
					},
				},
			},
		}}
		manager := workspace.NewManager(cfg.workspace())

		manager.RegisterScheme(workspace.MemoryScheme, workspace.NewMemoryScheme)

		uri, err := workspaceapi.ParseURI("memory:///tmp")
		require.NoError(t, err)

		// plugin.Runner.Run is called asynchronously
		var called string

		runner := FuncPluginsRunner(
			func(locker sync.Locker, _uri workspaceapi.URI,
				res map[plugin.Permission]plugin.ResourceRegistrar, s string) (plugin.Runner, error) {
				assert.Equal(t, uri, _uri)
				return fnRunner{fn: func(pluginID, path string, cfg config.Config) error {
					called = pluginID
					assert.Equal(t, "myPath", path)
					assert.Equal(t, config.MapConfig(map[string]interface{}{"a": "b"}), cfg)
					return nil
				},
				}, nil
			})
		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		m := newTestWorkspaceManagerHandlerWithManagerAndPlugins(t, manager,
			uri, cfg, runner, nil, nil, dir)
		defer m.Close()

		assert.Equal(t, "git", called)
	})

	t.Run("calls plugin runner with built-in plugins", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		cfg := ideConfig{cfg: map[string]interface{}{}}
		manager := workspace.NewManager(cfg.workspace())

		manager.RegisterScheme(workspace.MemoryScheme, workspace.NewMemoryScheme)

		uri, err := workspaceapi.ParseURI("memory:///tmp")
		require.NoError(t, err)

		plugins := map[string]Plugin{
			"myID": Plugin{
				ID:     "myID",
				Path:   "myPath2",
				Config: config.MapConfig(map[string]interface{}{"a": "b"}),
			},
		}

		// plugin.Runner.Run is called asynchronously
		var called string

		runner := FuncPluginsRunner(
			func(locker sync.Locker, _uri workspaceapi.URI,
				res map[plugin.Permission]plugin.ResourceRegistrar, s string) (plugin.Runner, error) {
				assert.Equal(t, uri, _uri)
				return fnRunner{fn: func(pluginID, path string, cfg config.Config) error {
					called = pluginID
					assert.Equal(t, "myPath2", path)
					assert.Equal(t, config.MapConfig(map[string]interface{}{"a": "b"}), cfg)
					return nil
				},
				}, nil
			})
		m := newTestWorkspaceManagerHandlerWithManagerAndPlugins(t, manager,
			uri, cfg, runner, plugins, nil, dir)
		defer m.Close()

		assert.Equal(t, "myID", called)
	})
}

func TestWorkspaceManagerHandlerDraw(t *testing.T) {
	var closeFns []func() error

	defer func() {
		for _, close := range closeFns {
			close()
		}
	}()

	fn := func(t *testing.T) tui.Handler {
		m := newTestWorkspaceManagerHandler(t, defaultCfg(), nil)
		closeFns = append(closeFns, m.Close)
		return m
	}

	cases := []testutil.HandlerSequenceTestCase{
		{"",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│workspaceWallpaper│
│                  │
│                  │
│                  │
└──────────────────┘`},
		{":edit",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│workspaceWallpaper│
┌──────────────────┐
│edit▐             │
│                  │
└──────────────────┘`},
		{":edit /tmp/12345aZZ>ihello<yyp",
			`┌──────────────────┐
│12345aZZ*         │
├──────────────────┤
│hello             │
│▐ello             │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
		{":edit /tmp/12345aZZ>ihello<yyp:reloadWorkspace>", // un-saved
			`┌──────────────────┐
│12345aZZ          │
├──────────────────┤
│▐                 │
│                  │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
		{":edit /tmp/12345aZZ>ihello<yyp:w>:reloadWorkspace>", // saved
			`┌──────────────────┐
│12345aZZ          │
├──────────────────┤
│▐ello             │
│hello             │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
		{":edit memory:///12345aZZ>ihello<yyp:w>:reloadWorkspace>", // full uri
			`┌──────────────────┐
│12345aZZ          │
├──────────────────┤
│▐ello             │
│hello             │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
		{":cwo>:aw /tmp>:edit 12345aZZ>:w>:cwo>:aw  /tmp>", // prompt
			`┌──────────────────┐
│                  │
│Do you want to res│
│                  │
│                  │
│ ┌─────┐  ┌────┐  │
│ │ Yes │  │ No │  │
│ └─────┘  └────┘  │
│                  │
└──────────────────┘`},
		// prompt resets cache (use file scheme to avoid needing
		// to use ':' to indicate memory scheme)
		{":cwo>:aw /tmp>:edit 12345aZZ>:w>:cwo>:aw  /tmp>y",
			`┌──────────────────┐
│12345aZZ          │
├──────────────────┤
│▐                 │
│                  │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
		{":cwo>:aw /tmp>edit 12345aZZ>:w>:cwo>:aw  /tmp>n:cwo>:aw  /tmp>", // prompt no: resets cache
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│workspaceWallpaper│
│                  │
│                  │
│                  │
└──────────────────┘`},
		{":sw 3>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│workspaceWallpaper│
│                  │
│                  │
├──────────────────┤
│1  3              │
└──────────────────┘`},
		{":cwo>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│workspaceWallpaper│
│                  │
│                  │
├──────────────────┤
│1                 │
└──────────────────┘`},
		{":q!>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│workspaceWallpaper│
│                  │
│                  │
│                  │
└──────────────────┘`},
		{":cwo>:cwo>",
			`┌──────────────────┐
│workspace tab is  │
│empty             │
└──────────────────┘
│workspaceWallpaper│
│                  │
│                  │
├──────────────────┤
│1                 │
└──────────────────┘`},
		{":sw 100>",
			`┌──────────────────┐
│invalid           │
│workspace:        │
│there's only 10   │
└──────────────────┘
│workspaceWallpaper│
│                  │
│                  │
│                  │
└──────────────────┘`},
		{":addBlaBla>", // addWorkspace should work on a workspace, use next avail
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│workspaceWallpaper│
│                  │
│                  │
├──────────────────┤
│1  2              │
└──────────────────┘`},
		{"1234567890",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│workspaceWallpaper│
│                  │
│                  │
├──────────────────┤
│1  10             │
└──────────────────┘`},
		{"2:aw>", // uses tmp dir as workspace in the absence of a uri
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│workspaceWallpaper│
│                  │
│                  │
├──────────────────┤
│1  2              │
└──────────────────┘`},
		{"2:sw>",
			`┌──────────────────┐
│invalid           │
│arguments.        │
│Expecting 1       │
│argument with     │
└──────────────────┘
│                  │
├──────────────────┤
│1  2              │
└──────────────────┘`},
		{":sw 3>:addBlaBla>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│workspaceWallpaper│
│                  │
│                  │
├──────────────────┤
│1  3              │
└──────────────────┘`},
		{":sw 4>:addWorkspace />", // can give path as arg to addWorkspace
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│workspaceWallpaper│
│                  │
│                  │
├──────────────────┤
│1  4              │
└──────────────────┘`},
	}
	testutil.TestHandlerIsolated(t, fn, 20, 10, cases)
}

func TestWorkspaceManagerHandlerDrawWithInitialFiles(t *testing.T) {
	filenames := []string{"1234", "4567"}
	// re-use storage
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	m := newTestWorkspaceManagerHandlerWithDir(t, defaultCfg(), filenames, dir)

	cases := []testutil.HandlerSequenceTestCase{
		{"",
			`┌──────────────────┐
│1234  4567        │
├──────────────────┤
│▐                 │
│                  │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
	}
	testutil.TestHandlerSequence(t, m, 20, 10, cases)

	require.NoError(t, m.Close())
	m = newTestWorkspaceManagerHandlerWithDir(t, defaultCfg(), nil /* no filenames this time */, dir)

	cases = []testutil.HandlerSequenceTestCase{
		{"",
			`┌──────────────────┐
│                  │
│Do you want to res│
│                  │
│                  │
│ ┌─────┐  ┌────┐  │
│ │ Yes │  │ No │  │
│ └─────┘  └────┘  │
│                  │
└──────────────────┘`},
		{"y",
			`┌──────────────────┐
│1234  4567        │
├──────────────────┤
│▐                 │
│                  │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
	}
	testutil.TestHandlerSequence(t, m, 20, 10, cases)
	require.NoError(t, m.Close())
}

func newTestWorkspaceManagerHandlerWithManager(
	t *testing.T, manager *workspace.Manager,
	uri workspaceapi.URI, cfg ideConfig, filenames []string,
) *testWorkspaceManagerHandler {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	return newTestWorkspaceManagerHandlerWithManagerAndPlugins(t, manager,
		uri, cfg, FuncPluginsRunner(testRunnerFn), nil, filenames, dir)
}

func newTestWorkspaceManagerHandlerWithManagerAndPlugins(
	t *testing.T, manager *workspace.Manager,
	uri workspaceapi.URI, cfg ideConfig, runner PluginsRunner,
	plugins map[string]Plugin, files []string, dir string,
) *testWorkspaceManagerHandler {
	m := new(testWorkspaceManagerHandler)
	m.workspaceManagerHandler = new(workspaceManagerHandler)
	err := m.workspaceManagerHandler.init(uri, manager, cfg, "", files,
		dir, func(term.Event) bool {
			return true
		}, runner, new(sync.Mutex), plugins, ".sixrc")
	require.NoError(t, err)
	return m
}

func newTestWorkspaceManagerHandlerWithDir(
	t *testing.T, cc ideConfig, filenames []string, dir string,
) *testWorkspaceManagerHandler {
	manager := workspace.NewManager(config.NopConfig())
	require.NoError(t, manager.RegisterScheme(workspace.MemoryScheme,
		workspace.NewMemoryScheme))
	require.NoError(t, manager.RegisterScheme(workspace.FileScheme,
		workspace.NewFileScheme))

	uri, err := workspaceapi.ParseURI("memory:///tmp")
	require.NoError(t, err)
	runner := FuncPluginsRunner(testRunnerFn)
	return newTestWorkspaceManagerHandlerWithManagerAndPlugins(t, manager,
		uri, cc, runner, nil, filenames, dir)
}

func newTestWorkspaceManagerHandler(
	t *testing.T, cc ideConfig, filenames []string,
) *testWorkspaceManagerHandler {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	return newTestWorkspaceManagerHandlerWithDir(t, cc, filenames, dir)
}

// deterministic usage of search list
type testWorkspaceManagerHandler struct {
	*workspaceManagerHandler
}

func (t *testWorkspaceManagerHandler) Handle(ev term.Event) (bool, bool) {
	quit, handle := t.workspaceManagerHandler.Handle(ev)
	handler := t.workspaceManagerHandler.focusHandler()
	ex, ok := handler.(*ex)
	if !ok {
		ex = handler.(*workspaceHandler).ex
	}
	ex.Wait()
	return quit, handle
}

func defaultCfg() ideConfig {
	return ideConfig{cfg: map[string]interface{}{
		"clipboard": "memory",
		"command": map[string]interface{}{
			"key": "<c-\\>", // see testutil.TestHandlerIsolated
			"key_bindings": map[string]interface{}{
				"1": "switchToWorkspace 1",
				"2": "switchToWorkspace 2",
				"3": "switchToWorkspace 3",
				"4": "switchToWorkspace 4",
				"5": "switchToWorkspace 5",
				"6": "switchToWorkspace 6",
				"7": "switchToWorkspace 7",
				"8": "switchToWorkspace 8",
				"9": "switchToWorkspace 9",
				"0": "switchToWorkspace 10",
			},
			"aliases": map[string]interface{}{
				"addBlaBla": "addWorkspace memory:///blabla",
			},
		},
		"workspace": map[string]interface{}{
			"wallpaper": "workspaceWallpaper",
		},
		"notifications": map[string]interface{}{
			"progress_bar": false,
		},
	}}
}

type fnRunner struct {
	fn func(pluginID, path string, config config.Config) error
}

func (f fnRunner) Run(pluginID, path string, config config.Config) error {
	return f.fn(pluginID, path, config)
}

func (f fnRunner) Close() error {
	return nil
}
