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
package component

import (
	"math"

	"unstable.build/go-tui/term"
)

// DefaultAspectRatio returns an aspect ratio that looks like 16:9 in squared pixels
// Note that cells are not square, so this ratio compensates for that.
const DefaultAspectRatio = 16.0 / 9.0 * 5 / 2

// FloatingResponsive wraps a Responsive component and satisfies Floating by
// using its Height method to find a width such that the resulting
// dimensions approximate a given aspect ratio.
type FloatingResponsive struct {
	Responsive
	aspectRatio float64
}

// NewFloatingResponsive allocates storage for a new FloatingResponsive and initializes it.
func NewFloatingResponsive(responsive Responsive, aspectRatio float64) *FloatingResponsive {
	ret := new(FloatingResponsive)
	ret.Init(responsive, aspectRatio)
	return ret
}

// Init initializes this FloatingResponsive with responsive and aspectRatio.
func (r *FloatingResponsive) Init(responsive Responsive, aspectRatio float64) {
	if aspectRatio == 0 {
		panic("aspect ratio cannot be 0")
	}
	r.Responsive = responsive
	r.aspectRatio = aspectRatio
}

var _ Floating = (*FloatingResponsive)(nil)
var _ Responsive = (*FloatingResponsive)(nil)
var _ WithAttributes = (*FloatingResponsive)(nil)

// SetAttr satisfies WithAttributes, if the underlying Responsive satisfies WithAttributes,
// otherwise this method panics.
func (r *FloatingResponsive) SetAttr(attr term.Attributes) term.Attributes {
	return r.Responsive.(WithAttributes).SetAttr(attr)
}

// Dimensions satisfies Floating by using the underlying component's
// Height to find the set of dimensions that respect the instructed
// aspect ratio.
func (r *FloatingResponsive) Dimensions() (width int, height int) {
	const (
		budget            = 10
		initialIncrements = 10
		biggerIncrements  = 50
	)
	var prevWidth, prevHeight int
	var prevActualLoss float64

	var i int
	iterate := func(increments int) {
		for i = 0; i < budget; i++ {
			width += increments * (i + 1)
			height = r.Responsive.Height(width)

			actualLoss, ok := r.calculateLoss(width, height)
			if ok {
				return
			}
			if actualLoss < 0 {
				// handle first iteration negative loss
				if i == 0 && increments == initialIncrements {
					prevWidth = width / 2
					prevHeight = height / 2
					prevActualLoss = 1
				}
				break
			}
			prevActualLoss = actualLoss
			prevWidth = width
			prevHeight = height
		}
	}

	// first try in small increments, if we don't get a negative actualLoss
	// then try in bigger increments. This is useful for components that need
	// hundreds of cells in width or height.
	iterate(initialIncrements)
	if i == budget {
		iterate(biggerIncrements)
		if i == budget {
			return
		}
	}

	prevActualLoss = math.Abs(prevActualLoss)
	// now that we now the inflection point, increments can be more precise
	increments := int(math.Max(1, (float64(width)-float64(prevWidth))/float64(budget)))
	width = prevWidth
	height = prevHeight
	for i := 0; i < budget; i++ {
		width += increments
		height = r.Responsive.Height(width)
		actualLoss, ok := r.calculateLoss(width, height)
		if ok {
			return
		}
		actualLoss = math.Abs(actualLoss)
		if actualLoss >= prevActualLoss {
			width = prevWidth
			height = prevHeight
			return
		}
		prevWidth = width
		prevHeight = height
		prevActualLoss = actualLoss
	}

	return
}

func (r *FloatingResponsive) calculateLoss(width, height int) (float64, bool) {
	const (
		acceptedLoss = 0.1
	)
	aspectRatio := float64(width) / float64(height)
	actualLoss := (r.aspectRatio - aspectRatio) / r.aspectRatio
	ok := (actualLoss > 0 && actualLoss < acceptedLoss) ||
		(actualLoss <= 0 && -actualLoss < acceptedLoss)
	return actualLoss, ok
}
