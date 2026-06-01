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

package exoeditor

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// gotoTemplate is a parsed editor.exo.goto template. Literal segments
// are pre-parsed into KeyComb slices and placeholders are kept as
// markers so Render can splice the rendered line/col digits in at
// runtime without re-parsing the template.
type gotoTemplate struct {
	segments []gotoTemplateSegment
}

type gotoTemplateSegment struct {
	literal     []term.KeyComb
	placeholder string // "line" or "col" when set, else empty
}

// parseGotoTemplate parses tpl into a renderable gotoTemplate. An
// empty template returns an empty template; Render of which is a nop.
func parseGotoTemplate(tpl string) (gotoTemplate, error) {
	if tpl == "" {
		return gotoTemplate{}, nil
	}
	var ret gotoTemplate
	parts := splitGotoTemplate(tpl)
	for _, p := range parts {
		if p.placeholder != "" {
			ret.segments = append(ret.segments, gotoTemplateSegment{
				placeholder: p.placeholder,
			})
			continue
		}
		if p.literal == "" {
			continue
		}
		keys, err := term.ParseKeys(p.literal)
		if err != nil {
			return gotoTemplate{}, fmt.Errorf(
				"segment %q: %w", p.literal, err)
		}
		ret.segments = append(ret.segments, gotoTemplateSegment{
			literal: keys,
		})
	}
	return ret, nil
}

// IsEmpty reports whether this template would render to no keystrokes.
func (g gotoTemplate) IsEmpty() bool {
	return len(g.segments) == 0
}

// Render returns the key sequence to inject for the given 1-based
// line/col coordinates.
func (g gotoTemplate) Render(line, col int) []term.KeyComb {
	var ret []term.KeyComb
	for _, seg := range g.segments {
		if seg.placeholder == "" {
			ret = append(ret, seg.literal...)
			continue
		}
		var n int
		switch seg.placeholder {
		case "line":
			n = line
		case "col":
			n = col
		}
		for _, r := range strconv.Itoa(n) {
			ret = append(ret, term.KeyComb{Ch: r})
		}
	}
	return ret
}

// splitGotoTemplate splits tpl into literal/placeholder pieces.
// Unknown {...} placeholders are returned as literal segments.
func splitGotoTemplate(tpl string) []rawGotoSegment {
	var out []rawGotoSegment
	for tpl != "" {
		lineIdx := strings.Index(tpl, "{line}")
		colIdx := strings.Index(tpl, "{col}")
		var idx int
		var name string
		var width int
		switch {
		case lineIdx == -1 && colIdx == -1:
			out = append(out, rawGotoSegment{literal: tpl})
			return out
		case lineIdx == -1:
			idx, name, width = colIdx, "col", len("{col}")
		case colIdx == -1:
			idx, name, width = lineIdx, "line", len("{line}")
		case lineIdx < colIdx:
			idx, name, width = lineIdx, "line", len("{line}")
		default:
			idx, name, width = colIdx, "col", len("{col}")
		}
		if idx > 0 {
			out = append(out, rawGotoSegment{literal: tpl[:idx]})
		}
		out = append(out, rawGotoSegment{placeholder: name})
		tpl = tpl[idx+width:]
	}
	return out
}

type rawGotoSegment struct {
	literal     string
	placeholder string
}
