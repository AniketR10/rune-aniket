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

package extension

import (
	"sync"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/rune/cmd/rune-agent/agent"
	"unstable.build/rune/cmd/rune-agent/dialogue/dialoguetui"
)

// syncComponent wraps a dialoguetui.Component with the mutex that guards
// every UI mutation. Helper methods (addStatusHint, removeStatusHint,
// setContextHint) take the lock so callers do not have to coordinate
// access manually.
type syncComponent struct {
	mu       *sync.Mutex
	comp     *dialoguetui.Component
	h        *aiEditorHandler
	hintSlot *hintSlot // shared across copies, protected by mu
}

// hintSlot stores the active hint component so that compactFn can
// re-add it after Reset + message replay during compaction.
type hintSlot struct {
	comp tui.Component
	conf component.SpanConfig
}

func (s syncComponent) addStatusHint() *statusHint {
	hint := newStatusHint(s.h.p, s.h.backgroundAttr, s.h.cfg.DurationPrecision, s.comp.TaskActiveForm)
	bg := component.WithBackground(hint, term.NewCell(' ', 1, s.h.backgroundAttr))
	conf := component.SpanConfig{
		ContentAlignment: component.AlignmentLeft,
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	s.comp.AddReceiveMessageHint(bg, conf)
	s.hintSlot.comp = bg
	s.hintSlot.conf = conf

	return hint
}

// completionOpen reports whether the chat's '#' completion band is
// showing.
func (s syncComponent) completionOpen() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.comp.CompletionOpen()
}

func (s syncComponent) removeStatusHint(hint *statusHint) {
	_ = hint.Close()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.comp.RemoveReceiveMessageHint()
	s.hintSlot.comp = nil
}

func (s syncComponent) setContextHint(ev agent.Event) {
	hint := &contextHint{
		segments: buildContextHintSegments(ev, s.h.contextHintCfg, s.h.cfg.DurationPrecision),
	}
	bg := component.WithBackground(hint, term.NewCell(' ', 1, s.h.backgroundAttr))
	s.mu.Lock()
	defer s.mu.Unlock()

	s.comp.AddReceiveMessageHint(bg, component.SpanConfig{
		ContentAlignment: component.AlignmentLeft,
	})
}
