// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestOptionToChoiceMapping(t *testing.T) {
	cases := []struct {
		option string
		want   string
	}{
		{optVimYes, editorModal},
		{optVimNo, editorStandard},
		{optEmacs, editorEmacs},
		{"unknown", editorModal}, // default fallback
	}
	for _, tc := range cases {
		t.Run(tc.option, func(t *testing.T) {
			require.Equal(t, tc.want, optionToChoice(tc.option))
		})
	}
}

// TestRenderPreset pins that the vim-mode choice maps to the modal
// preset, the standard-editor choice maps to the standard preset
// (which switches editor.mode to standard), the emacs choice maps to
// the emacs preset, the deprecated modeless alias resolves to the
// standard preset, and that an unknown choice is an error.
func TestRenderPreset(t *testing.T) {
	modal, err := renderPreset(editorModal)
	require.NoError(t, err)
	require.NotContains(t, modal, "mode: standard",
		"vim mode must not switch the editor into standard")

	std, err := renderPreset(editorStandard)
	require.NoError(t, err)
	require.Contains(t, std, "mode: standard",
		"the standard choice must switch the editor into standard")

	deprecated, err := renderPreset(editorModeless)
	require.NoError(t, err)
	require.Equal(t, std, deprecated,
		"the deprecated modeless alias must resolve to the standard preset")

	ema, err := renderPreset(editorEmacs)
	require.NoError(t, err)
	require.Contains(t, ema, "mode: emacs",
		"the emacs choice must switch the editor into emacs")
	require.Contains(t, ema, `"<m-f>": "windowfocus right"`,
		"emacs must use the PNBF direction layer for window focus")
	for _, binding := range []string{
		`"<m-d>": "windownew down"`,
		`"<m-r>": "windownew right"`,
		`"<s-m-w>": windowclose`,
		`"<m-k>": windowcloseall`,
		`"<m-e>": windowtogglemaximize`,
		`"<m-left>": "windowresize decrease width"`,
	} {
		require.Contains(t, ema, binding,
			"emacs layout bindings must remain reachable from terminals")
	}
	for _, key := range []string{`"<c-x>0"`, `"<c-x>1"`, `"<c-x>2"`, `"<c-x>3"`, `"<c-x>9"`} {
		require.NotContains(t, ema, key,
			"terminal-inaccessible C-x lifecycle bindings must not return")
	}
	require.NotContains(t, ema,
		`"<m-f>": "echo {prompt}jumptoast<space>locals.scm<space>`,
		"the displaced function search binding must remain prompt-only")

	_, err = renderPreset("bogus")
	require.Error(t, err)
}

// TestGuardedPromptChainReopensOnUnadvancedClose proves that an Esc
// (or any dismissal that does not call OnSelect) re-opens the same
// prompt, while a normal OnSelect-then-Close sequence does not, and
// neither does a close fired by the pre-config IDE's own teardown.
func TestGuardedPromptChainReopensOnUnadvancedClose(t *testing.T) {
	t.Run("esc reopens", func(t *testing.T) {
		var reopened int
		g := &guardedPromptChain{closing: func() bool { return false }}
		_ = g.onClose(func() { reopened++ })()
		require.Equal(t, 1, reopened, "Esc-equivalent close must re-open")
	})

	t.Run("select does not reopen", func(t *testing.T) {
		var reopened int
		var advanced int
		g := &guardedPromptChain{closing: func() bool { return false }}
		// Simulate the normal selection-then-close ordering: the
		// SDK calls OnSelect first (inside Handle), which marks
		// advanced, then Close → OnClose.
		g.onSelect(func(_ int, _ string) { advanced++ })(0, "")
		_ = g.onClose(func() { reopened++ })()
		require.Equal(t, 1, advanced)
		require.Equal(t, 0, reopened, "selection must not re-open")
	})

	t.Run("closing pre-config IDE does not reopen", func(t *testing.T) {
		var reopened int
		g := &guardedPromptChain{closing: func() bool { return true }}
		_ = g.onClose(func() { reopened++ })()
		require.Equal(t, 0, reopened,
			"teardown-driven close must not re-open the prompt")
	})
}

// TestShouldSwallowBootstrapEvent enumerates the dangerous keys that
// must not reach the pre-config IDE while the bootstrap flow is in
// progress, plus a handful of events that must pass through untouched.
func TestShouldSwallowBootstrapEvent(t *testing.T) {
	cases := []struct {
		name string
		ev   term.Event
		want bool
	}{
		// Dangerous: ':' opens the command prompt (configurable
		// activation key from default rune.star).
		{"colon opens command prompt", keyEv(':', 0), true},

		// Dangerous: default quit / close-window / close-tab
		// bindings from the editor presets.
		{"meta-q quit", keyEv('q', term.ModMeta), true},
		{"meta-w windowclose", keyEv('w', term.ModMeta), true},
		{"alt-w tabclose", keyEv('w', term.ModAlt), true},
		{"ctrl-w tabclose", keyEv('w', term.ModCtrl), true},
		{"meta-shift-w", keyEv('w', term.ModMeta|term.ModShift), true},

		// Pass-through: arrow keys (prompt navigation), mouse,
		// resize, normal text input, Enter (used for one-button
		// welcome screen), Esc (the prompt's own close path, which
		// the guarded close callback re-opens).
		{"arrow left", namedKeyEv(term.KeyArrowLeft), false},
		{"enter", namedKeyEv(term.KeyEnter), false},
		{"esc", namedKeyEv(term.KeyEsc), false},
		{"plain rune", keyEv('a', 0), false},
		{"resize", term.Event{Type: term.EventResize, Width: 80, Height: 24}, false},
		{"mouse", term.Event{Type: term.EventMouse}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldSwallowBootstrapEvent(tc.ev))
		})
	}
}

func keyEv(ch rune, mod term.Modifier) term.Event {
	return term.Event{Type: term.EventKey, Ch: ch, Mod: mod}
}

func namedKeyEv(k term.Key) term.Event {
	return term.Event{Type: term.EventKey, Key: k}
}
