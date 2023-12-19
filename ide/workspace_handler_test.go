package ide

import (
	"context"
	"fmt"
	"io/ioutil"
	_ "net/http/pprof"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/api/config"
	schemeapi "unstable.build/go-tui/api/scheme"
	workspaceapi "unstable.build/go-tui/api/workspace"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/extension"
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
	uri, err := workspaceapi.ParseURI("memory:///tmp")
	require.NoError(t, err)

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

		m := newTestWorkspaceManagerHandlerWithManager(t, manager, uri, cfg, nil)
		defer m.Close()

		assert.EqualValues(t, mockConfig, passed)
	})

	t.Run("does not reload workspace config", func(t *testing.T) {
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

		m := newTestWorkspaceManagerHandlerWithManager(t, manager, uri, cfg, nil)
		assert.EqualValues(t, mockConfig, passed)

		m.reloadConfig = func() (ideConfig, error) {
			cfg := defaultCfg()
			cfg.cfg["workspace"].(map[string]interface{})["1"] = "!!!!"
			return cfg, nil
		}

		require.NoError(t, m.commandReloadWorkspace())
		assert.EqualValues(t, mockConfig, passed)

		require.NoError(t, m.Close())
	})
}

func TestWorkspaceExtensions(t *testing.T) {
	t.Run("calls extension runner with user extensions", func(t *testing.T) {
		cfg := ideConfig{cfg: map[string]interface{}{
			"command":           map[string]interface{}{},
			"show_manual_after": "1h",
			"extensions": map[string]interface{}{
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

		// extension.Runner.Run is called asynchronously
		var called string

		runner := FuncExtensionsRunner(
			func(locker sync.Locker, _uri workspaceapi.URI,
				res map[extension.Permission]extension.ResourceRegistrar, s string, n browser.Notifications,
			) (extension.Runner, error) {
				assert.Equal(t, uri, _uri)
				return fnRunner{fn: func(extensionID, path string, cfg config.Config) error {
					called = extensionID
					assert.Equal(t, "myPath", path)
					assert.Equal(t, config.MapConfig(map[string]interface{}{"a": "b"}), cfg)
					return nil
				},
				}, nil
			})
		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		m := newTestWorkspaceManagerHandlerWithManagerAndExtensions(t, manager,
			uri, cfg, runner, nil, nil, dir)
		defer m.Close()

		assert.Equal(t, "git", called)
	})

	t.Run("calls extension runner with built-in extensions", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		cfg := ideConfig{cfg: map[string]interface{}{}}
		manager := workspace.NewManager(cfg.workspace())

		manager.RegisterScheme(workspace.MemoryScheme, workspace.NewMemoryScheme)

		uri, err := workspaceapi.ParseURI("memory:///tmp")
		require.NoError(t, err)

		extensions := map[string]Extension{
			"myID": {
				ID:     "myID",
				Path:   "myPath2",
				Config: config.MapConfig(map[string]interface{}{"a": "b"}),
			},
		}

		// extension.Runner.Run is called asynchronously
		var called string

		runner := FuncExtensionsRunner(
			func(locker sync.Locker, _uri workspaceapi.URI,
				res map[extension.Permission]extension.ResourceRegistrar, s string, n browser.Notifications) (extension.Runner, error) {
				assert.Equal(t, uri, _uri)
				return fnRunner{fn: func(extensionID, path string, cfg config.Config) error {
					called = extensionID
					assert.Equal(t, "myPath2", path)
					assert.Equal(t, config.MapConfig(map[string]interface{}{"a": "b"}), cfg)
					return nil
				},
				}, nil
			})
		m := newTestWorkspaceManagerHandlerWithManagerAndExtensions(t, manager,
			uri, cfg, runner, extensions, nil, dir)
		defer m.Close()

		assert.Equal(t, "myID", called)
	})
}

func TestWorkspaceManagerHandlerDraw(t *testing.T) {
	fn := func(t *testing.T) tui.Handler {
		m := newTestWorkspaceManagerHandler(t, defaultCfg(), nil)
		t.Cleanup(func() { m.Close() })
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
│                  │
│▐                 │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
		{":edit /tmp/12345aZZ>ihello<yyp:w>:reloadWorkspace>", // saved
			`┌──────────────────┐
│12345aZZ          │
├──────────────────┤
│hello             │
│▐ello             │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
		{":edit memory\\:///12345aZZ>ihello<yyp:w>:reloadWorkspace>", // full uri
			`┌──────────────────┐
│12345aZZ          │
├──────────────────┤
│hello             │
│▐ello             │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
		{":cwo>:aw memory\\:///tmp2>:edit 12345aZZ>:w>:cwo>:aw  memory\\:///tmp2>", // prompt
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│  Do you want     │
│  to restore      │
│  the previous    │
│  session?        │
│                  │
└──────────────────┘`},
		// prompt resets cache (use file scheme to avoid needing
		// to use ':' to indicate memory scheme)
		{":cwo>:aw memory\\:///tmp2>:edit 12345aZZ>:w>:cwo>:aw  memory\\:///tmp2>y",
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
		{":cwo>:aw memory\\:///tmp2>edit 12345aZZ>:w>:cwo>:aw  memory\\:///tmp2>n:cwo>:aw  memory\\:///tmp2>", // prompt no: resets cache
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
│workspace tab     │
│is empty          │
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
│workspaces        │
└──────────────────┘
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
│workspace number  │
└──────────────────┘
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
		{":sw 4>:addWorkspace memory\\:///>", // can give path as arg to addWorkspace
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
		{":sw 4>:addWorkspace memory\\:///tmp2>:edit memory\\:///tmp2/12>:reloadWorkspace>", // reloads non-primary workspace
			`┌──────────────────┐
│12                │
├──────────────────┤
│▐                 │
│                  │
│                  │
│:           NORMAL│
├──────────────────┤
│1  4              │
└──────────────────┘`},
	}
	testutil.TestHandlerIsolated(t, fn, 20, 10, cases)
}

func TestWorkspaceManagerClosePromptIntegration(t *testing.T) {
	t.Run("prompts on quit if files are dirty, user continues", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		filenames := []string{"1234", "4567"}
		m := newTestWorkspaceManagerHandlerWithDir(t, defaultCfg(), filenames, dir)

		cases := []testutil.HandlerSequenceTestCase{
			{"ihola <",
				`┌──────────────────┐
│1234*  4567       │
├──────────────────┤
│hola▐             │
│                  │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
			{":quit>",
				`┌──────────────────┐
│1234*  4567       │
├──────────────────┤
│  There are       │
│  open files      │
│  with changes    │
│  pending to be   │
│  written. Are    │
│  you sure you    │
└──────────────────┘`},
		}
		testutil.TestHandlerSequence(t, m, 20, 10, cases)

		exit, handled := m.Handle(term.Event{Type: term.EventKey, Ch: 'y'})
		assert.True(t, exit)
		assert.True(t, handled)

		require.NoError(t, m.Close())
	})

	t.Run("prompts on quit if files are dirty, user backs down", func(t *testing.T) {
		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)
		filenames := []string{"1234", "4567"}
		m := newTestWorkspaceManagerHandlerWithDir(t, defaultCfg(), filenames, dir)

		cases := []testutil.HandlerSequenceTestCase{
			{"ihola <",
				`┌──────────────────┐
│1234*  4567       │
├──────────────────┤
│hola▐             │
│                  │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
			{":quit>",
				`┌──────────────────┐
│1234*  4567       │
├──────────────────┤
│  There are       │
│  open files      │
│  with changes    │
│  pending to be   │
│  written. Are    │
│  you sure you    │
└──────────────────┘`},
		}
		testutil.TestHandlerSequence(t, m, 20, 10, cases)

		exit, handled := m.Handle(term.Event{Type: term.EventKey, Ch: 'n'})
		assert.False(t, exit)
		assert.True(t, handled)

		exit, handled = m.Handle(term.Event{Type: term.EventNone})
		assert.False(t, exit)
		assert.True(t, handled)

		require.NoError(t, m.Close())
	})

	for _, cmd := range []string{"forceQuit!", "writeQuit", "writeForceQuit!"} {
		t.Run(fmt.Sprintf("does not prompt on %s", cmd), func(t *testing.T) {
			dir, err := ioutil.TempDir("", "")
			require.NoError(t, err)
			filenames := []string{"1234", "4567"}
			m := newTestWorkspaceManagerHandlerWithDir(t, defaultCfg(), filenames, dir)

			cases := []testutil.HandlerSequenceTestCase{
				{"ihola <",
					`┌──────────────────┐
│1234*  4567       │
├──────────────────┤
│hola▐             │
│                  │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
			}

			testutil.TestHandlerSequence(t, m, 20, 10, cases)

			exit, handled := m.Handle(term.Event{Key: testCommandKey.Key, Type: term.EventKey})
			assert.False(t, exit)
			assert.True(t, handled)

			for i, ch := range fmt.Sprintf("%s", cmd) {
				exit, handled := m.Handle(term.Event{Type: term.EventKey, Ch: ch})
				assert.False(t, exit)
				assert.True(t, handled, i)
			}

			// sut
			exit, handled = m.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
			assert.True(t, exit)
			assert.True(t, handled)

			require.NoError(t, m.Close())
		})
	}
}

func TestWorkspaceManagerHandlerDrawWithInitialFiles(t *testing.T) {
	// re-use storage
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)

	for _, wrap := range []bool{false, true} {

		t.Run(fmt.Sprintf("wrap=%v", wrap), func(t *testing.T) {

			t.Run("initial files from arguments", func(t *testing.T) {
				filenames := []string{"1234", "4567"}
				m := newTestWorkspaceManagerHandlerWithDir(t, defaultConfigWithWrap(wrap), filenames, dir)

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
			})

			t.Run("initial files from restore previous session prompt", func(t *testing.T) {
				m := newTestWorkspaceManagerHandlerWithDir(t, defaultConfigWithWrap(wrap), nil, dir)

				cases := []testutil.HandlerSequenceTestCase{
					{"",
						`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│  Do you want     │
│  to restore      │
│  the previous    │
│  session?        │
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
			})

			t.Run("initial files from auto restore", func(t *testing.T) {
				cfg := defaultConfigWithWrap(wrap)
				cfg.cfg["workspace"].(map[string]interface{})["auto_restore"] = true
				m := newTestWorkspaceManagerHandlerWithDir(t, cfg, nil, dir)

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
			})

			t.Run("position is restored on close and open again", func(t *testing.T) {
				dir, err := ioutil.TempDir("", "")
				require.NoError(t, err)
				manager := workspace.NewManager(config.NopConfig())
				require.NoError(t, manager.RegisterScheme(workspace.MemoryScheme,
					workspace.NewMemoryScheme))
				uri, err := workspaceapi.ParseURI(fmt.Sprintf("memory:///%s", dir))
				require.NoError(t, err)
				cfg := defaultConfigWithWrap(wrap)
				cfg.cfg["workspace"].(map[string]interface{})["auto_restore"] = true
				runner := FuncExtensionsRunner(testRunnerFn)

				m1 := newTestWorkspaceManagerHandlerWithManagerAndExtensions(t, manager,
					uri, cfg, runner, nil, nil, dir)

				cases := []testutil.HandlerSequenceTestCase{
					{":edit 1234>ih3ll0\nw1rld <:write>:edit 4567>ihello\nworld <:write>",
						`┌──────────────────┐
│1234  4567        │
├──────────────────┤
│hello             │
│world▐            │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
				}
				testutil.TestHandlerSequence(t, m1, 20, 10, cases)
				require.NoError(t, m1.Close())

				m2 := newTestWorkspaceManagerHandlerWithManagerAndExtensions(t, manager,
					uri, cfg, runner, nil, nil, dir)

				cases = []testutil.HandlerSequenceTestCase{
					{"",
						`┌──────────────────┐
│1234  4567        │
├──────────────────┤
│hello             │
│world▐            │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
					{"i\na\nb\nc\nd\ne\nf<:write>",
						`┌──────────────────┐
│1234  4567        │
├──────────────────┤
│b                 │
│c                 │
│d                 │
│e                 │
│▐                 │
│:           NORMAL│
└──────────────────┘`},
				}
				testutil.TestHandlerSequence(t, m2, 20, 10, cases)
				require.NoError(t, m2.Close())

				m3 := newTestWorkspaceManagerHandlerWithManagerAndExtensions(t, manager,
					uri, cfg, runner, nil, nil, dir)

				cases = []testutil.HandlerSequenceTestCase{
					{"",
						`┌──────────────────┐
│1234  4567        │
├──────────────────┤
│b                 │
│c                 │
│d                 │
│e                 │
│▐                 │
│:           NORMAL│
└──────────────────┘`},
				}
				testutil.TestHandlerSequence(t, m3, 20, 10, cases)
				require.NoError(t, m3.Close())
			})

			t.Run("position is restored on reloadWorkspace", func(t *testing.T) {
				dir, err := ioutil.TempDir("", "")
				require.NoError(t, err)
				m := newTestWorkspaceManagerHandlerWithDir(t, defaultConfigWithWrap(wrap), nil, dir)

				cases := []testutil.HandlerSequenceTestCase{
					{":edit A>ih3ll0\nw1rld <:write>:edit B>ihello\nworld <:write>",
						`┌──────────────────┐
│A  B              │
├──────────────────┤
│hello             │
│world▐            │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
					{":reloadWorkspace>",
						`┌──────────────────┐
│A  B              │
├──────────────────┤
│hello             │
│world▐            │
│                  │
│                  │
│                  │
│:           NORMAL│
└──────────────────┘`},
					{"i\na\nb\nc\nd\ne\nf<:write>",
						`┌──────────────────┐
│A  B              │
├──────────────────┤
│b                 │
│c                 │
│d                 │
│e                 │
│▐                 │
│:           NORMAL│
└──────────────────┘`},
					{":reloadWorkspace>",
						`┌──────────────────┐
│A  B              │
├──────────────────┤
│b                 │
│c                 │
│d                 │
│e                 │
│▐                 │
│:           NORMAL│
└──────────────────┘`},
				}
				testutil.TestHandlerSequence(t, m, 20, 10, cases)
				require.NoError(t, m.Close())
			})
		})
	}
}

func newTestWorkspaceManagerHandlerWithManager(
	t *testing.T, manager *workspace.Manager,
	uri workspaceapi.URI, cfg ideConfig, filenames []string,
) *testWorkspaceManagerHandler {
	dir, err := ioutil.TempDir("", "")
	require.NoError(t, err)
	return newTestWorkspaceManagerHandlerWithManagerAndExtensions(t, manager,
		uri, cfg, FuncExtensionsRunner(testRunnerFn), nil, filenames, dir)
}

func newTestWorkspaceManagerHandlerWithManagerAndExtensions(
	t *testing.T, manager *workspace.Manager,
	uri workspaceapi.URI, cfg ideConfig, runner ExtensionsRunner,
	extensions map[string]Extension, files []string, dir string,
) *testWorkspaceManagerHandler {
	homeURI, err := workspaceapi.ParseURI("memory:///home")
	require.NoError(t, err)

	m := new(testWorkspaceManagerHandler)
	m.workspaceManagerHandler = new(workspaceManagerHandler)
	// ensure that command manual is never shown
	cfg.cfg["command"] = defaultCfg().cfg["command"]
	err = m.workspaceManagerHandler.init(uri, homeURI, manager, cfg, "", files,
		dir, func(term.Event) bool {
			return true
		}, runner, new(sync.Mutex), extensions,
		func() (ideConfig, error) { return cfg, nil }, ".sixrc")
	require.NoError(t, err)
	return m
}

func newTestWorkspaceManagerHandlerWithDir(
	t *testing.T, cc ideConfig, filenames []string, dir string,
) *testWorkspaceManagerHandler {
	manager := workspace.NewManager(config.NopConfig())
	require.NoError(t, manager.RegisterScheme(workspace.MemoryScheme,
		workspace.NewMemoryScheme))

	uri, err := workspaceapi.ParseURI(fmt.Sprintf("memory://%s", dir))
	require.NoError(t, err)
	runner := FuncExtensionsRunner(testRunnerFn)
	return newTestWorkspaceManagerHandlerWithManagerAndExtensions(t, manager,
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
			"show_manual_after": "1h",
			"key":               "<c-\\>", // see testutil.TestHandlerIsolated
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
			"wallpaper":    "workspaceWallpaper",
			"auto_restore": false,
		},
		"notifications": map[string]interface{}{
			"progress_bar": false,
		},
	}}
}

func defaultConfigWithWrap(wrap bool) ideConfig {
	ret := defaultCfg()
	ret.cfg["editor"] = map[string]interface{}{
		"modal": map[string]interface{}{
			"wrap": wrap,
		},
	}
	return ret
}

type fnRunner struct {
	fn func(extensionID, path string, config config.Config) error
}

func (f fnRunner) Run(extensionID, path string, config config.Config) error {
	return f.fn(extensionID, path, config)
}

func (f fnRunner) Close() error {
	return nil
}
