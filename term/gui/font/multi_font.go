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

	"github.com/ernestrc/go-multierror"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

var _ font.Face = (*multi)(nil)

type multi struct {
	fonts []font.Face
}

func newMultiFont(fonts ...font.Face) *multi {
	if len(fonts) == 0 {
		panic("multi font with no fonts")
	}
	return &multi{fonts: fonts}
}

func (m *multi) Close() (ret error) {
	for _, font := range m.fonts {
		if err := font.Close(); err != nil {
			ret = multierror.Append(ret, err)
		}
	}
	return
}

func (m *multi) Glyph(dot fixed.Point26_6, r rune) (
	dr image.Rectangle, mask image.Image,
	maskp image.Point, advance fixed.Int26_6, ok bool,
) {
	for _, font := range m.fonts {
		dr, mask, maskp, advance, ok = font.Glyph(dot, r)
		if ok {
			return
		}
	}
	return
}

func (m *multi) GlyphBounds(r rune) (
	bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool,
) {
	for _, font := range m.fonts {
		bounds, advance, ok = font.GlyphBounds(r)
		if ok {
			return
		}
	}
	return
}

func (m *multi) GlyphAdvance(r rune) (
	advance fixed.Int26_6, ok bool,
) {
	for _, font := range m.fonts {
		advance, ok = font.GlyphAdvance(r)
		if ok {
			return
		}
	}
	return
}

func (m *multi) Kern(r0, r1 rune) fixed.Int26_6 {
	for _, font := range m.fonts {
		// find the right font first
		_, _, ok := font.GlyphBounds(r0)
		if ok {
			return font.Kern(r0, r1)
		}
	}
	return fixed.I(0)
}

func (m *multi) Metrics() font.Metrics {
	// return the metrics of the leading font for overall metrics
	return m.fonts[0].Metrics()
}
