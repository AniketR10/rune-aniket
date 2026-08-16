// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package drawrect

import (
	"testing"

	ebiten "github.com/hajimehoshi/ebiten/v2"
	"github.com/stretchr/testify/assert"
)

// blendFactor resolves a BlendFactor the same way ebiten's internal
// driver does (see ebiten's blend.go): BlendFactorDefault means
// source-over, which is BlendFactorOne on the source side and
// BlendFactorOneMinusSourceAlpha on the destination side.
func blendFactor(f ebiten.BlendFactor, source bool, srcAlpha float64) float64 {
	switch f {
	case ebiten.BlendFactorDefault:
		if source {
			return 1
		}
		return 1 - srcAlpha
	case ebiten.BlendFactorZero:
		return 0
	case ebiten.BlendFactorOne:
		return 1
	case ebiten.BlendFactorOneMinusSourceAlpha:
		return 1 - srcAlpha
	default:
		panic("blendFactor: unsupported factor in test simulator")
	}
}

func blendOperation(op ebiten.BlendOperation, src, dst float64) float64 {
	switch op {
	case ebiten.BlendOperationAdd:
		return src + dst
	case ebiten.BlendOperationMax:
		return max(src, dst)
	default:
		panic("blendOperation: unsupported operation in test simulator")
	}
}

// compositeAlpha applies b's formula (as documented on ebiten.Blend) to
// the alpha channel only, given a premultiplied source alpha srcA over a
// destination alpha dstA.
func compositeAlpha(b ebiten.Blend, srcA, dstA float64) float64 {
	srcFactor := blendFactor(b.BlendFactorSourceAlpha, true, srcA)
	dstFactor := blendFactor(b.BlendFactorDestinationAlpha, false, srcA)
	return blendOperation(b.BlendOperationAlpha, srcFactor*srcA, dstFactor*dstA)
}

// TestDefaultDrawTrianglesOptionsAccumulateAlpha guards against the
// frame's alpha channel under-accumulating when a translucent rect
// (background fill or underline stroke) draws over the frame's own
// translucent background. Window opacity leaves that background alpha
// below 1 (dstA = bgOpacity), so compositing must follow standard
// source-over (srcA + (1-srcA)*dstA); resolving the alpha operation to
// Max instead under-reports alpha there, letting more of the desktop
// show through than the rect's own alpha intends.
func TestDefaultDrawTrianglesOptionsAccumulateAlpha(t *testing.T) {
	for _, tc := range []struct {
		name      string
		srcAlpha  float64
		bgOpacity float64
	}{
		{name: "translucent rect over half-opaque window", srcAlpha: 0.6, bgOpacity: 0.5},
		{name: "opaque rect over half-opaque window", srcAlpha: 1, bgOpacity: 0.5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.srcAlpha + (1-tc.srcAlpha)*tc.bgOpacity
			got := compositeAlpha(defaultDrawTrianglesOptions.Blend, tc.srcAlpha, tc.bgOpacity)
			assert.InDeltaf(t, want, got, 1e-9,
				"rect alpha should follow source-over, not clamp to max(src, dst)")
		})
	}
}
