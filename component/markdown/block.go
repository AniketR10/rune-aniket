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

package markdown

import (
	"strings"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
)

// LinkInfo contains information about a link at a given position.
type LinkInfo struct {
	URL  string
	Text string
}

type block interface {
	Height(width int) int
	Resize(width, height int)
	Draw(w term.Writer)
	Dimensions() (width, height int)
	SpanAt(x, y int) (text, url string, ok bool)
	CharAt(x, y int) (rune, bool)
}

type inlineStyle uint8

const (
	styleNone          inlineStyle = 0
	styleBold          inlineStyle = 1 << iota
	styleItalic        inlineStyle = 1 << iota
	styleCode          inlineStyle = 1 << iota
	styleStrikethrough inlineStyle = 1 << iota
	styleLink          inlineStyle = 1 << iota
)

type span struct {
	text  string
	style inlineStyle
	url   string // populated when style includes styleLink
}

type textRun []span

func (tr textRun) String() string {
	var b strings.Builder
	for _, sp := range tr {
		b.WriteString(sp.text)
	}
	return b.String()
}

// Width returns the display width of the run in terminal columns.
func (tr textRun) Width() int {
	var n int
	for _, sp := range tr {
		n += textWidth(sp.text)
	}
	return n
}

// textWidth returns the number of columns text occupies when drawn.
// Zero-width clusters still take a cell, so they are counted as one to keep
// measurement, wrapping, drawing and hit-testing in agreement.
func textWidth(text string) int {
	var n int
	state := -1
	var width uint8
	for len(text) > 0 {
		// An ASCII byte whose successor is also ASCII (or end of string) is a
		// complete width-1 cluster: combining marks are never ASCII.
		if text[0] < utf8.RuneSelf &&
			(len(text) == 1 || text[1] < utf8.RuneSelf) {
			n++
			text = text[1:]
			state = -1
			continue
		}
		_, text, width, state = graphemecluster.StepString(text, state)
		n += max(1, int(width))
	}
	return n
}

func spanAtInLine(
	line textRun, x int,
) (text, url string, ok bool) {
	pos := 0
	for _, sp := range line {
		spWidth := textWidth(sp.text)
		if x >= pos && x < pos+spWidth {
			return sp.text, sp.url, true
		}
		pos += spWidth
	}
	return
}

func charAtInLine(line textRun, x int) (rune, bool) {
	pos := 0
	for _, sp := range line {
		text := sp.text
		state := -1
		var cluster string
		var width uint8
		for len(text) > 0 {
			cluster, text, width, state = graphemecluster.StepString(text, state)
			cols := max(1, int(width))
			if x >= pos && x < pos+cols {
				for _, r := range cluster {
					return r, true
				}
				return 0, false
			}
			pos += cols
		}
	}
	return 0, false
}

func resolveStyle(
	style inlineStyle, cfg *Config,
) term.Attributes {
	attr := cfg.Paragraph
	if style&styleBold != 0 {
		attr = cfg.Bold
	}
	if style&styleItalic != 0 {
		attr = cfg.Italic
	}
	if style&styleCode != 0 {
		attr = cfg.InlineCode
	}
	if style&styleStrikethrough != 0 {
		attr = cfg.Strikethrough
	}
	if style&styleLink != 0 {
		attr = cfg.Link
	}
	return attr
}
