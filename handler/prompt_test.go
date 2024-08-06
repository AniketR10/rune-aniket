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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/tcell/v3"
	"unstable.build/go-tui/component"
	"unstable.build/go-tui/term"
)

func TestPromptDefaults(t *testing.T) {
	t.Run("sets default callback", func(t *testing.T) {
		p := NewPrompt(PromptConfig{
			PromptConfig: component.PromptConfig{
				Message: "blah", Options: []string{"a"},
			}})
		exit, handled := p.Handle(term.Event{
			Type: term.EventKey, Key: term.KeyEnter})
		assert.True(t, exit)
		assert.True(t, handled)
	})
	t.Run("sets default highlight attrs", func(t *testing.T) {
		p := NewPrompt(PromptConfig{
			PromptConfig: component.PromptConfig{
				Message: "blah", Options: []string{"a"},
			}})
		attr := term.Attributes{Attrs: tcell.AttrReverse}
		assert.Equal(t, attr, p.cfg.HighlightAttr)
	})
}

func TestPromptHandle(t *testing.T) {
	opt0 := "Say what?"
	opt1 := "Yes"

	var calledI int
	var calledOpt string
	cfg := PromptConfig{
		PromptConfig: component.PromptConfig{
			Message: "What's for supper?",
			Options: []string{opt0, opt1},
			Frame:   component.FrameCharSetDefault(),
		},
		OptionCallback: func(i int, opt string) {
			calledI = i
			calledOpt = opt
		},
		OptionBindings: []term.KeyComb{
			{Ch: 'W'},
			{Ch: 'Y'},
		},
	}

	h := NewPrompt(cfg)

	resetStub := func() {
		calledI = -1
		calledOpt = "-1"
		h.Init(cfg)
	}
	resetStub()

	t.Run("enter after init calls first option", func(t *testing.T) {
		defer resetStub()

		exit, handled := h.Handle(term.Event{
			Type: term.EventKey, Key: term.KeyEnter})
		assert.True(t, exit)
		assert.True(t, handled)

		assert.Equal(t, 0, calledI)
		assert.Equal(t, opt0, calledOpt)
	})

	t.Run("arrow key right allow for moving right", func(t *testing.T) {
		defer resetStub()

		exit, handled := h.Handle(term.Event{
			Type: term.EventKey, Key: term.KeyArrowRight})
		assert.False(t, exit)
		assert.True(t, handled)

		exit, handled = h.Handle(term.Event{
			Type: term.EventKey, Key: term.KeyArrowRight})
		assert.False(t, exit)
		assert.False(t, handled)

		exit, handled = h.Handle(term.Event{
			Type: term.EventKey, Key: term.KeyEnter})
		assert.True(t, exit)
		assert.True(t, handled)

		assert.Equal(t, 1, calledI)
		assert.Equal(t, opt1, calledOpt)
	})

	t.Run("arrow key left allow for moving left", func(t *testing.T) {
		defer resetStub()

		exit, handled := h.Handle(term.Event{
			Type: term.EventKey, Key: term.KeyArrowRight})
		assert.False(t, exit)
		assert.True(t, handled)

		exit, handled = h.Handle(term.Event{
			Type: term.EventKey, Key: term.KeyArrowLeft})
		assert.False(t, exit)
		assert.True(t, handled)

		exit, handled = h.Handle(term.Event{
			Type: term.EventKey, Key: term.KeyArrowLeft})
		assert.False(t, exit)
		assert.False(t, handled)

		exit, handled = h.Handle(term.Event{
			Type: term.EventKey, Key: term.KeyEnter})
		assert.True(t, exit)
		assert.True(t, handled)

		assert.Equal(t, 0, calledI)
		assert.Equal(t, opt0, calledOpt)
	})

	t.Run("valid auto key binding no conflict", func(t *testing.T) {
		defer resetStub()

		exit, handled := h.Handle(term.Event{
			Type: term.EventKey, Ch: 'Y'})
		assert.True(t, exit)
		assert.True(t, handled)

		assert.Equal(t, 1, calledI)
		assert.Equal(t, opt1, calledOpt)
	})

	t.Run("invalid key binding", func(t *testing.T) {
		defer resetStub()

		exit, handled := h.Handle(term.Event{Type: term.EventKey, Ch: ' '})
		assert.False(t, exit)
		assert.False(t, handled)

		assert.Equal(t, -1, calledI)
		assert.Equal(t, "-1", calledOpt)
	})
}
