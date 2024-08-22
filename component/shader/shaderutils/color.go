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
	"math"

	"github.com/unstablebuild/tcell/v3"
)

// InterpolateColor calculates a color that is a linear blend between the
// provided color and target color.
func InterpolateColor(factor float64, color, target tcell.Color) tcell.Color {
	r, g, b := color.RGB()
	tr, tg, tb := target.RGB()
	return tcell.NewRGBColor(
		int32(float64(r)+((float64(tr)-float64(r))*factor)),
		int32(float64(g)+((float64(tg)-float64(g))*factor)),
		int32(float64(b)+((float64(tb)-float64(b))*factor)),
	)
}

// ColorBrightness returns a normalized value 0..1 indicating the intensity of the color.
func ColorBrightness(color tcell.Color) float64 {
	r, g, b := color.RGB()
	return float64(r+g+b) / float64(255*3)
}

// SampleGradient interpolates a color from a gradient at a given position
// where factor=0 is the first color and factor=1 the last.
func SampleGradient(factor float64, gradient []tcell.Color) tcell.Color {
	t := math.Min(1, math.Max(0, factor))
	stops := float64(len(gradient) - 1)
	tt := t * stops

	currIdx := int(math.Floor(tt))
	nextIdx := currIdx + 1
	if nextIdx >= len(gradient) {
		nextIdx = len(gradient) - 1
	}

	tDec := tt - math.Floor(tt)

	col1 := gradient[currIdx]
	col2 := gradient[nextIdx]

	return InterpolateColor(tDec, col1, col2)
}
