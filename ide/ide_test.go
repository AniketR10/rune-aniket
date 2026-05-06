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
	"unstable.build/go-tui/component/shader"
	"unstable.build/go-tui/extension"
	"unstable.build/go-tui/ide/ideauthorizer"
	"unstable.build/go-tui/ide/idepkg/idepkgtest"
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
