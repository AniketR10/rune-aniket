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

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/ide"
	"unstable.build/go-tui/term/gui/appmenu"
	"unstable.build/go-tui/term/gui/openpanel"
)

// appMenuCommands are the Rune commands reachable from the menu bar.
// Commands that exit the IDE must keep a key binding in every shipped
// preset: menu activation replays the bound chord, and the exit signal
// is only observable on the handler event path.
var appMenuCommands = struct {
	settings, quit []string
}{
	settings: []string{"config"},
	quit:     []string{"quit"},
}

// appMenuPanelCommands maps menu commands whose single argument is a
// path to the native open panel that collects it. Menu activation for
// these commands always shows the panel, even when the command is
// bound to a chord; the keyboard prompt flow stays reachable through
// the bindings themselves.
var appMenuPanelCommands = map[string]openpanel.Options{
	"edit":          {Multiple: true},
	"view":          {Multiple: true, Message: "Choose files to open read-only"},
	"workspaceopen": {Directories: true, Message: "Choose a project folder to open"},
}

// showOpenPanel is a seam over openpanel.Show for tests.
var showOpenPanel = openpanel.Show

// bootstrapQuitChord is the fallback for the quit accelerator. Before
// the wizard writes an editor preset the config binds nothing, and this
// is the chord the wizard itself recognizes as a clean exit.
var bootstrapQuitChord = term.KeyComb{Mod: term.ModMeta, Ch: 'q'}

// appMenuKeyBindings resolves the chords backing the menu's command
// items from the live configuration.
func appMenuKeyBindings(cfg config.Config) map[string]term.KeyComb {
	bindings := ide.CommandKeyBindings(cfg)
	if bindings == nil {
		bindings = map[string]term.KeyComb{}
	}
	quit := strings.Join(appMenuCommands.quit, " ")
	if _, ok := bindings[quit]; !ok {
		bindings[quit] = bootstrapQuitChord
	}
	return bindings
}

// quitEvent is the key event a window close request is routed to, so
// the red button and the Dock's Quit reach the same confirm-exit flow
// as the chord itself.
func quitEvent(bindings map[string]term.KeyComb) term.Event {
	key := bindings[strings.Join(appMenuCommands.quit, " ")]
	return term.Event{Type: term.EventKey, Mod: key.Mod, Key: key.Key, Ch: key.Ch}
}

// appMenus is the built-in macOS menu bar, covering the cheatsheet's
// command inventory. Command accelerators are derived from the live key
// bindings rather than hardcoded, so the menu never drifts from the
// user's configuration. Native items carry an accelerator only when the
// chord is unbound in every shipped preset: AppKit intercepts menu key
// equivalents before they reach the window, so a colliding one would
// shadow the preset binding.
func appMenus(bindings map[string]term.KeyComb) []appmenu.Menu {
	cmd := func(title string, cmdAndArgs ...string) appmenu.Command {
		return appmenu.Command{
			Title:   title,
			Command: cmdAndArgs[0],
			Args:    cmdAndArgs[1:],
			Key:     bindings[strings.Join(cmdAndArgs, " ")],
		}
	}
	// prefill opens the command prompt pre-typed with cmdAndArgs via the
	// same `echo {prompt}...` macro shape the presets bind, so the
	// accelerator lookup matches their bindings.
	prefill := func(title string, cmdAndArgs ...string) appmenu.Command {
		return cmd(title, "echo",
			"{prompt}"+strings.Join(cmdAndArgs, "<space>")+"<space>")
	}
	sep := appmenu.Separator{}
	return []appmenu.Menu{
		{Title: "Rune", Items: []appmenu.Item{
			appmenu.Native{Title: "About Rune", Selector: "orderFrontStandardAboutPanel:"},
			sep,
			cmd("Settings…", appMenuCommands.settings...),
			sep,
			appmenu.Native{Title: "Hide Rune", Selector: "hide:"},
			cmd("Quit Rune", appMenuCommands.quit...),
		}},
		{Title: "File", Items: []appmenu.Item{
			cmd("Open File…", "edit"),
			cmd("Open Read-Only…", "view"),
			cmd("Open Project…", "workspaceopen"),
			sep,
			cmd("Save", "write"),
			cmd("Reload File", "reloadfile!"),
			sep,
			cmd("New Tab", "tabnew"),
			cmd("Close Tab", "tabclose"),
			cmd("Next Tab", "tabnext"),
			cmd("Previous Tab", "tabprevious"),
			prefill("Focus Tab…", "tabfocus"),
			prefill("Move Tab…", "tabmove"),
		}},
		{Title: "Edit", Items: []appmenu.Item{
			cmd("Copy", "clipboardcopy"),
			cmd("Paste", "clipboardpaste"),
			sep,
			cmd("Rename Symbol…", "lsp", "rename"),
			cmd("Format File", "lsp", "format"),
			cmd("Trigger Completion", "lsp", "complete"),
		}},
		{Title: "View", Items: []appmenu.Item{
			cmd("File Explorer", "fexplorer"),
			cmd("Command History", "history"),
			prefill("Change Opacity…", "guiopacity"),
			sep,
			cmd("Expand Fold", "foldexpand"),
			cmd("Collapse Fold", "foldcollapse"),
			cmd("Toggle Fold", "foldtoggle"),
			cmd("Expand All Folds", "foldexpandall"),
			cmd("Collapse All Folds", "foldcollapseall"),
			cmd("Toggle All Folds", "foldtoggleall"),
			sep,
			cmd("New Window", "windownew"),
			cmd("Close Window", "windowclose"),
			cmd("Toggle Maximize", "windowtogglemaximize"),
			cmd("Convert to Tab", "windowconverttab"),
			prefill("Resize Window…", "windowresize"),
			prefill("Focus Window…", "windowfocus"),
			prefill("Move Window…", "windowmove"),
			prefill("Change Default Split…", "windowdefaultsplit"),
			sep,
			appmenu.Native{
				Title:    "Enter Full Screen",
				Selector: "toggleFullScreen:",
				Key:      term.KeyComb{Mod: term.ModCtrlMeta, Ch: 'f'},
			},
		}},
		{Title: "Find", Items: []appmenu.Item{
			cmd("Find File…", "searchfile"),
			cmd("Find in Files…", "searchtext"),
			sep,
			cmd("Find Function…", "searchfunc"),
			cmd("Find Variable…", "searchvar"),
			cmd("Find Type…", "searchtype"),
			sep,
			prefill("Jump to Function in File…", "jumptoast", "locals.scm",
				"local.definition.method|local.definition.function"),
			prefill("Jump to Variable in File…", "jumptoast", "locals.scm",
				"local.definition.var"),
			prefill("Jump to Type in File…", "jumptoast", "locals.scm",
				"local.definition.type"),
			sep,
			prefill("Find Definition…", "lsp", "definition"),
			prefill("Find Implementation…", "lsp", "implementation"),
			prefill("Find References…", "lsp", "references"),
			prefill("Find Documentation…", "lsp", "hover"),
		}},
		{Title: "Go", Items: []appmenu.Item{
			cmd("Back", "cursorhistory", "prev"),
			cmd("Forward", "cursorhistory", "next"),
			prefill("Cursor History…", "cursorhistory", "jump"),
			sep,
			cmd("Go to Definition", "lsp", "definition"),
			cmd("Show References", "lsp", "references"),
			cmd("Go to Implementation", "lsp", "implementation"),
			cmd("Show Documentation", "lsp", "hover"),
			sep,
			cmd("Next Diagnostic", "lspnextdiagnostic"),
			cmd("Previous Diagnostic", "lspprevdiagnostic"),
			cmd("Next Git Change", "gitnextchange"),
			cmd("Previous Git Change", "gitprevchange"),
		}},
		{Title: "Workspace", Items: []appmenu.Item{
			prefill("Open Workspace…", "workspaceopen"),
			cmd("Reload Workspace", "workspacereload"),
			sep,
			prefill("Focus Workspace…", "workspacefocus"),
			prefill("Move Workspace…", "workspacemove"),
			sep,
			prefill("New Worktree…", "worktreenew"),
			prefill("Open Worktree…", "worktreeopen"),
			prefill("Remove Worktree…", "worktreeremove"),
		}},
		{Title: "Tools", Items: []appmenu.Item{
			cmd("New Terminal", "terminalneworsplit"),
			cmd("Open Companion Terminal", "!"),
			prefill("Run…", "!"),
			sep,
			cmd("New Agent Session", "agent"),
			cmd("Debugger", "debugger"),
			cmd("Console", "console"),
		}},
		{Title: "Help", Items: []appmenu.Item{
			cmd("Rune Help", "help"),
			cmd("Documentation", "docs"),
			sep,
			cmd("Cheatsheet", "cheatsheet"),
			cmd("Key Bindings", "keybindings"),
			sep,
			cmd("Basics Tutorial", "tutorial", "start", "basics"),
			cmd("Check for Updates", "upgrade"),
		}},
	}
}
