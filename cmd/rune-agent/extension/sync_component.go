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

package extension

import (
	"sync"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
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
