package main

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	testutil "unstable.build/go-tui/util/test"
	"unstable.build/go-tui/workspace"
)

func defaultCfg() ideConfig {
	return ideConfig{cfg: map[string]interface{}{
		"command": map[string]interface{}{
			"key":    "<c-\\>", // see testutil.TestHandlerIsolated
			"width":  10,
			"height": 5,
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
	}}
}

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
			func(cfg config.Config, uri workspace.URI) (workspace.Scheme, error,
			) {
				cfg.Iterate(func(k string, v interface{}) {
					passed[k] = v
				})
				return workspace.NewMemoryScheme(cfg, uri)
			})

		uri, err := workspace.ParseURI("memory:///tmp")
		require.NoError(t, err)

		m := newTestWorkspaceManagerHandlerWithManager(t, manager, uri, cfg)
		defer m.Close()

		assert.EqualValues(t, mockConfig, passed)
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
		m := newTestWorkspaceManagerHandler(t, defaultCfg())
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
├────┌────────┐────┤
│    │edit▐   │    │
│    │edit    │    │
│work│        │aper│
│    └────────┘    │
│                  │
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
		{":cw>",
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
		{":cw>:cw>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│workspaceWallpaper│
│workspace tab is e│
│mpty              │
├──────────────────┤
│1                 │
└──────────────────┘`},
		{":sw 100>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│workspaceWallpaper│
│invalid workspace:│
│ there's only 10 w│
│orkspaces         │
└──────────────────┘`},
		{":addBlaBla>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│workspaceWallpaper│
│Unknown command "a│
│ddBlaBla" or alias│
│ targets          │
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
		{"2:aw>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│invalid arguments.│
│ Expecting 1 argum│
│ent with workspace│
│ URI              │
├──────────────────┤
│1  2              │
└──────────────────┘`},
		{"2:sw>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│invalid arguments.│
│ Expecting 1 argum│
│ent with workspace│
│ number           │
├──────────────────┤
│1  2              │
└──────────────────┘`},
		{":sw 3>:addBlaBla>", // test workspace handler aliases
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

type clipboardManagerTest struct {
	text.Clipboard
}

func (m *clipboardManagerTest) Serve(
	string, uint32, proto.MuxBroker, sync.Locker,
) error {
	return nil
}
func (m *clipboardManagerTest) Close() error {
	return nil
}

func newTestWorkspaceManagerHandlerWithManager(
	t *testing.T, manager *workspace.Manager,
	uri workspace.URI, cfg ideConfig,
) *workspaceManagerHandler {
	clip := &clipboardManagerTest{Clipboard: text.NewInMemoryClipboard()}

	m := new(workspaceManagerHandler)
	err := m.init(clip, uri, manager, cfg, "", []string{},
		func(term.Event) bool {
			return true
		})
	require.NoError(t, err)
	return m
}

func newTestWorkspaceManagerHandler(
	t *testing.T, cc ideConfig,
) *workspaceManagerHandler {
	manager := workspace.NewManager(config.NopConfig())
	require.NoError(t, manager.RegisterScheme(workspace.MemoryScheme,
		workspace.NewMemoryScheme))
	require.NoError(t, manager.RegisterScheme(workspace.FileScheme,
		workspace.NewFileScheme))

	uri, err := workspace.ParseURI("memory:///tmp")
	require.NoError(t, err)

	return newTestWorkspaceManagerHandlerWithManager(t, manager, uri, cc)
}
