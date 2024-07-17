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
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/image/math/fixed"
)

var (
	handled = []rune{
		'░', '▒', '▓', '█',
		'─', '━', '╴', '╶', '╸', '╺',
		'│', '┃', '╵', '╷', '╹', '╻',
		'┌', '┍', '┎', '┏', '┐', '┑', '┒', '┓',
		'└', '┕', '┖', '┗', '┘', '┙', '┚', '┛',
		'├', '┝', '┞', '┟', '┠', '┡', '┢', '┣',
		'┤', '┥', '┦', '┧', '┨', '┩', '┪', '┫', '┬', '┭',
		'┮', '┯', '┰', '┱', '┲', '┳', '┴', '┵', '┶', '┷',
		'┸', '┹', '┺', '┻', '┼', '┽', '┾', '┿',
		'╀', '╁', '╂', '╃', '╄', '╅', '╆', '╇', '╈', '╉', '╊', '╋',
		'╼', '╽', '╾', '╿',
	}
	unhandled       = []rune{'a', 'b', 'A'}
	allRunes        = append(append([]rune{}, handled...), unhandled...)
	standardWidths  = []float64{0, 10}
	standardHeights = []float64{0, 10}
	standardDots    = []fixed.Point26_6{
		{X: fixed.I(-1)},
		{X: fixed.I(0)},
		{X: fixed.I(1)},
		{Y: fixed.I(-1)},
		{Y: fixed.I(0)},
		{Y: fixed.I(1)},
		{X: fixed.I(1), Y: fixed.I(-1)},
		{X: fixed.I(-1), Y: fixed.I(0)},
		{X: fixed.I(-1), Y: fixed.I(1)},
	}
	standardOffsets = []float64{-25, 0, 25}
)

func TestCustomFace(t *testing.T) {
	t.Run("none of the special characters cause a panic", func(t *testing.T) {
		for _, width := range standardWidths {
			for _, height := range standardWidths {
				for _, offsetY := range standardOffsets {
					f := newCustomFace(width, height, offsetY, &mockFace{}, false)
					for _, r := range handled {
						assert.NotPanics(t, func() {
							f.GlyphBounds(r)
						})
						assert.NotPanics(t, func() {
							f.GlyphAdvance(r)
						})
						assert.NotPanics(t, func() {
							f.Kern(r, 'a')
						})
						assert.NotPanics(t, func() {
							f.Metrics()
						})
						for _, dot := range standardDots {
							assert.NotPanics(t, func() {
								f.Glyph(dot, r)
							})
						}
					}
				}
			}
		}
	})
}
