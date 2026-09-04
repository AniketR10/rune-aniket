// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// listPicker is a floating handler that presents a list of labels and
// reports the selected index (or -1 on cancel) over a channel. It shares
// the keyboard model of lspcmd.CodeActionPicker but decouples selection
// from any concrete item type so callers map the index back themselves.
type listPicker struct {
	labels         []string
	list           *component.FocusList
	ch             chan<- int
	idealW, idealH int
}

const maxPickerHeight = 15

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
