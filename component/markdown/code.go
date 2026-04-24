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
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/debug"
	"unstable.build/go-tui/ide/idelsp/languages"
)

type codeBlock struct {
	cells  [][]term.Cell
	cfg    *Config
	w      int // width from last Height call
	cancel context.CancelFunc
}

var _ block = (*codeBlock)(nil)

func newCodeBlock(language, code string, cfg *Config) *codeBlock {
	cells := term.StringToCells(code)
	// Strip trailing empty line (goldmark includes trailing newline).
	if len(cells) > 0 && len(cells[len(cells)-1]) == 0 {
		cells = cells[:len(cells)-1]
	}

	// Apply base CodeBlock attributes to every cell.
	for y := range cells {
		for x := range cells[y] {
			cells[y][x].Attributes = cfg.CodeBlock
		}
	}

	cb := &codeBlock{cells: cells, cfg: cfg}

	if cfg.Parser == nil || language == "" {
		return cb
	}

	ctx, cancel := context.WithCancel(context.Background())
	cb.cancel = cancel

	parser := cfg.Parser
	codeBlock := cfg.CodeBlock
	schedule := cfg.ScheduleNextTick
	go debug.CapturePanicReport(func() {

		hls := collectHighlights(ctx, parser, language, code)
		if len(hls) == 0 {
			return
		}
		schedule(func() {
			applyHighlights(cb.cells, hls, codeBlock)
		})

	})

	return cb
}

// collectHighlights performs I/O by calling Parser.Highlight and
// collects all resulting locations into a slice. The context controls
// cancellation of the iterator consumption.
func collectHighlights(ctx context.Context, parser syntaxapi.Parser, language, code string) []textapi.Location {
	filename := languages.FilenameForLanguage(language)
	uri, err := workspaceapi.ParseURI("file:///" + filename)
	if err != nil {
		return nil
	}
	iter, err := parser.Highlight(uri, code)
	if err != nil {
		return nil
	}
	defer func() { _ = iter.Close() }()

	var hls []textapi.Location
	for hl, ok := iter.Next(ctx); ok; hl, ok = iter.Next(ctx) {
		hls = append(hls, hl)
	}
	return hls
}

func (c *codeBlock) close() {
	if c.cancel != nil {
		c.cancel()
	}
}

// applyHighlights writes highlight attributes into pre-built cells.
// Must only be called on the event-loop thread (or synchronously
// before the codeBlock is used).
func applyHighlights(cells [][]term.Cell, hls []textapi.Location, codeBlock term.Attributes) {
	for _, hl := range hls {
		for y := hl.From.Y; y <= hl.To.Y && y < len(cells); y++ {
			startX := 0
			if y == hl.From.Y {
				startX = hl.From.X
			}
			endX := len(cells[y])
			if y == hl.To.Y {
				endX = hl.To.X
			}
			for x := startX; x < endX && x < len(cells[y]); x++ {
				cells[y][x].Attributes = hl.Attr
				// Preserve the code block background color.
				if codeBlock.Bg != tcell.ColorDefault {
					cells[y][x].Attributes = term.Attributes(tcell.Style{
						Fg:    hl.Attr.Fg,
						Bg:    codeBlock.Bg,
						Attrs: hl.Attr.Attrs,
					})
				}
			}
		}
	}
}

func (c *codeBlock) Height(width int) int {
	if width <= 0 {
		return 0
	}
	lines := 0
	for _, row := range c.cells {
		lines += c.wrappedLineCount(row, width)
	}
	return lines + 1
}

func (c *codeBlock) Resize(width, _ int) {
	c.w = width
}

func (c *codeBlock) Draw(w term.Writer) {
	if c.w <= 0 {
		return
	}

	if c.cfg.CodeBlock.Bg != tcell.ColorDefault {
		bgAttr := term.Attributes{Bg: c.cfg.CodeBlock.Bg}
		for y := range c.Height(c.w) - 1 {
			for x := range c.w {
				w.UnionAttributes(term.Coordinates{X: x, Y: y}, bgAttr)
			}
		}
	}

	drawY := 0
	for _, row := range c.cells {
		for start := 0; start < len(row) || (len(row) == 0 && start == 0); start += c.w {
			for x := 0; x < c.w; x++ {
				srcX := start + x
				if srcX >= len(row) {
					break
				}
				w.SetCell(term.Coordinates{X: x, Y: drawY}, row[srcX])
			}
			drawY++
			if len(row) == 0 {
				break
			}
		}
	}
}

func (c *codeBlock) Dimensions() (width, height int) {
	return c.maxLineWidth(), len(c.cells) + 1
}

func (c *codeBlock) SpanAt(x, y int) (text, url string, ok bool) {
	if c.w <= 0 {
		return
	}
	row, _, ok := c.cellAt(x, y)
	if !ok {
		return
	}
	return cellsToString(row), "", true
}

func (c *codeBlock) CharAt(x, y int) (rune, bool) {
	if c.w <= 0 {
		return 0, false
	}
	_, cell, ok := c.cellAt(x, y)
	if ok {
		return cell.Ch, true
	}
	return 0, false
}

func (c *codeBlock) maxLineWidth() int {
	maxWidth := 0
	for _, row := range c.cells {
		if len(row) > maxWidth {
			maxWidth = len(row)
		}
	}
	return maxWidth
}

func (c *codeBlock) wrappedLineCount(row []term.Cell, width int) int {
	if width <= 0 {
		return 0
	}
	if len(row) == 0 {
		return 1
	}
	return (len(row)-1)/width + 1
}

func (c *codeBlock) cellAt(x, y int) ([]term.Cell, term.Cell, bool) {
	if x < 0 || x >= c.w || y < 0 {
		return nil, term.Cell{}, false
	}

	rowY := 0
	for _, row := range c.cells {
		wrapped := c.wrappedLineCount(row, c.w)
		if y < rowY+wrapped {
			srcX := (y-rowY)*c.w + x
			if srcX < 0 || srcX >= len(row) {
				return nil, term.Cell{}, false
			}
			return row, row[srcX], true
		}
		rowY += wrapped
	}

	return nil, term.Cell{}, false
}

func cellsToString(row []term.Cell) string {
	runes := make([]rune, len(row))
	for i, c := range row {
		runes[i] = c.Ch
	}
	return string(runes)
}
