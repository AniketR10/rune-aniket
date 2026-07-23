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
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"

	"unstable.build/go-tui/browser"
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
		component.FrameCharSet{},
		browser.PromptConfig{},
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
	assert.Equal(t, "33", tut.Version())
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
		component.FrameCharSet{},
		browser.PromptConfig{},
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
	assert.Equal(t, "33", tut.Version())
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
		term.Attributes{}, component.FrameCharSet{}, browser.PromptConfig{},
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
// users to press <alt-shift-d> (which opens the command prompt prefilled
// with `lsp definition `). The teaching window must dismiss on that key
// and let it reach the IDE root, rather than swallowing it and pulsing
// the hint. Emacs has no such prefill binding, so it is not exercised.
func TestNavigationTutorialDefinitionByNameKeyFallsThrough(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"standard", "modal"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			tut := advanceToDefinitionWindow(t, mode)

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

			// <alt-shift-d> must dismiss the window AND fall through so the
			// IDE root opens the prefilled command prompt.
			asd, err := term.ParseKeys("<alt-shift-d>")
			require.NoError(t, err)
			require.Len(t, asd, 1)
			_, handled := tut.Handle(term.Event{
				Type: term.EventKey, Key: asd[0].Key, Mod: asd[0].Mod, Ch: asd[0].Ch,
			})
			assert.False(t, handled,
				"<alt-shift-d> must fall through to the IDE root to open the "+
					"prefilled command prompt")
			require.True(t, tut.WaitActive("wait_command", 2*time.Second),
				"pressing <alt-shift-d> must dismiss the window and arm the "+
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
		term.Attributes{}, component.FrameCharSet{}, browser.PromptConfig{},
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

// TestEmbeddedTutorialOptionsRegistersBasics asserts that the embedded
// tutorial option list registers at least one tutorial.
func TestEmbeddedTutorialOptionsRegistersBasics(t *testing.T) {
	t.Parallel()
	opts := embeddedTutorialOptions()
	require.NotEmpty(t, opts,
		"embeddedTutorialOptions must register at least basics")
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
		component.FrameCharSet{},
		browser.PromptConfig{},
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
	assert.Equal(t, "8", tut.Version())
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
		component.FrameCharSet{},
		browser.PromptConfig{},
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
	assert.Equal(t, "8", tut.Version())
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
		component.FrameCharSet{},
		browser.PromptConfig{},
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
	assert.Equal(t, "8", tut.Version())
}

// TestBasicsTutorialParsesEmacsMode asserts the embedded basics tutorial
// also parses under the emacs editor mode, exercising the emacs branch of
// the direction-phrasing logic (home-row <meta> focus, GNU-Emacs buffer
// motion keys, arrow-key completer).
func TestBasicsTutorialParsesEmacsMode(t *testing.T) {
	t.Parallel()

	tut, err := starlarktutorial.New(
		"basics", basicsTutorial,
		nil,
		nil,
		nil,
		nil,
		term.Attributes{},
		component.FrameCharSet{},
		browser.PromptConfig{},
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
	assert.Equal(t, "33", tut.Version())
}
