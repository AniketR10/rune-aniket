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
	"unstable.build/go-tui/component/shader"
)

// Reverse plays the animation starting from the last frame until the first.
//
//	xxx·······································································
//	···xx·····································································
//	·····xxx··································································
//	········x·································································
//	·········xx·······························································
//	···········xx·····························································
//	·············xxx··························································
//	················xx························································
//	··················xx······················································
//	····················xx····················································
//	······················xx··················································
//	························xx················································
//	··························xx··············································
//	····························xx············································
//	······························xxx·········································
//	·································xx·······································
//	···································xx·····································
//	·····································xx···································
//	·······································xx·································
//	·········································xx·······························
//	···········································xx·····························
//	·············································xx···························
//	···············································xxx························
//	··················································xx······················
//	····················································x·····················
//	·····················································xxx··················
//	························································xx················
//	··························································xx··············
//	····························································xx············
//	······························································xxx·········
//	·································································xx·······
//	···································································x······
//	····································································xx····
//	······································································xxx·
//	+++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++++x
func Reverse(baseShader shader.Shader) shader.Shader {
	return reverse(baseShader)
}

func reverse(baseShader shader.Shader) *timeShader {
	return &timeShader{baseShader, funcTimeRemapper(
		func(frame, total int) int {
			return total - frame - 1
		},
	)}
}
