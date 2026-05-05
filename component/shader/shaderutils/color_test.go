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
		name                string
		colA                tcell.Color
		colB                tcell.Color
		resolveColorDefault tcell.Color
		factor              float64
		expectRGB           []int32
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
		{
			name:                "interpolating default color",
			colA:                tcell.ColorDefault,
			colB:                tcell.NewRGBColor(0, 0, 0),
			factor:              0.5,
			resolveColorDefault: tcell.NewRGBColor(200, 0, 0),
			expectRGB:           []int32{100, 0, 0},
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			res := InterpolateColor(
				tcase.factor, tcase.colA, tcase.colB, tcase.resolveColorDefault,
			)
			r, g, b := res.RGB()
			assert.Equal(t, tcase.expectRGB, []int32{r, g, b})
		})
	}
}

// TestInterpolateColorPreservesIdentityWhenColorAndTargetMatch ensures that
// blending a color with itself returns the exact same Color value (e.g. a
// named/palette index), not its RGB-converted equivalent. This matters for
// terminals that render named/palette colors via their themed palette but
// render literal RGB as TrueColor — converting would visibly shift the hue.
func TestInterpolateColorPreservesIdentityWhenColorAndTargetMatch(t *testing.T) {
	tsuite := []struct {
		name   string
		color  tcell.Color
		factor float64
	}{
		{name: "named color blue", color: tcell.ColorBlue, factor: 0.5},
		{name: "named color red at factor 0", color: tcell.ColorRed, factor: 0.0},
		{name: "named color green at factor 1", color: tcell.ColorGreen, factor: 1.0},
		{name: "default color preserved", color: tcell.ColorDefault, factor: 0.5},
		{name: "rgb color round-trips", color: tcell.NewRGBColor(12, 34, 56), factor: 0.7},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			res := InterpolateColor(
				tcase.factor, tcase.color, tcase.color, tcell.ColorDefault,
			)
			assert.Equal(t, tcase.color, res)
		})
	}
}

// TestInterpolateColorWithUnresolvableDefault guards against a class of bugs
// where blending a ColorDefault input with a target while passing
// ColorDefault as resolveColorDefault would compute lerps over (-1,-1,-1)
// (Color.RGB returns -1 for unresolved colors). The masking inside
// NewRGBColor turns those into garbage pseudo-colors (e.g. 24 for low
// factors, 255 for factor=0) producing visible "black cliffs" or flashes
// at band edges. Instead, when the color cannot be resolved, blending
// should be a no-op and the original ColorDefault must be preserved so the
// terminal keeps rendering it as the user's default text color.
func TestInterpolateColorWithUnresolvableDefault(t *testing.T) {
	tsuite := []struct {
		name   string
		factor float64
	}{
		{name: "factor 0", factor: 0.0},
		{name: "factor near 0 (would yield dark gray)", factor: 0.1},
		{name: "factor 0.5 (would yield mid gray)", factor: 0.5},
		{name: "factor 1", factor: 1.0},
	}
	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			res := InterpolateColor(
				tcase.factor,
				tcell.ColorDefault,
				tcell.NewRGBColor(255, 255, 255),
				tcell.ColorDefault, // also unresolvable
			)
			assert.Equal(t, tcell.ColorDefault, res)
		})
	}
}

func TestColorBrightness(t *testing.T) {
	tsuite := []struct {
		name                string
		col                 tcell.Color
		resolveColorDefault tcell.Color
		expect              float64
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
		{
			name:                "default color",
			col:                 tcell.ColorDefault,
			resolveColorDefault: tcell.NewRGBColor(255, 255, 255),
			expect:              1.0,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			res := ColorBrightness(tcase.col, tcase.resolveColorDefault)
			assert.Equal(t, res, tcase.expect)
		})
	}
}

func TestSampleGradient(t *testing.T) {
	tsuite := []struct {
		name     string
		factor   float64
		gradient []tcell.Color
		expect   tcell.Color
	}{
		{
			name:   "start",
			factor: 0.0,
			gradient: []tcell.Color{
				tcell.NewRGBColor(255, 0, 0),
				tcell.NewRGBColor(0, 255, 0),
				tcell.NewRGBColor(0, 0, 255),
			},
			expect: tcell.NewRGBColor(255, 0, 0),
		},
		{
			name:   "inbetween, first half",
			factor: 0.22,
			gradient: []tcell.Color{
				tcell.NewRGBColor(255, 0, 0),
				tcell.NewRGBColor(0, 255, 0),
				tcell.NewRGBColor(0, 0, 255),
			},
			expect: tcell.NewRGBColor(142, 112, 0),
		},
		{
			name:   "inbetween, middle",
			factor: 0.5,
			gradient: []tcell.Color{
				tcell.NewRGBColor(255, 0, 0),
				tcell.NewRGBColor(0, 255, 0),
				tcell.NewRGBColor(0, 0, 255),
			},
			expect: tcell.NewRGBColor(0, 255, 0),
		},
		{
			name:   "inbetween second half",
			factor: 0.864,
			gradient: []tcell.Color{
				tcell.NewRGBColor(255, 0, 0),
				tcell.NewRGBColor(0, 255, 0),
				tcell.NewRGBColor(0, 0, 255),
			},
			expect: tcell.NewRGBColor(0, 69, 185),
		},
		{
			name:   "end",
			factor: 0.0,
			gradient: []tcell.Color{
				tcell.NewRGBColor(255, 0, 0),
				tcell.NewRGBColor(0, 255, 0),
				tcell.NewRGBColor(0, 0, 255),
			},
			expect: tcell.NewRGBColor(255, 0, 0),
		},
		{
			name:   "beyond start",
			factor: -0.5,
			gradient: []tcell.Color{
				tcell.NewRGBColor(255, 0, 0),
				tcell.NewRGBColor(0, 255, 0),
				tcell.NewRGBColor(0, 0, 255),
			},
			expect: tcell.NewRGBColor(255, 0, 0),
		},
		{
			name:   "beyond end",
			factor: 1.5,
			gradient: []tcell.Color{
				tcell.NewRGBColor(255, 0, 0),
				tcell.NewRGBColor(0, 255, 0),
				tcell.NewRGBColor(0, 0, 255),
			},
			expect: tcell.NewRGBColor(0, 0, 255),
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			result := SampleGradient(tcase.factor, tcase.gradient)
			assert.Equal(t, tcase.expect, result)
		})
	}
}
