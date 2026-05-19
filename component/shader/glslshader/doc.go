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
Package glslshader is a collection of shaders and utils that leverage graphics
math functions inspired by OpenGL API (used by GLSL).

It does not contain GPU code, but rather a client-side adaptation, hence don't
be misguided by the word GLSL.

Some of the shaders here will be pixel shaders (i.e. [glslshader.Blaze],
[glslshader.Inferno]), therefore we recommend reading out [Tutorial: Write a
Pixel Shader] if you don't know what a pixel shader is.

But also there are non-pixel shaders that are here because they simply use
GLSL-like functions (i.e. [glslshader.Flame], [glslshader.ProgressViz123]).

If you want to port from [Shadertoy] then you will likely want to satisfy the
cellRunner interface by defining the runCell() method on your shader struct and
calling shadeGLSL() from Shade() as illustrated below. This saves you from
performing any screen space transformation such as normalizing coords or
flipping vertical axis to match the pixel shader specification, it's all done
by shadeGLSL() and you only responsible for your runCell().

	func (s *trippy) Shade(frame, total int, in [][]term.Cell) {
		shadeGLSL(frame, total, s.fps, in, s) // only works if trippy.runCell defined
	}

	func (s *trippy) runCell(
		frame, total int, fps float, time float,
		fragCoordX, fragCoordY int,
		resolutionX, resolutionY int,
		inChar rune, inFg, inBg term.Color,
	) (char rune, fg, bg term.Color) {
		// generate your art based on those parameters or pass through by
	    // piping the input values to the out return values.
		char = inChar
		fg = inFg
		bg = inBg
	}

Check out [Tutorial: Write a Pixel Shader] for a complete guide on the topic.

# Design & Workflow

We recommend using [ShaderToy] or [KodeLife] for shader look development, and
when you are done port over the GLSL code into Go to avoid having to recompile
Go as you are exploring the parameter space or writing new shader code.

If you, Unstable Build developer, are given the task of updating the aesthetic
of a shader you don't need to write it directly in Go. For complex shaders we
store [KodeLife] project and the .glsl file under Unstable Build's
engineering/Shaders Google Drive folder. Either open up the [KodeLife] project
or adapt the .glsl code within [ShaderToy] for your development iteration
cycles, and when you are done translate to Go into the project.

[Shadertoy]: https://www.shadertoy.com
[KodeLife]: https://hexler.net/kodelife
[Tutorial: Write a Pixel Shader]: https://x.unstable.build/docs/tutorials/ox/pixel_shader
*/
package glslshader
