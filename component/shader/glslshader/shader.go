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

package glslshader

import (
	"math"

	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component/asciiart"
	"unstable.build/go-tui/term"
)

type cellRunner interface {
	// runCell adapts the shader interface to be closer to pixel-shader.
	//
	// For instance Shadertoy's fragCoord would be fragCoordX and fragCoordY,
	// iResolution would be resolutionX and resolutionY and iTime would be
	// time (which is in seconds too).
	runCell(
		frame, total int, fps float, time float,
		fragCoordX, fragCoordY int,
		resolutionX, resolutionY int,
		inChar rune, inFg, inBg tcell.Color,
	) (char rune, fg, bg tcell.Color)
}

// shadeGLSL adapts the input space to look like common pixel shader's input
// interface.
//
// To make it look like a pixel shader the Y axis is flipped and terminal cell
// aspect is taken into account, since pixels are squared and cells aren't.
//
// Intended to be run by the Shade() method of any pixel shader.
func shadeGLSL(frame, total int, fps float, in [][]term.Cell, shader cellRunner) {
	if frame >= total {
		return
	}

	rows := len(in)
	cols := len(in[0])

	time := float(frame) / fps

	for y, row := range in {
		yFlipARCorrect := int(math.Round(float(rows-y-1) *
			asciiart.HeightToWidthCellAspectRatio))
		rowsFlipARCorrect := int(math.Round(float(rows) *
			asciiart.HeightToWidthCellAspectRatio))
		for x := range row {
			char, fg, bg := shader.runCell(
				frame, total, fps, time,
				x, yFlipARCorrect,
				cols, rowsFlipARCorrect,
				in[y][x].Ch, in[y][x].Fg, in[y][x].Bg,
			)
			in[y][x].Ch = char
			in[y][x].Fg = fg
			in[y][x].Bg = bg
		}
	}

}
