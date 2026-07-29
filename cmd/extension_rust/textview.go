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
	"strings"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// textView is a floating, scrollable, read-only viewer for multi-line
// server output (syntax trees, HIR/MIR, macro expansions, ...). It reuses
// a FocusList so vertical scrolling comes for free; each line is one
// non-wrapping entry so wide output scrolls off-screen rather than
// reflowing, matching how rust-analyzer's text views are meant to be read.
type textView struct {
	list           *component.FocusList
	text           string
	idealW, idealH int
}

const (
	maxTextViewHeight = 30
	maxTextViewWidth  = 120
)

var _ browserapi.Floating = (*textView)(nil)

func newTextView(text string) *textView {
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	list := component.NewFocusList()
	maxW := 0
	for _, line := range lines {
		list.PushBack(component.NewResponsiveString(
			line, component.StringResponsiveConfig{},
		))
		if w := utf8.RuneCountInString(line); w > maxW {
			maxW = w
		}
	}
	return &textView{
		list:   list,
		text:   text,
		idealW: min(max(maxW, 1), maxTextViewWidth),
		idealH: min(max(len(lines), 1), maxTextViewHeight),
	}
}

func (v *textView) Resize(width, height int) { v.list.Resize(width, height) }

func (v *textView) Draw(w term.Writer) { v.list.Draw(w) }

func (v *textView) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return false, false
	}
	switch ev.Key {
	case term.KeyEsc, term.KeyEnter:
		return true, true
	case term.KeyArrowUp:
		v.list.FocusUp()
		return false, true
	case term.KeyArrowDown:
		v.list.FocusDown()
		return false, true
	}
	if ev.Mod == term.ModCtrl {
		switch ev.Ch {
		case 'j', 'n':
			v.list.FocusDown()
			return false, true
		case 'k', 'p':
			v.list.FocusUp()
			return false, true
		case 'c':
			return true, true
		}
	}
	return false, false
}

func (v *textView) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, term.CursorStyleDefault, false
}

func (v *textView) Selection() (string, bool) { return "", false }

func (v *textView) Close() error { return nil }

func (v *textView) Dimensions() (int, int) { return v.idealW, v.idealH }
