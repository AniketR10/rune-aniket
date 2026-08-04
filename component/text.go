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

package component

import (
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
)

// WriteText draws text at (x, y), one grapheme cluster per cell, and returns
// the column following the last cluster written.
//
// Wide clusters are written as a single Width 2 cell and their continuation
// column is left untouched, which is what the renderer expects. A cluster that
// would cross maxX is not drawn, so wide clusters never straddle the boundary.
func WriteText(
	w term.Writer, x, y, maxX int, text string, attr term.Attributes,
) int {
	state := -1
	var cluster string
	var width uint8
	for len(text) > 0 {
		// An ASCII byte whose successor is also ASCII (or end of string) is a
		// complete width-1 cluster: combining marks are never ASCII. This skips
		// the grapheme state machine for the dominant case.
		if text[0] < utf8.RuneSelf &&
			(len(text) == 1 || text[1] < utf8.RuneSelf) {
			if x >= maxX {
				break
			}
			w.SetCell(term.Coordinates{X: x, Y: y},
				term.NewCell(rune(text[0]), 1, attr))
			x++
			text = text[1:]
			state = -1
			continue
		}
		cluster, text, width, state = graphemecluster.StepString(text, state)
		cols := max(1, int(width))
		if x+cols > maxX {
			break
		}
		if len(cluster) == 0 {
			continue
		}
		var cell term.Cell
		if len(cluster) == 1 { // single byte, avoids the []rune alloc
			cell = term.NewCell(rune(cluster[0]), uint8(cols), attr)
		} else {
			runes := []rune(cluster)
			cell = term.NewCell(runes[0], uint8(cols), attr)
			if len(runes) > 1 {
				cell.SetCombining(runes[1:])
			}
		}
		w.SetCell(term.Coordinates{X: x, Y: y}, cell)
		x += cols
	}
	return x
}
