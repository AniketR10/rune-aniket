// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/term"
	"unstable.build/go-tui/text"
)

func TestStatusBarLayout(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantComps  []text.StatusBarComponent
		wantErr    bool
		wantErrSub string // substring to look for in error
	}{
		{
			name:  "single component - language",
			input: `{{ .Language }}`,
			wantComps: []text.StatusBarComponent{
				{Type: text.StatusBarLanguage, Template: "%s"},
			},
		},
		{
			name:  "prefix text before component",
			input: `⚑{{ .Status }}`,
			wantComps: []text.StatusBarComponent{
				{Type: text.StatusBarStatus, Template: "⚑%s"},
			},
		},
		{
			name:  "multiple components with separator",
			input: `{{ .GitDiffAdd }}+{{ .GitDiffDel }}`,
			wantComps: []text.StatusBarComponent{
				{Type: text.StatusBarGitDiffAdded, Template: "%d+"},
				{Type: text.StatusBarGitDiffDeleted, Template: "%d"},
			},
		},
		{
			name:  "trailing padding appended to last component",
			input: `{{ .Language }}  `,
			wantComps: []text.StatusBarComponent{
				{Type: text.StatusBarLanguage, Template: "%s  "},
			},
		},
		{
			name:  "prefix/mid/trailing text is parsed into templates",
			input: `pre {{ .CursorColumn }} mid {{ .CursorLine }} post`,
			wantComps: []text.StatusBarComponent{
				{Type: text.StatusBarCoordinatesCursorX, Template: "pre %d mid "},
				// second receives " mid " during parsing, plus trailing " post" appended after loop
				{Type: text.StatusBarCoordinatesCursorY, Template: "%d post"},
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
			input: `{{ .Status | fg "red" }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarStatus,
					Template:   "%s",
					Attributes: term.Attributes{Fg: tcell.ColorRed},
				},
			},
		},
		{
			name:  "bg color - quoted",
			input: `{{ .Status | bg "blue" }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarStatus,
					Template:   "%s",
					Attributes: term.Attributes{Bg: tcell.ColorBlue},
				},
			},
		},
		{
			name:  "fg and bg together",
			input: `{{ .Status | fg "black" | bg "red" }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarStatus,
					Template:   "%s",
					Attributes: term.Attributes{Fg: tcell.ColorBlack, Bg: tcell.ColorRed},
				},
			},
		},
		{
			name:  "bold style",
			input: `{{ .Language | bold }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarLanguage,
					Template:   "%s",
					Attributes: term.Attributes{Attrs: tcell.AttrBold},
				},
			},
		},
		{
			name:  "italic style",
			input: `{{ .Filepath | italic }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarFilePath,
					Template:   "%s",
					Attributes: term.Attributes{Attrs: tcell.AttrItalic},
				},
			},
		},
		{
			name:  "multiple style attributes combined",
			input: `{{ .Status | bold | underline }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarStatus,
					Template:   "%s",
					Attributes: term.Attributes{Attrs: tcell.AttrBold | tcell.AttrUnderline},
				},
			},
		},
		{
			name:  "full styling - colors and attributes",
			input: `{{ .Status | bg "red" | fg "black" | bold }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:     text.StatusBarStatus,
					Template: "%s",
					Attributes: term.Attributes{
						Bg:    tcell.ColorRed,
						Fg:    tcell.ColorBlack,
						Attrs: tcell.AttrBold,
					},
				},
			},
		},
		{
			name:  "hex color - with hash",
			input: `{{ .Status | fg "#ff5500" }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarStatus,
					Template:   "%s",
					Attributes: term.Attributes{Fg: tcell.NewRGBColor(0xff, 0x55, 0x00)},
				},
			},
		},
		{
			name:       "hex color - without hash is an error",
			input:      `{{ .Status | bg "00ff00" }}`,
			wantErr:    true,
			wantErrSub: "error is not a hex color starting with #, nor a known named W3C color in lowercase",
		},
		{
			name:  "multiple components with different attributes",
			input: `{{ .GitDiffAdd | fg "green" }}  {{ .GitDiffDel | fg "red" }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarGitDiffAdded,
					Template:   "%d",
					Attributes: term.Attributes{Fg: tcell.ColorGreen},
				},
				{
					Type:       text.StatusBarGitDiffDeleted,
					Template:   "  %d",
					Attributes: term.Attributes{Fg: tcell.ColorRed},
				},
			},
		},
		{
			name:  "component with prefix and attributes and suffix, attrs are grouped until blank space",
			input: `█{{ .Status | bg "red" | bold }}█▓▒░  {{ .GitDiffDel }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:     text.StatusBarStatus,
					Template: "█%s█▓▒░",
					Attributes: term.Attributes{
						Bg:    tcell.ColorRed,
						Attrs: tcell.AttrBold,
					},
				},
				{
					Type:       text.StatusBarGitDiffDeleted,
					Template:   "  %d",
					Attributes: term.Attributes{},
				},
			},
		},
		{
			name:  "components are grouped by double blank space",
			input: `█{{ .Status }}█▓▒░  {{ .TotalLines | bg "red" }} lines  {{ .GitDiffDel }} {{ .GitDiffAdd }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarStatus,
					Template:   "█%s█▓▒░",
					Attributes: term.Attributes{},
				},
				{
					Type:     text.StatusBarTotalLines,
					Template: "  %d lines",
					Attributes: term.Attributes{
						Bg: tcell.ColorRed,
					},
				},
				{
					Type:       text.StatusBarGitDiffDeleted,
					Template:   "  %d ",
					Attributes: term.Attributes{},
				},
				{
					Type:       text.StatusBarGitDiffAdded,
					Template:   "%d",
					Attributes: term.Attributes{},
				},
			},
		},
		{
			name:  "all style attributes",
			input: `{{ .Status | bold | italic | underline | dim | reverse | strikethrough | blink }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:     text.StatusBarStatus,
					Template: "%s",
					Attributes: term.Attributes{
						Attrs: tcell.AttrBold | tcell.AttrItalic | tcell.AttrUnderline |
							tcell.AttrDim | tcell.AttrReverse | tcell.AttrStrikeThrough | tcell.AttrBlink,
					},
				},
			},
		},
		{
			name:  "void component with attributes ignored",
			input: `{{ .ShiftRight }}`,
			wantComps: []text.StatusBarComponent{
				{Type: text.StatusBarVoid, Template: ""},
			},
		},
		{
			name:  "default color",
			input: `{{ .Status | fg "default" }}`,
			wantComps: []text.StatusBarComponent{
				{
					Type:       text.StatusBarStatus,
					Template:   "%s",
					Attributes: term.Attributes{Fg: tcell.ColorDefault},
				},
			},
		},
		// Error cases
		{
			name:       "unknown color name",
			input:      `{{ .Status | fg "belindings" }}`,
			wantErr:    true,
			wantErrSub: "error is not a hex color starting with #, nor a known named W3C color in lowercase",
		},
		{
			name:       "fg missing color argument",
			input:      `{{ .Status | fg }}`,
			wantErr:    true,
			wantErrSub: "requires a color argument",
		},
		{
			name:       "bg missing color argument",
			input:      `{{ .Status | bg }}`,
			wantErr:    true,
			wantErrSub: "requires a color argument",
		},
		{
			name:       "unknown attribute command",
			input:      `{{ .Status | sparkle }}`,
			wantErr:    true,
			wantErrSub: "template: status_bar.layout:1: function \"sparkle\" not defined",
		},
		{
			name:       "invalid hex color - too short",
			input:      `{{ .Status | fg "#fff" }}`,
			wantErr:    true,
			wantErrSub: "error is not a hex color starting with #, nor a known named W3C color in lowercase",
		},
		{
			name:       "invalid hex color - bad characters",
			input:      `{{ .Status | fg "#gggggg" }}`,
			wantErr:    true,
			wantErrSub: "error is not a hex color starting with #, nor a known named W3C color in lowercase",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := statusBarLayout(tt.input)

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
