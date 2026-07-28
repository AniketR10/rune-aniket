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

package extension

import (
	"fmt"
	"time"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cmd/rune-agent/agent"
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
