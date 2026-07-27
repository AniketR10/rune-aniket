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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/ide/idetutorial"
	"unstable.build/go-tui/ide/idetutorial/starlarktutorial"
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
		"standard",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)

	assert.Equal(t, "basics", tut.ID())
	assert.Equal(t, "Rune basics", tut.Title())
	assert.Equal(t, "41", tut.Version())
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
		"modal",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "41", tut.Version())
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
				"<ctrl-x>0 closes a window, <ctrl-x>1 closes the others",
				"<ctrl-x>2 / <ctrl-x>3 split below or right",
				"<ctrl-x>9 toggles maximization",
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
					"windowfocus left":     "<alt-j>",
					"windowmove left":      "<alt-shift-j>",
					"windownew down":       "<ctrl-x>2",
					"windownew right":      "<ctrl-x>3",
					"tabprevious":          "<alt-[>",
					"tabnext":              "<alt-]>",
					"tabmove left":         "<alt-shift-[>",
					"tabmove right":        "<alt-shift-]>",
					"windownew":            "<alt-n>",
					"terminalneworsplit":   "<alt-enter>",
					"windowclose":          "<alt-q>",
					"windowcloseall":       "<ctrl-x>1",
					"windowtogglemaximize": "<ctrl-x>9",
					"tabnew":               "<alt-t>",
					"tabclose":             "<alt-w>",
				}
				if tt.mode == "emacs" {
					keys["windowclose"] = "<ctrl-x>0"
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
				tt.mode, keyFor,
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
		"standard", keyFor,
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
		"standard", keyFor,
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
		"emacs", keyFor,
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

func TestBasicsTutorialDirectionalCommandFlow(t *testing.T) {
	t.Parallel()

	notis := &capturingNotis{}
	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil, nil, notis, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'},
		"standard", nil,
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
	wait("wait_command")
	tut.ObserveCommand("windowfocus", "windowfocus", []string{"left"}, nil)
	observe("windowfocus", "right")

	dismissPromptStep()
	observe("windowmove", "left")
	observe("windowmove", "right")

	dismissPromptStep()
	observe("windowresize", "increase", "height")
	wait("wait_command")
	tut.ObserveCommand("windowresize", "windowresize",
		[]string{"increase", "width"}, nil)
	observe("windowresize", "decrease", "width")

	dismissPromptStep()
	observe("windowtogglemaximize")
	dismissPromptStep()
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

	wait("floating_window")
	assert.Contains(t, notis.successes(), "You resized a window.")
	assert.Contains(t, notis.successes(), "You reordered the tabs.")
}

func TestBasicsTutorialEmacsWindowFlow(t *testing.T) {
	t.Parallel()

	notis := &capturingNotis{}
	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil, nil, notis, nil,
		term.Attributes{}, nil, nil,
		term.KeyComb{Ch: ':'},
		"emacs", nil,
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
	tut.ObserveCommand("windownew", "windownew", []string{"right"}, nil)
	wait("wait_command")
	tut.ObserveCommand("windownew", "windownew", []string{"down"}, nil)

	dismissPromptStep()
	observe("windownew", "right")
	dismissPromptStep()
	observe("terminalneworsplit")
	dismissPromptStep()
	observe("windowdefaultsplit", "h")
	dismissPromptStep()
	observe("terminalneworsplit")

	wait("floating_window")
	_, _ = tut.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	dismissPromptStep()
	observe("windowfocus", "left")
	observe("windowfocus", "right")
	dismissPromptStep()
	observe("windowmove", "left")
	observe("windowmove", "right")
	dismissPromptStep()
	observe("windowresize", "increase", "width")
	observe("windowresize", "decrease", "width")
	dismissPromptStep()
	observe("windowtogglemaximize")
	dismissPromptStep()
	observe("windowclose")
	dismissPromptStep()
	observe("windowcloseall")

	assert.Contains(t, notis.successes(), "You used the Emacs split family.")
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
		"standard", nil,
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

	// Command prompt instructions for opening the console.
	wait("floating_window")
	dismiss(term.Event{Type: term.EventKey, Ch: ':'})
	wait("wait_command")
	tut.ObserveCommand("console", "console", nil, nil)

	// Installation requires the console's shell observation.
	wait("floating_window")
	dismiss(term.Event{Type: term.EventKey, Key: term.KeyEnter})
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
	assert.Equal(t, []string{"Rune Agent installed.", "That is the help command."},
		notis.successes())
}

func TestTutorialPackageInstallOwnership(t *testing.T) {
	t.Parallel()
	assert.Contains(t, basicsTutorial, "pkg install fuzzy-search")
	assert.NotContains(t, basicsTutorial, "pkg install rune-agent")
	assert.NotContains(t, agentTutorial, "console_intro_md")
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
	// searchfile/searchtext only teach their full command+event flow
	// when the fuzzy-search extension is "installed" (their key
	// resolves); give them a bound key so the flow runs end to end.
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
		"standard", keyFor,
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

	// Intro window.
	waitFW()
	dismiss()

	// searchfile: window -> command -> file-open event.
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("searchfile", "searchfile", nil, nil)
	waitEvent()
	tut.ObserveEvent("open", "file:///workspace/a.go")

	// searchtext: window -> command -> file-open event.
	waitFW()
	dismiss()
	waitCmd()
	tut.ObserveCommand("searchtext", "searchtext", nil, nil)
	waitEvent()
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
		mode, keyFor,
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
		"standard",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)

	assert.Equal(t, "navigation", tut.ID())
	assert.Equal(t, "Navigate code", tut.Title())
	assert.Equal(t, "9", tut.Version())
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
		"standard", nil,
		nil,
		nil, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "agent", tut.ID())
	assert.Equal(t, "Rune Agent", tut.Title())
	assert.Equal(t, "1", tut.Version())
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
		"modal",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "9", tut.Version())
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
		"emacs",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "9", tut.Version())
}

func TestNavigationTutorialUsesEmacsNavigationPrefills(t *testing.T) {
	t.Parallel()
	require.Contains(t, navigationTutorial, `jump_symbol_key = "<ctrl-x>j"`)
	require.Contains(t, navigationTutorial, `def_by_name_key = "<ctrl-alt-.>"`)
	require.Contains(t, navigationTutorial,
		`"press `+"`"+`" + def_by_name_key + "`+"`"+` to prefill`)
	require.NotContains(t, navigationTutorial,
		`press `+"`"+`<alt-shift-d>`+"`"+` to prefill`)
	require.NotContains(t, navigationTutorial, `jump_symbol_key = "<meta-f>"`)
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
		"emacs",
		nil,
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	require.NotNil(t, tut)
	assert.Equal(t, "41", tut.Version())
}
