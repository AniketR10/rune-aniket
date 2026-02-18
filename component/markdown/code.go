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
	"github.com/unstablebuild/tcell/v3"
)

type codeBlock struct {
	language string
	code     string
	cfg      *Config
	w        int // width from last Height call
}

var _ block = (*codeBlock)(nil)

func newCodeBlock(language, code string, cfg *Config) *codeBlock {
	return &codeBlock{language: language, code: code, cfg: cfg}
}

func (c *codeBlock) Height(width int) int {
	c.w = width
	if width <= 0 {
		return 0
	}
	lines := c.codeLines()
	return len(lines) + 1
}

func (c *codeBlock) Draw(w term.Writer) {
	if c.w <= 0 {
		return
	}

	lines := c.codeLines()

	if c.cfg.CodeBlock.Bg != tcell.ColorDefault {
		bgAttr := term.Attributes{Bg: c.cfg.CodeBlock.Bg}
		for y := range len(lines) {
			for x := range c.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}

	for lineIdx, line := range lines {
		x := 0
		for _, r := range line {
			if x >= c.w {
				break
			}
			w.SetCell(term.Coordinates{X: x, Y: lineIdx}, term.Cell{
				Ch:         r,
				Width:      1,
				Attributes: c.cfg.CodeBlock,
			})
			x++
		}
	}
}

func (c *codeBlock) Dimensions() (width, height int) {
	lines := c.codeLines()
	maxWidth := 0
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}
	return maxWidth, len(lines) + 1
}

func (c *codeBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if c.w <= 0 {
		return
	}
	lines := c.codeLines()
	if y < 0 || y >= len(lines) {
		return
	}
	line := lines[y]
	if x >= 0 && x < len(line) {
		return line, "", true
	}
	return
}

func (c *codeBlock) CharAt(x, y int) (rune, bool) {
	if c.w <= 0 {
		return 0, false
	}
	lines := c.codeLines()
	if y < 0 || y >= len(lines) {
		return 0, false
	}
	line := lines[y]
	if x >= 0 && x < len(line) {
		return rune(line[x]), true
	}
	return 0, false
}

func (c *codeBlock) codeLines() []string {
	lines := strings.Split(c.code, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
