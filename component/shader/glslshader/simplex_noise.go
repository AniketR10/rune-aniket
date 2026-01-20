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

// noiseSimplex returns spatial noise that is between -1.0 and 1.0
// [Noise - simplex - 2D by iq]: https://www.shadertoy.com/view/Msf3WH
func noiseSimplex(p vec2D) float {
	const (
		k1 = 0.366025404 // (sqrt(3)-1)/2
		k2 = 0.211324865 // (3-sqrt(3))/6
		k3 = 70.0
	)

	i := floor2D(p.addSc((p.x + p.y) * k1))
	a := p.sub(i).addSc((i.x + i.y) * k2)
	m := step(a.y, a.x)
	o := vec2(m, 1.0-m)
	b := a.sub(o).addSc(k2)
	c := a.subSc(1.0).addSc(2.0 * k2)
	h := max3D(vec3FromScalar(0.5).sub(vec3(dot2D(a, a), dot2D(b, b), dot2D(c, c))), vec3FromScalar(0.0))
	n := h.mult(h).mult(h).mult(h).mult(vec3(dot2D(a, hash(i.addSc(0.0))), dot2D(b, hash(i.add(o))), dot2D(c, hash(i.addSc(1.0)))))
	return dot3D(n, vec3FromScalar(k3))
}

// noiseSimplex returns spatial noise that is between 0.0 and 1.0
// (originally from -1.0 to 1.0 but this function remaps it)
// [Noise - simplex - 2D by iq]: https://www.shadertoy.com/view/Msf3WH
func noiseSimplex01(p vec2D) float {
	return 0.5 + 0.5*noiseSimplex(p)
}

func hash(p vec2D) vec2D {
	const (
		k1 = 127.1
		k2 = 311.7
		k3 = 269.5
		k4 = 183.3
		k5 = 43758.5453123
	)
	p = vec2(dot2D(p, vec2(k1, k2)), dot2D(p, vec2(k3, k4)))
	return vec2FromScalar(-1.0).add(vec2FromScalar(2.0).mult(fract2D(sin2D(p).multSc(k5))))
}
