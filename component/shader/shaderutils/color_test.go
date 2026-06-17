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

	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestInterpolateColor(t *testing.T) {
	tsuite := []struct {
		name                string
		colA                term.Color
		colB                term.Color
		resolveColorDefault term.Color
		factor              float64
		expectRGB           []int32
	}{
		{
			name:      "begins with colA",
			colA:      term.NewRGBColor(0, 10, 100),
			colB:      term.NewRGBColor(10, 0, 200),
			factor:    0.0,
			expectRGB: []int32{0, 10, 100},
		},
		{
			name:      "ends with colA",
			colA:      term.NewRGBColor(0, 10, 100),
			colB:      term.NewRGBColor(10, 0, 200),
			factor:    1.0,
			expectRGB: []int32{10, 0, 200},
		},
		{
			name:      "linearly interpolates each channel",
			colA:      term.NewRGBColor(0, 10, 100),
			colB:      term.NewRGBColor(10, 0, 200),
			factor:    0.2,
			expectRGB: []int32{2, 8, 120},
		},
		{
			name:      "negative factor",
			colA:      term.NewRGBColor(0, 10, 100),
			colB:      term.NewRGBColor(10, 0, 200),
			factor:    -0.5,
			expectRGB: []int32{251, 15, 50},
		},
		{
			name:      "factor over 1",
			colA:      term.NewRGBColor(0, 10, 100),
			colB:      term.NewRGBColor(10, 0, 200),
			factor:    1.5,
			expectRGB: []int32{15, 251, 250},
		},
		{
			name:                "interpolating default color",
			colA:                term.ColorDefault,
			colB:                term.NewRGBColor(0, 0, 0),
			factor:              0.5,
			resolveColorDefault: term.NewRGBColor(200, 0, 0),
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
		color  term.Color
		factor float64
	}{
		{name: "named color blue", color: term.ColorBlue, factor: 0.5},
		{name: "named color red at factor 0", color: term.ColorRed, factor: 0.0},
		{name: "named color green at factor 1", color: term.ColorGreen, factor: 1.0},
		{name: "default color preserved", color: term.ColorDefault, factor: 0.5},
		{name: "rgb color round-trips", color: term.NewRGBColor(12, 34, 56), factor: 0.7},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			res := InterpolateColor(
				tcase.factor, tcase.color, tcase.color, term.ColorDefault,
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
				term.ColorDefault,
				term.NewRGBColor(255, 255, 255),
				term.ColorDefault, // also unresolvable
			)
			assert.Equal(t, term.ColorDefault, res)
		})
	}
}

func TestColorBrightness(t *testing.T) {
	tsuite := []struct {
		name                string
		col                 term.Color
		resolveColorDefault term.Color
		expect              float64
	}{
		{
			name:   "black",
			col:    term.NewRGBColor(0, 0, 0),
			expect: 0.0,
		},
		{
			name:   "white",
			col:    term.NewRGBColor(255, 255, 255),
			expect: 1.0,
		},
		{
			name:   "high red",
			col:    term.NewRGBColor(255, 10, 10),
			expect: 0.35947712418300654,
		},
		{
			name:   "high green",
			col:    term.NewRGBColor(10, 255, 10),
			expect: 0.35947712418300654,
		},
		{
			name:   "high blue",
			col:    term.NewRGBColor(10, 10, 255),
			expect: 0.35947712418300654,
		},
		{
			name:   "negative values should not panic",
			col:    term.NewRGBColor(-10, -10, -255),
			expect: 0.6444444444444445, // strange result but can stimulate creativity
		},
		{
			name:                "default color",
			col:                 term.ColorDefault,
			resolveColorDefault: term.NewRGBColor(255, 255, 255),
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
		gradient []term.Color
		expect   term.Color
	}{
		{
			name:   "start",
			factor: 0.0,
			gradient: []term.Color{
				term.NewRGBColor(255, 0, 0),
				term.NewRGBColor(0, 255, 0),
				term.NewRGBColor(0, 0, 255),
			},
			expect: term.NewRGBColor(255, 0, 0),
		},
		{
			name:   "inbetween, first half",
			factor: 0.22,
			gradient: []term.Color{
				term.NewRGBColor(255, 0, 0),
				term.NewRGBColor(0, 255, 0),
				term.NewRGBColor(0, 0, 255),
			},
			expect: term.NewRGBColor(142, 112, 0),
		},
		{
			name:   "inbetween, middle",
			factor: 0.5,
			gradient: []term.Color{
				term.NewRGBColor(255, 0, 0),
				term.NewRGBColor(0, 255, 0),
				term.NewRGBColor(0, 0, 255),
			},
			expect: term.NewRGBColor(0, 255, 0),
		},
		{
			name:   "inbetween second half",
			factor: 0.864,
			gradient: []term.Color{
				term.NewRGBColor(255, 0, 0),
				term.NewRGBColor(0, 255, 0),
				term.NewRGBColor(0, 0, 255),
			},
			expect: term.NewRGBColor(0, 69, 185),
		},
		{
			name:   "end",
			factor: 0.0,
			gradient: []term.Color{
				term.NewRGBColor(255, 0, 0),
				term.NewRGBColor(0, 255, 0),
				term.NewRGBColor(0, 0, 255),
			},
			expect: term.NewRGBColor(255, 0, 0),
		},
		{
			name:   "beyond start",
			factor: -0.5,
			gradient: []term.Color{
				term.NewRGBColor(255, 0, 0),
				term.NewRGBColor(0, 255, 0),
				term.NewRGBColor(0, 0, 255),
			},
			expect: term.NewRGBColor(255, 0, 0),
		},
		{
			name:   "beyond end",
			factor: 1.5,
			gradient: []term.Color{
				term.NewRGBColor(255, 0, 0),
				term.NewRGBColor(0, 255, 0),
				term.NewRGBColor(0, 0, 255),
			},
			expect: term.NewRGBColor(0, 0, 255),
		},
		{
			name:     "empty gradient",
			factor:   0.5,
			gradient: []term.Color{},
			expect:   term.ColorDefault,
		},
		{
			name:     "nil gradient",
			factor:   0.5,
			gradient: nil,
			expect:   term.ColorDefault,
		},
	}

	for _, tcase := range tsuite {
		t.Run(tcase.name, func(t *testing.T) {
			result := SampleGradient(tcase.factor, tcase.gradient)
			assert.Equal(t, tcase.expect, result)
		})
	}
}

func TestDesaturateColor(t *testing.T) {
	// amount=0: passthrough.
	c := term.NewRGBColor(200, 50, 10)
	assert.Equal(t, c, DesaturateColor(c, 0, term.ColorDefault))

	// amount=1: each cell collapses to its own luminance gray, not a
	// fixed shared gray. Cells with different RGB stay distinct.
	red := DesaturateColor(term.NewRGBColor(255, 0, 0), 1, term.ColorDefault)
	green := DesaturateColor(term.NewRGBColor(0, 255, 0), 1, term.ColorDefault)
	rr, rg, rb := red.RGB()
	gr, gg, gb := green.RGB()
	assert.Equal(t, rr, rg)
	assert.Equal(t, rr, rb)
	assert.Equal(t, gr, gg)
	assert.Equal(t, gr, gb)
	assert.NotEqual(t, rr, gr, "different sources should not collapse to the same gray")

	// Amount clamped above 1 behaves like amount=1.
	clamped := DesaturateColor(term.NewRGBColor(255, 0, 0), 2, term.ColorDefault)
	assert.Equal(t, red, clamped)

	// ColorDefault is resolved before computing luminance.
	resolved := DesaturateColor(term.ColorDefault, 1, term.NewRGBColor(255, 255, 255))
	r, g, b := resolved.RGB()
	assert.Equal(t, int32(255), r)
	assert.Equal(t, int32(255), g)
	assert.Equal(t, int32(255), b)

	// Unresolvable defaults are returned unchanged so the terminal keeps
	// rendering them natively.
	assert.Equal(t,
		term.ColorDefault,
		DesaturateColor(term.ColorDefault, 1, term.ColorDefault),
	)
}
