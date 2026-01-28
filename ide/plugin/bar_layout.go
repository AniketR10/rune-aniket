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

package plugin

import (
	"errors"
	"fmt"
	"strings"
	"text/template/parse"

	"unstable.build/go-tui/component/template"
)

// ParseBarLayout parses the given layout string into a set of
// BarComponent. The expected format is Go templates.
func ParseBarLayout(layoutStr string) (
	ret []BarComponent, err error,
) {
	ret = nil
	tree, err := parse.Parse("status_bar.layout",
		layoutStr, "{{", "}}", template.AllowedFuncs())
	if err != nil {
		return
	}

	root := tree["status_bar.layout"].Root
	if root == nil {
		err = errors.New("parse template error: nil root")
		return
	}

	var tmpl string
	var alignRight, alignCenter bool
	for _, node := range root.Nodes {
		switch n := node.(type) {
		case *parse.TextNode:
			// suffix attributes
			if len(n.Text) != 0 && len(ret) > 0 {
				all := strings.Split(string(n.Text), "  ")
				if alignRight || alignCenter {
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

			var compType BarComponentType
			switch fieldName {
			case "Command":
				compType = BarCommand
				tmpl += "%s"
			case "StatusIcon":
				compType = BarStatusIcon
				tmpl += "%s"
			case "ExitStatus":
				compType = BarExitStatus
				tmpl += "%s"
			case "Elapsed":
				compType = BarElapsed
				tmpl += "%s"
			case "AlignCenter":
				alignRight = false
				alignCenter = true
				ret = append(ret, BarComponent{
					Type: BarAlignCenter,
				})
				continue
			case "AlignRight":
				alignCenter = false
				alignRight = true
				ret = append(ret, BarComponent{
					Type: BarAlignRight,
				})
				continue
			default:
				err = fmt.Errorf("unknown status bar component: %q", fieldName)
				return nil, err
			}
			ret = append(ret, BarComponent{
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
