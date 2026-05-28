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

package shader

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Virtual returns a Shader that forwards only the cells inside
// (offset.X, offset.Y, width, height) to inner, presented as a
// (height x width) matrix with origin re-based to (0, 0). Cells
// outside the rectangle pass through unchanged. Negative offsets
// or zero-sized rectangles are no-ops.
func Virtual(inner Shader, offset term.Coordinates, width, height int) Shader {
	return &virtualShader{
		inner:  inner,
		offset: offset,
		width:  width,
		height: height,
	}
}

type virtualShader struct {
	inner  Shader
	offset term.Coordinates
	width  int
	height int
}

func (s *virtualShader) Shade(frame, total int, cells [][]term.Cell) {
	if s.width <= 0 || s.height <= 0 {
		return
	}
	if s.offset.Y >= len(cells) || s.offset.X < 0 || s.offset.Y < 0 {
		return
	}
	maxY := min(s.offset.Y+s.height, len(cells))
	if maxY <= s.offset.Y {
		return
	}
	view := make([][]term.Cell, 0, maxY-s.offset.Y)
	for y := s.offset.Y; y < maxY; y++ {
		row := cells[y]
		if s.offset.X >= len(row) {
			view = append(view, nil)
			continue
		}
		end := min(s.offset.X+s.width, len(row))
		view = append(view, row[s.offset.X:end])
	}
	s.inner.Shade(frame, total, view)
}