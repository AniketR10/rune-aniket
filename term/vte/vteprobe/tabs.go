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

package vteprobe

import "github.com/unstablebuild/rune-go-sdk/term/graphemecluster"

// expandTabs returns the visual rendering of line according to tabstop:
// tab characters are expanded to the next multiple of tabstop. The
// returned string contains only the visual cells; downstream code uses
// graphemecluster widths to map back to runes.
func expandTabs(line string, tabstop int) string {
	if tabstop <= 0 {
		tabstop = 1
	}
	if !containsTab(line) {
		return line
	}
	out := make([]rune, 0, len(line))
	col := 0
	for _, r := range line {
		if r != '\t' {
			out = append(out, r)
			col++
			continue
		}
		pad := tabstop - (col % tabstop)
		for k := 0; k < pad; k++ {
			out = append(out, ' ')
		}
		col += pad
	}
	return string(out)
}

func containsTab(line string) bool {
	for i := 0; i < len(line); i++ {
		if line[i] == '\t' {
			return true
		}
	}
	return false
}

// visualToRawCol converts a visual rune-cell offset on the expanded
// rendering of line (with tabstop) back to a 1-based rune column in the
// raw file content. A tab in the raw file always counts as a single
// rune column even though it expands to multiple visual cells.
func visualToRawCol(line string, runeOffset int, tabstop int) int {
	if tabstop <= 0 {
		tabstop = 1
	}
	if runeOffset <= 0 {
		return 1
	}
	visual := 0
	rawCol := 0
	state := -1
	remainder := line
	for len(remainder) > 0 {
		var cluster string
		var width uint8
		cluster, remainder, width, state = graphemecluster.StepString(remainder, state)
		if cluster == "" {
			break
		}
		rawCol++
		if cluster == "\t" {
			pad := tabstop - (visual % tabstop)
			if pad <= 0 {
				pad = tabstop
			}
			if visual+pad > runeOffset {
				return rawCol
			}
			visual += pad
			continue
		}
		// graphemecluster.StringWidth reports 0 for combining-only
		// clusters; treat as zero-width without advancing the visual
		// cursor but still consuming a rune column.
		visual += int(width)
		if visual > runeOffset {
			return rawCol
		}
	}
	// Past end of line: 1-based column equals rawCol+1 to allow cursor
	// past the last char (common in vim insert mode).
	return rawCol + 1
}
