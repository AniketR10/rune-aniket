// Copyright (C) 2017-2026 The Rune Authors
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

package vi

import (
	"unicode/utf8"

	sdkcomp "github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const (
	completionPadHorizontal = 2
	completionMaxHeight     = 15
)

// insertCompletionFloating is a floating handler for the word completion
// popup. It handles keyboard navigation and selection.
type insertCompletionFloating struct {
	labels   []string
	list     *sdkcomp.FocusList
	span     *sdkcomp.Span
	selected bool
	apply    func(string)
}

func newInsertCompletionFloating(
	candidates []string,
	focusIdx int,
	apply func(string),
) *insertCompletionFloating {
	list := sdkcomp.NewFocusList()
	list.SetFocusAttr(term.Attributes{Fg: term.ColorWhite})
	strCfg := sdkcomp.StringConfig{
		Attributes: term.Attributes{Fg: term.ColorGray},
	}
	for _, label := range candidates {
		list.PushBack(sdkcomp.NewStringWithConfig(label, strCfg))
	}
	for range focusIdx {
		list.FocusDown()
	}
	f := &insertCompletionFloating{
		labels: candidates,
		list:   list,
		apply:  apply,
	}
	inner := &completionInner{labels: candidates, list: list}
	f.span = sdkcomp.NewSpan(inner, sdkcomp.SpanConfig{
		PadHorizontal:    completionPadHorizontal,
		ContentAlignment: sdkcomp.AlignmentCentered,
	})
	return f
}

func (f *insertCompletionFloating) Handle(ev term.Event) (bool, bool) {
	exit, handled := f.handleKey(ev)
	if exit && f.selected {
		idx := f.list.FocusOffset()
		if idx < len(f.labels) && f.apply != nil {
			f.apply(f.labels[idx])
		}
	}
	return exit, handled
}

func (f *insertCompletionFloating) handleKey(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return false, false
	}
	switch ev.Key {
	case term.KeyEsc:
		return true, true
	case term.KeyEnter, term.KeyTab:
		f.selected = true
		return true, true
	case term.KeyArrowDown:
		f.list.FocusDown()
		return false, true
	case term.KeyArrowUp:
		f.list.FocusUp()
		return false, true
	}
	if ev.Mod == term.ModCtrl {
		switch ev.Ch {
		case 'j':
			f.list.FocusDown()
			return false, true
		case 'k':
			f.list.FocusUp()
			return false, true
		}
	}
	return false, false
}

func (f *insertCompletionFloating) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (f *insertCompletionFloating) Selection() (string, bool) { return "", false }

func (f *insertCompletionFloating) Draw(w term.Writer) { f.span.Draw(w) }

func (f *insertCompletionFloating) Resize(width, height int) { f.span.Resize(width, height) }

func (f *insertCompletionFloating) Dimensions() (int, int) { return f.span.Dimensions() }

func (f *insertCompletionFloating) Close() error { return nil }

// completionInner wraps the FocusList to satisfy component.Floating,
// which is required by Span.Dimensions.
type completionInner struct {
	labels []string
	list   *sdkcomp.FocusList
}

func (c *completionInner) Dimensions() (int, int) {
	maxW := 0
	for _, label := range c.labels {
		if n := utf8.RuneCountInString(label); n > maxW {
			maxW = n
		}
	}
	return maxW, min(len(c.labels), completionMaxHeight)
}

func (c *completionInner) Resize(width, height int) { c.list.Resize(width, height) }

func (c *completionInner) Draw(w term.Writer) { c.list.Draw(w) }
