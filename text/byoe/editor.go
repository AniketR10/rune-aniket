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

// Package byoe implements a "bring-your-own-editor" text.Editor that
// hosts an external TUI editor (vim, neovim, helix, kakoune, …) inside
// a Rune-managed vte. Rune keeps owning the tab, the cell.Buffer
// (read-only mirror of disk), and IDE-wide commands; the external
// editor owns the editing UX and is the source of truth for buffer
// contents.
package byoe

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/ide/vctrl"
	"unstable.build/go-tui/ide/vctrl/vctrlcmd"
	"unstable.build/go-tui/term/vte"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/cmdenv"
	"unstable.build/go-tui/workspace"
)

// New allocates a new byoe Editor.
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
	reloader Reloader,
	env cmdenv.Source,
	overrideHighlights bool,
	registry text.WorkspaceCommandRegistry,
	vctrlSvc vctrl.Service,
	clip clipboard.Register,
) *Editor {
	switch {
	case command == "":
		panic("byoe.New: command is required")
	case scheduleNextTick == nil:
		panic("byoe.New: scheduleNextTick is required")
	case cwd == nil:
		panic("byoe.New: cwd is required")
	case notifications == nil:
		panic("byoe.New: notifications is required")
	case publisher == nil:
		panic("byoe.New: publisher is required")
	case terminal == nil:
		panic("byoe.New: terminal is required")
	case executor == nil:
		panic("byoe.New: executor is required")
	case tabManager == nil:
		panic("byoe.New: tabManager is required")
	case reloader == nil:
		panic("byoe.New: reloader is required")
	}
	tpl, err := parseGotoTemplate(gotoTemplate)
	if err != nil {
		panic("byoe.New: invalid gotoTemplate: " + err.Error())
	}
	ret := &Editor{
		command:            command,
		gotoTemplate:       tpl,
		scheduleNextTick:   scheduleNextTick,
		cwd:                cwd,
		workspaceURI:       workspaceURI,
		notifications:      notifications,
		publisher:          publisher,
		terminal:           terminal,
		executor:           executor,
		tabManager:         tabManager,
		vteCfg:             vteCfg,
		reloader:           reloader,
		env:                env,
		overrideHighlights: overrideHighlights,
		vctrlSvc:           vctrlSvc,
		clipboard:          clip,
	}
	if registry != nil {
		ret.fileRegistry = text.NewFileCommandRegistry(workspaceURI, registry)
	}
	ret.pub.Init()
	return ret
}

// Editor implements text.Editor. Its zero value is not usable; use
// the New constructor.
type Editor struct {
	command            string
	gotoTemplate       gotoTemplate
	scheduleNextTick   func(func()) bool
	cwd                workspace.Workspace
	workspaceURI       workspaceapi.URI
	notifications      browserapi.Notifications
	publisher          browser.EventPublisher
	terminal           schemeapi.Terminal
	executor           schemeapi.Executor
	tabManager         browser.TabManager
	vteCfg             vte.Config
	reloader           Reloader
	env                cmdenv.Source
	overrideHighlights bool
	fileRegistry       text.FileCommandRegistry
	vctrlSvc           vctrl.Service
	clipboard          clipboard.Register

	pub text.Publisher
}

// IsExternal reports true: byoe hosts an external TUI editor that
// owns the buffer contents.
func (e *Editor) IsExternal() bool { return true }

// Edit opens file in a fresh vte hosting the configured external
// editor. The returned handler reads/writes through the vte; Rune
// mirrors the on-disk file into buf via a workspace watcher.
func (e *Editor) Edit(
	ctx context.Context,
	file workspaceapi.URI, buf *cell.Buffer, readOnly, recovered bool,
) (text.Handler, error) {
	// Substitute {file}/{line}/{col} into the configured argv
	// template, then hand the resulting string to vte as a
	// single-element slice. vte.Component.createPty joins
	// CommandAndArgs with spaces and then runs shell.Fields on the
	// joined string to do POSIX-style tokenisation, so tokenising
	// here too would double-process the input (quoted segments
	// would lose their quotes and bare punctuation such as the
	// parentheses in `vim "+call cursor(1, 1)" {file}` would
	// re-tokenise as syntax errors). Pre-substituted-string in,
	// shell.Fields out, no double-tokenisation.
	cmdStr := substituteCommand(e.command, file.Path(), 1, 1)
	// Apply Rune-side shell-style expansion so $WORKSPACE,
	// $WORKSPACE_HASH, $FILE, $RUNE_DATADIR, … resolve before the
	// vte's own shell.Fields pass runs. Expansion failures (e.g.
	// command substitution rejected) propagate as a clear error
	// rather than silently producing a malformed argv.
	if expanded, expErr := cmdenv.Expand(ctx, cmdStr, e.env); expErr == nil {
		cmdStr = expanded
	} else {
		return nil, fmt.Errorf("byoe: expand command %q: %w", cmdStr, expErr)
	}

	cfg := e.vteCfg
	cfg.CommandAndArgs = []string{cmdStr}
	cfg.Modal = false
	cfg.ScheduleNextTick = e.scheduleNextTick
	// refreshProbe must see every grid mutation to keep vteprobe
	// in sync with the embedded editor's repaint cadence; opt out
	// of the publisher-side coalescing that terminal sessions use.
	cfg.DisablePerformanceInterrupt = true

	pub := newEventPublisher(e.publisher)
	vteH, err := vte.NewHandler(
		pub, e.notifications,
		e.terminal, e.executor, e.tabManager, cfg)
	if err != nil {
		return nil, fmt.Errorf("byoe: new vte handler: %w", err)
	}

	h := newHandler(vteH, buf, file, e.gotoTemplate,
		e.cwd, e.notifications, e.scheduleNextTick, e.reloader,
		e.overrideHighlights)
	pub.refresh = h.refreshProbe
	var ret text.Handler = h
	if e.fileRegistry != nil {
		var err error
		ret, err = text.SubscribeLocationCommands(file, e.fileRegistry, ret)
		if err != nil {
			return nil, fmt.Errorf("byoe: subscribe location commands: %w", err)
		}
		ret, err = vctrlcmd.SubscribeGitCommands(file, e.fileRegistry,
			ret, e.vctrlSvc, e.clipboard, e.notifications)
		if err != nil {
			return nil, fmt.Errorf("byoe: subscribe git commands: %w", err)
		}
	}
	return e.pub.PublishExternalEdit(file, buf, ret), nil
}

// SubscribeCommand returns an error: byoe does not host Rune-side
// editing commands.
func (e *Editor) SubscribeCommand(textapi.CommandManual, text.CommandHandler) error {
	return errors.New("not supported")
}

// RegisterREPLCommand returns an error: byoe does not host Rune-side
// editing commands.
func (e *Editor) RegisterREPLCommand(textapi.CommandManual, textapi.REPLHandler) error {
	return errors.New("not supported")
}

// REPLCommands returns nil.
func (e *Editor) REPLCommands() []textapi.CommandManual { return nil }

// UnsubscribeCommand returns an error: nothing was ever registered.
func (e *Editor) UnsubscribeCommand(string) error { return errors.New("not supported") }

// UnregisterREPLCommand returns an error: nothing was ever registered.
func (e *Editor) UnregisterREPLCommand(string) error { return errors.New("not supported") }

// Editor returns an error: byoe does not track multiple handlers.
func (e *Editor) Editor(workspaceapi.URI) (text.Handler, error) {
	return nil, errors.New("not supported")
}

// SubscribeEvents forwards subscriptions to the internal Publisher.
func (e *Editor) SubscribeEvents(
	evs []textapi.EventType, sub text.EventHandler,
) error {
	e.pub.SubscribeEvents(evs, sub)
	return nil
}

// UnsubscribeEvents forwards unsubscriptions to the internal Publisher.
func (e *Editor) UnsubscribeEvents(sub text.EventHandler) (bool, error) {
	return e.pub.UnsubscribeEvents(sub), nil
}

// substituteCommand expands {file}/{line}/{col} placeholders inside
// the argv template before shell tokenisation. line/col are 1-based.
func substituteCommand(tpl, file string, line, col int) string {
	repl := strings.NewReplacer(
		"{file}", file,
		"{line}", strconv.Itoa(line),
		"{col}", strconv.Itoa(col),
	)
	return repl.Replace(tpl)
}
