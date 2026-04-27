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

package command

import (
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
)

// EditDone classifies why an EditSession's Handle call ended modal
// edit mode (or did not).
type EditDone int

const (
	// EditDoneNone is the zero value: the event was forwarded to
	// the spawned editor handler and the session is still active.
	EditDoneNone EditDone = iota
	// EditDoneExit is returned when the user toggled edit mode off
	// via <shift-esc> (the configured exit key), <tab>, or
	// <ctrl-c>. Hosts typically commit the edited buffer back to
	// their own state and continue.
	EditDoneExit
	// EditDoneSubmit is returned when the user pressed <enter> in
	// edit mode. Hosts typically commit the edited buffer back to
	// their own state and then forward the original <enter> event
	// through their normal dispatch path so the input is acted
	// upon (e.g. dispatched as a command).
	EditDoneSubmit
)

// EditSession is a reusable modal editing harness on top of an
// Editor. It owns the lifecycle of the spawned EditHandler: Begin
// creates one bound to a caller-supplied cell.Buffer, Handle routes
// terminal events to it (intercepting a small set of "exit" keys
// upstream so the underlying editor never sees them), Resize and
// Cursor delegate to it, and End tears it down.
//
// EditSession does NOT know how to commit the edited buffer back to
// the host's state — that is intentionally host-specific (the
// command Prompt replays it through its own paste pipeline; an
// ideshell host would instead replace the inner inputbox's text).
// Hosts inspect Handle's EditDone return and perform that commit
// themselves before/after calling End.
type EditSession struct {
	editor  Editor
	exitKey term.KeyComb

	handler       EditHandler
	width, height int
}

// NewEditSession returns a session that spawns EditHandlers from
// editor and treats exitKey (in addition to <tab>, <ctrl-c>, and
// <enter>) as a modal-exit signal. editor must be non-nil.
//
// A zero-valued exitKey disables Begin: hosts should not offer the
// modal toggle in that configuration.
func NewEditSession(editor Editor, exitKey term.KeyComb) *EditSession {
	if editor == nil {
		panic("command: EditSession requires an Editor")
	}
	return &EditSession{editor: editor, exitKey: exitKey}
}

// Enabled reports whether Begin will produce a session. It returns
// false when no exit key is configured, i.e. the host should treat
// edit mode as turned off.
func (s *EditSession) Enabled() bool {
	return s.exitKey != (term.KeyComb{})
}

// Active reports whether a session is currently in progress.
func (s *EditSession) Active() bool { return s.handler != nil }

// Begin spawns a fresh EditHandler bound to buf, sizes it to the
// most recently observed dimensions, and seeds its cursor to the
// given buffer-relative coordinates so the user can keep typing
// from where the host's cursor was already pointing.
//
// Begin is a no-op when the session is already active or disabled.
func (s *EditSession) Begin(buf *cell.Buffer, cursor term.Coordinates) {
	if s.handler != nil || !s.Enabled() {
		return
	}
	h := s.editor.Edit(buf)
	h.Resize(s.width, s.height)
	h.SetCursorAtScroll(cursor)
	s.handler = h
}

// End tears down the active EditHandler (calling Close if it
// satisfies io.Closer). It is a no-op when the session is inactive.
func (s *EditSession) End() {
	if s.handler == nil {
		return
	}
	if c, ok := s.handler.(interface{ Close() error }); ok {
		_ = c.Close()
	}
	s.handler = nil
}

// Resize stores the dimensions and forwards them to the active
// handler. Hosts should call it from their own Resize so the
// editor stays in sync with the available area.
func (s *EditSession) Resize(width, height int) {
	s.width, s.height = width, height
	if s.handler != nil {
		s.handler.Resize(width, height)
	}
}

// Cursor returns the active handler's cursor; the third return is
// false when the session is inactive (so hosts can fall through to
// their own cursor logic).
func (s *EditSession) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	if s.handler == nil {
		return term.Coordinates{}, 0, false
	}
	return s.handler.Cursor()
}

// Drawer returns the active handler as a Draw-only view so hosts
// can composite the editor onto their own writer. The second return
// is false when the session is inactive.
func (s *EditSession) Drawer() (interface{ Draw(term.Writer) }, bool) {
	if s.handler == nil {
		return nil, false
	}
	return s.handler, true
}

// Handle routes ev to the active handler unless it matches one of
// the upstream-intercepted keys, in which case it returns the
// matching EditDone value without forwarding the event.
//
// Intercepted keys (only for term.EventKey events):
//   - exitKey configured at construction time → EditDoneExit
//   - <tab>                                   → EditDoneExit
//   - <ctrl-c>                                → EditDoneExit
//   - <enter>                                 → EditDoneSubmit
//
// Hosts should pattern-match the returned EditDone:
//
//	switch done {
//	case command.EditDoneExit:
//	    h.commitBuffer(); s.End()
//	case command.EditDoneSubmit:
//	    h.commitBuffer(); s.End(); h.dispatchEnter(ev)
//	}
//
// When the session is inactive Handle returns (EditDoneNone, false)
// so callers can short-circuit to their non-modal logic.
func (s *EditSession) Handle(ev term.Event) (done EditDone, handled bool) {
	if s.handler == nil {
		return EditDoneNone, false
	}
	if ev.Type == term.EventKey {
		switch {
		case s.exitKey != (term.KeyComb{}) && ev.KeyComb() == s.exitKey:
			return EditDoneExit, true
		case ev.Mod == 0 && ev.Key == term.KeyTab:
			return EditDoneExit, true
		case ev.Mod == term.ModCtrl && ev.Ch == 'c':
			return EditDoneExit, true
		case ev.Mod == 0 && ev.Key == term.KeyEnter:
			return EditDoneSubmit, true
		}
	}
	s.handler.Handle(ev)
	return EditDoneNone, true
}
