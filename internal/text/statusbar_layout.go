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

package text

import (
	"errors"
	"fmt"
	"strings"
	"text/template/parse"

	"unstable.build/rune/internal/component/template"
)

// ParseStatusBarLayout parses the given layout string into a set of
// text.StatusBarComponent. The expected format is Go templates.
//
// The following ActionNode's are available:
//   - Status: the status message determined by the editor.
//   - Filepath: the path of the file being edited, relative to the workspace.
//   - GitShortRef: the git reference pointed at by HEAD.
//   - GitDiffAdd: the number of lines added.
//   - GitDiffDel: the number of lines deleted.
//   - ShiftRight: up until this component, all components are aligned left.
//   - CursorColumn: cursor column
//   - CursorLine: cursor line
//   - TotalLines: total number of lines in the file
//   - Language: language parser status indicator
//
// Each component can have custom attributes via a pipe operator "|".
//
// The supported functions are the following:
//   - bg <color>: set the background color as a hex value or a W3C named color.
//   - fg <color>: set the foreground color as a hex value or a W3C named color.
//   - bold: set the text style as bold
//   - italic: set the text style as italic
//   - underline: set the text style as underline
//   - reverse: reverse the text foreground and background
//   - dim: dim the foreground color
func ParseStatusBarLayout(layoutStr string) (
	ret []StatusBarComponent, err error,
) {
	ret = nil
	tree, err := parse.Parse("status_bar.layout", layoutStr, "{{", "}}",
		template.AllowedFuncs())
	if err != nil {
		return
	}

	root := tree["status_bar.layout"].Root
	if root == nil {
		err = errors.New("parse template error: nil root")
		return
	}

	var tmpl string
	var shiftRight bool
	for _, node := range root.Nodes {
		switch n := node.(type) {
		case *parse.TextNode:
			// suffix attributes
			if len(n.Text) != 0 && len(ret) > 0 {
				all := strings.Split(string(n.Text), "  ")
				if shiftRight {
					ret[len(ret)-1].Template += all[0]
					if len(all) > 1 {
						ret[len(ret)-1].Template += "  "
					}
					for i, chunk := range all[1:] {
						tmpl += chunk
						if i < len(all[1:])-1 {
							tmpl += "  "
						}
					}
				} else {
					ret[len(ret)-1].Template += all[0]
					for _, chunk := range all[1:] {
						tmpl += "  "
						tmpl += chunk
					}
				}
			} else {
				tmpl += string(n.Text)
			}

		case *parse.ActionNode:
			fieldName, attrs, err := template.ParseAction(n)
			if err != nil {
				return nil, err
			}

			var compType StatusBarComponentType
			switch fieldName {
			case "Status":
				compType = StatusBarStatus
				tmpl += "%s"
			case "Filepath":
				compType = StatusBarFilePath
				tmpl += "%s"
			case "GitShortRef":
				compType = StatusBarGitShortRef
				tmpl += "%s"
			case "GitDiffAdd":
				compType = StatusBarGitDiffAdded
				tmpl += "%d"
			case "GitDiffDel":
				compType = StatusBarGitDiffDeleted
				tmpl += "%d"
			case "DiagError":
				compType = StatusBarDiagnosticsError
				tmpl += "%d"
			case "DiagWarn":
				compType = StatusBarDiagnosticsWarning
				tmpl += "%d"
			case "DiagInfo":
				compType = StatusBarDiagnosticsInfo
				tmpl += "%d"
			case "Language":
				compType = StatusBarLanguage
				tmpl += "%s"
			case "CursorColumn":
				compType = StatusBarCoordinatesCursorX
				tmpl += "%d"
			case "CursorLine":
				compType = StatusBarCoordinatesCursorY
				tmpl += "%d"
			case "TotalLines":
				compType = StatusBarTotalLines
				tmpl += "%d"
			case "ShiftRight":
				shiftRight = true
				ret = append(ret, StatusBarComponent{
					Type: StatusBarVoid,
				})
				continue
			default:
				err = fmt.Errorf("unknown status bar component: %q", fieldName)
				return nil, err
			}
			ret = append(ret, StatusBarComponent{
				Type:       compType,
				Template:   tmpl,
				Attributes: attrs,
			})
			tmpl = ""

		default:
			err = fmt.Errorf("unsupported node type %T at position %d", n, node.Position())
			return nil, err
		}
	}

	// Trailing text after the last component - append to last component's template
	// This handles cases like "{{ .Language }}  " where there's padding at the end
	if tmpl != "" && len(ret) > 0 {
		ret[len(ret)-1].Template += tmpl
	}

	return
}
