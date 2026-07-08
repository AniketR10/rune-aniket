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

package vi

import (
	"unicode/utf8"

	sdkcomp "github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	tcomponent "unstable.build/go-tui/component"
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

var _ tcomponent.WindowBarOptOut = (*insertCompletionFloating)(nil)

// NoWindowBar keeps the completion popup rendered with a plain frame
// instead of the floating window bar.
func (f *insertCompletionFloating) NoWindowBar() {}

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
