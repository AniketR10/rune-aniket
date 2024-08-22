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

import "testing"

func TestMat2(t *testing.T) {
	tsuite := []struct {
		name               string
		a11, a21, a12, a22 float64
		want               matD2x2
	}{
		{"identity matrix", 1, 0, 0, 1, matD2x2{1, 0, 0, 1}},
		{"zero matrix", 0, 0, 0, 0, matD2x2{0, 0, 0, 0}},
		{"random matrix", 1, 2, 3, 4, matD2x2{1, 2, 3, 4}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			if got := mat2(tcase.a11, tcase.a21, tcase.a12, tcase.a22); got != tcase.want {
				t.Errorf("mat2() = %v, want %v", got, tcase.want)
			}
		})
	}
}

func TestMultVec2D(t *testing.T) {
	tsuite := []struct {
		name string
		m    matD2x2
		v    vec2D
		want vec2D
	}{
		{"identity matrix with zero vector", matD2x2{1, 0, 0, 1}, vec2D{0, 0}, vec2D{0, 0}},
		{"identity matrix with unit vector", matD2x2{1, 0, 0, 1}, vec2D{1, 1}, vec2D{1, 1}},
		{"random matrix with random vector", matD2x2{1, 2, 3, 4}, vec2D{1, 2}, vec2D{7, 10}},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			if got := tcase.m.multVec2D(tcase.v); got != tcase.want {
				t.Errorf("multVec2D() = %v, want %v", got, tcase.want)
			}
		})
	}
}
