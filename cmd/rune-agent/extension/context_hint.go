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
	"fmt"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/cmd/rune-agent/agent"
)

// contextHintConfig holds styling for the post-turn context hint.
type contextHintConfig struct {
	labelAttr term.Attributes // separators and labels (·, context:, compacts at)
	valueAttr term.Attributes // numeric values (duration, token counts, percentages)
}

// contextHintSegment is a styled span of text within the context hint.
type contextHintSegment struct {
	text string
	attr term.Attributes
}

// contextHint is a static tui.Component that renders a one-line
// summary of context usage after a turn completes.
type contextHint struct {
	segments []contextHintSegment
	width    int
}

func (c *contextHint) Resize(width, height int) { c.width = width }

func (c *contextHint) Draw(w term.Writer) {
	x := 0
	for _, seg := range c.segments {
		for _, r := range seg.text {
			if x >= c.width {
				return
			}
			w.SetCell(term.Coordinates{X: x, Y: 0}, term.NewCell(r, 1, seg.attr))
			x++
		}
	}
}

// buildContextHintSegments produces styled segments for post-turn display.
func buildContextHintSegments(ev agent.Event, cfg contextHintConfig, durationPrecision time.Duration) []contextHintSegment {
	u := ev.Usage
	dur := u.TotalDuration
	if durationPrecision > 0 {
		dur = dur.Truncate(durationPrecision)
	} else {
		dur = dur.Truncate(time.Second)
	}
	label := cfg.labelAttr
	value := cfg.valueAttr

	segs := []contextHintSegment{
		{text: formatStatusDuration(dur), attr: value},
		{text: " · ", attr: label},
		{text: formatTokenCount(u.TokensSent), attr: value},
		{text: " sent", attr: label},
	}

	if u.TokensCached > 0 {
		hitPct := float64(u.TokensCached) / float64(u.TokensSent) * 100
		segs = append(segs,
			contextHintSegment{text: " · cache: ", attr: label},
			contextHintSegment{text: fmt.Sprintf("%.0f%%", hitPct), attr: value},
			contextHintSegment{text: fmt.Sprintf(" (%s/%s)", formatTokenNumber(u.TokensCached), formatTokenNumber(u.TokensSent)), attr: label},
		)
	}

	if ev.Context.Window > 0 {
		contextTokens := ev.Context.TokensSent + ev.Context.TokensReceived
		pct := float64(contextTokens) / float64(ev.Context.Window) * 100
		segs = append(segs,
			contextHintSegment{text: " · context: ", attr: label},
			contextHintSegment{text: formatTokenNumber(contextTokens), attr: value},
			contextHintSegment{text: fmt.Sprintf(" (%.0f%%)", pct), attr: value},
		)
		if ev.Context.AutoCompactAt > 0 {
			segs = append(segs,
				contextHintSegment{text: " · compacts at ", attr: label},
				contextHintSegment{text: formatTokenNumber(ev.Context.AutoCompactAt), attr: value},
			)
		}
	}

	return segs
}
