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

package shaderutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/tcell/v3"
)

func TestInterpolateColor(t *testing.T) {
	tsuite := []struct {
		name      string
		colA      tcell.Color
		colB      tcell.Color
		factor    float64
		expectRGB []int32
	}{
		{
			name:      "begins with colA",
			colA:      tcell.NewRGBColor(0, 10, 100),
			colB:      tcell.NewRGBColor(10, 0, 200),
			factor:    0.0,
			expectRGB: []int32{0, 10, 100},
		},
		{
			name:      "ends with colA",
			colA:      tcell.NewRGBColor(0, 10, 100),
			colB:      tcell.NewRGBColor(10, 0, 200),
			factor:    1.0,
			expectRGB: []int32{10, 0, 200},
		},
		{
			name:      "linearly interpolates each channel",
			colA:      tcell.NewRGBColor(0, 10, 100),
			colB:      tcell.NewRGBColor(10, 0, 200),
			factor:    0.2,
			expectRGB: []int32{2, 8, 120},
		},
		{
			name:      "negative factor",
			colA:      tcell.NewRGBColor(0, 10, 100),
			colB:      tcell.NewRGBColor(10, 0, 200),
			factor:    -0.5,
			expectRGB: []int32{251, 15, 50},
		},
		{
			name:      "factor over 1",
			colA:      tcell.NewRGBColor(0, 10, 100),
			colB:      tcell.NewRGBColor(10, 0, 200),
			factor:    1.5,
			expectRGB: []int32{15, 251, 250},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			res := InterpolateColor(tcase.factor, tcase.colA, tcase.colB)
			r, g, b := res.RGB()
			assert.Equal(t, tcase.expectRGB, []int32{r, g, b})
		})
	}
}

func TestColorBrightness(t *testing.T) {
	tsuite := []struct {
		name   string
		col    tcell.Color
		expect float64
	}{
		{
			name:   "black",
			col:    tcell.NewRGBColor(0, 0, 0),
			expect: 0.0,
		},
		{
			name:   "white",
			col:    tcell.NewRGBColor(255, 255, 255),
			expect: 1.0,
		},
		{
			name:   "high red",
			col:    tcell.NewRGBColor(255, 10, 10),
			expect: 0.35947712418300654,
		},
		{
			name:   "high green",
			col:    tcell.NewRGBColor(10, 255, 10),
			expect: 0.35947712418300654,
		},
		{
			name:   "high blue",
			col:    tcell.NewRGBColor(10, 10, 255),
			expect: 0.35947712418300654,
		},
		{
			name:   "negative values should not panic",
			col:    tcell.NewRGBColor(-10, -10, -255),
			expect: 0.6444444444444445, // strange result but can stimulate creativity
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			res := ColorBrightness(tcase.col)
			assert.Equal(t, res, tcase.expect)
		})
	}
}
