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

package term

import (
	"context"
	"math"

	"github.com/unstablebuild/rune-go-sdk/term"
)

type bwWriter struct {
	w term.Writer
	// defaultFg resolves a ColorDefault foreground before grayscaling so
	// unfocused body text tracks the theme's default foreground instead of
	// keeping its native hue.
	defaultFg term.Color
}

// BWWriter returns a Writer that converts each cell's foreground and
// background colors to grayscale, stripping all color while preserving
// relative brightness. A ColorDefault foreground is resolved to defaultFg
// before grayscaling; a ColorDefault background is left untouched so the
// terminal keeps rendering it natively.
func BWWriter(w term.Writer, defaultAttr term.Attributes) term.Writer {
	return bwWriter{w: w, defaultFg: defaultAttr.Fg}
}

func (w bwWriter) SetCell(pos term.Coordinates, c term.Cell) {
	c.Attributes.Fg = grayscaleFg(c.Attributes.Fg, w.defaultFg)
	c.Attributes.Bg = grayscale(c.Attributes.Bg)
	w.w.SetCell(pos, c)
}

func (w bwWriter) UnionAttributes(pos term.Coordinates, attr term.Attributes) {
	// A ColorDefault foreground in a union overlay means "leave the
	// foreground unchanged". Resolving it to defaultFg here would repaint
	// cells the caller meant to leave alone (e.g. the aux bar's
	// background-only overlay would overwrite the gray line numbers).
	attr.Fg = grayscale(attr.Fg)
	attr.Bg = grayscale(attr.Bg)
	w.w.UnionAttributes(pos, attr)
}

func (w bwWriter) Context() context.Context {
	return w.w.Context()
}

// grayscaleFg resolves a ColorDefault foreground to defaultFg before
// grayscaling so unfocused body text follows the theme default rather
// than keeping its native hue. It is only appropriate for SetCell, where
// every cell carries a concrete foreground; union overlays must leave a
// ColorDefault foreground untouched.
func grayscaleFg(c, defaultFg term.Color) term.Color {
	if c == term.ColorDefault {
		c = defaultFg
	}
	return grayscale(c)
}

// grayscale returns the ITU-R BT.601 luminance gray of c. Colors that
// are the terminal default or not RGB-expressible are returned unchanged
// so the terminal keeps rendering them natively.
func grayscale(c term.Color) term.Color {
	if c == term.ColorDefault {
		return c
	}
	if c.Hex() < 0 {
		return c
	}
	r, g, b := c.RGB()
	lum := int32(math.Round(0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)))
	return term.NewRGBColor(lum, lum, lum)
}
