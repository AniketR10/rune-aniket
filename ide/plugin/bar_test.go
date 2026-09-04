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

package plugin

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestBarDraw(t *testing.T) {
	suite := []struct {
		desc     string
		width    int
		layout   string
		action   func(b *pluginHandlerBar)
		expected string
	}{
		{"empty layout renders nothing", 20, "", nil,
			"                    ",
		},
		{"status icon, not done", 20, "{{ .StatusIcon }}", nil,
			"⠃                   ",
		},
		{"status icon with padding on the left NOT DONE, next align left", 20,
			" {{ .StatusIcon | bg \"gray\" | fg \"white\" }}   {{ .Elapsed }}", nil,
			" ⠃   0s             ",
		},
		{"status icon with padding on the left NOT DONE, next align right", 20,
			" {{ .StatusIcon | bg \"gray\" | fg \"white\" }}   {{ .AlignRight }}{{ .Elapsed }}", nil,
			" ⠃                0s",
		},
		{"status icon with padding on the left NOT DONE, next align center", 20,
			" {{ .StatusIcon | bg \"gray\" | fg \"white\" }}   {{ .AlignCenter }}{{ .Elapsed }}", nil,
			" ⠃        0s        ",
		},
		{"status icon with padding on the left DONE, next align center", 20,
			" {{ .StatusIcon | bg \"gray\" | fg \"white\" }}   {{ .AlignCenter }}{{ .Elapsed }}",
			func(h *pluginHandlerBar) {
				h.setDone(nil)
			},
			" ▀        0s        ",
		},
		{"status icon right aligned with no padding NOT DONE, next align center", 20,
			"{{ .AlignCenter }}{{ .Elapsed }}{{ .AlignRight }}{{ .StatusIcon | bg \"gray\" | fg \"white\" }}", nil,
			"         0s        ⠃",
		},
		{"status icon right aligned with no padding DONE, next align center", 20,
			"{{ .AlignCenter }}{{ .Elapsed }}{{ .AlignRight}}{{ .StatusIcon | bg \"gray\" | fg \"white\" }}",
			func(h *pluginHandlerBar) {
				h.setDone(nil)
			},
			"         0s        ▀",
		},
		{"status icon right aligned with padding on the right NOT DONE, next align center", 20,
			"{{ .AlignCenter }}{{ .Elapsed }}{{ .AlignRight }}{{ .StatusIcon | bg \"gray\" | fg \"white\" }} ", nil,
			"         0s       ⠃ ",
		},
		{"status icon right aligned with padding on the right DONE, next align center", 20,
			"{{ .AlignCenter }}{{ .Elapsed }}{{ .AlignRight}}{{ .StatusIcon | bg \"gray\" | fg \"white\" }} ",
			func(h *pluginHandlerBar) {
				h.setDone(nil)
			},
			"         0s       ▀ ",
		},
		{"status icon right aligned with padding on the left NOT DONE, next align center", 20,
			"{{ .AlignCenter }}{{ .Elapsed }}{{ .AlignRight }} {{ .StatusIcon | bg \"gray\" | fg \"white\" }}", nil,
			"         0s        ⠃",
		},
		{"status icon right aligned with padding on the left DONE, next align center", 20,
			"{{ .AlignCenter }}{{ .Elapsed }}{{ .AlignRight}} {{ .StatusIcon | bg \"gray\" | fg \"white\" }}",
			func(h *pluginHandlerBar) {
				h.setDone(nil)
			},
			"         0s        ▀",
		},
		{"status icon right aligned with padding on the left and right NOT DONE, next align center", 20,
			"{{ .AlignCenter }}{{ .Elapsed }}{{ .AlignRight }} {{ .StatusIcon | bg \"gray\" | fg \"white\" }} ", nil,
			"         0s       ⠃ ",
		},
		{"status icon right aligned with padding on the left and right DONE with error, next align center", 20,
			"{{ .AlignCenter }}{{ .ExitStatus }}{{ .AlignRight}} {{ .StatusIcon | bg \"gray\" | fg \"white\" }} ",
			func(h *pluginHandlerBar) {
				h.setDone(errors.New("swift error"))
			},
			"      non-zero    ▀ ",
		},
		{"status icon right aligned with padding on the left and right NOT DONE, next align center", 20,
			"{{ .AlignCenter }}{{ .Elapsed }}{{ .AlignRight}} {{ .StatusIcon | bg \"gray\" | fg \"white\" }} ",
			func(h *pluginHandlerBar) {
			},
			"         0s       ⠃ ",
		},
		{"status icon right aligned with padding on the left and right DONE, double elapsed", 20,
			"{{ .AlignRight}}{{ .Elapsed }}   {{ .StatusIcon | bg \"gray\" | fg \"white\" }}   {{ .Elapsed }}  ",
			func(h *pluginHandlerBar) {
			},
			"            ⠃   0s  ",
		},
		{"status icon center aligned with padding on the left and right DONE", 20,
			"{{ .Elapsed }}{{ .AlignCenter }} {{ .StatusIcon | bg \"gray\" | fg \"white\" }} {{ .AlignRight }}{{ .Elapsed }}  ",
			func(h *pluginHandlerBar) {
			},
			"         ⠃      0s  ",
		},
	}

	for _, test := range suite {
		t.Run(test.desc, func(t *testing.T) {
			var err error
			config := DefaultBarConfig()
			config.BackgroundColor = term.ColorRed // exercise bg setting for panics
			config.StatusAnimationFrames, _ = component.ProgressAnimationFrames()
			config.Layout, err = ParseBarLayout(test.layout)
			require.NoError(t, err)
			h := newPluginHandlerBar("", term.NopInterrupter(), config)
			h.runningPrecision = time.Minute
			h.donePrecision = time.Minute
			h.rebuildElapsed()
			h.Resize(test.width, 1)
			if test.action != nil {
				test.action(h)
			}
			w := term.NewStringWriter(test.width, 1)
			h.Draw(w)
			require.NoError(t, w.Flush())
			assert.Equal(t, test.expected, w.String())
			assert.NoError(t, h.Close())
		})
	}
}
