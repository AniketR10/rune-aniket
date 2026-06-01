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
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/term/vte/vteprobe"
)

// drawLocations overlays loc.Attr on every cell of w covered by a
// location in locs, using probe.Bands/Rows to translate file
// coordinates back to screen coordinates. Locations that map to a
// folded row or to a row outside the content band are skipped.
//
// The projector mirrors text.DrawLocations but reads its layout from
// a vteprobe.Result rather than a component.Scroll because the exo
// editor does not own a Rune scroll: the embedded vte editor does.
func drawLocations(
	w term.Writer, locs []textapi.Location, probe *vteprobe.Result,
) {
	if probe == nil {
		return
	}
	bodyWidth := probe.Bands.GridWidth - probe.Bands.GutterWidth
	if bodyWidth <= 0 {
		return
	}
	for _, loc := range locs {
		drawLocation(w, loc, probe, bodyWidth)
	}
}

// drawLocation paints a single location across every terminal row in
// the content band that holds a slice of the location's [From, To]
// range. Cells outside the band, on folded rows, or to the right of
// the visible body are dropped.
func drawLocation(
	w term.Writer, loc textapi.Location, probe *vteprobe.Result,
	bodyWidth int,
) {
	fromY := loc.From.Y
	toY := loc.To.Y
	if toY < fromY {
		fromY, toY = toY, fromY
	}
	for y := probe.Bands.Top; y <= probe.Bands.Bottom && y < len(probe.Rows); y++ {
		rm := probe.Rows[y]
		if rm.FileLine == 0 || rm.Folded {
			continue
		}
		fileLine := rm.FileLine - 1
		if fileLine < fromY || fileLine > toY {
			continue
		}
		startRaw, endRaw := lineCellRange(loc, fileLine, fromY, toY)
		lineText := ""
		if fileLine >= 0 && fileLine < len(probe.FileLines) {
			lineText = probe.FileLines[fileLine]
		}
		startCol := vteprobe.RawToVisualCol(lineText, startRaw, probe.Tabstop)
		endCol := vteprobe.RawToVisualCol(lineText, endRaw, probe.Tabstop)
		segStart := rm.WrapOffset
		segEnd := segStart + bodyWidth
		if startCol < segStart {
			startCol = segStart
		}
		if endCol > segEnd {
			endCol = segEnd
		}
		for col := startCol; col < endCol; col++ {
			screenX := probe.Bands.GutterWidth + (col - rm.WrapOffset)
			if screenX < 0 || screenX >= probe.Bands.GridWidth {
				continue
			}
			w.UnionAttributes(
				term.Coordinates{X: screenX, Y: y}, loc.Attr)
		}
	}
}

// lineCellRange returns [start, end) cell columns covered by loc on
// fileLine, assuming the location spans file lines [fromY, toY]. For
// intermediate lines the range is the whole visible body; for the
// first/last line the range is clipped by loc.From.X / loc.To.X.
//
// Values are in raw rune columns (file coordinates), not visual
// columns: the caller converts them to visual columns once it knows
// which file line each row belongs to.
func lineCellRange(
	loc textapi.Location, fileLine, fromY, toY int,
) (start, end int) {
	const wholeLine = 1 << 30
	switch {
	case fileLine == fromY && fileLine == toY:
		return loc.From.X, loc.To.X
	case fileLine == fromY:
		return loc.From.X, wholeLine
	case fileLine == toY:
		return 0, loc.To.X
	default:
		return 0, wholeLine
	}
}
