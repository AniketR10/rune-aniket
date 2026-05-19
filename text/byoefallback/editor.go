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

package byoefallback

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/byoe"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/workspacessh"
)

// Editor hosts a byoe.Editor for URIs BYOE accepts (file://, ssh://)
// and a Rune-native fallback for everything else.
//
// Editor is not safe for concurrent use; all methods are expected to
// be called from the IDE's event-loop goroutine, like every other
// text.Editor implementation in this tree.
type Editor struct {
	byoe     text.Editor
	fallback text.Editor
	routes   map[string]text.Editor
}

// New constructs a byoe.Editor with the supplied arguments and wraps
// it with fallback. The byoe.* arguments are the same set
// byoe.New requires; the trailing fallback argument is the
// Rune-native editor to use for URIs BYOE cannot serve. Panics if
// fallback is nil; the byoe.New constructor panics for any missing
// BYOE dependency.
func New(
	command, gotoTemplate string,
	scheduleNextTick func(func()) bool,
	cwd workspace.Workspace,
	workspaceURI workspaceapi.URI,
	notifications browserapi.Notifications,
	publisher browser.EventPublisher,
	terminal schemeapi.Terminal,
	executor schemeapi.Executor,
	tabManager browser.TabManager,
	vteCfg vte.Config,
	reloader byoe.Reloader,
	fallback text.Editor,
) *Editor {
	return newWithEditors(
		byoe.New(
			command, gotoTemplate, scheduleNextTick,
			cwd, workspaceURI, notifications, publisher,
			terminal, executor, tabManager, vteCfg, reloader,
		),
		fallback,
	)
}

// newWithEditors is the dependency-injection seam used by tests. It
// is unexported because production code must go through New so the
// BYOE editor is the real one.
func newWithEditors(byoeEd, fallback text.Editor) *Editor {
	switch {
	case byoeEd == nil:
		panic("byoefallback: byoe editor is required")
	case fallback == nil:
		panic("byoefallback: fallback editor is required")
	}
	return &Editor{
		byoe:     byoeEd,
		fallback: fallback,
		routes:   make(map[string]text.Editor),
	}
}

// IsExternal always returns true: the IDE-level invariants (no
// auto-save, no reload notifications, read-only mirror buffers) must
// hold workspace-wide whenever editor.mode = "byoe".
func (*Editor) IsExternal() bool { return true }

// accepts reports whether uri's scheme can be served by BYOE.
func accepts(uri workspaceapi.URI) bool {
	switch uri.Scheme() {
	case workspace.FileScheme, workspacessh.Scheme:
		return true
	}
	return false
}

// Edit dispatches to byoe for BYOE-accepted URIs and to fallback
// otherwise. The choice is recorded so subsequent Editor(uri) lookups
// hit the same child.
func (e *Editor) Edit(
	ctx context.Context,
	file workspaceapi.URI, buf *cell.Buffer, readOnly, recovered bool,
) (text.Handler, error) {
	ed := e.pickEditor(file)
	h, err := ed.Edit(ctx, file, buf, readOnly, recovered)
	if err != nil {
		return nil, err
	}
	e.routes[file.String()] = ed
	return h, nil
}

// Editor returns the handler previously returned by Edit for uri. If
// uri has not been seen, dispatch follows the same accepts() rule as
// Edit.
func (e *Editor) Editor(uri workspaceapi.URI) (text.Handler, error) {
	ed, ok := e.routes[uri.String()]
	if !ok {
		ed = e.pickEditor(uri)
	}
	return ed.Editor(uri)
}

// pickEditor returns the byoe editor when uri's scheme is
// BYOE-accepted, else the fallback.
func (e *Editor) pickEditor(uri workspaceapi.URI) text.Editor {
	if accepts(uri) {
		return e.byoe
	}
	return e.fallback
}

// SubscribeEvents forwards to byoe only. The IDE wires its
// workspace-level subscribers (autosaver, FS reload, etc.) onto the
// editor.EventPublisher and BYOE is the source of truth for edit
// and save events in a BYOE workspace; the fallback only serves
// IDE-owned pseudo-URIs (memory://) whose events the IDE does not
// react to.
func (e *Editor) SubscribeEvents(
	evs []textapi.EventType, sub text.EventHandler,
) error {
	return e.byoe.SubscribeEvents(evs, sub)
}

// UnsubscribeEvents forwards to byoe only; see SubscribeEvents.
func (e *Editor) UnsubscribeEvents(sub text.EventHandler) (bool, error) {
	return e.byoe.UnsubscribeEvents(sub)
}

// SubscribeCommand forwards to the fallback only. BYOE returns
// "not supported" for IDE-wide command registration by design, so the
// fallback is the only meaningful target.
func (e *Editor) SubscribeCommand(
	man textapi.CommandManual, h text.CommandHandler,
) error {
	return e.fallback.SubscribeCommand(man, h)
}

// RegisterREPLCommand forwards to the fallback only. See
// SubscribeCommand for the rationale.
func (e *Editor) RegisterREPLCommand(
	man textapi.CommandManual, h textapi.REPLHandler,
) error {
	return e.fallback.RegisterREPLCommand(man, h)
}

// UnsubscribeCommand forwards to the fallback only.
func (e *Editor) UnsubscribeCommand(name string) error {
	return e.fallback.UnsubscribeCommand(name)
}

// UnregisterREPLCommand forwards to the fallback only.
func (e *Editor) UnregisterREPLCommand(name string) error {
	return e.fallback.UnregisterREPLCommand(name)
}

// compile-time check
var _ text.Editor = (*Editor)(nil)
