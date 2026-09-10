// Copyright (C) 2017-2026 The Rune Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestBarLayout(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantComps  []BarComponent
		wantErr    bool
		wantErrSub string // substring to look for in error
	}{
		{
			name:  "single component - language",
			input: `{{ .StatusIcon }}`,
			wantComps: []BarComponent{
				{Type: BarStatusIcon, Template: "%s"},
			},
		},
		{
			name:  "prefix before component",
			input: `⚑{{ .ExitStatus }}`,
			wantComps: []BarComponent{
				{Type: BarExitStatus, Template: "⚑%s"},
			},
		},
		{
			name:  "multiple components with separator",
			input: `{{ .Command }}+{{ .Elapsed }}`,
			wantComps: []BarComponent{
				{Type: BarCommand, Template: "%s+"},
				{Type: BarElapsed, Template: "%s"},
			},
		},
		{
			name:  "trailing padding appended to last component",
			input: `{{ .StatusIcon }}  `,
			wantComps: []BarComponent{
				{Type: BarStatusIcon, Template: "%s  "},
			},
		},
		{
			name:  "prefix/mid/trailing is parsed into templates",
			input: `pre {{ .StatusIcon }} mid {{ .ExitStatus}} post`,
			wantComps: []BarComponent{
				{Type: BarStatusIcon, Template: "pre %s mid "},
				// second receives " mid " during parsing, plus trailing " post" appended after loop
				{Type: BarExitStatus, Template: "%s post"},
			},
		},
		{
			name:       "unknown component name returns error",
			input:      `{{ .IAmNoComponent }}`,
			wantErr:    true,
			wantErrSub: "unknown status bar component",
		},
		{
			name:       "unsupported action (function call) returns error",
			input:      `{{ printf "%s" "x" }}`,
			wantErr:    true,
			wantErrSub: "template: status_bar.layout:1: function \"printf\" not defined",
		},
		{
			name:       "unsupported node type (if) returns error",
			input:      `{{ if true }}{{ .Language }}{{ end }}`,
			wantErr:    true,
			wantErrSub: "unsupported node type",
		},
		{
			name:  "fg color - quoted",
			input: `{{ .StatusIcon | fg "red" }}`,
			wantComps: []BarComponent{
				{
					Type:       BarStatusIcon,
					Template:   "%s",
					Attributes: term.Attributes{Fg: term.ColorRed},
				},
			},
		},
		{
			name:  "bg color - quoted",
			input: `{{ .StatusIcon | bg "blue" }}`,
			wantComps: []BarComponent{
				{
					Type:       BarStatusIcon,
					Template:   "%s",
					Attributes: term.Attributes{Bg: term.ColorBlue},
				},
			},
		},
		{
			name:  "fg and bg together",
			input: `{{ .StatusIcon | fg "black" | bg "red" }}`,
			wantComps: []BarComponent{
				{
					Type:       BarStatusIcon,
					Template:   "%s",
					Attributes: term.Attributes{Fg: term.ColorBlack, Bg: term.ColorRed},
				},
			},
		},
		{
			name:  "bold style",
			input: `{{ .Command | bold }}`,
			wantComps: []BarComponent{
				{
					Type:       BarCommand,
					Template:   "%s",
					Attributes: term.Attributes{Attrs: term.AttrBold},
				},
			},
		},
		{
			name:  "italic style",
			input: `{{ .ExitStatus | italic }}`,
			wantComps: []BarComponent{
				{
					Type:       BarExitStatus,
					Template:   "%s",
					Attributes: term.Attributes{Attrs: term.AttrItalic},
				},
			},
		},
		{
			name:  "multiple style attributes combined",
			input: `{{ .Elapsed | bold | underline }}`,
			wantComps: []BarComponent{
				{
					Type:       BarElapsed,
					Template:   "%s",
					Attributes: term.Attributes{Attrs: term.AttrBold | term.AttrUnderline},
				},
			},
		},
		{
			name:  "full styling - colors and attributes",
			input: `{{ .Elapsed | bg "red" | fg "black" | bold }}`,
			wantComps: []BarComponent{
				{
					Type:     BarElapsed,
					Template: "%s",
					Attributes: term.Attributes{
						Bg:    term.ColorRed,
						Fg:    term.ColorBlack,
						Attrs: term.AttrBold,
					},
				},
			},
		},
		{
			name:  "hex color - with hash",
			input: `{{ .Elapsed | fg "#ff5500" }}`,
			wantComps: []BarComponent{
				{
					Type:       BarElapsed,
					Template:   "%s",
					Attributes: term.Attributes{Fg: term.NewRGBColor(0xff, 0x55, 0x00)},
				},
			},
		},
		{
			name:       "hex color - without hash is an error",
			input:      `{{ .Elapsed | bg "00ff00" }}`,
			wantErr:    true,
			wantErrSub: "error is not a hex color starting with #, nor a known named W3C color in lowercase",
		},
		{
			name:  "multiple components with different attributes",
			input: `{{ .Elapsed | fg "green" }}  {{ .Command | fg "red" }}`,
			wantComps: []BarComponent{
				{
					Type:       BarElapsed,
					Template:   "%s",
					Attributes: term.Attributes{Fg: term.ColorGreen},
				},
				{
					Type:       BarCommand,
					Template:   "  %s",
					Attributes: term.Attributes{Fg: term.ColorRed},
				},
			},
		},
		{
			name:  "blank lines should be grouped with the succeeding compoent, before shift right",
			input: `█{{ .StatusIcon | bg "red" | bold }}█▓▒░  {{ .Elapsed }}`,
			wantComps: []BarComponent{
				{
					Type:     BarStatusIcon,
					Template: "█%s█▓▒░",
					Attributes: term.Attributes{
						Bg:    term.ColorRed,
						Attrs: term.AttrBold,
					},
				},
				{
					Type:       BarElapsed,
					Template:   "  %s",
					Attributes: term.Attributes{},
				},
			},
		},
		{
			name:  "components are grouped by double blank space",
			input: `█{{ .StatusIcon }}█▓▒░  {{ .Command | bg "red" }} lines  {{ .Elapsed }} {{ .ExitStatus }}`,
			wantComps: []BarComponent{
				{
					Type:       BarStatusIcon,
					Template:   "█%s█▓▒░",
					Attributes: term.Attributes{},
				},
				{
					Type:     BarCommand,
					Template: "  %s lines",
					Attributes: term.Attributes{
						Bg: term.ColorRed,
					},
				},
				{
					Type:       BarElapsed,
					Template:   "  %s ",
					Attributes: term.Attributes{},
				},
				{
					Type:       BarExitStatus,
					Template:   "%s",
					Attributes: term.Attributes{},
				},
			},
		},
		{
			name:  "all style attributes",
			input: `{{ .StatusIcon | bold | italic | underline | dim | reverse | strikethrough | blink }}`,
			wantComps: []BarComponent{
				{
					Type:     BarStatusIcon,
					Template: "%s",
					Attributes: term.Attributes{
						Attrs: term.AttrBold | term.AttrItalic | term.AttrUnderline |
							term.AttrDim | term.AttrReverse | term.AttrStrikeThrough | term.AttrBlink,
					},
				},
			},
		},
		{
			name:  "align right component with attributes ignored",
			input: `{{ .AlignRight }}`,
			wantComps: []BarComponent{
				{Type: BarAlignRight, Template: ""},
			},
		},
		{
			name:  "align center component with attributes ignored",
			input: `{{ .AlignCenter }}`,
			wantComps: []BarComponent{
				{Type: BarAlignCenter, Template: ""},
			},
		},
		{
			name:  "default color",
			input: `{{ .StatusIcon | fg "default" }}`,
			wantComps: []BarComponent{
				{
					Type:       BarStatusIcon,
					Template:   "%s",
					Attributes: term.Attributes{Fg: term.ColorDefault},
				},
			},
		},
		// Error cases
		{
			name:       "unknown color name",
			input:      `{{ .StatusIcon | fg "belindings" }}`,
			wantErr:    true,
			wantErrSub: "error is not a hex color starting with #, nor a known named W3C color in lowercase",
		},
		{
			name:       "fg missing color argument",
			input:      `{{ .StatusIcon | fg }}`,
			wantErr:    true,
			wantErrSub: "requires a color argument",
		},
		{
			name:       "bg missing color argument",
			input:      `{{ .StatusIcon | bg }}`,
			wantErr:    true,
			wantErrSub: "requires a color argument",
		},
		{
			name:       "unknown attribute command",
			input:      `{{ .StatusIcon | sparkle }}`,
			wantErr:    true,
			wantErrSub: "template: status_bar.layout:1: function \"sparkle\" not defined",
		},
		{
			name:       "invalid hex color - too short",
			input:      `{{ .StatusIcon | fg "#fff" }}`,
			wantErr:    true,
			wantErrSub: "error is not a hex color starting with #, nor a known named W3C color in lowercase",
		},
		{
			name:       "invalid hex color - bad characters",
			input:      `{{ .StatusIcon | fg "#gggggg" }}`,
			wantErr:    true,
			wantErrSub: "error is not a hex color starting with #, nor a known named W3C color in lowercase",
		},
		{
			name:  "blank lines should be grouped with the preceding component, after shiftright",
			input: `{{ .AlignRight }}{{ .StatusIcon }}:{{ .ExitStatus }}  {{ .Command }} lines   {{ .Elapsed | bg "navy" | bold }} `,
			wantComps: []BarComponent{
				{
					Type:     BarAlignRight,
					Template: "",
				},
				{
					Type:     BarStatusIcon,
					Template: "%s:",
				},
				{
					Type:       BarExitStatus,
					Template:   "%s  ",
					Attributes: term.Attributes{Fg: term.ColorDefault},
				},
				{
					Type:       BarCommand,
					Template:   "%s lines  ",
					Attributes: term.Attributes{Fg: term.ColorDefault},
				},
				{
					Type:       BarElapsed,
					Template:   " %s ",
					Attributes: term.Attributes{Bg: term.ColorNavy, Attrs: term.AttrBold},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBarLayout(tt.input)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrSub != "" {
					assert.Contains(t, err.Error(), tt.wantErrSub)
				}
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.ElementsMatch(t, tt.wantComps, got)
		})
	}
}
