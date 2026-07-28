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

package markdown

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

type searchResult int

const (
	searchIgnored searchResult = iota
	searchConsumed
	searchConfirm
	searchCancel
)

// searchPrompt manages a less-style "/" search input bar.
type searchPrompt struct {
	active bool
	buf    []rune
}

func (p *searchPrompt) isActive() bool { return p.active }

func (p *searchPrompt) open() { p.active = true; p.buf = p.buf[:0] }

func (p *searchPrompt) query() string { return string(p.buf) }

func (p *searchPrompt) handleKey(ev term.Event) searchResult {
	if !p.active {
		return searchIgnored
	}

	switch ev.Key {
	case term.KeyEsc:
		p.active = false
		return searchCancel
	case term.KeyEnter:
		p.active = false
		return searchConfirm
	case term.KeyBackspace:
		if len(p.buf) > 0 {
			p.buf = p.buf[:len(p.buf)-1]
		}
		return searchConsumed
	}

	if ev.Ch != 0 {
		p.buf = append(p.buf, ev.Ch)
		return searchConsumed
	}

	return searchConsumed
}

func (p *searchPrompt) draw(w term.Writer, y, width int) {
	if !p.active || width <= 0 {
		return
	}

	for x := range width {
		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
			Ch: ' ', Width: 1,
		})
	}

	w.SetCell(term.Coordinates{X: 0, Y: y}, term.Cell{
		Ch: '/', Width: 1,
		Attributes: term.Attributes{Attrs: term.AttrBold},
	})

	for i, r := range p.buf {
		x := i + 1
		if x >= width {
			break
		}
		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{
			Ch: r, Width: 1,
		})
	}
}

func (p *searchPrompt) cursor(y int) (term.Coordinates, term.CursorStyle, bool) {
	if !p.active {
		return term.Coordinates{}, term.CursorStyleDefault, false
	}
	return term.Coordinates{X: len(p.buf) + 1, Y: y}, term.CursorStyleDefault, true
}
