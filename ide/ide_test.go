// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package ide

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/blue/release"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	tcomponent "unstable.build/go-tui/component"
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/ide/ideauthorizer"
	"unstable.build/go-tui/ide/idepkg/idepkgtest"
	"unstable.build/go-tui/term/vte/vtereservoir"
	"unstable.build/go-tui/text"
)

func TestIDEInitializationIntegration(t *testing.T) {
	t.Run("does not panic with sample config", func(t *testing.T) {
		configFile, _ := makeTestFiles(t)
		err := os.WriteFile(configFile.Name(), []byte(sampleConfig), 0666)
		require.NoError(t, err)

		cwdURI, err := workspaceapi.CurrentUserHostURI(".")
		require.NoError(t, err)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})

		i := new(IDE)
		err = i.init(cwdURI.String(), configFile.Name(), dir,
			WithPublishEvent(nopPublishEvent),
			WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
			WithLocker(new(sync.Mutex)))
		require.NoError(t, err)

		require.NotNil(t, i.workspace)
		require.NotNil(t, i.clipboard)

		assert.NoError(t, i.closeResources())
	})

	t.Run("does not panic with empty config", func(t *testing.T) {
		configFile, _ := makeTestFiles(t)

		err := os.WriteFile(configFile.Name(), []byte("{}"), 0666)
		require.NoError(t, err)

		cwdURI, err := workspaceapi.CurrentUserHostURI(".")
		require.NoError(t, err)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})

		i := new(IDE)
		err = i.init(cwdURI.String(), configFile.Name(),
			dir, WithPublishEvent(nopPublishEvent),
			WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
			WithLocker(new(sync.Mutex)))
		require.NoError(t, err)

		require.NotNil(t, i.workspace)
		require.NotNil(t, i.clipboard)

		assert.NoError(t, i.closeResources())
	})

	t.Run("takes a non-URI as a workspace", func(t *testing.T) {
		configFile, _ := makeTestFiles(t)

		err := os.WriteFile(configFile.Name(), []byte("{}"), 0666)
		require.NoError(t, err)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})

		i := new(IDE)
		err = i.init(".", configFile.Name(), dir,
			WithPublishEvent(nopPublishEvent),
			WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
			WithLocker(new(sync.Mutex)))
		require.NoError(t, err)

		require.NotNil(t, i.workspace)
		require.NotNil(t, i.clipboard)

		// addWorkspace launches the workspace install (Phase B) in a
		// goroutine; closeResources must wait for that to finish
		// before tearing down the workspace.Manager, otherwise an
		// in-flight vtereservoir StartCommand races with the manager
		// closing the file scheme's open *os.Files.
		i.WaitWorkspaces()
		assert.NoError(t, i.closeResources())
	})

	t.Run("creates non-existing directories for log file", func(t *testing.T) {
		configFile, _ := makeTestFiles(t)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})

		configData := fmt.Sprintf("log_path: %s/bla/bla/bla/debug.log", dir)
		err = os.WriteFile(configFile.Name(), []byte(configData), 0666)
		require.NoError(t, err)

		i := new(IDE)
		err = i.init(".", configFile.Name(), dir,
			WithPublishEvent(nopPublishEvent),
			WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
			WithLocker(new(sync.Mutex)))
		require.NoError(t, err)

		require.NotNil(t, i.workspace)
		require.NotNil(t, i.clipboard)

		assert.NoError(t, i.closeResources())
	})

	t.Run("is able to initialize without a cwd", func(t *testing.T) {
		configFile, _ := makeTestFiles(t)

		dir, err := os.MkdirTemp("", "")
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})

		i := new(IDE)
		err = i.init("", configFile.Name(), dir,
			WithPublishEvent(nopPublishEvent),
			WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
			WithLocker(new(sync.Mutex)))
		require.NoError(t, err)

		require.NotNil(t, i.root)
		assert.NoError(t, i.closeResources())
	})

	t.Run("init shader is run when passed WithInitShader option", func(t *testing.T) {
		configFile, _ := makeTestFiles(t)
		initShader := new(mockShader)

		dataDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(dataDir)
		})

		i := new(IDE)
		err = i.init("", configFile.Name(), dataDir,
			WithPublishEvent(nopPublishEvent),
			WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
			WithLocker(new(sync.Mutex)),
			WithInitShader(
				func(_ term.Attributes, _ component.FrameCharSet) shader.Shader {
					return initShader
				},
				30, 1*time.Second,
			))
		require.NoError(t, err)

		i.initRunning()
		i.root.Draw(&term.NoopWriter{})

		assert.True(t, initShader.called)
		assert.NoError(t, i.closeResources())
	})

	t.Run("init shader factory receives attrs set via SetDefaultAttributes before Ready", func(t *testing.T) {
		// Documents RUNE-203 contract: callers MUST invoke
		// SetDefaultAttributes before Ready() so the init-shader
		// factory observes the configured GUI theme background.
		// Calling SetDefaultAttributes after Ready() leaves the
		// shader runner painting with the stale config defAttr.
		configFile, _ := makeTestFiles(t)

		dataDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(dataDir)
		})

		var captured term.Attributes
		i, err := New("", configFile.Name(), dataDir,
			WithPublishEvent(nopPublishEvent),
			WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
			WithLocker(new(sync.Mutex)),
			WithInitShader(
				func(attr term.Attributes, _ component.FrameCharSet) shader.Shader {
					captured = attr
					return new(mockShader)
				},
				30, 1*time.Second,
			))
		require.NoError(t, err)

		wantAttr := term.Attributes{
			Fg: term.ColorWhite,
			Bg: term.ColorBlack,
		}
		i.SetDefaultAttributes(wantAttr)

		_ = i.Ready()

		assert.Equal(t, wantAttr, captured,
			"init shader factory must observe attrs set via "+
				"SetDefaultAttributes prior to Ready()")
		assert.NoError(t, i.closeResources())
	})
}

func TestOpen(t *testing.T) {
	t.Parallel()
	assertURI := func(t *testing.T, i *IDE, expected workspaceapi.URI) {
		ex := i.workspaceHandler.exHandler(i.workspaceHandler.focusHandler())
		uri, _, ok := ex.handlerInFocus()
		require.True(t, ok)
		assert.Equal(t, expected, uri)
	}

	t.Run("empty workspace", func(t *testing.T) {
		t.Parallel()
		file, config := makeTestFiles(t)
		dataDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dataDir) })
		i, err := New("", config.Name(), dataDir, WithPublishEvent(nopPublishEvent))
		require.NoError(t, err)
		uri, err := workspaceapi.CurrentUserHostURI(file.Name())
		require.NoError(t, err)

		i.workspaceHandler.mu.Lock()
		defer i.workspaceHandler.mu.Unlock()

		require.NoError(t, i.Open(uri))
		assertURI(t, i, uri)
	})

	t.Run("a workspace", func(t *testing.T) {
		t.Parallel()
		file, config := makeTestFiles(t)
		dataDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dataDir) })
		i, err := New(os.TempDir(), config.Name(), dataDir, WithPublishEvent(nopPublishEvent))
		require.NoError(t, err)
		uri, err := workspaceapi.CurrentUserHostURI(file.Name())
		require.NoError(t, err)

		i.workspaceHandler.mu.Lock()
		defer i.workspaceHandler.mu.Unlock()

		require.NoError(t, i.Open(uri))
		assertURI(t, i, uri)
	})

	t.Run("syntax enabled, empty workspace", func(t *testing.T) {
		t.Parallel()
		pkgs := idepkgtest.MakePackages(
			release.Package{Name: "go", Latest: "3"},
		)
		bundles := idepkgtest.MakeBundles(
			[]release.Bundle{
				{Package: "go", Version: "3"},
			},
		)
		file, config := makeTestFiles(t)
		dataDir, err := os.MkdirTemp("", "")
		require.NoError(t, err)
		t.Cleanup(func() { _ = os.RemoveAll(dataDir) })
		rm := idepkgtest.NewReleaseManager(pkgs, bundles)
		i, err := New("", config.Name(), dataDir, WithReleaseManager(rm), WithPublishEvent(nopPublishEvent))
		require.NoError(t, err)
		uri, err := workspaceapi.CurrentUserHostURI(file.Name())
		require.NoError(t, err)

		logrus.SetLevel(logrus.TraceLevel)

		i.workspaceHandler.mu.Lock()
		defer i.workspaceHandler.mu.Unlock()

		require.NoError(t, i.Open(uri))
		assertURI(t, i, uri)

		// allow for syntax to unpack things
		time.Sleep(200 * time.Millisecond)
	})
}

// TestWonAliasIntegration is an end-to-end test that wires the IDE
// through real configuration to verify the `won` alias from the user's
// `~/.runedev/config.yaml`:
//
//	command:
//	  aliases:
//	    won:
//	      command: workspacenew
//	      completer:
//	        - '{history}'
//	        - '{file}'
//
// The test goes through the real configuration loader, the real
// command alias parser, the real workspace history (backed by
// localstorage on a temp dir), and the real text.Component completion
// path. After dispatching `:won <repoA>` once, querying the alias
// completion again must surface "<repoA>" as the first match — proving
// that `{history}` is wired correctly all the way from the YAML
// completer chain through search.History.HistoryIterator.
func TestWonAliasIntegration(t *testing.T) {
	dataDir := t.TempDir()
	repoA := t.TempDir()
	repoB := t.TempDir()

	// Real config file with the `won` alias in YAML form, identical to
	// what the user has in ~/.runedev/config.yaml.
	configPath := filepath.Join(dataDir, "rune.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
editor:
  mode: modal
command:
  show_manual_after: 1h
  key: ":"
  aliases:
    won:
      command: workspacenew
      completer:
        - '{history}'
        - '{file}'
`), 0666))

	// Initial cwd workspace. We use repoB (different from repoA) so
	// the file-based completer's results are clearly distinguishable
	// from the history entries.
	// Seed repoB with a file so we can Open it as the initial
	// workspace; without an opened workspace the IDE root forwards
	// events differently and the command prompt would not even be
	// reachable from the empty root handler.
	repoBFile := filepath.Join(repoB, "seed.txt")
	require.NoError(t, os.WriteFile(repoBFile, nil, 0666))

	mu := new(sync.Mutex)
	i, err := New(repoB, configPath, dataDir,
		WithPublishEvent(nopPublishEvent),
		WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
		WithLocker(mu),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = i.Close() })
	root := i.Ready()

	repoBURI, err := workspaceapi.CurrentUserHostURI(repoBFile)
	require.NoError(t, err)
	mu.Lock()
	require.NoError(t, i.Open(repoBURI))
	mu.Unlock()

	// Sanity: alias is wired through configuration.
	wh := i.workspaceHandler
	aliases := i.ideConfig.commandAliases()
	wonAlias, ok := aliases["won"]
	require.True(t, ok, "won alias must be registered from YAML config")
	require.Len(t, wonAlias.Completers, 2,
		"won alias must have two completer factories ({history} + {file})")

	// Dispatch the alias by driving keyboard input through the IDE
	// root handler — the same code path a real user takes. This goes
	// through the command Prompt, which is what records the entered
	// command line into search.History on Enter.
	// term.ParseKeys requires `<space>` rather than literal spaces;
	// the alias name has none, but the command itself needs the
	// space token between `won` and the path argument.
	wonInvocation := ":won<space>" + repoA + "<enter>"
	keys, err := term.ParseKeys(wonInvocation)
	require.NoError(t, err)
	root.Resize(80, 24)
	for _, k := range keys {
		mu.Lock()
		root.Handle(term.Event{Type: term.EventKey, Ch: k.Ch, Mod: k.Mod, Key: k.Key})
		mu.Unlock()
	}

	// Wait for any async completion machinery to settle (the
	// dispatch path runs the alias handler on a goroutine and
	// records history when it returns).
	wh.focusEx().Wait()

	// Now ask the focused text.Component to complete the same alias
	// with no partial last arg. This is exactly the call the prompt
	// issues when the user types `:won ` (alias + space).
	ex := wh.exHandler(wh.focusHandler())
	require.NotNil(t, ex, "expected a focused ex handler after dispatch")
	mu.Lock()
	it, _, err := ex.comp.CompleteCommand(t.Context(),
		textapi.Command{Name: "won", Args: []string{""}})
	mu.Unlock()
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	got, err := iterator.ToSlice(t.Context(), it)
	require.NoError(t, err)
	require.NotEmpty(t, got,
		"won completion must surface at least the prior `won %s` history entry, "+
			"got nothing — `{history}` is not wired correctly", repoA)

	// History must come first per chain order in the YAML config. The
	// HistoryCompleter strips the alias-name prefix, so the entry the
	// user sees back is just the arg that was passed (the repoA path).
	assert.Equal(t, repoA, got[0],
		"first completion must be the prior `won` argument from history; "+
			"got %q. full result: %v", got[0], got)
}

// TestE2EWorkspaceReloadRestoresLayoutAndTerminalOutput drives the
// full real IDE — real config, real file scheme, real vte handler —
// through the same flow that surfaced the original
// :workspacereload DeadlineExceeded bug, and asserts that the
// post-reload IDE preserves both the workspace layout and the
// captured terminal output.
//
// Flow:
//  1. cwd workspace points to a temp dir on disk; auto_restore is on
//     so the reload skips the restore-prompt.
//  2. Open a test file (left window).
//  3. windownew right creates a fresh empty window on the right.
//  4. terminalnew opens a real vte.Handler in that right window
//     running `echo abc`.
//  5. After the terminal output settles, the test snapshots the
//     layout topology and the textual content of the terminal cells.
//  6. :workspacereload is dispatched the same way a user would
//     dispatch it — through the command prompt.
//  7. After the workspace re-installs, the layout must be the same
//     vertical split, the right window must still hold a vte
//     handler, and its snapshot must still contain "abc".
func TestE2EWorkspaceReloadRestoresLayoutAndTerminalOutput(t *testing.T) {
	dir := t.TempDir()
	dataDir := t.TempDir()

	// The command key is Ctrl+backslash. Tests pick that combination
	// instead of `:` because, after opening a real vte terminal,
	// keyboard focus lands on the vte and a plain `:` would be eaten
	// by the shell — `<c-\\>` reliably opens the command prompt no
	// matter which handler is in focus.
	//
	// auto_restore: true prevents the post-reload "Do you want to
	// restore the previous session?" prompt from interposing itself
	// between the reload and the test assertions.
	configPath := filepath.Join(dataDir, "rune.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`
editor:
  mode: modal
command:
  key: "<c-\\\\>"
workspace:
  auto_restore: true
`), 0o666))

	testFile := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(testFile, []byte("hello world\n"), 0o644))

	mu := new(sync.Mutex)
	// Scheduler that mirrors the production event loop: scheduled
	// callbacks run on a fresh goroutine while holding mu, so async
	// addWorkspace / reload installs can land back into IDE state.
	scheduleNextTick := func(fn func()) bool {
		go func() {
			mu.Lock()
			defer mu.Unlock()
			fn()
		}()
		return true
	}

	i, err := New(dir, configPath, dataDir,
		WithLocker(mu),
		WithScheduleNextTick(scheduleNextTick),
		WithPublishEvent(func(term.Event) bool { return true }),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = i.Close() })

	root := i.Ready()
	mu.Lock()
	root.Resize(80, 24)
	mu.Unlock()
	i.WaitWorkspaces()

	sendKeys := func(t *testing.T, seq string) {
		t.Helper()
		keys, err := term.ParseKeys(seq)
		require.NoError(t, err)
		for _, k := range keys {
			mu.Lock()
			root.Handle(term.Event{
				Type: term.EventKey, Ch: k.Ch, Mod: k.Mod, Key: k.Key,
			})
			mu.Unlock()
			i.WaitInflight()
		}
	}

	// 1) Open the file (left window).
	// 2) Open an empty window on the right.
	// 3) Open a terminal in that right window running `echo abc`.
	sendKeys(t,
		"<c-\\\\>edit<space>"+testFile+"<enter>"+
			"<c-\\\\>windownew<space>right<enter>"+
			"<c-\\\\>terminalnew<space>echo<space>abc<enter>",
	)

	// Find the terminal we just created and wait for "abc" to land
	// in its active cells.
	type windowSnapshot struct {
		windowID  uint64
		cellsText string
	}
	snapshotTerminalWindow := func() (windowSnapshot, bool) {
		mu.Lock()
		defer mu.Unlock()
		ex := i.workspaceHandler.focusEx()
		var found windowSnapshot
		var ok bool
		ex.comp.Browser().IterateWindows(func(win browser.Window) {
			if ok {
				return
			}
			content, cerr := win.Content()
			if cerr != nil {
				return
			}
			vte, isVTE := content.(vtereservoir.VTE)
			if !isVTE {
				return
			}
			snap, sErr := vte.Snapshot()
			if sErr != nil {
				return
			}
			found = windowSnapshot{
				windowID:  win.WindowID(),
				cellsText: term.CellsToString(snap.ActiveCells()),
			}
			ok = true
		})
		return found, ok
	}
	var before windowSnapshot
	require.Eventually(t, func() bool {
		snap, ok := snapshotTerminalWindow()
		if !ok {
			return false
		}
		if !strings.Contains(snap.cellsText, "abc") {
			return false
		}
		before = snap
		return true
	}, 10*time.Second, 50*time.Millisecond,
		"terminal did not produce `abc` before workspacereload")

	// Capture the layout structure before reload — must remain
	// identical after reload.
	mu.Lock()
	layoutBefore := i.workspaceHandler.focusEx().comp.Browser().TileLayout()
	mu.Unlock()
	require.Equal(t, tcomponent.SplitOrientationVertical, layoutBefore.Split,
		"pre-reload layout must be a single vertical split "+
			"(left file / right terminal)")
	require.Len(t, layoutBefore.Children, 2,
		"pre-reload layout must have exactly two leaves")
	rightLeafBefore := layoutBefore.Children[1]
	require.Equal(t, before.windowID, rightLeafBefore.WindowID,
		"the terminal must live in the right leaf of the pre-reload layout")

	// 4) Drive :workspacereload through the command prompt — the
	// same path a real user takes.
	sendKeys(t, "<c-\\\\>workspacereload<enter>")

	// reload tears down and re-adds the workspace asynchronously
	// through addWorkspace.
	i.WaitWorkspaces()
	i.WaitInflight()

	require.Eventually(t, func() bool {
		mu.Lock()
		ex := i.workspaceHandler.focusEx()
		layout := ex.comp.Browser().TileLayout()
		mu.Unlock()
		return len(layout.Children) == 2
	}, 30*time.Second, 50*time.Millisecond,
		"workspace reload did not restore the split layout")

	// Layout must be preserved: still one vertical split with two
	// leaves.
	mu.Lock()
	layoutAfter := i.workspaceHandler.focusEx().comp.Browser().TileLayout()
	mu.Unlock()
	require.Equal(t, tcomponent.SplitOrientationVertical, layoutAfter.Split,
		"post-reload layout must remain a vertical split")
	require.Len(t, layoutAfter.Children, 2,
		"post-reload layout must still have two leaves")

	// The right window's terminal must be re-created with the same
	// captured output. RestoreTileLayout allocates fresh window IDs,
	// so we do not require the leaves' WindowIDs to match pre-reload
	// — only that a vte still lives in the workspace and that its
	// snapshot contains "abc".
	var after windowSnapshot
	require.Eventually(t, func() bool {
		snap, ok := snapshotTerminalWindow()
		if !ok {
			return false
		}
		if !strings.Contains(snap.cellsText, "abc") {
			return false
		}
		after = snap
		return true
	}, 10*time.Second, 50*time.Millisecond,
		"post-reload terminal must still contain `abc`")

	// The restored terminal must live in the right leaf of the
	// post-reload layout.
	rightLeafAfter := layoutAfter.Children[1]
	require.NotZero(t, rightLeafAfter.WindowID,
		"post-reload right leaf must reference a concrete window")
	require.Equal(t, rightLeafAfter.WindowID, after.windowID,
		"the restored terminal must live in the right leaf of the post-reload layout")
}

type mockShader struct {
	called bool
	frames []int
}

func (s *mockShader) Shade(frame, total int, in [][]term.Cell) {
	s.called = true
	s.frames = append(s.frames, frame)
}

func makeTestFiles(t *testing.T) (*os.File, *os.File) {
	configFile, err := os.CreateTemp("", "six_ide_test.*.yaml")
	require.NoError(t, err)

	_, err = configFile.WriteString("{}")
	require.NoError(t, err)

	require.NoError(t, configFile.Close())

	file, err := os.CreateTemp("", "six_ide_test.*.go")
	require.NoError(t, err)
	require.NoError(t, file.Close())

	t.Cleanup(func() {
		_ = os.Remove(configFile.Name())
		_ = os.Remove(file.Name())
	})

	return configFile, file
}

func testRunnerFn(
	uri workspaceapi.URI,
	res map[extensionapi.Permission]extension.ResourceRegistrar,
	dataDir string, n browser.Notifications,
	exec, extExec schemeapi.Executor,
	grantor extension.Grantor,
	editor text.Editor,
	promptOpener ideauthorizer.PromptOpener, storage storageapi.Service,
	scheduleNextTick func(func()) bool) (extension.Runner, error) {
	return testRunner{}, nil
}

type testRunner struct {
}

func (r testRunner) Run(extensionID, path string, config config.Config) error {
	return nil
}

func (r testRunner) Close() error {
	return nil
}

// TestIDEBYOEMisconfigurationFallsBackToDefault is an end-to-end
// guard against byoe.New panics when the user's config selects
// `editor.mode = "byoe"` but does not supply both required fields
// (`editor.byoe.command` containing {file}, and `editor.byoe.goto`).
// validateBYOE rewrites the mode back to "modal" so the IDE boots
// with the built-in modal editor; this test asserts that the
// rewrite actually happens at the config layer so the workspace
// handler never reaches byoe.New on a misconfigured input.
//
// Reproduces the panic chain that motivated this guard:
//
//	byoe.New: command is required
//	byoe.New: invalid gotoTemplate: ...
//
// Either panic would crash the IDE on startup when a user
// previously experimented with `editor.mode = "byoe"` and removed
// only part of the byoe block.
func TestIDEBYOEMisconfigurationFallsBackToDefault(t *testing.T) {
	cases := []struct {
		name   string
		byoe   string // YAML body inserted under editor:byoe
		hasKey bool   // when false, omit the byoe block entirely
	}{
		{
			name:   "no byoe block at all",
			hasKey: false,
		},
		{
			name: "empty command, valid goto",
			byoe: `    command: ""
    goto: "<esc>:{line}<enter>{col}|"`,
			hasKey: true,
		},
		{
			name: "command without {file}, valid goto",
			byoe: `    command: "vim"
    goto: "<esc>:{line}<enter>{col}|"`,
			hasKey: true,
		},
		{
			name: "valid command, empty goto",
			byoe: `    command: "vim {file}"
    goto: ""`,
			hasKey: true,
		},
		{
			name:   "valid command, missing goto field",
			byoe:   `    command: "vim {file}"`,
			hasKey: true,
		},
		{
			name: "valid command, invalid goto",
			byoe: `    command: "vim {file}"
    goto: "<bogus-key>"`,
			hasKey: true,
		},
		{
			name: "empty command, empty goto",
			byoe: `    command: ""
    goto: ""`,
			hasKey: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configFile, _ := makeTestFiles(t)
			cfg := "editor:\n  mode: byoe\n"
			if tc.hasKey {
				cfg += "  byoe:\n" + tc.byoe + "\n"
			}
			require.NoError(t,
				os.WriteFile(configFile.Name(), []byte(cfg), 0666))

			dir, err := os.MkdirTemp("", "")
			require.NoError(t, err)
			t.Cleanup(func() { _ = os.RemoveAll(dir) })

			cwdURI, err := workspaceapi.CurrentUserHostURI(".")
			require.NoError(t, err)

			// Use init() rather than New() so we can introspect
			// the post-load ideConfig before any workspace
			// handler reaches byoe.New. init() must not panic
			// for any of these inputs: validateBYOE rewrites
			// the mode back to "modal" before the workspace
			// handler instantiates the editor.
			i := new(IDE)
			require.NotPanics(t, func() {
				err = i.init(cwdURI.String(),
					configFile.Name(), dir,
					WithPublishEvent(nopPublishEvent),
					WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
					WithLocker(new(sync.Mutex)))
			}, "IDE init must not panic for misconfigured byoe; "+
				"validateBYOE must rewrite editor.mode to a "+
				"safe fallback before reaching byoe.New")
			require.NoError(t, err,
				"IDE init must still succeed for "+
					"misconfigured byoe; validateBYOE "+
					"surfaces a non-fatal config error and "+
					"the IDE boots with the fallback mode")

			assert.NotEqual(t, "byoe", i.ideConfig.editorMode(),
				"after validateBYOE, editor.mode must not "+
					"remain byoe; got %q",
				i.ideConfig.editorMode())
			assert.Equal(t, "modal", i.ideConfig.editorMode(),
				"validateBYOE falls back to the safe "+
					"default mode (modal); a different "+
					"value means the validator regressed "+
					"or a new code path skipped the "+
					"rewrite")

			assert.NoError(t, i.closeResources())
		})
	}
}

// TestIDEBYOEWellFormedConfigDoesNotFallBack guards against an
// over-eager validateBYOE that would rewrite legitimate byoe
// configurations back to "modal". This is the positive
// counterexample to TestIDEBYOEMisconfigurationFallsBackToDefault.
func TestIDEBYOEWellFormedConfigDoesNotFallBack(t *testing.T) {
	configFile, _ := makeTestFiles(t)
	const cfg = `editor:
  mode: byoe
  byoe:
    command: "vim {file}"
    goto: "<esc>:{line}<enter>{col}|"
`
	require.NoError(t,
		os.WriteFile(configFile.Name(), []byte(cfg), 0666))

	dir, err := os.MkdirTemp("", "")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	cwdURI, err := workspaceapi.CurrentUserHostURI(".")
	require.NoError(t, err)

	i := new(IDE)
	require.NotPanics(t, func() {
		err = i.init(cwdURI.String(),
			configFile.Name(), dir,
			WithPublishEvent(nopPublishEvent),
			WithExtensionsRunner(FuncExtensionsRunner(testRunnerFn)),
			WithLocker(new(sync.Mutex)))
	})
	require.NoError(t, err)

	assert.Equal(t, "byoe", i.ideConfig.editorMode(),
		"a complete byoe config (command + goto) must be "+
			"preserved through validateBYOE")
	assert.Equal(t, "vim {file}", i.ideConfig.byoeCommand())
	assert.Equal(t, "<esc>:{line}<enter>{col}|", i.ideConfig.byoeGoto())

	assert.NoError(t, i.closeResources())
}
