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

// nolint: unused
package glslshader

import (
	"fmt"
	"math"
)

type vec2D struct {
	x, y float
}

func vec2(vals ...float) (v vec2D) {
	if len(vals) == 1 {
		vals = append(vals, vals[0])
	}
	if len(vals) != 2 {
		panic(fmt.Sprintf(
			"incorrent number of components (%d) for 2D vector", len(vals),
		))
	}
	v.x = vals[0]
	v.y = vals[1]
	return
}

func (v1 vec2D) add(v2 vec2D) (v vec2D) {
	v.x = v1.x + v2.x
	v.y = v1.y + v2.y
	return
}

func (v1 vec2D) addSc(sc float) (v vec2D) {
	v.x = v1.x + sc
	v.y = v1.y + sc
	return
}

func (v1 vec2D) sub(v2 vec2D) (v vec2D) {
	v.x = v1.x - v2.x
	v.y = v1.y - v2.y
	return
}

func (v1 vec2D) subSc(sc float) (v vec2D) {
	v.x = v1.x - sc
	v.y = v1.y - sc
	return
}

func (v1 vec2D) mult(v2 vec2D) (v vec2D) {
	v.x = v1.x * v2.x
	v.y = v1.y * v2.y
	return
}

func (v1 vec2D) multSc(sc float) (v vec2D) {
	v.x = v1.x * sc
	v.y = v1.y * sc
	return
}

func (v1 vec2D) div(v2 vec2D) (v vec2D) {
	v.x = v1.x / v2.x
	v.y = v1.y / v2.y
	return
}

func (v1 vec2D) divSc(sc float) (v vec2D) {
	v.x = v1.x / sc
	v.y = v1.y / sc
	return
}

func (v1 vec2D) floor() (v vec2D) {
	v.x = math.Floor(v1.x)
	v.y = math.Floor(v1.y)
	return
}

type vec3D struct {
	x, y, z float
}

func vec3(vals ...float) (v vec3D) {
	if len(vals) == 1 {
		vals = append(vals, vals[0])
		vals = append(vals, vals[0])
	}
	if len(vals) != 3 {
		panic(fmt.Sprintf(
			"incorrent number of components (%d) for 3D vector", len(vals),
		))
	}
	v.x = vals[0]
	v.y = vals[1]
	v.z = vals[2]
	return
}

func (v1 vec3D) add(v2 vec3D) (v vec3D) {
	v.x = v1.x + v2.x
	v.y = v1.y + v2.y
	v.z = v1.z + v2.z
	return
}

func (v1 vec3D) addSc(sc float) (v vec3D) {
	v.x = v1.x + sc
	v.y = v1.y + sc
	v.z = v1.z + sc
	return
}

func (v1 vec3D) sub(v2 vec3D) (v vec3D) {
	v.x = v1.x - v2.x
	v.y = v1.y - v2.y
	v.z = v1.z - v2.z
	return
}

func (v1 vec3D) subSc(sc float) (v vec3D) {
	v.x = v1.x - sc
	v.y = v1.y - sc
	v.z = v1.z - sc
	return
}

func (v1 vec3D) mult(v2 vec3D) (v vec3D) {
	v.x = v1.x * v2.x
	v.y = v1.y * v2.y
	v.z = v1.z * v2.z
	return
}

func (v1 vec3D) multSc(sc float) (v vec3D) {
	v.x = v1.x * sc
	v.y = v1.y * sc
	v.z = v1.z * sc
	return
}

func (v1 vec3D) div(v2 vec3D) (v vec3D) {
	v.x = v1.x / v2.x
	v.y = v1.y / v2.y
	v.z = v1.z / v2.z
	return
}

func (v1 vec3D) divSc(sc float) (v vec3D) {
	v.x = v1.x / sc
	v.y = v1.y / sc
	v.z = v1.z / sc
	return
}

func (v1 vec3D) floor() (v vec3D) {
	v.x = math.Floor(v1.x)
	v.y = math.Floor(v1.y)
	v.z = math.Floor(v1.z)
	return
}

type vec4D struct {
	x, y, z, w float
}

func vec4(vals ...float) (v vec4D) {
	if len(vals) == 1 {
		vals = append(vals, vals[0])
		vals = append(vals, vals[0])
		vals = append(vals, vals[0])
	}
	if len(vals) != 4 {
		panic(fmt.Sprintf(
			"incorrent number of components (%d) for 4D vector", len(vals),
		))
	}
	v.x = vals[0]
	v.y = vals[1]
	v.z = vals[2]
	v.w = vals[3]
	return
}

func (v1 vec4D) add(v2 vec4D) (v vec4D) {
	v.x = v1.x + v2.x
	v.y = v1.y + v2.y
	v.z = v1.z + v2.z
	v.w = v1.w + v2.w
	return
}

func (v1 vec4D) addSc(sc float) (v vec4D) {
	v.x = v1.x + sc
	v.y = v1.y + sc
	v.z = v1.z + sc
	v.w = v1.w + sc
	return
}

func (v1 vec4D) sub(v2 vec4D) (v vec4D) {
	v.x = v1.x - v2.x
	v.y = v1.y - v2.y
	v.z = v1.z - v2.z
	v.w = v1.w - v2.w
	return
}

func (v1 vec4D) subSc(sc float) (v vec4D) {
	v.x = v1.x - sc
	v.y = v1.y - sc
	v.z = v1.z - sc
	v.w = v1.w - sc
	return
}

func (v1 vec4D) mult(v2 vec4D) (v vec4D) {
	v.x = v1.x * v2.x
	v.y = v1.y * v2.y
	v.z = v1.z * v2.z
	v.w = v1.w * v2.w
	return
}

func (v1 vec4D) multSc(sc float) (v vec4D) {
	v.x = v1.x * sc
	v.y = v1.y * sc
	v.z = v1.z * sc
	v.w = v1.w * sc
	return
}

func (v1 vec4D) div(v2 vec4D) (v vec4D) {
	v.x = v1.x / v2.x
	v.y = v1.y / v2.y
	v.z = v1.z / v2.z
	v.w = v1.w / v2.w
	return
}

func (v1 vec4D) divSc(sc float) (v vec4D) {
	v.x = v1.x / sc
	v.y = v1.y / sc
	v.z = v1.z / sc
	v.w = v1.w / sc
	return
}

func dot2D(v1, v2 vec2D) float {
	return v1.x*v2.x + v1.y*v2.y
}

func dot3D(v1, v2 vec3D) float {
	return v1.x*v2.x + v1.y*v2.y + v1.z*v2.z
}

func length2D(v vec2D) float {
	return math.Sqrt(v.x*v.x + v.y*v.y)
}

func length3D(v vec3D) float {
	return math.Sqrt(v.x*v.x + v.y*v.y + v.z*v.z)
}

func length4D(v vec4D) float {
	return math.Sqrt(v.x*v.x + v.y*v.y + v.z*v.z + v.w*v.w)
}
