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

package emacs

import "github.com/unstablebuild/rune-go-sdk/term"

// minibuffer is a small Emacs-style echo-area prompt. It reads a line of text
// from the user at the bottom of the editor while the handler routes every
// keystroke to it. It is deliberately minimal: no cursor movement, kill-ring
// or history. It backs go-to-line and the query-replace search/replacement
// prompts.
//
// The handler owns display: the minibuffer only exposes prompt() so the
// handler can render "<label><input>" through the less message bar and keeps
// the buffer cursor untouched, matching how Emacs leaves point in place while
// reading from the echo area.
type minibuffer struct {
	active bool
	label  string
	input  []rune
	// onSubmit fires when the user presses Enter; onCancel fires on C-g or
	// Esc. Exactly one runs per activation, after which the minibuffer is
	// inactive.
	onSubmit func(string)
	onCancel func()
}

func (m *minibuffer) start(label string, onSubmit func(string), onCancel func()) {
	m.active = true
	m.label = label
	m.input = m.input[:0]
	m.onSubmit = onSubmit
	m.onCancel = onCancel
}

func (m *minibuffer) reset() {
	m.active = false
	m.label = ""
	m.input = m.input[:0]
	m.onSubmit = nil
	m.onCancel = nil
}

// prompt returns the text to display in the echo area.
func (m *minibuffer) prompt() string {
	return m.label + string(m.input)
}

// handle consumes a key event while the minibuffer is active. It always
// reports handled=true so the key cannot leak to the buffer. submitted and
// canceled report which terminal transition (if any) occurred so the handler
// can run follow-up work after the callback.
func (m *minibuffer) handle(ev term.Event) (submitted, canceled bool) {
	if ev.Type != term.EventKey {
		return
	}
	if ev.Mod == term.ModCtrl && ev.Ch == 'g' {
		cb := m.onCancel
		m.reset()
		if cb != nil {
			cb()
		}
		return false, true
	}
	switch ev.Key {
	case term.KeyEsc:
		cb := m.onCancel
		m.reset()
		if cb != nil {
			cb()
		}
		return false, true
	case term.KeyEnter:
		text := string(m.input)
		cb := m.onSubmit
		m.reset()
		if cb != nil {
			cb(text)
		}
		return true, false
	case term.KeyBackspace:
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
		return
	case term.KeySpace:
		m.input = append(m.input, ' ')
		return
	default:
		if ev.Mod == 0 && ev.Ch != 0 {
			m.input = append(m.input, ev.Ch)
		}
		return
	}
}
