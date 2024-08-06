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
	"image"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var _ font.Face = (*mockFace)(nil)

type mockFace struct {
	glyph   int
	bounds  int
	close   int
	advance int
	kern    int
	metrics int
}

func newMockFace() *mockFace {
	return &mockFace{}
}

func (m *mockFace) Close() (ret error) {
	m.close++
	return
}

func (m *mockFace) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image,
	maskp image.Point, advance fixed.Int26_6, ok bool,
) {
	m.glyph++
	mask = image.NewRGBA(image.Rectangle{})
	return
}

func (m *mockFace) GlyphBounds(r rune) (
	bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool,
) {
	m.bounds++
	return
}

func (m *mockFace) GlyphAdvance(r rune) (
	advance fixed.Int26_6, ok bool,
) {
	m.advance++
	return
}

func (m *mockFace) Kern(r0, r1 rune) fixed.Int26_6 {
	m.kern++
	return fixed.I(0)
}

func (m *mockFace) Metrics() font.Metrics {
	m.metrics++
	return font.Metrics{}
}
