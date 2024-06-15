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
package handler

import (
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

// PromptConfig holds configuration for a Prompt.
type PromptConfig struct {
	component.PromptConfig

	OptionBindings []term.KeyComb
	OptionCallback func(i int, option string)
	HighlightAttr  term.Attributes
	OptionAttr     term.Attributes
}

var _ tui.Handler = (*Prompt)(nil)

// Prompt wraps a component.Prompt to satisfy tui.Handler.
type Prompt struct {
	component.Prompt

	hi       int
	cfg      PromptConfig
	bindings map[term.KeyComb]int
}

// NewPrompt allocates storage for a new Prompt and initializes it.
// See Init for more details.
func NewPrompt(cfg PromptConfig) (f *Prompt) {
	f = new(Prompt)
	f.Init(cfg)
	return
}

// Init initializes this Prompt with the given PromptConfig.
// Note that if OptionBindings is defined, it should be of the same
// length as Options.
func (f *Prompt) Init(cfg PromptConfig) {
	f.Prompt.Init(cfg.PromptConfig)
	f.hi = 0 // allow for Init to be used as reset

	if len(cfg.OptionBindings) != 0 &&
		len(cfg.OptionBindings) != len(cfg.Options) {
		panic("invalid OptionBindings; length should match of Options")
	}
	if cfg.OptionCallback == nil {
		cfg.OptionCallback = func(i int, option string) {}
	}

	if cfg.HighlightAttr == (term.Attributes{}) {
		cfg.HighlightAttr = term.Attributes{
			Bg:    cfg.OptionAttr.Bg,
			Fg:    cfg.OptionAttr.Fg,
			Attrs: cfg.OptionAttr.Attrs | tcell.AttrReverse,
		}
	}

	f.cfg = cfg
	f.bindings = make(map[term.KeyComb]int)
	for i, ev := range f.cfg.OptionBindings {
		f.bindings[ev] = i
	}

	// Prompt guarantees that there's at least one option
	f.highlightOption()
}

func (f *Prompt) highlightOption() {
	for j := range f.cfg.Options {
		f.Prompt.SetOptionAttr(j, f.cfg.OptionAttr)
	}
	f.Prompt.SetOptionAttr(f.hi, f.cfg.HighlightAttr)
}

// Handle satisfies tui.Handler.
func (f *Prompt) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey || ev.Mod != 0 {
		return
	}
	if i, ok := f.bindings[ev.KeyComb()]; ok {
		f.cfg.OptionCallback(i, f.cfg.Options[i])
		exit = true
		handled = true
		return
	}

	switch ev.Key {
	case term.KeyArrowLeft:
		if f.hi > 0 {
			f.hi--
			handled = true
			f.highlightOption()
		}
	case term.KeyArrowRight:
		if f.hi < len(f.cfg.Options)-1 {
			f.hi++
			handled = true
			f.highlightOption()
		}
	case term.KeyEnter:
		f.cfg.OptionCallback(f.hi, f.cfg.Options[f.hi])
		exit = true
		handled = true
	case term.KeyEsc:
		exit = true
		handled = true
	}
	return
}

// Cursor satisfies tui.Handler.
func (f *Prompt) Cursor() (pos term.Coordinates, style term.CursorStyle, show bool) {
	return
}

// Man satisfies tui.Component.
func (f *Prompt) Man() tui.Manual {
	panic("TODO")
}
