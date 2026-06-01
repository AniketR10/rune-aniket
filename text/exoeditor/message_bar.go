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

package exoeditor

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/text"
)

const maxMessageBarHeight = 20

type messageBar struct {
	text.Handler

	source    *editorHandler
	width     int
	height    int
	barHeight int
	lastMsg   string
}

func withMessageBar(inner text.Handler, source *editorHandler) text.Handler {
	return &messageBar{Handler: inner, source: source}
}

func (b *messageBar) Resize(width, height int) {
	b.width = width
	b.height = height
	b.lastMsg = b.source.LocationMessageAtCursor()
	b.barHeight = barHeightFor(b.lastMsg, width, height)
	b.Handler.Resize(width, height-b.barHeight)
}

func (b *messageBar) Draw(w term.Writer) {
	msg := b.source.LocationMessageAtCursor()
	want := barHeightFor(msg, b.width, b.height)
	if want != b.barHeight || msg != b.lastMsg {
		b.barHeight = want
		b.lastMsg = msg
		b.Handler.Resize(b.width, b.height-b.barHeight)
	}
	b.Handler.Draw(w)
	if b.barHeight == 0 {
		return
	}
	top := b.height - b.barHeight
	attrs := term.Attributes{Bg: term.ColorGray}
	x := 0
	y := top
	for _, r := range msg {
		if y >= b.height {
			break
		}
		w.SetCell(term.Coordinates{X: x, Y: y},
			term.Cell{Ch: r, Attributes: attrs})
		x++
		if x >= b.width {
			x = 0
			y++
		}
	}
	for ; y < b.height; y++ {
		for ; x < b.width; x++ {
			w.SetCell(term.Coordinates{X: x, Y: y},
				term.Cell{Ch: ' ', Attributes: attrs})
		}
		x = 0
	}
	offset := term.Attributes{Attrs: term.AttrVerticalRenderOffset}
	for y := top; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			w.UnionAttributes(term.Coordinates{X: x, Y: y}, offset)
		}
	}
}

func barHeightFor(msg string, width, height int) int {
	if msg == "" || width <= 0 || height <= 0 {
		return 0
	}
	runes := 0
	for range msg {
		runes++
	}
	rows := (runes + width - 1) / width
	rows = min(rows, maxMessageBarHeight, height)
	return rows
}
