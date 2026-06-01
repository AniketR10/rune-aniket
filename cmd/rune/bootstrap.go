// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2024 Unstable Build, All Rights Reserved.
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
	"context"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
)

const (
	editorModal       = "modal"
	editorModeless    = "modeless"
	editorExoModal    = "exo-modal"
	editorExoModeless = "exo-modeless"
)

// Editor-mode option labels. The strings are also used as map keys in
// optionToChoice, so they must stay stable across the prompt and the
// callback.
const (
	optModal       = "  Modal (vi)              "
	optModeless    = "  Modeless                "
	optExoModal    = "  exo (modal fallback)   "
	optExoModeless = "  exo (modeless fallback)"
)

var (
	bootstrapEditorKeys = []term.KeyComb{
		{Ch: 'm'}, {Ch: 'l'}, {Ch: 'b'}, {Ch: 'e'},
	}

	bootstrapFormatKeys = []term.KeyComb{
		{Ch: 's'}, {Ch: 'y'},
	}

	bootstrapPresetKeys = []term.KeyComb{
		{Ch: 'v'}, {Ch: 'n'}, {Ch: 'h'}, {Ch: 'k'}, {Ch: 'e'},
	}

	bootstrapWelcomeKeys = []term.KeyComb{
		{Ch: 'g'},
	}

	// "Maybe later" is first so Enter (which selects the highlighted
	// option, initially index 0) does not trigger a browser-based
	// OAuth flow on the welcome screen. The user must explicitly
	// navigate to or press the key for "Sign in".
	bootstrapLoginKeys = []term.KeyComb{
		{Ch: 'l'}, {Ch: 's'},
	}
)

const (
	optFormatStar = "  Starlark (.star) "
	optFormatYAML = "  YAML (.yaml)     "

	optPresetVim   = "  vim   "
	optPresetNvim  = "  nvim  "
	optPresetHelix = "  helix "
	optPresetKak   = "  kak   "
	optPresetEmacs = "  emacs "

	optWelcomeGo = "  Let's go "

	optLoginYes = "  Sign in      "
	optLoginNo  = "  Maybe later  "
)

// openBootstrapFlow chains the welcome screen and three configuration
// prompts (editor mode, config format, and—if exo was picked—editor
// preset), then a login prompt against the still-alive pre-config IDE.
// Login tokens are persisted to the shared on-disk auth partition, so
// the configured IDE built by performSwap picks them up transparently.
func (b *bootstrapHandler) openBootstrapFlow() {
	b.openWelcomePrompt()
}

func (b *bootstrapHandler) openWelcomePrompt() {
	msg := "**Welcome to Rune.**\n\n" +
		"The Unix way, finished as a product.\n" +
		"Let's get you configured."
	guard := &guardedPromptChain{}
	b.preIDE.Prompt(
		msg,
		[]string{optWelcomeGo},
		bootstrapWelcomeKeys,
		handler.FuncPromptHandler(
			guard.onSelect(func(_ int, _ string) {
				b.openEditorPrompt()
			}),
			guard.onClose(b.openWelcomePrompt),
		),
	)
}

func (b *bootstrapHandler) openEditorPrompt() {
	msg := "**Pick an editor mode.**\n\n" +
		"**Modal (vi)** — Rune's built-in editor with vi keybindings.\n" +
		"Modes, motions, operators, and the full Rune command set.\n" +
		"Pick this if you already think in vi.\n\n" +
		"**Modeless** — Rune's built-in editor without modes.\n" +
		"Type to insert; arrows, shift-select, system clipboard.\n" +
		"Pick this if you want a familiar IDE feel.\n\n" +
		"**exo** — exoeditor.\n" +
		"Rune owns tabs, files, panes, and commands; your external\n" +
		"editor (vim, nvim, helix, kak, emacs) owns the buffer and\n" +
		"cursor. The `(modal fallback)` and `(modeless fallback)`\n" +
		"variants only choose which built-in editor handles buffers\n" +
		"your external editor cannot open — for example Rune's file\n" +
		"explorer at `memory:///fexplorer`.\n" +
		"You give up: syntax/diagnostic highlights, incremental edit\n" +
		"dispatch, cursor read-back, and inline agent edits inside\n" +
		"the exo pane."
	guard := &guardedPromptChain{}
	b.preIDE.Prompt(
		msg,
		[]string{optModal, optModeless, optExoModal, optExoModeless},
		bootstrapEditorKeys,
		handler.FuncPromptHandler(
			guard.onSelect(func(_ int, option string) {
				b.chosenEditor = optionToChoice(option)
				b.openFormatPrompt()
			}),
			guard.onClose(b.openEditorPrompt),
		),
	)
}

func (b *bootstrapHandler) openFormatPrompt() {
	msg := "**Pick a configuration format.**\n\n" +
		"**Starlark (.star)** — Python-like config with variables,\n" +
		"functions, and conditionals. Pick this if you want to share\n" +
		"settings across machines or compose keybindings\n" +
		"programmatically.\n\n" +
		"**YAML (.yaml)** — plain declarative key/value config.\n" +
		"Pick this if you want the simplest possible file.\n\n" +
		"Either format lives at `~/.rune/config.<ext>` and can be\n" +
		"changed later by editing or replacing the file."
	guard := &guardedPromptChain{}
	b.preIDE.Prompt(
		msg,
		[]string{optFormatStar, optFormatYAML},
		bootstrapFormatKeys,
		handler.FuncPromptHandler(
			guard.onSelect(func(_ int, option string) {
				b.chosenFormat = optionToFormat(option)
				if b.chosenEditor == editorExoModal || b.chosenEditor == editorExoModeless {
					b.openPresetPrompt()
					return
				}
				b.openLoginPrompt()
			}),
			guard.onClose(b.openFormatPrompt),
		),
	)
}

func (b *bootstrapHandler) openPresetPrompt() {
	msg := "**Pick an external editor.**\n\n" +
		"Rune launches this binary inside each editor pane and uses\n" +
		"its goto-line protocol to jump to file:line:col. The binary\n" +
		"must be on your `PATH`; you can edit the exact command and\n" +
		"goto sequence in the generated config later.\n\n" +
		"`vim` · `nvim` · `helix (hx)` · `kak` · `emacs -nw`"
	guard := &guardedPromptChain{}
	b.preIDE.Prompt(
		msg,
		[]string{optPresetVim, optPresetNvim, optPresetHelix, optPresetKak, optPresetEmacs},
		bootstrapPresetKeys,
		handler.FuncPromptHandler(
			guard.onSelect(func(_ int, option string) {
				b.chosenExoPreset = optionToPreset(option)
				b.openLoginPrompt()
			}),
			guard.onClose(b.openPresetPrompt),
		),
	)
}

// openLoginPrompt runs the final bootstrap prompt against the still-
// alive pre-config IDE, *before* performSwap. The bootstrap-phase
// apiclient.Client is constructed lazily here against a notifications
// service that routes to whichever IDE is currently active, so the
// OAuth goroutine's success / failure message lands on the configured
// IDE if the user is still in their browser when performSwap runs.
//
// If client construction fails (e.g. invalid endpoint flags) the prompt
// is skipped and the swap proceeds straight away. On "Sign in" Login is
// fired (returns immediately; the OAuth2 flow runs on a goroutine the
// SDK owns) and performSwap runs synchronously on the spot — the swap
// does not wait for the browser dance to complete. Tokens land on the
// shared on-disk auth partition keyed by dataDir, so the configured
// IDE's real client reads them back transparently after the swap.
func (b *bootstrapHandler) openLoginPrompt() {
	if b.bootstrapClient == nil {
		client, err := newBootstrapAPIClient(b.notifications(), b.preIDE.Storage())
		if err != nil {
			log.Warnf("bootstrap api client: %v", err)
			b.performSwap()
			return
		}
		b.bootstrapClient = client
	}
	msg := "**Rune is free. Sign in to unlock paid features and updates.**\n\n" +
		"**Free:** terminal-first environment, multi-workspace UI,\n" +
		"tiled window manager, bring your own agent, bring your own\n" +
		"editor.\n\n" +
		"**Paid:** syntactic + semantic LSP, debugger, built-in agent\n" +
		"with IDE-grade tools, skills, curated extensions.\n" +
		"$10/mo or $100/year. Cancel anytime.\n\n" +
		"You can run `:login` later from the command bar."
	guard := &guardedPromptChain{}
	b.preIDE.Prompt(
		msg,
		// Order matches bootstrapLoginKeys: "Maybe later" is first
		// so Enter on the default highlight does not open a browser.
		[]string{optLoginNo, optLoginYes},
		bootstrapLoginKeys,
		handler.FuncPromptHandler(
			guard.onSelect(func(_ int, option string) {
				if option == optLoginYes {
					if err := b.bootstrapClient.Login(context.Background()); err != nil {
						log.Warnf("bootstrap login: %v", err)
					}
				}
				b.performSwap()
			}),
			guard.onClose(b.openLoginPrompt),
		),
	)
}

// guardedPromptChain wires a bootstrap prompt so that any close that
// did NOT come from a user selection (Esc, mouse-driven dismissal we
// missed, or a future SDK path) re-opens the same prompt. The SDK's
// handler.Prompt.Handle calls OnSelect synchronously before the prompt
// window is closed, and only then does the runtime invoke OnClose, so
// the "selection-then-close" ordering is observable here.
type guardedPromptChain struct {
	advanced bool
}

// onSelect wraps the OnSelect callback so the chain remembers that an
// option was picked. The returned func has the SDK's expected signature.
func (g *guardedPromptChain) onSelect(next func(int, string)) func(int, string) {
	return func(idx int, option string) {
		g.advanced = true
		next(idx, option)
	}
}

// onClose wraps the OnClose callback so the prompt is re-opened via
// reopen unless onSelect already advanced the flow. The returned func
// has the SDK's expected signature.
func (g *guardedPromptChain) onClose(reopen func()) func() error {
	return func() error {
		if !g.advanced {
			reopen()
		}
		return nil
	}
}

// shouldSwallowBootstrapEvent reports whether the given event must not
// reach the pre-config IDE while the bootstrap flow is in progress.
// The set of dangerous events is small and explicit: the ':' command
// prompt activation, and the default rune.star quit / window-close /
// tab-close keybindings. Everything else passes through so the user
// can still navigate the prompt, resize the window, etc.
//
// Esc is deliberately NOT swallowed here — the SDK prompt's own Handle
// relies on Esc to exit, and the guardedPromptChain.onClose callback
// re-opens the prompt right after.
func shouldSwallowBootstrapEvent(ev term.Event) bool {
	if ev.Type != term.EventKey {
		return false
	}
	// Command-prompt activation key. Default rune.star sets this
	// to ':' (text.OptionsDefault.CommandEvent). We don't honor a
	// user-customized activation key here because the preIDE only
	// ever loads the embedded defaults — no user config exists yet.
	if ev.Mod == 0 && ev.Ch == ':' {
		return true
	}
	// Quit / close keybindings from rune.star and
	// override_modeless.star:
	//   <m-q> quit
	//   <m-w> windowclose, <a-w> tabclose, <c-w> tabclose
	//   <m-s-w> / <s-m-w> windowclose (modeless overrides)
	if ev.Ch == 'q' && ev.Mod&term.ModMeta != 0 {
		return true
	}
	if ev.Ch == 'w' && ev.Mod != 0 {
		switch {
		case ev.Mod&term.ModMeta != 0,
			ev.Mod&term.ModAlt != 0,
			ev.Mod&term.ModCtrl != 0:
			return true
		}
	}
	return false
}

// optionToChoice maps a placeholder display string to its persisted
// editor choice constant. Unknown options default to editorModal.
func optionToChoice(option string) string {
	switch option {
	case optModal:
		return editorModal
	case optModeless:
		return editorModeless
	case optExoModal:
		return editorExoModal
	case optExoModeless:
		return editorExoModeless
	}
	return editorModal
}

// optionToFormat maps a config-format prompt display string to the
// persisted format constant. Unknown options default to YAML.
func optionToFormat(option string) string {
	switch option {
	case optFormatStar:
		return configFormatStar
	case optFormatYAML:
		return configFormatYAML
	}
	return configFormatYAML
}

// optionToPreset maps a exo-preset prompt display string to the
// preset key. Unknown options default to vim.
func optionToPreset(option string) string {
	switch option {
	case optPresetVim:
		return exoPresetVim
	case optPresetNvim:
		return exoPresetNvim
	case optPresetHelix:
		return exoPresetHelix
	case optPresetKak:
		return exoPresetKak
	case optPresetEmacs:
		return exoPresetEmacs
	}
	return exoPresetVim
}
