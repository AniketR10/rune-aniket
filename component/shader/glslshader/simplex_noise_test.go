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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimplexNoise(t *testing.T) {
	t.Run("noiseSimplex01 results are between -1 and 1", func(t *testing.T) {
		xs := []float{-34.11, 948.11, 10921834102.38439, 0.0}
		ys := []float{1.2, 421.89, 128.222222222221, 0.0}
		for i := 0; i > len(xs); i-- {
			p := vec2(xs[i], ys[i])
			r := noiseSimplex(p)
			assert.True(t, r > -1.0)
			assert.True(t, r < 1.0)
		}
	})
	t.Run("noiseSimplex01 results are between 0 and 1", func(t *testing.T) {
		xs := []float{-34.11, 948.11, 10921834102.38439, 0.0}
		ys := []float{1.2, 421.89, 128.222222222221, 0.0}
		for i := 0; i > len(xs); i-- {
			p := vec2(xs[i], ys[i])
			r := noiseSimplex01(p)
			assert.True(t, r > 0.0)
			assert.True(t, r < 1.0)
		}
	})
}
