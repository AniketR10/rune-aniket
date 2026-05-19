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

package locationsearch

import (
	"context"
	"io"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/debug"
)

// loadPreview parses line as a `path[:line[:col]]` location, opens
// the file via the workspace filesystem, and stores the cell matrix
// for drawing. When a parser is configured the syntax-highlighting
// is computed off the rendering goroutine and applied via schedule.
func (h *Handler) loadPreview(line string) {
	h.prevLine = line
	parsed := parseLocationLineDetailed(h.fs, line)
	if !parsed.ok {
		h.previewCells = nil
		h.highlightTargetLine = false
		return
	}
	uri := parsed.uri
	coords := parsed.coords
	f, err := h.fs.OpenFile(uri.Path(), os.O_RDONLY, 0)
	if err != nil {
		h.previewCells = nil
		h.highlightTargetLine = false
		return
	}
	defer f.Close() //nolint:errcheck
	data, err := io.ReadAll(f)
	if err != nil {
		h.previewCells = nil
		h.highlightTargetLine = false
		return
	}
	content := string(data)
	h.previewCells = term.StringToCells(content)
	h.targetLine = coords.Y
	h.targetStartX = coords.X
	h.targetEndX = coords.X
	h.highlightTargetLine = parsed.hasColumn
	if h.parser != nil && h.schedule != nil {
		baseCells := term.CloneCells(h.previewCells)
		fileURI := uri
		go debug.CapturePanicReport(func() {
			h.loadHighlights(fileURI, content, baseCells)
		})
	}
}

func (h *Handler) loadHighlights(
	uri workspaceapi.URI, content string, baseCells [][]term.Cell,
) {
	iter, err := h.parser.Highlight(uri, content)
	if err != nil {
		return
	}
	defer func() { _ = iter.Close() }()
	highlighted := term.CloneCells(baseCells)
	for {
		loc, ok := iter.Next(context.Background())
		if !ok {
			break
		}
		y := loc.From.Y
		if y < 0 || y >= len(highlighted) {
			continue
		}
		row := highlighted[y]
		for x := loc.From.X; x < loc.To.X && x < len(row); x++ {
			row[x].Attributes = loc.Attr
		}
	}
	if err := iter.Err(); err != nil {
		h.log.Warn("highlight iteration", "uri", uri, "err", err)
		return
	}
	h.schedule(func() {
		h.previewCells = highlighted
	})
}

func (h *Handler) drawPreview(w term.Writer) {
	if h.previewCells == nil {
		return
	}
	targetLine := h.targetLine
	if targetLine >= len(h.previewCells) {
		targetLine = len(h.previewCells) - 1
	}
	if targetLine < 0 {
		targetLine = 0
	}
	startLine := max(0, targetLine-h.previewH/2)
	if startLine+h.previewH > len(h.previewCells) {
		startLine = max(0, len(h.previewCells)-h.previewH)
	}
	// Match innerSpan's horizontal padding so the preview, separator
	// and inner finder share the same left/right margins.
	leftPad := spanHorizontalPad / 2
	rightPad := spanHorizontalPad - leftPad
	contentW := max(0, h.innerW-leftPad-rightPad)
	for row := range h.previewH {
		srcLine := startLine + row
		if srcLine >= len(h.previewCells) {
			break
		}
		isTarget := srcLine == targetLine
		cells := h.previewCells[srcLine]
		for x := range contentW {
			var cell term.Cell
			if x < len(cells) {
				cell = cells[x]
			} else {
				cell = term.Cell{Ch: ' ', Width: 1}
			}
			if isTarget && h.highlightTargetLine {
				cell.Attributes = term.AttributesUnion(cell.Attributes, h.cfg.PreviewAttr)
				if h.targetStartX < h.targetEndX &&
					x >= h.targetStartX && x < h.targetEndX {
					cell.Attrs |= term.AttrReverse
				}
			}
			w.SetCell(term.Coordinates{X: leftPad + x, Y: row}, cell)
		}
	}
}

func (h *Handler) drawSeparator(w term.Writer) {
	if h.cfg.SeparatorHeight <= 0 {
		return
	}
	ch := component.FrameCharSetDefault().HorizontalTop
	attr := term.Attributes{Fg: term.ColorGray}
	leftPad := spanHorizontalPad / 2
	rightPad := spanHorizontalPad - leftPad
	contentW := max(0, h.innerW-leftPad-rightPad)
	for sy := range h.cfg.SeparatorHeight {
		y := h.previewH + sy
		for x := range contentW {
			w.SetCell(term.Coordinates{X: leftPad + x, Y: y},
				term.Cell{Ch: ch, Width: 1, Attributes: attr})
		}
	}
}
