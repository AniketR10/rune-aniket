// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/rune/internal/browser"
	"unstable.build/rune/internal/handler/command"
	"unstable.build/rune/internal/ide"
	"unstable.build/rune/internal/ide/idetutorial"
	"unstable.build/rune/internal/ide/idetutorial/starlarktutorial"
)

// TestBasicsTutorialParses asserts that the embedded basics.star
// tutorial parses through starlarktutorial.New, registers a
// callable entry, and reports the expected id/title/version.
func TestBasicsTutorialParses(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, basicsTutorial,
		"basicsTutorial embed must not be empty")
	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil,
		nil,
		nil,
		nil,
		term.Attributes{},
		nil,
		nil,
		term.KeyComb{Ch: ':'},
		"standard", "",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)

	assert.Equal(t, "basics", tut.ID())
	assert.Equal(t, "Rune basics", tut.Title())
	assert.Equal(t, "58", tut.Version())
}

// TestBasicsTutorialParsesModalMode asserts the embedded basics
// tutorial also parses under modal editor mode, exercising the
// modal-only branches (e.g. the modal-surfaces step).
func TestBasicsTutorialParsesModalMode(t *testing.T) {
	t.Parallel()

	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil,
		nil,
		nil,
		nil,
		term.Attributes{},
		nil,
		nil,
		term.KeyComb{Ch: ':'},
		"modal", "",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "58", tut.Version())
}

// TestBasicsTutorialWorkspaceOpenCopyByOS asserts the welcome window's
// workspace-open step teaches the native macOS File ▸ Open Project…
// flow on darwin and keeps the command-prompt steps on other systems.
func TestBasicsTutorialWorkspaceOpenCopyByOS(t *testing.T) {
	t.Parallel()

	tests := []struct {
		os        string
		contains  []string
		forbidden []string
	}{
		{
			os:        "darwin",
			contains:  []string{"Opening a project", "Open Project…", "File"},
			forbidden: []string{"Opening a workspace"},
		},
		{
			os:        "linux",
			contains:  []string{"Opening a workspace", "workspaceopen"},
			forbidden: []string{"Open Project…"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.os, func(t *testing.T) {
			t.Parallel()
			overlay := idetutorial.NewOverlayBrowser(
				browser.NewComponent(idetutorial.DefaultOverlayBrowserConfig()))
			tut, err := starlarktutorial.New(
				"basics", basicsTutorial,
				overlay, nil, nil, nil,
				term.Attributes{}, nil, nil,
				term.KeyComb{Ch: ':'},
				"standard", tt.os, nil,
				nil, nil, nil,
			)
			require.NoError(t, err)
			tut.Resize(120, 40)
			tut.Reset()
			require.True(t, tut.WaitActive("floating_window", time.Second))

			w := term.NewStringWriter(120, 40)
			tut.Draw(w)
			overlay.Draw(w)
			require.NoError(t, w.Flush())
			rendered := strings.Join(strings.Fields(
				strings.ReplaceAll(w.String(), "│", " ")), " ")
			for _, expected := range tt.contains {
				assert.Contains(t, rendered, expected)
			}
			for _, forbidden := range tt.forbidden {
				assert.NotContains(t, rendered, forbidden)
			}
		})
	}
}

// TestBasicsTutorialCompleterKeysByMode asserts the completion-list
// phrasing names each preset's own list bindings rather than a single
// hardcoded arrow-key spelling.
func TestBasicsTutorialCompleterKeysByMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode      string
		expected  []string
		forbidden []string
	}{
		{mode: "modal", expected: []string{"<ctrl-j>", "<ctrl-k>", "<up>", "<down>"}},
		{
			mode:      "standard",
			expected:  []string{"<up>", "<down>"},
			forbidden: []string{"<ctrl-i>", "<ctrl-k>"},
		},
		{mode: "emacs", expected: []string{"<ctrl-p>", "<ctrl-n>", "<up>", "<down>"}},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			src := `
def run():
    floating_window(text = completer_pick_phrase)
tutorial(entry=run)
`
			modeSrc := strings.Replace(basicsTutorial,
				`tutorial(id = "basics", title = "Rune basics", version = "58", entry = run)`,
				"", 1) + src
			tut, err := starlarktutorial.New(
				"basics-completer-keys", modeSrc,
				nil, nil, nil, nil,
				term.Attributes{}, nil, nil,
				term.KeyComb{Ch: ':'}, tt.mode, "",
				nil, nil, nil, nil,
			)
			require.NoError(t, err)
			tut.Resize(80, 24)
			tut.Reset()
			require.True(t, tut.WaitActive("floating_window", time.Second))

			text := tut.ActiveText()
			for _, key := range tt.expected {
				assert.Contains(t, text, key)
			}
			for _, key := range tt.forbidden {
				assert.NotContains(t, text, key)
			}
		})
	}
}

func TestBasicsTutorialLayoutIntro(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mode     string
		contains []string
	}{
		{
			name: "modal",
			mode: "modal",
			contains: []string{
				"HJKL controls the layout",
				"Hold <meta> and press HJKL",
				"Hold <alt> and press H/L",
				"Add <shift> to move content instead of focus it",
			},
		},
		{
			name: "emacs",
			mode: "emacs",
			contains: []string{
				"Emacs directions control the layout",
				"Rather than teach a second direction map, Rune changes the target",
				"hold <meta> with the same PNBF directions to focus windows",
				"add <shift> to move window content instead",
				"Reusing that muscle memory keeps repeated layout actions fast",
				"host Meta layer stays reachable from terminals",
				"<shift-meta-w> closes a window, <meta-k> closes the others",
				"<meta-d> / <meta-r> split below or right",
				"<meta-e> toggles maximization",
				"<meta-w> closes a tab; adding <shift> escalates",
				"<meta-[> / <meta-]> cycle tabs",
				"<shift-meta-[> / <shift-meta-]> reorder the current tab",
			},
		},
		{
			name: "standard",
			mode: "standard",
			contains: []string{
				"Alt drives the layout",
				"Vim made generations of programmers extraordinarily productive",
				"Keyboard-driven does not have to mean learning an entirely new way to edit",
				"Rune brings that advantage to a familiar, non-modal editor",
				"Hold <alt> and press IJKL to focus a window",
				"Add <shift> to move its content, or add <meta> to resize it",
				"Use <alt-[> / <alt-]> to switch tabs",
				"Use <alt-n> to split a window, <alt-enter> to open a terminal",
				"and <alt-q> to close a window",
				"Use <alt-t> to create a tab and <alt-w> to close it",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keyFor := func(cmd string, args []string) string {
				keys := map[string]string{
					"windowfocus left":   "<alt-j>",
					"windowmove left":    "<alt-shift-j>",
					"tabprevious":        "<alt-[>",
					"tabnext":            "<alt-]>",
					"tabmove left":       "<alt-shift-[>",
					"tabmove right":      "<alt-shift-]>",
					"windownew":          "<alt-n>",
					"terminalneworsplit": "<alt-enter>",
					"windowclose":        "<alt-q>",
					"tabnew":             "<alt-t>",
					"tabclose":           "<alt-w>",
				}
				if tt.mode == "emacs" {
					keys["windownew down"] = "<meta-d>"
					keys["windownew right"] = "<meta-r>"
					keys["windowclose"] = "<shift-meta-w>"
					keys["windowcloseall"] = "<meta-k>"
					keys["windowtogglemaximize"] = "<meta-e>"
					keys["tabclose"] = "<meta-w>"
					keys["tabprevious"] = "<meta-[>"
					keys["tabnext"] = "<meta-]>"
					keys["tabmove left"] = "<shift-meta-[>"
					keys["tabmove right"] = "<shift-meta-]>"
				}
				return keys[strings.Join(append([]string{cmd}, args...), " ")]
			}
			overlay := idetutorial.NewOverlayBrowser(
				browser.NewComponent(idetutorial.DefaultOverlayBrowserConfig()))
			tut, err := starlarktutorial.New(
				"basics", basicsTutorial,
				overlay, nil, nil, nil,
				term.Attributes{}, nil, nil,
				term.KeyComb{Ch: ':'},
				tt.mode, "", keyFor,
				nil, nil, nil,
			)
			require.NoError(t, err)
			tut.Resize(120, 40)
			tut.Reset()

			require.True(t, tut.WaitActive("floating_window", time.Second))
			_, _ = tut.Handle(term.Event{Type: term.EventKey, Ch: ':'})
			require.True(t, tut.WaitActive("wait_command", time.Second))
			tut.ObserveCommand("workspaceopen", "workspaceopen", []string{"/tmp/workspace"}, nil)
			require.True(t, tut.WaitActive("floating_window", time.Second))
			_, _ = tut.Handle(term.Event{Type: term.EventKey, Ch: ':'})
			require.True(t, tut.WaitActive("wait_command", time.Second))
			tut.ObserveCommand("edit", "edit", []string{"README.md"}, nil)
			require.True(t, tut.WaitActive("floating_window", time.Second))

			w := term.NewStringWriter(120, 40)
			tut.Draw(w)
			overlay.Draw(w)
			require.NoError(t, w.Flush())
			rendered := strings.Join(strings.Fields(
				strings.ReplaceAll(w.String(), "│", " ")), " ")
			for _, expected := range tt.contains {
				assert.Contains(t, rendered, expected)
			}
			assert.NotContains(t, rendered, "standard and Emacs")
		})
	}
}

func TestBasicsTutorialWelcomeUsesResolvedBindings(t *testing.T) {
	t.Parallel()

	keyFor := func(cmd string, args []string) string {
		key := strings.Join(append([]string{cmd}, args...), " ")
		if strings.HasPrefix(key, "workspacefocus ") {
			return "<f" + args[0] + ">"
		}
		if key == "terminalneworsplit" {
			return "<f10>"
		}
		return ""
	}
	overlay := idetutorial.NewOverlayBrowser(
		browser.NewComponent(idetutorial.DefaultOverlayBrowserConfig()))
	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		overlay, nil, nil, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", "", keyFor,
		nil, nil, nil,
	)
	require.NoError(t, err)
	tut.Resize(120, 40)
	tut.Reset()
	require.True(t, tut.WaitActive("floating_window", time.Second))

	w := term.NewStringWriter(120, 40)
	tut.Draw(w)
	overlay.Draw(w)
	require.NoError(t, w.Flush())
	rendered := strings.Join(strings.Fields(
		strings.ReplaceAll(w.String(), "│", " ")), " ")
	assert.Contains(t, rendered,
		"<f1> <f2> <f3> <f4> <f5> <f6> <f7> <f8> <f9>")
	assert.Contains(t, rendered, "Bindings like <f1> and <f10>")

	for _, key := range []term.Key{term.KeyF1, term.KeyF10} {
		exit, handled := tut.Handle(term.Event{Type: term.EventKey, Key: key})
		assert.False(t, exit)
		assert.Falsef(t, handled, "%v must fall through to the IDE", key)
		assert.True(t, tut.WaitActive("floating_window", time.Second))
	}
}

func TestBasicsTutorialHasNoHardcodedCommandKeys(t *testing.T) {
	t.Parallel()

	for _, key := range []string{
		"<meta-1>",
		"<ctrl-x>0",
		"<ctrl-x>1",
		"<ctrl-x>2",
		"<ctrl-x>3",
		"<ctrl-x>9",
		"<ctrl-tab>",
		"<ctrl-shift-tab>",
	} {
		assert.NotContainsf(t, basicsTutorial, key,
			"command key %s must be resolved through key_for", key)
	}
}

func TestBasicsTutorialResolvesDirectionalBindings(t *testing.T) {
	t.Parallel()

	requested := map[string]bool{}
	keyFor := func(cmd string, args []string) string {
		requested[strings.Join(append([]string{cmd}, args...), " ")] = true
		return ""
	}

	_, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil, nil, nil, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", "", keyFor,
		nil, nil, nil,
	)
	require.NoError(t, err)

	for _, command := range []string{
		"windowfocus up",
		"windowfocus left",
		"windowfocus down",
		"windowfocus right",
		"windowmove up",
		"windowmove left",
		"windowmove down",
		"windowmove right",
		"windowresize increase height",
		"windowresize decrease width",
		"windowresize decrease height",
		"windowresize increase width",
		"windowdefaultsplit h",
		"tabnext",
		"tabprevious",
		"tabmove left",
		"tabmove right",
	} {
		assert.Truef(t, requested[command],
			"the basics tutorial must resolve %q through the active preset", command)
	}
}

func TestBasicsTutorialResolvesEmacsLayoutBindings(t *testing.T) {
	t.Parallel()

	requested := map[string]bool{}
	keyFor := func(cmd string, args []string) string {
		requested[strings.Join(append([]string{cmd}, args...), " ")] = true
		return ""
	}

	_, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil, nil, nil, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'},
		"emacs", "", keyFor,
		nil, nil, nil,
	)
	require.NoError(t, err)

	for _, command := range []string{
		"windowtogglemaximize",
		"windownew down",
		"windownew right",
		"windowclose",
		"windowcloseall",
		"windowfocus up",
		"windowfocus left",
		"windowfocus down",
		"windowfocus right",
		"windowmove up",
		"windowmove left",
		"windowmove down",
		"windowmove right",
		"windowresize increase height",
		"windowresize decrease width",
		"windowresize decrease height",
		"windowresize increase width",
		"tabnext",
		"tabprevious",
		"tabmove left",
		"tabmove right",
		"tabnew",
		"tabclose",
	} {
		assert.Truef(t, requested[command],
			"the Emacs basics tutorial must resolve %q through the active preset", command)
	}
}

// TestBasicsTutorialLayoutKeysPlayable asserts that the "Your current
// layout keys" page lets the user actually try the bindings it lists:
// they must reach the IDE root while the page stays up, otherwise the
// table is just something to read past.
func TestBasicsTutorialLayoutKeysPlayable(t *testing.T) {
	t.Parallel()

	layoutKeys := map[string]string{
		"windowfocus up":    "<alt-i>",
		"windowfocus left":  "<alt-j>",
		"windowfocus down":  "<alt-k>",
		"windowfocus right": "<alt-l>",
		"windowmove right":  "<alt-shift-l>",
	}
	keyFor := func(cmd string, args []string) string {
		return layoutKeys[strings.Join(append([]string{cmd}, args...), " ")]
	}
	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil, nil, &capturingNotis{}, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", "", keyFor,
		nil, nil, nil,
	)
	require.NoError(t, err)
	tut.Resize(100, 30)
	tut.Reset()

	wait := func(kind string) {
		t.Helper()
		require.True(t, tut.WaitActive(kind, time.Second),
			"expected a %s step", kind)
	}
	dismissPromptStep := func() {
		t.Helper()
		wait("floating_window")
		_, _ = tut.Handle(term.Event{Type: term.EventKey, Ch: ':'})
	}
	observe := func(command string, args ...string) {
		t.Helper()
		wait("wait_command")
		tut.ObserveCommand(command, command, args, nil)
	}

	dismissPromptStep()
	observe("workspaceopen", "/tmp/tutorial-workspace")
	dismissPromptStep()
	observe("edit", "README.md")
	dismissPromptStep()
	dismissPromptStep()
	observe("windownew")
	dismissPromptStep()
	observe("terminalneworsplit")
	dismissPromptStep()
	observe("windowdefaultsplit", "h")
	dismissPromptStep()
	observe("terminalneworsplit")

	wait("floating_window")
	for _, spec := range []string{"<alt-i>", "<alt-l>", "<alt-shift-l>"} {
		ks, err := term.ParseKeys(spec)
		require.NoError(t, err)
		require.Len(t, ks, 1)
		exit, handled := tut.Handle(term.Event{
			Type: term.EventKey, Key: ks[0].Key, Mod: ks[0].Mod, Ch: ks[0].Ch,
		})
		assert.False(t, exit)
		assert.Falsef(t, handled,
			"%s must fall through to the IDE so the user can try it", spec)
		assert.Truef(t, tut.WaitActive("floating_window", time.Second),
			"%s must not advance past the layout table", spec)
	}
}

// TestBasicsTutorialDirectionalHintNamesKey asserts that a follow-up
// hint in a two-direction lesson names the chord it is waiting for.
// The hint's generic "Or you can press ..." line resolves the bare
// command name, which is unbound for direction-qualified commands, so
// without this the second half of the lesson shows no key at all.
func TestBasicsTutorialDirectionalHintNamesKey(t *testing.T) {
	t.Parallel()

	layoutKeys := map[string]string{
		"windowfocus up":   "<alt-i>",
		"windowfocus left": "<alt-j>",
		"windowmove right": "<alt-shift-l>",
		"windowmove left":  "<alt-shift-j>",
	}
	keyFor := func(cmd string, args []string) string {
		return layoutKeys[strings.Join(append([]string{cmd}, args...), " ")]
	}
	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil, nil, &capturingNotis{}, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", "", keyFor,
		nil, nil, nil,
	)
	require.NoError(t, err)
	tut.Resize(100, 30)
	tut.Reset()

	dismissPromptStep := func() {
		t.Helper()
		require.True(t, tut.WaitActive("floating_window", time.Second))
		_, _ = tut.Handle(term.Event{Type: term.EventKey, Ch: ':'})
	}
	observe := func(command string, args ...string) {
		t.Helper()
		require.True(t, tut.WaitActive("wait_command", time.Second))
		tut.ObserveCommand(command, command, args, nil)
	}

	dismissPromptStep()
	observe("workspaceopen", "/tmp/tutorial-workspace")
	dismissPromptStep()
	observe("edit", "README.md")
	dismissPromptStep()
	dismissPromptStep()
	observe("windownew")
	dismissPromptStep()
	observe("terminalneworsplit")
	dismissPromptStep()
	observe("windowdefaultsplit", "h")
	dismissPromptStep()
	observe("terminalneworsplit")

	require.True(t, tut.WaitActive("floating_window", time.Second))
	_, _ = tut.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	dismissPromptStep()
	require.True(t, tut.WaitActive("wait_command", time.Second))
	assert.Contains(t, tut.ActiveText(), "<alt-i>")
	tut.ObserveCommand("windowfocus", "windowfocus", []string{"up"}, nil)
	require.True(t, tut.WaitActive("wait_command", time.Second))
	assert.Contains(t, tut.ActiveText(), "<alt-j>")
	tut.ObserveCommand("windowfocus", "windowfocus", []string{"left"}, nil)

	dismissPromptStep()
	require.True(t, tut.WaitActive("wait_command", time.Second))
	assert.Contains(t, tut.ActiveText(), "<alt-shift-l>")
	tut.ObserveCommand("windowmove", "windowmove", []string{"right"}, nil)
	require.True(t, tut.WaitActive("wait_command", time.Second))
	assert.Contains(t, tut.ActiveText(), "<alt-shift-j>")
}

func TestBasicsTutorialEmacsSplitHintNamesKey(t *testing.T) {
	t.Parallel()

	var (
		lookupMu sync.Mutex
		lookups  []string
	)
	keyFor := func(cmd string, args []string) string {
		lookupMu.Lock()
		lookups = append(lookups,
			strings.Join(append([]string{cmd}, args...), " "))
		lookupMu.Unlock()
		if cmd == "windownew" && slices.Equal(args, []string{"right"}) {
			return "<meta-r>"
		}
		return ""
	}
	overlay := idetutorial.NewOverlayBrowser(
		browser.NewComponent(idetutorial.DefaultOverlayBrowserConfig()))
	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		overlay, nil, &capturingNotis{}, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'}, "emacs", "", keyFor,
		nil, nil, nil,
	)
	require.NoError(t, err)
	tut.Resize(100, 30)
	tut.Reset()

	dismiss := func() {
		t.Helper()
		require.True(t, tut.WaitActive("floating_window", time.Second))
		_, _ = tut.Handle(term.Event{Type: term.EventKey, Ch: ':'})
	}
	observe := func(command string, args ...string) {
		t.Helper()
		require.True(t, tut.WaitActive("wait_command", time.Second))
		tut.ObserveCommand(command, command, args, nil)
	}

	dismiss()
	observe("workspaceopen", "/tmp/tutorial-workspace")
	dismiss()
	observe("edit", "README.md")
	dismiss()
	lookupMu.Lock()
	lookups = nil
	lookupMu.Unlock()
	dismiss()
	require.True(t, tut.WaitActive("wait_command", time.Second))
	assert.Equal(t, "Split the active window to the right with `<meta-r>`.",
		tut.ActiveText())
	lookupMu.Lock()
	require.NotEmpty(t, lookups)
	assert.Equal(t, "windownew right", lookups[len(lookups)-1],
		"the active wait hint must resolve the expected invocation, not its bare command")
	lookupMu.Unlock()
}

func TestBasicsTutorialDirectionalCommandFlow(t *testing.T) {
	t.Parallel()

	notis := &capturingNotis{}
	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil, nil, notis, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", "", nil,
		nil, nil, nil,
	)
	require.NoError(t, err)
	tut.Resize(100, 30)
	tut.Reset()

	wait := func(kind string) {
		t.Helper()
		require.True(t, tut.WaitActive(kind, time.Second),
			"expected a %s step", kind)
	}
	dismiss := func(ev term.Event) {
		t.Helper()
		_, _ = tut.Handle(ev)
	}
	dismissPromptStep := func() {
		t.Helper()
		wait("floating_window")
		dismiss(term.Event{Type: term.EventKey, Ch: ':'})
	}
	observe := func(command string, args ...string) {
		t.Helper()
		wait("wait_command")
		tut.ObserveCommand(command, command, args, nil)
	}

	dismissPromptStep()
	observe("workspaceopen", "/tmp/tutorial-workspace")
	dismissPromptStep()
	observe("edit", "README.md")

	dismissPromptStep()
	dismissPromptStep()
	observe("windownew")
	dismissPromptStep()
	observe("terminalneworsplit")
	dismissPromptStep()
	observe("windowdefaultsplit", "h")
	dismissPromptStep()
	observe("terminalneworsplit")

	wait("floating_window")
	dismiss(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	dismissPromptStep()
	observe("windowfocus", "up")
	// A wrong direction keeps the step armed.
	wait("wait_command")
	tut.ObserveCommand("windowfocus", "windowfocus", []string{"right"}, nil)
	observe("windowfocus", "left")

	dismissPromptStep()
	observe("windowmove", "right")
	observe("windowmove", "left")

	dismissPromptStep()
	observe("windowresize", "increase", "height")
	wait("wait_command")
	tut.ObserveCommand("windowresize", "windowresize",
		[]string{"increase", "width"}, nil)
	observe("windowresize", "decrease", "width")

	dismissPromptStep()
	observe("windowtogglemaximize")
	dismissPromptStep()
	observe("windowfocus", "right")
	observe("windowclose")
	dismissPromptStep()
	observe("fexplorer")
	wait("wait_event")
	tut.ObserveEvent("open", "file:///README.md")
	dismissPromptStep()
	observe("fexplorer")
	dismissPromptStep()
	observe("tabnext")
	observe("tabprevious")

	dismissPromptStep()
	observe("tabmove", "left")
	observe("tabmove", "right")

	dismissPromptStep()
	observe("tabclose")
	dismissPromptStep()
	observe("!", "git", "log")
	dismissPromptStep()
	observe("windowclose")

	dismissPromptStep()
	observe("guitheme", "mullen")

	dismissPromptStep()
	observe("config")

	wait("wait_event")
	assert.False(t, tut.ObserveEvent("flush", "file:///README.md"),
		"saving an unrelated buffer must not advance the config step")
	require.True(t, tut.WaitActive("wait_event", time.Second),
		"the config step stays armed after an unrelated flush")
	tut.ObserveEvent("flush", "file:///home/u/.rune/config.yaml")

	dismissPromptStep()
	dismissPromptStep()
	observe("cheatsheet")

	assert.Contains(t, notis.successes(), "You resized a window.")
	assert.Contains(t, notis.successes(), "You reordered the tabs.")
	assert.Contains(t, notis.successes(), "Config saved.")
	assert.Contains(t, notis.successes(), "That is your cheatsheet.")
}

func TestBasicsTutorialEmacsWindowFlow(t *testing.T) {
	t.Parallel()

	notis := &capturingNotis{}
	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil, nil, notis, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'},
		"emacs", "", nil,
		nil, nil, nil,
	)
	require.NoError(t, err)
	tut.Resize(100, 30)
	tut.Reset()

	wait := func(kind string) {
		t.Helper()
		require.True(t, tut.WaitActive(kind, time.Second),
			"expected a %s step", kind)
	}
	dismissPromptStep := func() {
		t.Helper()
		wait("floating_window")
		_, _ = tut.Handle(term.Event{Type: term.EventKey, Ch: ':'})
	}
	observe := func(command string, args ...string) {
		t.Helper()
		wait("wait_command")
		tut.ObserveCommand(command, command, args, nil)
	}

	dismissPromptStep()
	observe("workspaceopen", "/tmp/tutorial-workspace")
	dismissPromptStep()
	observe("edit", "README.md")

	dismissPromptStep()
	dismissPromptStep()
	wait("wait_command")
	tut.ObserveCommand("windownew", "windownew", []string{"down"}, nil)
	wait("wait_command")
	tut.ObserveCommand("windownew", "windownew", []string{"right"}, nil)

	dismissPromptStep()
	observe("terminalneworsplit")
	dismissPromptStep()
	observe("windowdefaultsplit", "h")
	dismissPromptStep()
	observe("terminalneworsplit")

	wait("floating_window")
	_, _ = tut.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	dismissPromptStep()
	observe("windowfocus", "up")
	observe("windowfocus", "left")
	dismissPromptStep()
	observe("windowmove", "right")
	observe("windowmove", "left")
	dismissPromptStep()
	observe("windowresize", "increase", "width")
	observe("windowresize", "decrease", "width")
	dismissPromptStep()
	observe("windowtogglemaximize")
	dismissPromptStep()
	observe("windowfocus", "right")
	observe("windowclose")
	dismissPromptStep()
	observe("windowcloseall")

	assert.Contains(t, notis.successes(), "Splits now land below.")
	assert.Contains(t, notis.successes(), "You cleaned up the window layout.")
}

func TestAgentTutorialInstallAndHelpFlow(t *testing.T) {
	t.Parallel()

	notis := &capturingNotis{}
	overlay := idetutorial.NewOverlayBrowser(
		browser.NewComponent(idetutorial.DefaultOverlayBrowserConfig()))
	tut, err := starlarktutorial.New(
		"agent", agentTutorial,
		overlay, nil, notis, nil,
		term.Attributes{},
		nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", "", nil,
		nil,
		nil, nil,
	)
	require.NoError(t, err)
	tut.Resize(80, 24)
	tut.Reset()

	dismiss := func(key term.Event) {
		t.Helper()
		_, _ = tut.Handle(key)
	}
	wait := func(kind string) {
		t.Helper()
		require.True(t, tut.WaitActive(kind, time.Second),
			"expected a %s step", kind)
	}

	// Layout cleanup comes first so the console does not open inside a
	// leftover floating window.
	wait("floating_window")
	dismiss(term.Event{Type: term.EventKey, Ch: ':'})
	wait("wait_command")
	tut.ObserveCommand("windowcloseall", "windowcloseall", nil, nil)

	// Command prompt instructions for opening the console.
	wait("floating_window")
	dismiss(term.Event{Type: term.EventKey, Ch: ':'})
	wait("wait_command")
	tut.ObserveCommand("console", "console", nil, nil)

	// Installation requires the console's shell observation. The hint is
	// rendered by wait_shell itself so console keystrokes pass through
	// instead of being swallowed by a blocking window.
	wait("wait_shell")
	tut.ObserveCommand("console", "console", []string{"pkg", "install", "rune-agent"}, nil)

	// Skipping provider configuration still reaches the standalone help step.
	wait("choice")
	dismiss(term.Event{Type: term.EventKey, Key: term.KeyEsc})
	wait("floating_window")
	dismiss(term.Event{Type: term.EventKey, Ch: ':'})
	wait("wait_command")
	tut.ObserveCommand("help", "help", nil, nil)
	require.True(t, tut.WaitFinished(time.Second))
	assert.Equal(t, []string{
		"Layout cleared.",
		"Rune Agent installed.",
		"That is the help command.",
	}, notis.successes())
}

func TestTutorialPackageInstallOwnership(t *testing.T) {
	t.Parallel()
	// The basics tutorial installs nothing and never sends the user to
	// the console; the agent tutorial owns both the console introduction
	// and the only package install of the playlist.
	assert.NotContains(t, basicsTutorial, "pkg install")
	assert.NotContains(t, basicsTutorial, `command  = "console"`)
	assert.Contains(t, agentTutorial, "Rune console")
	assert.Contains(t, agentTutorial, "pkg install rune-agent")
}

// TestNavigationTutorialFlow drives the embedded navigation tutorial
// through its full step sequence and pins the ordering the tutorial
// teaches: definition-under-cursor comes before definition-by-name, the
// cursor history is walked back then forward as two distinct steps, and
// the `lsp` verbs step comes last.
func TestNavigationTutorialFlow(t *testing.T) {
	t.Parallel()

	alwaysTrue := func() bool { return true }
	// Give searchfile/searchtext bound keys so their copy renders the
	// keypress wording. Whether the finder steps run at all is decided
	// by command_exists, which defaults to true with no manual lookup
	// wired, so this flow covers the already-installed path.
	keyFor := func(cmd string, _ []string) string {
		switch cmd {
		case "searchfile":
			return "<c-p>"
		case "searchtext":
			return "<c-f>"
		}
		return ""
	}
	notis := &capturingNotis{}
	tut, err := starlarktutorial.New(
		"navigation", navigationTutorial,
		nil, nil, notis, nil,
		term.Attributes{},
		nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", "", keyFor,
		nil,
		alwaysTrue, // workspace_open()
		alwaysTrue, // is_lsp_server_running()
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	tut.Resize(80, 24)
	tut.Reset()

	dismiss := func() {
		t.Helper()
		ks, err := term.ParseKeys(":")
		require.NoError(t, err)
		require.Len(t, ks, 1)
		_, _ = tut.Handle(term.Event{
			Type: term.EventKey, Key: ks[0].Key, Mod: ks[0].Mod, Ch: ks[0].Ch,
		})
	}
	waitFW := func() {
		t.Helper()
		require.True(t, tut.WaitActive("floating_window", 2*time.Second),
			"expected a floating_window step")
	}
	waitCmd := func() {
		t.Helper()
		require.True(t, tut.WaitActive("wait_command", 2*time.Second),
			"expected a wait_command step")
	}
	waitEvent := func() {
		t.Helper()
		require.True(t, tut.WaitActive("wait_event", 2*time.Second),
			"expected a wait_event step")
	}

	// Layout cleanup comes first so the tutorial starts on a clean screen.
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("windowcloseall", "windowcloseall", nil, nil)

	// Intro window.
	waitFW()
	dismiss()

	// searchfile: window -> command -> file-open event.
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("searchfile", "searchfile", nil, nil)
	waitEvent()
	assert.Contains(t, tut.ActiveText(), "do not have to be contiguous")
	assert.Contains(t, tut.ActiveText(), "<up>")
	assert.Contains(t, tut.ActiveText(), "<down>")
	tut.ObserveEvent("open", "file:///workspace/a.go")

	// searchtext: window -> command -> file-open event.
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("searchtext", "searchtext", nil, nil)
	waitEvent()
	assert.Contains(t, tut.ActiveText(),
		"Type a word you want to search for, or just a few characters from that")
	assert.Contains(t, tut.ActiveText(), "do not have to be contiguous")
	assert.Contains(t, tut.ActiveText(), "<up>")
	assert.Contains(t, tut.ActiveText(), "<down>")
	assert.NotContains(t, tut.ActiveText(), "<ctrl-i>")
	assert.NotContains(t, tut.ActiveText(), "<ctrl-k>")
	tut.ObserveEvent("open", "file:///workspace/b.go")

	// jumptoast: window -> command (fuzzy-jump to a function in the file).
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("jumptoast", "jumptoast",
		[]string{"locals.scm", "local.definition.method|local.definition.function", "run"}, nil)

	// lsp intro window.
	waitFW()
	dismiss()

	// Definition under the cursor comes FIRST (the basic verb)...
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("lsp", "lsp", []string{"definition"}, nil)

	// ...then definition by name (fuzzy symbol search).
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("lsp", "lsp", []string{"definition", "SomeSymbol"}, nil)

	// Cursor history: jump back...
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("cursorhistory", "cursorhistory", []string{"prev"}, nil)

	// ...then jump forward again as a distinct step.
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("cursorhistory", "cursorhistory", []string{"next"}, nil)

	// lsp references (the other verbs work the same way).
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("lsp", "lsp", []string{"references"}, nil)

	// Wrap-up window -> dismiss to finish.
	waitFW()
	dismiss()
	require.True(t, tut.WaitFinished(2*time.Second),
		"tutorial must finish after the wrap-up window")

	// Pin the teaching order via the success notifications each step
	// emits. Definition-under-cursor precedes definition-by-name and the
	// cursor history is walked back then forward.
	assert.Equal(t, []string{
		"Layout cleared.",
		"You found a file by name.",
		"You found text across the workspace.",
		"You jumped to a function in the current file.",
		"You jumped to the definition under your cursor.",
		"You jumped to a definition by name.",
		"You jumped back.",
		"You walked the cursor history back and forth.",
		"You asked the language server about a symbol.",
	}, notis.successes())
}

// TestNavigationTutorialPrefillKeysMatchPresets pins the chords the
// tutorial hardcodes against the presets that bind them. They cannot
// come from key_for because they are prompt-prefill macros, and the
// jumptoast copy went stale once already when the emacs preset moved
// that binding off <ctrl-x>j.
func TestNavigationTutorialPrefillKeysMatchPresets(t *testing.T) {
	t.Parallel()

	const (
		jumpPrefill = `": "echo {prompt}jumptoast<space>locals.scm<space>` +
			`local.definition.method|local.definition.function<space>"`
		defPrefill = `": "echo {prompt}lsp<space>definition<space>"`
	)
	tests := []struct {
		mode    string
		jumpKey string
		defKey  string
		preset  string
	}{
		{
			mode:    "emacs",
			jumpKey: "<meta-j>",
			defKey:  "<ctrl-alt-.>",
			preset:  presetEmacsYAML,
		},
		{
			mode:    "modal",
			jumpKey: "<alt-f>",
			defKey:  "<alt-shift-d>",
			preset:  presetModalYAML,
		},
		{
			mode:    "standard",
			jumpKey: "<alt-f>",
			defKey:  "<alt-shift-d>",
			preset:  presetStandardYAML,
		},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			t.Parallel()
			assert.Contains(t, tt.preset, `"`+tt.jumpKey+jumpPrefill,
				"the preset must bind the chord the tutorial teaches")
			assert.Contains(t, tt.preset, `"`+tt.defKey+defPrefill,
				"the preset must bind the chord the tutorial teaches")

			src := `
def run():
    floating_window(text = jump_symbol_md + lsp_definition_name_md)
tutorial(entry=run)
`
			modeSrc := strings.Replace(navigationTutorial,
				`tutorial(id = "navigation", title = "Navigate code", version = "17", entry = run)`,
				"", 1) + src
			tut, err := starlarktutorial.New(
				"navigation-prefill-keys", modeSrc,
				nil, nil, nil, nil,
				term.Attributes{}, nil, nil,
				term.KeyComb{Ch: ':'}, tt.mode, "",
				nil, nil, nil, nil,
			)
			require.NoError(t, err)
			tut.Resize(80, 24)
			tut.Reset()
			require.True(t, tut.WaitActive("floating_window", time.Second))
			assert.Contains(t, tut.ActiveText(), tt.jumpKey)
			assert.Contains(t, tut.ActiveText(), tt.defKey)
		})
	}
}

func TestNavigationTutorialFinderPickerKeysByMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		mode      string
		up        string
		down      string
		forbidden []string
	}{
		{mode: "modal", up: "<ctrl-k>", down: "<ctrl-j>"},
		{
			mode:      "standard",
			up:        "<up>",
			down:      "<down>",
			forbidden: []string{"<ctrl-i>", "<ctrl-k>"},
		},
		{mode: "emacs", up: "<ctrl-p>", down: "<ctrl-n>"},
	}
	pickers := []string{"searchfile_picker_md", "searchtext_picker_md"}
	for _, picker := range pickers {
		for _, tt := range tests {
			t.Run(picker+"/"+tt.mode, func(t *testing.T) {
				t.Parallel()
				src := `
def run():
    wait_event(event="open", text=` + picker + `)
tutorial(entry=run)
`
				modeSrc := strings.Replace(navigationTutorial,
					`tutorial(id = "navigation", title = "Navigate code", version = "17", entry = run)`,
					"", 1) + src
				tut, err := starlarktutorial.New(
					"navigation-picker-keys", modeSrc,
					nil, nil, nil, nil,
					term.Attributes{}, nil, nil,
					term.KeyComb{Ch: ':'}, tt.mode, "",
					nil, nil, nil, nil,
				)
				require.NoError(t, err)
				tut.Resize(80, 24)
				tut.Reset()
				require.True(t, tut.WaitActive("wait_event", time.Second))

				text := tut.ActiveText()
				assert.Contains(t, text, tt.up)
				assert.Contains(t, text, tt.down)
				assert.Contains(t, text, "<up>")
				assert.Contains(t, text, "<down>")
				for _, key := range tt.forbidden {
					assert.NotContains(t, text, key)
				}
			})
		}
	}
}

// TestNavigationTutorialInstallsFuzzySearchFirst asserts that when the
// fuzzy-search extension is missing, the tutorial walks the user
// through installing it from the console before reaching the finder
// steps. Without this, the first `searchfile` would trigger Rune's own
// install prompt while the tutorial assumed a finder was already open.
func TestNavigationTutorialInstallsFuzzySearchFirst(t *testing.T) {
	t.Parallel()

	alwaysTrue := func() bool { return true }
	lookup := func(name string) (command.Manual, bool) {
		return command.Manual{}, false
	}
	notis := &capturingNotis{}
	overlay := idetutorial.NewOverlayBrowser(
		browser.NewComponent(idetutorial.DefaultOverlayBrowserConfig()))
	tut, err := starlarktutorial.New(
		"navigation", navigationTutorial,
		overlay, nil, notis, nil,
		term.Attributes{},
		nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", "", nil, lookup,
		alwaysTrue, // workspace_open()
		alwaysTrue, // is_lsp_server_running()
	)
	require.NoError(t, err)
	tut.Resize(80, 24)
	tut.Reset()

	wait := func(kind string) {
		t.Helper()
		require.True(t, tut.WaitActive(kind, 2*time.Second),
			"expected a %s step", kind)
	}
	dismiss := func() {
		t.Helper()
		_, _ = tut.Handle(term.Event{Type: term.EventKey, Ch: ':'})
	}

	wait("floating_window") // clear the layout
	dismiss()
	wait("wait_command")
	tut.ObserveCommand("windowcloseall", "windowcloseall", nil, nil)

	wait("floating_window") // intro
	dismiss()

	wait("floating_window") // open the console
	dismiss()
	wait("wait_command")
	tut.ObserveCommand("console", "console", nil, nil)

	wait("wait_shell") // pkg install fuzzy-search
	// The console is focused here, so the step must let the user type
	// the install command instead of swallowing its keys.
	handled, _ := tut.Handle(term.Event{Type: term.EventKey, Ch: 'p'})
	assert.False(t, handled, "wait_shell must not swallow console input")
	assert.Contains(t, tut.ActiveText(), "Rune's console")
	tut.ObserveCommand("console", "console",
		[]string{"pkg", "install", "fuzzy-search"}, nil)

	// Command registration is asynchronous after the install completes,
	// so the live lookup can still be stale here. A successful install
	// must proceed to the finder lesson rather than claim it failed.
	wait("floating_window") // searchfile
	dismiss()
	wait("wait_command")
	tut.ObserveCommand("searchfile", "searchfile", nil, nil)
	wait("wait_event")
	tut.ObserveEvent("open", "file:///workspace/a.go")

	assert.Contains(t, notis.successes(), "Fuzzy search installed.")
	assert.Contains(t, notis.successes(), "You found a file by name.")
}

// TestNavigationTutorialCursorKeysMoveCursor is a regression test for a
// bug where the "put your cursor on a symbol, then press <binding>"
// teaching windows swallowed cursor-movement keys and pulsed their hint
// instead of letting the user reposition the cursor. The definition and
// references windows must let cursor-movement keys fall through to the
// IDE root (handled=false) without dismissing the window.
func TestNavigationTutorialCursorKeysMoveCursor(t *testing.T) {
	t.Parallel()

	cases := []struct {
		mode      string
		moveKeys  []string
		swallowed string
	}{
		{mode: "standard", moveKeys: []string{"<up>", "<down>", "<left>", "<right>"}, swallowed: "z"},
		{mode: "modal", moveKeys: []string{"h", "j", "k", "l", "<left>"}, swallowed: "z"},
		{mode: "emacs", moveKeys: []string{"<ctrl-p>", "<ctrl-n>", "<up>"}, swallowed: "z"},
	}
	for _, tc := range cases {
		t.Run(tc.mode, func(t *testing.T) {
			t.Parallel()
			tut := advanceToDefinitionWindow(t, tc.mode)

			for _, spec := range tc.moveKeys {
				ks, err := term.ParseKeys(spec)
				require.NoError(t, err)
				require.Len(t, ks, 1)
				_, handled := tut.Handle(term.Event{
					Type: term.EventKey, Key: ks[0].Key, Mod: ks[0].Mod, Ch: ks[0].Ch,
				})
				assert.False(t, handled,
					"cursor-movement key %q must fall through to the editor so "+
						"the user can position the cursor", spec)
				require.True(t, tut.WaitActive("floating_window", time.Second),
					"a movement key must not dismiss the teaching window (%q)", spec)
			}

			// A key that is neither a movement nor a dismiss key is still
			// swallowed so it cannot leak to the IDE root under the overlay.
			ks, err := term.ParseKeys(tc.swallowed)
			require.NoError(t, err)
			_, handled := tut.Handle(term.Event{
				Type: term.EventKey, Key: ks[0].Key, Mod: ks[0].Mod, Ch: ks[0].Ch,
			})
			assert.True(t, handled,
				"a non-movement stray key (%q) must remain swallowed", tc.swallowed)
		})
	}
}

// TestNavigationTutorialDefinitionByNameKeyFallsThrough is a regression
// test for the by-name definition step: its CTA tells modal/standard
// users to press <alt-shift-d> and Emacs users to press <ctrl-alt-.>
// (which opens the command prompt prefilled
// with `lsp definition `). The teaching window must dismiss on that key
// and let it reach the IDE root, rather than swallowing it and pulsing
// the hint.
func TestNavigationTutorialDefinitionByNameKeyFallsThrough(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mode string
		key  string
	}{
		{mode: "standard", key: "<alt-shift-d>"},
		{mode: "modal", key: "<alt-shift-d>"},
		{mode: "emacs", key: "<ctrl-alt-.>"},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			t.Parallel()
			tut := advanceToDefinitionWindow(t, tc.mode)

			// Advance past "Go to definition" to "Find a definition by
			// name": dismiss the cursor window, resolve its wait_command,
			// then wait for the by-name window.
			ks, err := term.ParseKeys(":")
			require.NoError(t, err)
			_, _ = tut.Handle(term.Event{
				Type: term.EventKey, Key: ks[0].Key, Mod: ks[0].Mod, Ch: ks[0].Ch,
			})
			require.True(t, tut.WaitActive("wait_command", 2*time.Second))
			tut.ObserveCommand("lsp", "lsp", []string{"definition"}, nil)
			require.True(t, tut.WaitActive("floating_window", 2*time.Second),
				"the by-name definition window should be active")

			// The prefill key must dismiss the window AND fall through so the
			// IDE root opens the prefilled command prompt.
			asd, err := term.ParseKeys(tc.key)
			require.NoError(t, err)
			require.Len(t, asd, 1)
			_, handled := tut.Handle(term.Event{
				Type: term.EventKey, Key: asd[0].Key, Mod: asd[0].Mod, Ch: asd[0].Ch,
			})
			assert.False(t, handled,
				tc.key+" must fall through to the IDE root to open the "+
					"prefilled command prompt")
			require.True(t, tut.WaitActive("wait_command", 2*time.Second),
				"pressing "+tc.key+" must dismiss the window and arm the "+
					"wait_command step")
		})
	}
}

// advanceToDefinitionWindow builds the navigation tutorial in the given
// editor mode and drives it up to (and stopping at) the "Go to
// definition" teaching window — the first step that asks the user to
// move the cursor before pressing a binding.
func advanceToDefinitionWindow(t *testing.T, mode string) *starlarktutorial.Tutorial {
	t.Helper()
	alwaysTrue := func() bool { return true }
	keyFor := func(cmd string, _ []string) string {
		switch cmd {
		case "searchfile":
			return "<c-p>"
		case "searchtext":
			return "<c-f>"
		}
		return ""
	}
	tut, err := starlarktutorial.New(
		"navigation", navigationTutorial,
		nil, nil, nil, nil,
		term.Attributes{},
		nil, nil,
		term.KeyComb{Ch: ':'},
		mode, "", keyFor,
		nil,
		alwaysTrue, alwaysTrue,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	tut.Resize(80, 24)
	tut.Reset()

	dismiss := func() {
		t.Helper()
		ks, err := term.ParseKeys(":")
		require.NoError(t, err)
		_, _ = tut.Handle(term.Event{
			Type: term.EventKey, Key: ks[0].Key, Mod: ks[0].Mod, Ch: ks[0].Ch,
		})
	}
	fw := func() {
		t.Helper()
		require.True(t, tut.WaitActive("floating_window", 2*time.Second))
	}
	cmd := func() {
		t.Helper()
		require.True(t, tut.WaitActive("wait_command", 2*time.Second))
	}
	evt := func() {
		t.Helper()
		require.True(t, tut.WaitActive("wait_event", 2*time.Second))
	}

	fw() // clear the layout
	dismiss()
	cmd()
	tut.ObserveCommand("windowcloseall", "windowcloseall", nil, nil)
	fw() // intro
	dismiss()
	fw() // searchfile
	dismiss()
	cmd()
	tut.ObserveCommand("searchfile", "searchfile", nil, nil)
	evt()
	tut.ObserveEvent("open", "file:///workspace/a.go")
	fw() // searchtext
	dismiss()
	cmd()
	tut.ObserveCommand("searchtext", "searchtext", nil, nil)
	evt()
	tut.ObserveEvent("open", "file:///workspace/b.go")
	fw() // jump to a function in this file
	dismiss()
	cmd()
	tut.ObserveCommand("jumptoast", "jumptoast",
		[]string{"locals.scm", "local.definition.method|local.definition.function", "run"}, nil)
	fw() // lsp intro
	dismiss()
	fw() // "Go to definition" — stop here.
	return tut
}

// capturingNotis records success-level notification messages in order so
// tests can assert the sequence of tutorial steps that completed.
type capturingNotis struct {
	mu   sync.Mutex
	msgs []string
}

func (n *capturingNotis) Notify(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	if level == browserapi.LevelSuccess {
		n.mu.Lock()
		n.msgs = append(n.msgs, fmt.Sprintf(msg, args...))
		n.mu.Unlock()
	}
	return "", nil
}

func (n *capturingNotis) NotifyOnce(
	level browserapi.NotificationLevel, msg string, args ...any,
) (string, error) {
	return n.Notify(level, msg, args...)
}

func (n *capturingNotis) UpdateNotificationProgress(
	_, _ string, _, _ int64,
) error {
	return nil
}

func (n *capturingNotis) successes() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	out := make([]string, len(n.msgs))
	copy(out, n.msgs)
	return out
}

// TestNavigationTutorialParses asserts that the embedded navigation.star
// tutorial parses through starlarktutorial.New, registers a callable
// entry, and reports the expected id/title/version.
func TestNavigationTutorialParses(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, navigationTutorial,
		"navigationTutorial embed must not be empty")
	tut, err := starlarktutorial.New(
		"navigation", navigationTutorial,
		nil,
		nil,
		nil,
		nil,
		term.Attributes{},
		nil,
		nil,
		term.KeyComb{Ch: ':'},
		"standard", "",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)

	assert.Equal(t, "navigation", tut.ID())
	assert.Equal(t, "Navigate code", tut.Title())
	assert.Equal(t, "17", tut.Version())
}

func TestAgentTutorialParses(t *testing.T) {
	t.Parallel()

	require.NotEmpty(t, agentTutorial, "agentTutorial embed must not be empty")
	tut, err := starlarktutorial.New(
		"agent", agentTutorial,
		nil, nil, nil, nil,
		term.Attributes{},
		nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", "", nil,
		nil,
		nil, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "agent", tut.ID())
	assert.Equal(t, "Rune Agent", tut.Title())
	assert.Equal(t, "4", tut.Version())
}

func TestEmbeddedTutorialPlaylist(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []ide.TutorialPlaylistItem{
		{
			Name:        "basics",
			Description: "Learn the essential Rune workspace and window management commands and key bindings.",
		},
		{
			Name:        "navigation",
			Description: "Learn about structural navigation and how to exploit Rune's code intelligence tools.",
		},
		{
			Name:        "agent",
			Description: "Install Rune Agent, connect a model provider, start a conversation, and get help.",
		},
	}, embeddedTutorialPlaylist)
	assert.NotEmpty(t, embeddedTutorialOptions())
}

// TestNavigationTutorialParsesModalMode asserts the embedded navigation
// tutorial also parses under modal editor mode.
func TestNavigationTutorialParsesModalMode(t *testing.T) {
	t.Parallel()

	tut, err := starlarktutorial.New(
		"navigation", navigationTutorial,
		nil,
		nil,
		nil,
		nil,
		term.Attributes{},
		nil,
		nil,
		term.KeyComb{Ch: ':'},
		"modal", "",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "17", tut.Version())
}

// TestNavigationTutorialParsesEmacsMode asserts the embedded navigation
// tutorial also parses under the emacs editor mode.
func TestNavigationTutorialParsesEmacsMode(t *testing.T) {
	t.Parallel()

	tut, err := starlarktutorial.New(
		"navigation", navigationTutorial,
		nil,
		nil,
		nil,
		nil,
		term.Attributes{},
		nil,
		nil,
		term.KeyComb{Ch: ':'},
		"emacs", "",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "17", tut.Version())
}

// TestBasicsTutorialParsesEmacsMode asserts the embedded basics tutorial
// also parses under the emacs editor mode, exercising the emacs branch of
// the direction-phrasing logic (IJKL layout, GNU-Emacs buffer motion keys,
// arrow-key completer).
func TestBasicsTutorialParsesEmacsMode(t *testing.T) {
	t.Parallel()

	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil,
		nil,
		nil,
		nil,
		term.Attributes{},
		nil,
		nil,
		term.KeyComb{Ch: ':'},
		"emacs", "",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "58", tut.Version())
}
