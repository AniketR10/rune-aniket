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

package main

import (
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// listPicker is a floating handler that presents a list of labels and
// reports the selected index (or -1 on cancel) over a channel. It shares
// the keyboard model of codeActionPicker but decouples selection from any
// concrete item type so callers map the index back themselves.
type listPicker struct {
	labels         []string
	list           *component.FocusList
	ch             chan<- int
	idealW, idealH int
}

var _ browserapi.Floating = (*listPicker)(nil)

func newListPicker(labels []string, ch chan<- int) *listPicker {
	list := component.NewFocusList()
	maxW := 0
	for _, label := range labels {
		list.PushBack(component.NewResponsiveString(
			label, component.StringResponsiveConfig{},
		))
		if w := utf8.RuneCountInString(label); w > maxW {
			maxW = w
		}
	}
	return &listPicker{
		labels: labels,
		list:   list,
		ch:     ch,
		idealW: max(maxW, 1),
		idealH: min(max(len(labels), 1), maxPickerHeight),
	}
}

func (p *listPicker) Resize(width, height int) { p.list.Resize(width, height) }

func (p *listPicker) Draw(w term.Writer) { p.list.Draw(w) }

func (p *listPicker) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return false, false
	}
	switch ev.Key {
	case term.KeyEsc:
		p.ch <- -1
		return true, true
	case term.KeyEnter:
		idx := p.list.FocusOffset()
		if idx >= 0 && idx < len(p.labels) {
			p.ch <- idx
		} else {
			p.ch <- -1
		}
		return true, true
	case term.KeyArrowUp:
		p.list.FocusUp()
		return false, true
	case term.KeyArrowDown:
		p.list.FocusDown()
		return false, true
	}
	if ev.Mod == term.ModCtrl {
		switch ev.Ch {
		case 'j', 'n':
			p.list.FocusDown()
			return false, true
		case 'k', 'p':
			p.list.FocusUp()
			return false, true
		case 'c':
			p.ch <- -1
			return true, true
		}
	}
	return false, false
}

func (p *listPicker) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (p *listPicker) Selection() (string, bool) { return "", false }

func (p *listPicker) Close() error {
	select {
	case p.ch <- -1:
	default:
	}
	return nil
}

func (p *listPicker) Dimensions() (int, int) { return p.idealW, p.idealH }
