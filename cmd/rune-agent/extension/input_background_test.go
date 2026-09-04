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

package extension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestResolveInputBackgroundColor(t *testing.T) {
	tests := []struct {
		name string
		cfg  map[string]any
		def  term.Color
		want term.Color
	}{
		{
			name: "no input_box_attr keeps the default",
			cfg:  map[string]any{},
			def:  term.ColorDefault,
			want: term.ColorDefault,
		},
		{
			name: "input_box_attr without bg keeps the default",
			cfg:  map[string]any{"input_box_attr": map[string]any{"fg": "default"}},
			def:  term.ColorDefault,
			want: term.ColorDefault,
		},
		{
			name: "input_box_attr bg default keeps the default",
			cfg: map[string]any{
				"input_box_attr": map[string]any{"fg": "default", "bg": "default"},
			},
			def:  term.ColorDefault,
			want: term.ColorDefault,
		},
		{
			name: "explicit bg overrides the default",
			cfg: map[string]any{
				"input_box_attr": map[string]any{"bg": "red"},
			},
			def:  term.ColorDefault,
			want: term.ColorRed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveInputBackgroundColor(tc.def, config.MapConfig(tc.cfg))
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDefaultComponentInputBackgroundIsDefault(t *testing.T) {
	assert.Equal(t, term.ColorDefault, defaultComponentCfg.InputBackgroundColor,
		"compose input must default to the terminal default background")
}
