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
		{optModal, editorModal},
		{optModeless, editorModeless},
		{optExoModal, editorExoModal},
		{optExoModeless, editorExoModeless},
		{"unknown", editorModal}, // default fallback
	}
	for _, tc := range cases {
		t.Run(tc.option, func(t *testing.T) {
			require.Equal(t, tc.want, optionToChoice(tc.option))
		})
	}
}

func TestOverrideTemplateRendersExo(t *testing.T) {
	cases := []struct {
		name           string
		editor         string
		format         string
		preset         string
		wantContains   []string
		wantNoContains []string
	}{
		{
			name:   "exo modal yaml with helix",
			editor: editorExoModal,
			format: configFormatYAML,
			preset: exoPresetHelix,
			wantContains: []string{
				"mode: exo",
				"command: 'hx {file}:{line}:{col}'",
				"fallback: modal",
			},
			// Comments may include escaped `{{.Command}}` literals
			// pointing to the substitution semantics. We only
			// require that the actual YAML values are substituted.
			wantNoContains: nil,
		},
		{
			name:   "exo modeless star with vim",
			editor: editorExoModeless,
			format: configFormatStar,
			preset: exoPresetVim,
			wantContains: []string{
				`"mode": "exo"`,
				`vim "+call cursor({line}, {col})" {file}`,
				`"fallback": "modeless"`,
			},
			wantNoContains: nil,
		},
		{
			// Regression: override_exo_modeless.yaml previously
			// set `editor.mode: modeless` instead of `exo`, which
			// disabled the exo pipeline despite the user picking
			// exo in the bootstrap flow. The fallback is what
			// must be modeless, not the mode.
			name:   "exo modeless yaml with helix",
			editor: editorExoModeless,
			format: configFormatYAML,
			preset: exoPresetHelix,
			wantContains: []string{
				"mode: exo",
				"command: 'hx {file}:{line}:{col}'",
				"fallback: modeless",
			},
			wantNoContains: []string{
				// active block must not say mode: modeless
				"mode: modeless",
			},
		},
		{
			// Same regression for the Starlark variant.
			name:   "exo modeless star with nvim",
			editor: editorExoModeless,
			format: configFormatStar,
			preset: exoPresetNvim,
			wantContains: []string{
				`"mode": "exo"`,
				`nvim "+call cursor({line}, {col})" {file}`,
				`"fallback": "modeless"`,
			},
			wantNoContains: []string{
				`"mode": "modeless"`,
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := renderOverride(tc.editor, tc.format, tc.preset)
			require.NoError(t, err)
			for _, want := range tc.wantContains {
				require.Contains(t, got, want)
			}
			for _, unwant := range tc.wantNoContains {
				require.NotContains(t, got, unwant)
			}
		})
	}
}

func TestOptionToFormatAndPresetMapping(t *testing.T) {
	require.Equal(t, configFormatStar, optionToFormat(optFormatStar))
	require.Equal(t, configFormatYAML, optionToFormat(optFormatYAML))
	require.Equal(t, configFormatYAML, optionToFormat("unknown"))

	require.Equal(t, exoPresetVim, optionToPreset(optPresetVim))
	require.Equal(t, exoPresetNvim, optionToPreset(optPresetNvim))
	require.Equal(t, exoPresetHelix, optionToPreset(optPresetHelix))
	require.Equal(t, exoPresetKak, optionToPreset(optPresetKak))
	require.Equal(t, exoPresetEmacs, optionToPreset(optPresetEmacs))
	require.Equal(t, exoPresetVim, optionToPreset("unknown"))
}

// TestGuardedPromptChainReopensOnUnadvancedClose proves that an Esc
// (or any dismissal that does not call OnSelect) re-opens the same
// prompt, while a normal OnSelect-then-Close sequence does not.
func TestGuardedPromptChainReopensOnUnadvancedClose(t *testing.T) {
	t.Run("esc reopens", func(t *testing.T) {
		var reopened int
		g := &guardedPromptChain{}
		_ = g.onClose(func() { reopened++ })()
		require.Equal(t, 1, reopened, "Esc-equivalent close must re-open")
	})

	t.Run("select does not reopen", func(t *testing.T) {
		var reopened int
		var advanced int
		g := &guardedPromptChain{}
		// Simulate the normal selection-then-close ordering: the
		// SDK calls OnSelect first (inside Handle), which marks
		// advanced, then Close → OnClose.
		g.onSelect(func(_ int, _ string) { advanced++ })(0, "")
		_ = g.onClose(func() { reopened++ })()
		require.Equal(t, 1, advanced)
		require.Equal(t, 0, reopened, "selection must not re-open")
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
		// bindings from rune.star and override_modeless.star.
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
