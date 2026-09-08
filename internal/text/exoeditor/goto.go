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
