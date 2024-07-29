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
package timeshader

import (
	"math"

	"unstable.build/go-tui/component/shader"
)

// Decelerate the reverse of the accelerate quadratic t².
//
// It starts fast and slows down.
//
//	f(t) = 1 -(1 - t)²
//
// This causes the animation to start slowly and then speed up.
//
// ····························································xxxxxxxxxxxxxx
// ·······················································xxxxx··············
// ···················································xxxx···················
// ·················································xx·······················
// ··············································xxx·························
// ···········································xxx····························
// ········································xxx·······························
// ······································xx··································
// ····································xx····································
// ·································xxx······································
// ··········································································
// ······························xxx·········································
// ·····························x············································
// ···························xx·············································
// ·························xx···············································
// ························x·················································
// ······················xx··················································
// ·····················x····················································
// ···················xx·····················································
// ··················x·······················································
// ················xx························································
// ···············x··························································
// ·············xx···························································
// ··········································································
// ···········xx·····························································
// ··········x·······························································
// ·········x································································
// ········x·································································
// ·······x··································································
// ·····xx···································································
// ····x·····································································
// ···x······································································
// ··x·······································································
// ·x········································································
// x+++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++
func Decelerate(baseShader shader.Shader) shader.Shader {
	return decelerate(baseShader)
}

func decelerate(baseShader shader.Shader) *timeShader {
	return &timeShader{baseShader, funcTimeRemapper(
		func(frame, total int) int {
			dur := float64(total - 1)
			t := float64(frame) / dur
			tt := 1.0 - (t-1.0)*(t-1.0)
			ret := int(math.Round(dur * tt))
			return ret
		},
	)}
}
