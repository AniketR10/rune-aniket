// Copyright 2026 Unstable Build, LLC.
//
// This program is free software: you can redistribute it and/or modify it
// under the terms of the GNU General Public License as published by the
// Free Software Foundation, either version 3 of the License, or (at your
// option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// See <https://www.gnu.org/licenses/> for a copy of the license.

package markdown

import (
	"strings"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// LinkInfo contains information about a link at a given position.
type LinkInfo struct {
	URL  string
	Text string
}

type block interface {
	Height(width int) int
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

func (tr textRun) Len() int {
	var n int
	for _, sp := range tr {
		n += len(sp.text)
	}
	return n
}

func spanAtInLine(
	line textRun, x int,
) (text, url string, ok bool) {
	pos := 0
	for _, sp := range line {
		spLen := len(sp.text)
		if x >= pos && x < pos+spLen {
			return sp.text, sp.url, true
		}
		pos += spLen
	}
	return
}

func charAtInLine(line textRun, x int) (rune, bool) {
	pos := 0
	for _, sp := range line {
		for _, r := range sp.text {
			if pos == x {
				return r, true
			}
			pos++
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
