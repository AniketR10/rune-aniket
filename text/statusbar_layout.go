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

package text

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"text/template/parse"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/term"
)

var errInvalidColor = errors.New("error is not a hex color starting with #, " +
	"nor a known named W3C color in lowercase")

var allowedFuncs = map[string]any{
	"fg":            func(string) string { return "" },
	"bg":            func(string) string { return "" },
	"bold":          func() string { return "" },
	"underline":     func() string { return "" },
	"reverse":       func() string { return "" },
	"blink":         func() string { return "" },
	"dim":           func() string { return "" },
	"italic":        func() string { return "" },
	"strikethrough": func() string { return "" },
}

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
	tree, err := parse.Parse("status_bar.layout", layoutStr, "{{", "}}", allowedFuncs)
	if err != nil {
		return
	}

	root := tree["status_bar.layout"].Root
	if root == nil {
		err = errors.New("parse template error: nil root")
		return
	}

	var template string
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
						template += chunk
						if i < len(all[1:])-1 {
							template += "  "
						}
					}
				} else {
					ret[len(ret)-1].Template += all[0]
					for _, chunk := range all[1:] {
						template += "  "
						template += chunk
					}
				}
			} else {
				template += string(n.Text)
			}

		case *parse.ActionNode:
			fieldName, attrs, err := parseAction(n)
			if err != nil {
				return nil, err
			}

			var compType StatusBarComponentType
			switch fieldName {
			case "Status":
				compType = StatusBarStatus
				template += "%s"
			case "Filepath":
				compType = StatusBarFilePath
				template += "%s"
			case "GitShortRef":
				compType = StatusBarGitShortRef
				template += "%s"
			case "GitDiffAdd":
				compType = StatusBarGitDiffAdded
				template += "%d"
			case "GitDiffDel":
				compType = StatusBarGitDiffDeleted
				template += "%d"
			case "DiagError":
				compType = StatusBarDiagnosticsError
				template += "%d"
			case "DiagWarn":
				compType = StatusBarDiagnosticsWarning
				template += "%d"
			case "DiagInfo":
				compType = StatusBarDiagnosticsInfo
				template += "%d"
			case "Language":
				compType = StatusBarLanguage
				template += "%s"
			case "CursorColumn":
				compType = StatusBarCoordinatesCursorX
				template += "%d"
			case "CursorLine":
				compType = StatusBarCoordinatesCursorY
				template += "%d"
			case "TotalLines":
				compType = StatusBarTotalLines
				template += "%d"
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
				Template:   template,
				Attributes: attrs,
			})
			template = ""

		default:
			err = fmt.Errorf("unsupported node type %T at position %d", n, node.Position())
			return nil, err
		}
	}

	// Trailing text after the last component - append to last component's template
	// This handles cases like "{{ .Language }}  " where there's padding at the end
	if template != "" && len(ret) > 0 {
		ret[len(ret)-1].Template += template
	}

	return
}

func parseAction(n *parse.ActionNode) (string, term.Attributes, error) {
	if n.Pipe == nil || len(n.Pipe.Cmds) == 0 {
		return "", term.Attributes{}, fmt.Errorf("empty pipeline at position %d", n.Pos)
	}

	// first command must be the field reference: {{ .Status }}
	first := n.Pipe.Cmds[0]
	if len(first.Args) != 1 {
		return "", term.Attributes{}, fmt.Errorf("invalid field reference")
	}

	field, ok := first.Args[0].(*parse.FieldNode)
	if !ok || len(field.Ident) != 1 {
		return "", term.Attributes{}, fmt.Errorf("unsupported field expression")
	}

	fieldName := field.Ident[0]

	// remaining commands are attribute filters: {{ .Status | attr "bold" "red" }}
	var attrs term.Attributes
	for _, cmd := range n.Pipe.Cmds[1:] {
		if len(cmd.Args) == 0 {
			return "", term.Attributes{}, fmt.Errorf("empty function call in pipeline")
		}

		ident, ok := cmd.Args[0].(*parse.IdentifierNode)
		if !ok {
			return "", term.Attributes{}, fmt.Errorf("expected identifier in pipeline")
		}

		switch ident.Ident {
		case "bg":
			for _, arg := range cmd.Args[1:] {
				s, ok := arg.(*parse.StringNode)
				if !ok {
					return "", term.Attributes{}, fmt.Errorf("fg arguments must be a string")
				}
				var err error
				attrs.Bg, err = getColor(s.Text)
				if err != nil {
					return "", term.Attributes{}, err
				}
			}
			if len(cmd.Args[1:]) == 0 {
				return "", term.Attributes{}, errors.New("bg requires a color argument")
			}
		case "fg":
			for _, arg := range cmd.Args[1:] {
				s, ok := arg.(*parse.StringNode)
				if !ok {
					return "", term.Attributes{}, fmt.Errorf("bg arguments must be a string")
				}
				var err error
				attrs.Fg, err = getColor(s.Text)
				if err != nil {
					return "", term.Attributes{}, err
				}
			}
			if len(cmd.Args[1:]) == 0 {
				return "", term.Attributes{}, errors.New("fg requires a color argument")
			}
		case "bold":
			attrs.Attrs |= tcell.AttrBold
		case "underline":
			attrs.Attrs |= tcell.AttrUnderline
		case "reverse":
			attrs.Attrs |= tcell.AttrReverse
		case "blink":
			attrs.Attrs |= tcell.AttrBlink
		case "dim":
			attrs.Attrs |= tcell.AttrDim
		case "italic":
			attrs.Attrs |= tcell.AttrItalic
		case "strikethrough":
			attrs.Attrs |= tcell.AttrStrikeThrough
		default:
			return "", term.Attributes{}, fmt.Errorf("unsupported pipeline command %q", ident.Ident)
		}
	}
	return fieldName, attrs, nil
}

func getColor(name string) (tcell.Color, error) {
	if name == "default" {
		return tcell.ColorDefault, nil
	}
	if c, ok := tcell.ColorNames[name]; ok {
		return c, nil
	}
	if len(name) == 7 && name[0] == '#' {
		v, e := strconv.ParseInt(name[1:], 16, 32)
		if e != nil {
			return 0, errInvalidColor
		}
		return tcell.NewHexColor(int32(v)), nil
	}
	return 0, errInvalidColor
}
