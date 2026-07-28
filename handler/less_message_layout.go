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

package handler

import (
	"errors"
	"fmt"
	"strings"
	"text/template/parse"

	"github.com/unstablebuild/rune-go-sdk/term"
	componenttemplate "unstable.build/go-tui/component/template"
)

// LessMessageLayout configures the message rendered in Less's command bar.
type LessMessageLayout struct {
	Template   string
	Attributes term.Attributes
}

// DefaultLessMessageLayout renders the message verbatim, styled with the
// command bar attributes.
func DefaultLessMessageLayout() LessMessageLayout {
	return LessMessageLayout{Template: "%s"}
}

// ParseLessMessageLayout parses a Go template containing one Message component.
func ParseLessMessageLayout(layout string) (LessMessageLayout, error) {
	tree, err := parse.Parse("less.message_bar.layout", layout, "{{", "}}",
		componenttemplate.AllowedFuncs())
	if err != nil {
		return LessMessageLayout{}, err
	}
	root := tree["less.message_bar.layout"].Root
	if root == nil {
		return LessMessageLayout{}, errors.New("parse template error: nil root")
	}

	var ret LessMessageLayout
	var found bool
	for _, node := range root.Nodes {
		switch n := node.(type) {
		case *parse.TextNode:
			ret.Template += string(n.Text)
		case *parse.ActionNode:
			field, attrs, err := componenttemplate.ParseAction(n)
			if err != nil {
				return LessMessageLayout{}, err
			}
			if field != "Message" {
				return LessMessageLayout{}, fmt.Errorf(
					"unknown less message bar component: %q", field)
			}
			if found {
				return LessMessageLayout{}, errors.New(
					"less message bar layout must contain exactly one .Message")
			}
			found = true
			ret.Template += "%s"
			ret.Attributes = attrs
		default:
			return LessMessageLayout{}, fmt.Errorf(
				"unsupported node type %T at position %d", n, node.Position())
		}
	}
	if !found {
		return LessMessageLayout{}, errors.New(
			"less message bar layout must contain .Message")
	}
	if strings.Count(ret.Template, "%s") != 1 {
		return LessMessageLayout{}, errors.New(
			"less message bar layout must contain exactly one .Message")
	}
	return ret, nil
}
