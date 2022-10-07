package main

import (
	"io/ioutil"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"unstable.build/go-tui"
	"unstable.build/go-tui/config"
	"unstable.build/go-tui/proto"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
	testutil "unstable.build/go-tui/util/test"
	"unstable.build/go-tui/workspace"
)

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

func TestWorkspaceManagerHandlerDraw(t *testing.T) {
	var closeFns []func() error

	defer func() {
		for _, close := range closeFns {
			close()
		}
	}()

	fn := func(t *testing.T) tui.Handler {
		cfg := ideConfig{
			errors: map[string]error{},
			cfg: map[string]interface{}{
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
				},
				"workspace": map[string]interface{}{
					"wallpaper": "workspaceWallpaper",
				},
			}}

		clip := &clipboardManagerTest{Clipboard: text.NewInMemoryClipboard()}

		dir, err := ioutil.TempDir("", "")
		require.NoError(t, err)

		uri, err := workspace.CurrentUserHostURI(dir)
		require.NoError(t, err)

		manager := workspace.NewManager(config.NopConfig())
		require.NoError(t, manager.RegisterScheme(workspace.FileScheme, workspace.NewFileScheme))
		m, err := newWorkspaceManagerHandler(clip, uri, manager, cfg, "", []string{},
			func(term.Event) bool {
				return true
			})
		require.NoError(t, err)

		closeFns = append(closeFns, m.Close)
		return m
	}

	// ensure test is bulletproof
	_ = os.Remove("/tmp/12345aZZ")

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
│Error: workspace t│
│ab is empty       │
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
│Error: invalid wor│
│kspace: there's on│
│ly 10 workspaces  │
└──────────────────┘`},
		{":addWorkspace blabla>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│                  │
│                  │
│workspaceWallpaper│
│Error: Unknown com│
│mand "addWorkspace│
│" or alias targets│
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
│Error: invalid arg│
│uments. Expecting │
│1 argument with wo│
│rkspace URI       │
├──────────────────┤
│1  2              │
└──────────────────┘`},
		{"2:sw>",
			`┌──────────────────┐
│                  │
├──────────────────┤
│Error: invalid arg│
│uments. Expecting │
│1 argument with wo│
│rkspace number    │
├──────────────────┤
│1  2              │
└──────────────────┘`},
	}
	testutil.TestHandlerIsolated(t, fn, 20, 10, cases)
}
