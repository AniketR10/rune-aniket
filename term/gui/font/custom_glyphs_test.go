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

package font

import (
	"fmt"
	"image"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPlusGlyphBounds(t *testing.T) {
	suite := []struct {
		description string
		// in
		bounds   image.Rectangle
		lhstroke int
		rhstroke int
		tvstroke int
		bvstroke int

		// expected
		xv     float64
		yh     float64
		xh     float64
		yv     float64
		lhsize float64
		rhsize float64
		tvsize float64
		bvsize float64
	}{
		{
			description: "full plus sign, bounds on 0, 0 coordinates origin, even size",
			bounds:      image.Rectangle{Max: image.Point{X: 6, Y: 8}},
			lhstroke:    1,
			rhstroke:    1,
			tvstroke:    1,
			bvstroke:    1,

			xv:     3,
			yh:     4,
			xh:     2,
			yv:     3,
			lhsize: 3,
			rhsize: 4,
			tvsize: 4,
			bvsize: 5,
		},
		{
			description: "bar sign, bounds on 0, 0 coordinates origin",
			bounds:      image.Rectangle{Max: image.Point{X: 6, Y: 8}},
			lhstroke:    0,
			rhstroke:    0,
			tvstroke:    1,
			bvstroke:    1,

			xv:     3,
			yh:     4,
			xh:     2,
			yv:     4,
			lhsize: 3,
			rhsize: 4,
			tvsize: 4,
			bvsize: 4,
		},
		{
			description: "top bar only sign, bounds on 0, 0 coordinates origin",
			bounds:      image.Rectangle{Max: image.Point{X: 6, Y: 8}},
			lhstroke:    0,
			rhstroke:    0,
			tvstroke:    1,
			bvstroke:    0,

			xv:     3,
			yh:     4,
			xh:     2,
			yv:     4,
			lhsize: 3,
			rhsize: 4,
			tvsize: 4,
			bvsize: 4,
		},
		{
			description: "horizontal bar sign, bounds on 0, 0 coordinates origin",
			bounds:      image.Rectangle{Max: image.Point{X: 6, Y: 8}},
			lhstroke:    0,
			rhstroke:    1,
			tvstroke:    1,
			bvstroke:    0,

			xv:     3,
			yh:     4,
			xh:     2,
			yv:     3,
			lhsize: 3,
			rhsize: 4,
			tvsize: 4,
			bvsize: 5,
		},
		{
			description: "full plus sign, bounds on 0, 0 coordinates origin, odd size",
			bounds:      image.Rectangle{Max: image.Point{X: 7, Y: 9}},
			lhstroke:    1,
			rhstroke:    1,
			tvstroke:    1,
			bvstroke:    1,

			xv:     3.5,
			yh:     4.5,
			xh:     3,
			yv:     4,
			lhsize: 4,
			rhsize: 4,
			tvsize: 5,
			bvsize: 5,
		},
		{
			description: "full plus sign, negative y bounds, odd size",
			bounds: image.Rectangle{
				Min: image.Point{X: 0, Y: -23},
				Max: image.Point{X: 11, Y: 0},
			},
			lhstroke: 1,
			rhstroke: 1,
			tvstroke: 1,
			bvstroke: 1,

			xv:     5.5,
			yh:     -11.5,
			xh:     5,
			yv:     -12,
			lhsize: 6,
			rhsize: 6,
			tvsize: 12,
			bvsize: 12,
		},
	}

	for i, test := range suite {
		t.Run(fmt.Sprintf("test number %d", i), func(t *testing.T) {
			xv, yh, xh, yv, lhsize, rhsize, tvsize, bvsize := plusGlyphBounds(
				test.bounds, test.lhstroke, test.rhstroke, test.tvstroke, test.bvstroke)

			assert.Equal(t, test.xv, xv, "xv")
			assert.Equal(t, test.yh, yh, "yh")
			assert.Equal(t, test.xh, xh, "xh")
			assert.Equal(t, test.yv, yv, "yv")

			assert.Equal(t, test.lhsize, lhsize, "lhsize")
			assert.Equal(t, test.rhsize, rhsize, "rhsize")
			assert.Equal(t, test.tvsize, tvsize, "tvsize")
			assert.Equal(t, test.bvsize, bvsize, "bvsize")
		})
	}
}
