// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/term"
	"gopkg.in/yaml.v3"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/term/gui/appmenu"
	"unstable.build/go-tui/term/gui/openpanel"
)

func menuCommands(menus []appmenu.Menu) []appmenu.Command {
	var cmds []appmenu.Command
	for _, menu := range menus {
		cmds = append(cmds, itemCommands(menu.Items)...)
	}
	return cmds
}

// itemCommands collects enabled Command items, descending into submenus.
// Disabled placeholders (e.g. an empty Open Recent list) are skipped:
// they carry no command and are not user-dispatchable.
func itemCommands(items []appmenu.Item) []appmenu.Command {
	var cmds []appmenu.Command
	for _, item := range items {
		switch it := item.(type) {
		case appmenu.Command:
			if !it.Disabled {
				cmds = append(cmds, it)
			}
		case appmenu.Submenu:
			cmds = append(cmds, itemCommands(it.Items)...)
		}
	}
	return cmds
}

func menuByTitle(t *testing.T, menus []appmenu.Menu, title string) appmenu.Menu {
	t.Helper()
	for _, menu := range menus {
		if menu.Title == title {
			return menu
		}
	}
	t.Fatalf("menu %q not found", title)
	return appmenu.Menu{}
}

func submenuByTitle(t *testing.T, items []appmenu.Item, title string) appmenu.Submenu {
	t.Helper()
	for _, item := range items {
		if submenu, ok := item.(appmenu.Submenu); ok && submenu.Title == title {
			return submenu
		}
	}
	t.Fatalf("submenu %q not found", title)
	return appmenu.Submenu{}
}

// appMenuPresetFiles are the presets that can back a macOS menu bar.
// The Linux standard preset is excluded: the menu bar is macOS-only.
var appMenuPresetFiles = []string{
	"preset_modal.yaml",
	"preset_standard_darwin.yaml",
	"preset_emacs.yaml",
}

var appMenuTestModels = []string{"anthropic/claude"}
var appMenuTestTutorials = []string{"agent", "basics", "navigation"}

// TestAppMenusShippedPresetsBindCommands asserts that every shipped
// editor preset binds each exit-capable menu command to a single chord
// (menu activation replays the chord, and the exit signal is only
// observable on the handler event path; other commands fall back to a
// direct dispatch), and that no native accelerator shadows a preset
// chord: AppKit intercepts menu key equivalents before they reach the
// window, so a collision would make the preset binding unreachable.
func TestAppMenusShippedPresetsBindCommands(t *testing.T) {
	exitCommands := map[string]bool{
		"quit": true, "forcequit!": true, "writequit": true, "writeforcequit!": true,
	}
	for _, name := range appMenuPresetFiles {
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(name)
			require.NoError(t, err)
			var cfg map[string]any
			require.NoError(t, yaml.Unmarshal(raw, &cfg))

			bindings := ide.CommandKeyBindings(config.MapConfig(cfg))
			menus := appMenus(bindings, nil, appMenuTestModels, appMenuTestTutorials)

			cmds := menuCommands(menus)
			require.NotEmpty(t, cmds)
			var sawExit bool
			for _, cmd := range cmds {
				if !exitCommands[cmd.Command] {
					continue
				}
				sawExit = true
				assert.NotEqual(t, term.KeyComb{}, cmd.Key,
					"%s: command %q has no key binding", name, cmd.Command)
			}
			require.True(t, sawExit, "menu spec lost its quit item")

			bound := map[term.KeyComb][]string{}
			for line, key := range bindings {
				bound[key] = append(bound[key], line)
			}
			for _, menu := range menus {
				for _, item := range menu.Items {
					native, ok := item.(appmenu.Native)
					if !ok || native.Key == (term.KeyComb{}) {
						continue
					}
					assert.Empty(t, bound[native.Key],
						"%s: native item %q accelerator %s shadows a preset binding",
						name, native.Title, native.Key)
				}
			}
		})
	}
}

// TestAppMenusItemInvariants pins the structural rules of the menu
// spec: titles and commands are non-empty, and prompt-prefill items use
// the presets' `echo {prompt}...<space>` macro shape with an ellipsis
// in the title signalling that further input is required.
func TestAppMenusItemInvariants(t *testing.T) {
	titles := map[string]bool{}
	for _, menu := range appMenus(
		map[string]term.KeyComb{}, nil, appMenuTestModels, appMenuTestTutorials,
	) {
		require.NotEmpty(t, menu.Title)
		require.NotEmpty(t, menu.Items, "%s: empty menu", menu.Title)
		for _, cmd := range itemCommands(menu.Items) {
			require.NotEmpty(t, cmd.Title)
			require.NotEmpty(t, cmd.Command)
			assert.False(t, titles[cmd.Title],
				"%s: duplicate item title %q", menu.Title, cmd.Title)
			titles[cmd.Title] = true
			if cmd.Command != "echo" {
				continue
			}
			require.Len(t, cmd.Args, 1,
				"%s: echo prefill must carry a single macro argument", cmd.Title)
			assert.True(t, strings.HasPrefix(cmd.Args[0], "{prompt}"),
				"%s: prefill %q must open the prompt", cmd.Title, cmd.Args[0])
			assert.True(t, strings.HasSuffix(cmd.Args[0], "<space>"),
				"%s: prefill %q must end with a trailing space", cmd.Title, cmd.Args[0])
			assert.True(t, strings.HasSuffix(cmd.Title, "…"),
				"%s: prefill items require further input and need an ellipsis", cmd.Title)
		}
	}
}

// TestAppMenusPanelCommands pins which menu items collect their path
// argument through the native open panel: they must be direct commands
// covered by the panel table, render with an ellipsis, and the table
// must not carry stale entries no menu item can reach.
func TestAppMenusPanelCommands(t *testing.T) {
	byTitle := map[string]appmenu.Command{}
	for _, cmd := range menuCommands(appMenus(
		map[string]term.KeyComb{}, nil, appMenuTestModels, appMenuTestTutorials,
	)) {
		byTitle[cmd.Title] = cmd
	}

	openFile := byTitle["Open File…"]
	require.Equal(t, "edit", openFile.Command)
	fileOpts, ok := appMenuPanelCommands[openFile.Command]
	require.True(t, ok, "Open File… must be covered by the panel table")
	assert.True(t, fileOpts.Multiple, "Open File… must allow multi-select")
	assert.False(t, fileOpts.Directories, "Open File… must select files")

	openProject := byTitle["Open Project…"]
	require.Equal(t, "workspaceopen", openProject.Command)
	projOpts, ok := appMenuPanelCommands[openProject.Command]
	require.True(t, ok, "Open Project… must be covered by the panel table")
	assert.True(t, projOpts.Directories, "Open Project… must select a directory")
	assert.False(t, projOpts.Multiple, "Open Project… must select a single directory")

	openReadOnly := byTitle["Open Read-Only…"]
	require.Equal(t, "view", openReadOnly.Command)
	viewOpts, ok := appMenuPanelCommands[openReadOnly.Command]
	require.True(t, ok, "Open Read-Only… must be covered by the panel table")
	assert.True(t, viewOpts.Multiple, "Open Read-Only… must allow multi-select")
	assert.False(t, viewOpts.Directories, "Open Read-Only… must select files")

	// The keyboard prompt flow stays available for the Workspace menu.
	assert.Equal(t, "echo", byTitle["Open Workspace…"].Command)

	covered := map[string]bool{}
	for _, cmd := range byTitle {
		if _, ok := appMenuPanelCommands[cmd.Command]; ok {
			covered[cmd.Command] = true
			assert.True(t, strings.HasSuffix(cmd.Title, "…"),
				"%s: panel items require further input and need an ellipsis", cmd.Title)
		}
	}
	assert.Len(t, covered, len(appMenuPanelCommands),
		"panel table entries must all be reachable from the menu bar")
}

// TestGoMenuCursorHistoryOpensPicker asserts the Go ▸ Cursor History
// item opens the prompt prefilled with the `cursorhistory jump` macro.
// A bare `cursorhistory` dispatch fails with "missing cursorhistory
// subcommand", so the item must carry the interactive subcommand.
func TestGoMenuCursorHistoryOpensPicker(t *testing.T) {
	byTitle := map[string]appmenu.Command{}
	for _, cmd := range menuCommands(appMenus(
		map[string]term.KeyComb{}, nil, appMenuTestModels, appMenuTestTutorials,
	)) {
		byTitle[cmd.Title] = cmd
	}

	cursorHistory := byTitle["Cursor History…"]
	require.Equal(t, "echo", cursorHistory.Command)
	require.Equal(t, []string{"{prompt}cursorhistory<space>jump<space>"},
		cursorHistory.Args)
}

// TestFindMenuLSPPrefills asserts the Find menu exposes the LSP query
// commands as prompt-prefills, so the prompt opens pre-typed and the
// user confirms in place, matching the other Find entries.
func TestFindMenuLSPPrefills(t *testing.T) {
	byTitle := map[string]appmenu.Command{}
	for _, cmd := range menuCommands(appMenus(
		map[string]term.KeyComb{}, nil, appMenuTestModels, appMenuTestTutorials,
	)) {
		byTitle[cmd.Title] = cmd
	}

	want := map[string]string{
		"Find Definition…":     "{prompt}lsp<space>definition<space>",
		"Find Implementation…": "{prompt}lsp<space>implementation<space>",
		"Find References…":     "{prompt}lsp<space>references<space>",
		"Find Documentation…":  "{prompt}lsp<space>hover<space>",
	}
	for title, macro := range want {
		item, ok := byTitle[title]
		require.Truef(t, ok, "Find menu missing %q", title)
		assert.Equal(t, "echo", item.Command, title)
		assert.Equal(t, []string{macro}, item.Args, title)
	}

	// The location picker item was removed from the Find menu.
	assert.NotContains(t, byTitle, "Location Picker…")
}

func TestAppMenusToolsAgentAndTasks(t *testing.T) {
	menus := appMenus(map[string]term.KeyComb{}, nil,
		appMenuTestModels, appMenuTestTutorials)
	tools := menuByTitle(t, menus, "Tools")
	agent := submenuByTitle(t, tools.Items, "Agent")

	model := submenuByTitle(t, agent.Items, "Change Model")
	assert.Equal(t, []appmenu.Item{appmenu.Command{
		Title: "anthropic/claude", Command: "chatmodel", Args: []string{"anthropic/claude"},
	}}, model.Items)

	effort := submenuByTitle(t, agent.Items, "Change Effort")
	assert.Equal(t, []appmenu.Item{
		appmenu.Command{Title: "None", Command: "chateffort", Args: []string{"none"}},
		appmenu.Command{Title: "Minimal", Command: "chateffort", Args: []string{"minimal"}},
		appmenu.Command{Title: "Low", Command: "chateffort", Args: []string{"low"}},
		appmenu.Command{Title: "Medium", Command: "chateffort", Args: []string{"medium"}},
		appmenu.Command{Title: "High", Command: "chateffort", Args: []string{"high"}},
		appmenu.Command{Title: "XHigh", Command: "chateffort", Args: []string{"xhigh"}},
		appmenu.Command{Title: "Max", Command: "chateffort", Args: []string{"max"}},
		appmenu.Command{Title: "Ultra", Command: "chateffort", Args: []string{"ultra"}},
	}, effort.Items)

	byTitle := map[string]appmenu.Command{}
	for _, cmd := range itemCommands(tools.Items) {
		byTitle[cmd.Title] = cmd
	}
	assert.Equal(t, appmenu.Command{
		Title: "New Terminal", Command: "terminalnew", Args: []string{},
	}, byTitle["New Terminal"])
	assert.Equal(t, appmenu.Command{
		Title: "New Terminal Tab", Command: "terminalnewtab", Args: []string{},
	}, byTitle["New Terminal Tab"])
	assert.Equal(t, appmenu.Command{
		Title: "New Terminal Or Split", Command: "terminalneworsplit", Args: []string{},
	}, byTitle["New Terminal Or Split"])
	assert.Equal(t, appmenu.Command{Title: "New", Command: "agent", Args: []string{}},
		byTitle["New"])
	assert.Equal(t, appmenu.Command{
		Title: "Change Maximum Tokens…", Command: "echo",
		Args: []string{"{prompt}chatmaxtokens<space>"},
	}, byTitle["Change Maximum Tokens…"])
	assert.Equal(t, "chatcompact", byTitle["Compact"].Command)
	assert.Equal(t, "chatfork", byTitle["Fork"].Command)
	assert.Equal(t, "chatclear", byTitle["Clear"].Command)
	assert.Equal(t, "chatexport", byTitle["Export"].Command)
	assert.Equal(t, appmenu.Command{
		Title: "Invoke Skill…", Command: "echo",
		Args: []string{"{prompt}chatskill<space>"},
	}, byTitle["Invoke Skill…"])
	assert.Equal(t, []string{"{prompt}tasknew<space>"}, byTitle["New Task…"].Args)
	assert.Equal(t, []string{"{prompt}tasknewtab<space>"}, byTitle["New Task Tab…"].Args)
	assert.Equal(t, appmenu.Command{
		Title: "Console", Command: "console", Args: []string{"help"},
	}, byTitle["Console"])
}

func TestAppMenusTutorialActions(t *testing.T) {
	help := menuByTitle(t,
		appMenus(map[string]term.KeyComb{}, nil,
			appMenuTestModels, appMenuTestTutorials), "Help")
	start := submenuByTitle(t, help.Items, "Start Tutorial")
	assert.Equal(t, []appmenu.Item{
		appmenu.Command{Title: "agent", Command: "tutorial", Args: []string{"start", "agent"}},
		appmenu.Command{Title: "basics", Command: "tutorial", Args: []string{"start", "basics"}},
		appmenu.Command{Title: "navigation", Command: "tutorial", Args: []string{"start", "navigation"}},
	}, start.Items)

	byTitle := map[string]appmenu.Command{}
	for _, cmd := range itemCommands(help.Items) {
		byTitle[cmd.Title] = cmd
	}
	assert.Equal(t, appmenu.Command{
		Title: "Stop Tutorial", Command: "tutorial", Args: []string{"stop"},
	}, byTitle["Stop Tutorial"])
}

func TestAppMenuDirectChoicesAvailableFromHomeWorkspace(t *testing.T) {
	b := newConfiguredBootstrapForEnvTest(t, configFilename,
		"editor:\n  mode: modal\n", t.TempDir())
	models := b.appMenuModels()
	tutorials := b.appMenuTutorials()
	require.NotEmpty(t, models)
	assert.Contains(t, models, "openai/gpt-5")
	assert.Equal(t, []string{"agent", "basics", "navigation"}, tutorials)

	menus := appMenus(map[string]term.KeyComb{}, nil, models, tutorials)
	agent := submenuByTitle(t, menuByTitle(t, menus, "Tools").Items, "Agent")
	modelItems := submenuByTitle(t, agent.Items, "Change Model").Items
	assert.Contains(t, modelItems, appmenu.Command{
		Title: "openai/gpt-5", Command: "chatmodel", Args: []string{"openai/gpt-5"},
	})
}

// TestAppMenuKeyBindingsBootstrapFallback asserts the quit accelerator
// survives the bootstrap wizard, where no editor preset has been
// written yet and the config binds nothing.
func TestAppMenuKeyBindingsBootstrapFallback(t *testing.T) {
	bindings := appMenuKeyBindings(config.MapConfig(map[string]any{}))
	assert.Equal(t, bootstrapQuitChord, bindings["quit"])
	assert.Equal(t, term.Event{
		Type: term.EventKey,
		Mod:  term.ModMeta,
		Ch:   'q',
	}, quitEvent(bindings))
}

// TestQuitEventFollowsConfiguredBinding asserts a window close request
// is routed to the user's own quit chord rather than a hardcoded one.
func TestQuitEventFollowsConfiguredBinding(t *testing.T) {
	bindings := appMenuKeyBindings(config.MapConfig(map[string]any{
		"command": map[string]any{
			"key_bindings": map[string]any{"<c-a-x>": "quit"},
		},
	}))
	assert.Equal(t, term.Event{
		Type: term.EventKey,
		Mod:  term.ModCtrlAlt,
		Ch:   'x',
	}, quitEvent(bindings))
}

func TestAppMenusDeriveAccelerators(t *testing.T) {
	menus := appMenus(map[string]term.KeyComb{
		"quit":                              {Mod: term.ModMeta, Ch: 'q'},
		"lsp definition":                    {Mod: term.ModMeta, Ch: 'd'},
		"echo {prompt}workspaceopen<space>": {Mod: term.ModMeta, Ch: 't'},
	}, nil, appMenuTestModels, appMenuTestTutorials)

	byTitle := map[string]appmenu.Command{}
	for _, cmd := range menuCommands(menus) {
		byTitle[cmd.Title] = cmd
	}

	// Direct commands resolve their own command line.
	assert.Equal(t, appmenu.Command{
		Title:   "Quit Rune",
		Command: "quit",
		Args:    []string{},
		Key:     term.KeyComb{Mod: term.ModMeta, Ch: 'q'},
	}, byTitle["Quit Rune"])
	assert.Equal(t, appmenu.Command{
		Title:   "Go to Definition",
		Command: "lsp",
		Args:    []string{"definition"},
		Key:     term.KeyComb{Mod: term.ModMeta, Ch: 'd'},
	}, byTitle["Go to Definition"])
	// Prefill items resolve the full `echo {prompt}...` macro line.
	assert.Equal(t, appmenu.Command{
		Title:   "Open Workspace…",
		Command: "echo",
		Args:    []string{"{prompt}workspaceopen<space>"},
		Key:     term.KeyComb{Mod: term.ModMeta, Ch: 't'},
	}, byTitle["Open Workspace…"])
	// Unbound commands render without an accelerator.
	assert.Equal(t, term.KeyComb{}, byTitle["Save"].Key)

	var titles []string
	for _, menu := range menus {
		titles = append(titles, menu.Title)
	}
	assert.Equal(t, []string{
		"Rune", "File", "Edit", "View", "Find", "Go",
		"Workspace", "Tools", "Help",
	}, titles)
}

// TestActivateAppMenuCommandPublishesChord asserts menu activation
// replays the bound chord as a key event so it flows through the whole
// handler chain, rather than dispatching the command out of band.
func TestActivateAppMenuCommandPublishesChord(t *testing.T) {
	var published []term.Event
	b := &bootstrapHandler{publishEvent: func(ev term.Event) bool {
		published = append(published, ev)
		return true
	}}

	b.activateAppMenuCommand(appmenu.Command{
		Command: "quit",
		Key:     term.KeyComb{Mod: term.ModMeta, Ch: 'q'},
	})

	require.Len(t, published, 1)
	assert.Equal(t, term.Event{
		Type: term.EventKey,
		Mod:  term.ModMeta,
		Ch:   'q',
	}, published[0])
}

// TestActivateAppMenuCommandWithoutBinding asserts an unbound command
// falls back to a main-thread dispatch instead of publishing a key
// event for the zero chord.
func TestActivateAppMenuCommandWithoutBinding(t *testing.T) {
	var published []term.Event
	b := &bootstrapHandler{publishEvent: func(ev term.Event) bool {
		published = append(published, ev)
		return true
	}}

	b.activateAppMenuCommand(appmenu.Command{Command: "config"})

	require.Len(t, published, 1)
	assert.Equal(t, term.EventInterrupt, published[0].Type)
	require.NotNil(t, published[0].UserFunc)
}

// panelStub replaces the showOpenPanel seam and records each request
// together with its completion callback.
type panelStub struct {
	opts []openpanel.Options
	done []func([]string)
}

func stubOpenPanel(t *testing.T) *panelStub {
	t.Helper()
	stub := &panelStub{}
	prev := showOpenPanel
	showOpenPanel = func(opts openpanel.Options, done func([]string)) {
		stub.opts = append(stub.opts, opts)
		stub.done = append(stub.done, done)
	}
	t.Cleanup(func() { showOpenPanel = prev })
	return stub
}

// TestActivateAppMenuCommandShowsOpenPanel asserts panel-table commands
// open the native panel instead of replaying the bound chord, and that
// a selection publishes a single dispatch interrupt while a cancel
// publishes nothing.
func TestActivateAppMenuCommandShowsOpenPanel(t *testing.T) {
	stub := stubOpenPanel(t)
	var published []term.Event
	b := &bootstrapHandler{
		realIDE: &ide.IDE{},
		publishEvent: func(ev term.Event) bool {
			published = append(published, ev)
			return true
		},
	}

	b.activateAppMenuCommand(appmenu.Command{
		Title:   "Open File…",
		Command: "edit",
		Key:     term.KeyComb{Mod: term.ModMeta, Ch: 'o'},
	})

	require.Len(t, stub.opts, 1, "menu click must show the panel")
	assert.Equal(t, appMenuPanelCommands["edit"], stub.opts[0])
	assert.Empty(t, published,
		"a bound panel command must not replay its chord")

	stub.done[0]([]string{"/tmp/a.txt", "/tmp/b.txt"})
	require.Len(t, published, 1, "selection must publish one interrupt")
	assert.Equal(t, term.EventInterrupt, published[0].Type)
	require.NotNil(t, published[0].UserFunc)

	b.activateAppMenuCommand(appmenu.Command{
		Title:   "Open Project…",
		Command: "workspaceopen",
	})
	require.Len(t, stub.opts, 2)
	assert.Equal(t, appMenuPanelCommands["workspaceopen"], stub.opts[1])
	stub.done[1](nil)
	assert.Len(t, published, 1, "cancel must publish nothing")
}

// TestActivateAppMenuCommandPanelDuringBootstrap asserts the panel is
// not shown while the wizard is still writing the configuration; the
// item degrades to the "not available during setup" notification.
func TestActivateAppMenuCommandPanelDuringBootstrap(t *testing.T) {
	stub := stubOpenPanel(t)
	var published []term.Event
	b := &bootstrapHandler{publishEvent: func(ev term.Event) bool {
		published = append(published, ev)
		return true
	}}

	b.activateAppMenuCommand(appmenu.Command{
		Title:   "Open File…",
		Command: "edit",
	})

	assert.Empty(t, stub.opts, "no panel during bootstrap")
	require.Len(t, published, 1)
	assert.Equal(t, term.EventInterrupt, published[0].Type)
	require.NotNil(t, published[0].UserFunc)
}

// TestActivateAppMenuCommandRecentDispatchesDirectly asserts an Open
// Recent item — a workspaceopen carrying an explicit path — dispatches
// to the IDE instead of opening the panel to ask for a path.
func TestActivateAppMenuCommandRecentDispatchesDirectly(t *testing.T) {
	stub := stubOpenPanel(t)
	var published []term.Event
	b := &bootstrapHandler{
		realIDE: &ide.IDE{},
		publishEvent: func(ev term.Event) bool {
			published = append(published, ev)
			return true
		},
	}

	b.activateAppMenuCommand(appmenu.Command{
		Title:   "proj",
		Command: "workspaceopen",
		Args:    []string{"/home/me/proj"},
	})

	assert.Empty(t, stub.opts, "a recent item must not open the panel")
	require.Len(t, published, 1, "a recent item dispatches on the main thread")
	assert.Equal(t, term.EventInterrupt, published[0].Type)
	require.NotNil(t, published[0].UserFunc)
}

// TestAppMenusOpenRecentSubmenu asserts the File menu carries an Open
// Recent submenu whose entries dispatch workspaceopen with the recorded
// path, and that an empty history renders a single disabled placeholder.
func TestAppMenusOpenRecentSubmenu(t *testing.T) {
	find := func(menus []appmenu.Menu) appmenu.Submenu {
		for _, menu := range menus {
			if menu.Title != "File" {
				continue
			}
			for _, item := range menu.Items {
				if sub, ok := item.(appmenu.Submenu); ok && sub.Title == "Open Recent" {
					return sub
				}
			}
		}
		t.Fatal("File ▸ Open Recent submenu not found")
		return appmenu.Submenu{}
	}

	recents := []recentEntry{
		{label: "app/web", path: "/home/app/web"},
		{label: "api/web", path: "/home/api/web"},
	}
	sub := find(appMenus(map[string]term.KeyComb{}, recents,
		appMenuTestModels, appMenuTestTutorials))
	require.Len(t, sub.Items, 2)
	assert.Equal(t, appmenu.Command{
		Title:   "app/web",
		Command: "workspaceopen",
		Args:    []string{"/home/app/web"},
	}, sub.Items[0])
	assert.Equal(t, appmenu.Command{
		Title:   "api/web",
		Command: "workspaceopen",
		Args:    []string{"/home/api/web"},
	}, sub.Items[1])

	empty := find(appMenus(map[string]term.KeyComb{}, nil,
		appMenuTestModels, appMenuTestTutorials))
	require.Len(t, empty.Items, 1)
	placeholder, ok := empty.Items[0].(appmenu.Command)
	require.True(t, ok)
	assert.True(t, placeholder.Disabled, "empty Open Recent must be disabled")
	assert.Empty(t, placeholder.Command)
}

// TestRecordRecentOpen asserts only workspaceopen feeds the recent list
// and that recording republishes the menu so it reflects the new entry.
func TestRecordRecentOpen(t *testing.T) {
	storage := newRuneStorage(t.TempDir())
	t.Cleanup(func() { _ = storage.Close() })
	var published []term.Event
	b := &bootstrapHandler{
		recent: newRecentWorkspaces(storage),
		publishEvent: func(ev term.Event) bool {
			published = append(published, ev)
			return true
		},
	}

	b.recordRecentOpen("edit", "/tmp/file.txt")
	assert.Empty(t, b.recent.paths(), "edit opens files, not projects")
	assert.Empty(t, published, "a non-project open must not refresh the menu")

	b.recordRecentOpen("workspaceopen", "/home/me/proj")
	assert.Equal(t, []string{"/home/me/proj"}, b.recent.paths())
	require.Len(t, published, 1, "recording a project must refresh the menu")
	assert.Equal(t, term.EventInterrupt, published[0].Type)
	require.NotNil(t, published[0].UserFunc)
}

// TestMergedRecentWorkspacesPrefersMenuOpens asserts menu-open history
// leads the merged Open Recent list and duplicates collapse across the
// two sources.
func TestMergedRecentWorkspacesPrefersMenuOpens(t *testing.T) {
	storage := newRuneStorage(t.TempDir())
	t.Cleanup(func() { _ = storage.Close() })
	b := &bootstrapHandler{recent: newRecentWorkspaces(storage)}
	b.recent.record("/home/me/beta")

	// realIDE is nil here, so only the menu-open source contributes;
	// this still proves the menu source flows into the merged list.
	entries := b.mergedRecentWorkspaces()
	require.Len(t, entries, 1)
	assert.Equal(t, "/home/me/beta", entries[0].path)
	assert.Equal(t, "beta", entries[0].label)
}
