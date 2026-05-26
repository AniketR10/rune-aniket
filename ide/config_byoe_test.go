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

package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewCommandPromptEditorBYOE asserts that the command prompt
// editor falls back to a Rune-native prompt (vi) in byoe mode rather
// than panicking. This reproduces the crash reported when opening a
// workspace with editor.mode = "byoe".
func TestNewCommandPromptEditorBYOE(t *testing.T) {
	h := &workspaceManagerHandler{}
	cfg := ideConfig{
		cfg: map[string]any{
			"editor": map[string]any{
				"mode": "byoe",
				"byoe": map[string]any{
					"command": "vim {file}",
				},
			},
		},
		errors: map[string]error{},
	}
	require.NotPanics(t, func() {
		ed := h.newCommandPromptEditor(cfg)
		assert.NotNil(t, ed)
	})
}

// TestByoeModeAndAccessors covers editorMode/byoeCommand/byoeGoto on a
// hand-crafted config map.
func TestByoeModeAndAccessors(t *testing.T) {
	cfg := &ideConfig{
		cfg: map[string]any{
			"editor": map[string]any{
				"mode": "byoe",
				"byoe": map[string]any{
					"command": "vim {file}",
					"goto":    "<esc>:{line}<enter>",
				},
			},
		},
		errors: map[string]error{},
	}
	assert.Equal(t, "byoe", cfg.editorMode())
	assert.Equal(t, "vim {file}", cfg.byoeCommand())
	assert.Equal(t, "<esc>:{line}<enter>", cfg.byoeGoto())
}

// TestPkgEditorModeSubstitutesByoeFallback verifies the value forwarded to
// package config.star scripts: byoe mode is rewritten to the configured
// byoe.fallback so packages always see a concrete modal or modeless mode.
func TestPkgEditorModeSubstitutesByoeFallback(t *testing.T) {
	cases := []struct {
		name     string
		fallback string
		want     string
	}{
		{"default_fallback_is_modeless", "", "modeless"},
		{"explicit_modal_fallback", "modal", "modal"},
		{"explicit_modeless_fallback", "modeless", "modeless"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			byoe := map[string]any{
				"command": "vim {file}",
				"goto":    "<esc>:{line}<enter>",
			}
			if tc.fallback != "" {
				byoe["fallback"] = tc.fallback
			}
			cfg := &ideConfig{
				cfg: map[string]any{
					"editor": map[string]any{
						"mode": "byoe",
						"byoe": byoe,
					},
				},
				errors: map[string]error{},
			}
			assert.Equal(t, "byoe", cfg.editorMode())
			assert.Equal(t, tc.want, cfg.pkgEditorMode())
		})
	}
}

// TestPkgEditorModePassesNonByoeThrough verifies modal/modeless are
// forwarded verbatim to packages.
func TestPkgEditorModePassesNonByoeThrough(t *testing.T) {
	for _, mode := range []string{"modal", "modeless"} {
		t.Run(mode, func(t *testing.T) {
			cfg := &ideConfig{
				cfg: map[string]any{
					"editor": map[string]any{"mode": mode},
				},
				errors: map[string]error{},
			}
			assert.Equal(t, mode, cfg.pkgEditorMode())
		})
	}
}

// TestValidateBYOEFallsBackOnMissingFile verifies that a byoe mode
// with an invalid (no {file}) command is rewritten back to "modal" so
// the IDE still boots.
func TestValidateBYOEFallsBackOnMissingFile(t *testing.T) {
	cfg := map[string]any{
		"editor": map[string]any{
			"mode": "byoe",
			"byoe": map[string]any{
				"command": "vim",
			},
		},
	}
	err := validateConfig(cfg)
	require.Error(t, err)
	ic := &ideConfig{cfg: cfg, errors: map[string]error{}}
	assert.Equal(t, "modal", ic.editorMode())
}

// TestValidateBYOEFallsBackOnInvalidGoto verifies that an unparseable
// goto rewrites editor.mode back to "modal". byoe relies on goto to
// position the cursor, so a bad value is a hard misconfiguration.
func TestValidateBYOEFallsBackOnInvalidGoto(t *testing.T) {
	cfg := map[string]any{
		"editor": map[string]any{
			"mode": "byoe",
			"byoe": map[string]any{
				"command": "vim {file}",
				"goto":    "<bogus-key>",
			},
		},
	}
	err := validateConfig(cfg)
	require.Error(t, err)
	ic := &ideConfig{cfg: cfg, errors: map[string]error{}}
	assert.Equal(t, "modal", ic.editorMode())
}

// TestValidateBYOEFallsBackOnMissingGoto verifies that an unset goto
// rewrites editor.mode back to "modal" for the same reason as an
// invalid goto: byoe requires both editor.byoe.command and
// editor.byoe.goto.
func TestValidateBYOEFallsBackOnMissingGoto(t *testing.T) {
	cfg := map[string]any{
		"editor": map[string]any{
			"mode": "byoe",
			"byoe": map[string]any{
				"command": "vim {file}",
			},
		},
	}
	err := validateConfig(cfg)
	require.Error(t, err)
	ic := &ideConfig{cfg: cfg, errors: map[string]error{}}
	assert.Equal(t, "modal", ic.editorMode())
}

// TestValidateBYOEAcceptsKnownTemplates table-tests the bundled sample
// goto templates parse without errors.
func TestValidateBYOEAcceptsKnownTemplates(t *testing.T) {
	templates := []string{
		"<esc>:{line}<enter>{col}|",
		"<esc>:goto<space>{line}<enter>",
		"<c-l>{line}:{col}<enter>",
		"<a-x>goto-line<enter>{line}<enter>",
		"<c-_>{line},{col}<enter>",
	}
	for _, tpl := range templates {
		err := parseGotoTemplate(tpl)
		assert.NoError(t, err, "template %q must parse", tpl)
	}
}

// TestByoeFallbackDefaults asserts the accessor returns "modeless"
// when no fallback key is set (the documented default).
func TestByoeFallbackDefaults(t *testing.T) {
	cfg := ideConfig{
		cfg: map[string]any{
			"editor": map[string]any{
				"mode": "byoe",
				"byoe": map[string]any{
					"command": "vim {file}",
					"goto":    "<esc>:{line}<enter>",
				},
			},
		},
		errors: map[string]error{},
	}
	assert.Equal(t, "modeless", cfg.byoeFallback())
}

// TestByoeFallbackExplicitValues asserts both "modal" and "modeless"
// round-trip through the accessor.
func TestByoeFallbackExplicitValues(t *testing.T) {
	for _, want := range []string{"modal", "modeless"} {
		t.Run(want, func(t *testing.T) {
			cfg := ideConfig{
				cfg: map[string]any{
					"editor": map[string]any{
						"mode": "byoe",
						"byoe": map[string]any{
							"command":  "vim {file}",
							"goto":     "<esc>:{line}<enter>",
							"fallback": want,
						},
					},
				},
				errors: map[string]error{},
			}
			assert.Equal(t, want, cfg.byoeFallback())
		})
	}
}

// TestValidateBYOEFallbackInvalidValueRewrites verifies an unknown
// fallback string is rewritten to "modeless" so the IDE still boots.
func TestValidateBYOEFallbackInvalidValueRewrites(t *testing.T) {
	cfg := map[string]any{
		"editor": map[string]any{
			"mode": "byoe",
			"byoe": map[string]any{
				"command":  "vim {file}",
				"goto":     "<esc>:{line}<enter>",
				"fallback": "bogus",
			},
		},
	}
	err := validateConfig(cfg)
	require.Error(t, err)

	ic := ideConfig{cfg: cfg, errors: map[string]error{}}
	assert.Equal(t, "modeless", ic.byoeFallback(),
		"invalid fallback must be rewritten to %q", "modeless")
	// editor.mode remains "byoe" since command/goto are valid.
	assert.Equal(t, "byoe", ic.editorMode())
}

// TestByoeOverrideHighlights table-tests the accessor: defaults to
// true when unset, round-trips explicit true/false, and falls back to
// true while recording an error when the value is the wrong type.
func TestByoeOverrideHighlights(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		raw        any
		setRaw     bool
		wantValue  bool
		wantErrKey bool
	}{
		{name: "unset defaults to true", wantValue: true},
		{name: "explicit true", setRaw: true, raw: true, wantValue: true},
		{name: "explicit false", setRaw: true, raw: false, wantValue: false},
		{
			name:       "non-bool falls back to true",
			setRaw:     true,
			raw:        "yes",
			wantValue:  true,
			wantErrKey: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			byoe := map[string]any{
				"command": "vim {file}",
				"goto":    "<esc>:{line}<enter>",
			}
			if tc.setRaw {
				byoe["override_highlights"] = tc.raw
			}
			cfg := ideConfig{
				cfg: map[string]any{
					"editor": map[string]any{
						"mode": "byoe",
						"byoe": byoe,
					},
				},
				errors: map[string]error{},
			}
			got := cfg.byoeOverrideHighlights()
			assert.Equal(t, tc.wantValue, got)
			_, hadErr := cfg.errors["editor.byoe.override_highlights"]
			assert.Equal(t, tc.wantErrKey, hadErr,
				"expected errors entry to %v", tc.wantErrKey)
		})
	}
}
