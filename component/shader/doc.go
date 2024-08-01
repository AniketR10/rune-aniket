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

/*
Package shader contains all you need to run effects on the characters,
foreground and background of Ox's TUI/GUI.

A shader takes a frame, the total number of frames and a [term.Cell] matrix and
simply writes to Ch (character), Fg (foreground color) or Bg (background color)
of that cell.

	func MyShader() shader.Shader {
		return &myShader{}
	}

	type myShader struct {}

	func (s *myShader) Shade(frame, total int, cells [][]term.Cell) {
		for y, row := range cells {
			for x, cell := range row {
				cells[y][x].Ch = ...
				cells[y][x].Fg = ...
				cells[y][x].Fg = ...
			}
		}
	}

If you want to do more complex visuals with maths like the shaders you see in
[Shadertoy] have a look at the [Writing Pixel Shaders Tutorial].

[Shadertoy]: https://www.shadertoy.com/
[Writing Pixel Shaders Tutorial]: https://x.unstable.build/docs/tutorials/ox/pixel_shader
*/
package shader
