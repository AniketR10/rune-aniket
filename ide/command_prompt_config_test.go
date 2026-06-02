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

// TestCommandPromptSeparatorCharsetDefaults ensures the four stitch
// glyphs fall back to the single-line T-junction defaults when the
// user config does not override command.separator_charset.
func TestCommandPromptSeparatorCharsetDefaults(t *testing.T) {
	c := newAnimConfig(t, `config = {}`)
	got := c.commandPromptSeparatorCharset()
	assert.Equal(t, defaultCommandPromptSeparatorCharset(), got)
	assert.Empty(t, c.errors)
}

// TestCommandPromptSeparatorCharsetOverrides verifies that each key
// under command.separator_charset overlays only its rune and the
// remaining glyphs keep their defaults.
func TestCommandPromptSeparatorCharsetOverrides(t *testing.T) {
	c := newAnimConfig(t, `
config = {
    "command": {
        "separator_charset": {
            "left":             "🭼",
            "horizontal_left":  "▁",
            "horizontal_right": "▁",
            "right":            "🭿",
        },
    },
}
`)
	got := c.commandPromptSeparatorCharset()
	assert.Equal(t, '🭼', got.Left)
	assert.Equal(t, '▁', got.HorizontalLeft)
	assert.Equal(t, '▁', got.HorizontalRight)
	assert.Equal(t, '🭿', got.Right)
	assert.Empty(t, c.errors)
}

// TestCommandPromptSeparatorCharsetPartialOverride keeps unspecified
// glyphs at their defaults so users can change only the corners.
func TestCommandPromptSeparatorCharsetPartialOverride(t *testing.T) {
	c := newAnimConfig(t, `
config = {
    "command": {
        "separator_charset": {
            "left":  "╠",
            "right": "╣",
        },
    },
}
`)
	got := c.commandPromptSeparatorCharset()
	def := defaultCommandPromptSeparatorCharset()
	assert.Equal(t, '╠', got.Left)
	assert.Equal(t, def.HorizontalLeft, got.HorizontalLeft)
	assert.Equal(t, def.HorizontalRight, got.HorizontalRight)
	assert.Equal(t, '╣', got.Right)
	assert.Empty(t, c.errors)
}

// TestCommandPromptSeparatorCharsetWrongType records an error under
// the offending key and keeps the default glyph so a typo never
// silently breaks the separator.
func TestCommandPromptSeparatorCharsetWrongType(t *testing.T) {
	c := newAnimConfig(t, `
config = {
    "command": {
        "separator_charset": {
            "left": ["nope"],
        },
    },
}
`)
	got := c.commandPromptSeparatorCharset()
	def := defaultCommandPromptSeparatorCharset()
	assert.Equal(t, def.Left, got.Left,
		"wrong type must keep default Left glyph")
	require.Contains(t, c.errors, "command.separator_charset.left")
}

// TestCommandPromptCfgComposes verifies that commandPromptCfg
// surfaces both the shader-enable flag and the separator glyphs in a
// single struct.
func TestCommandPromptCfgComposes(t *testing.T) {
	c := newAnimConfig(t, `
config = {
    "animations": {"command_prompt": False},
    "command": {
        "separator_charset": {
            "left":  "╠",
            "right": "╣",
        },
    },
}
`)
	got := c.commandPromptCfg()
	assert.False(t, got.shader.enabled)
	assert.Equal(t, '╠', got.separator.Left)
	assert.Equal(t, '╣', got.separator.Right)
	assert.Empty(t, c.errors)
}
