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

package lspcmd

import (
	"context"
	"log/slog"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/go-tui/handler/locationpicker"
)

// DefinitionConfig configures the "definition" subcommand.
type DefinitionConfig struct {
	RootURI    workspaceapi.URI
	ListConfig locationpicker.Config
}

// DefaultDefinitionConfig returns a DefinitionConfig with sensible defaults.
func DefaultDefinitionConfig() DefinitionConfig {
	return DefinitionConfig{}
}

// DefinitionHandler creates a textapi.CommandHandler that finds the definition
// of the symbol at the cursor position and displays the result in a floating
// window with a file preview.
func DefinitionHandler(
	lsp semanticapi.LSP, editor textapi.Editor,
	wm browserapi.WindowManager, opener browserapi.ResourceOpener,
	notify browserapi.Notifications, fs workspaceapi.FileSystem,
	scheduleNextTick func(func()) bool,
	parser syntaxapi.Parser, cfg DefinitionConfig,
	log *slog.Logger,
) textapi.CommandHandler {
	return &definitionHandler{
		lsp: lsp, editor: editor, wm: wm, opener: opener,
		notify: notify, fs: fs,
		scheduleNextTick: scheduleNextTick, parser: parser, cfg: cfg,
		log: log,
	}
}

type definitionHandler struct {
	lsp              semanticapi.LSP
	editor           textapi.Editor
	wm               browserapi.WindowManager
	opener           browserapi.ResourceOpener
	notify           browserapi.Notifications
	fs               workspaceapi.FileSystem
	scheduleNextTick func(func()) bool
	parser           syntaxapi.Parser
	cfg              DefinitionConfig
	log              *slog.Logger
}

func (h *definitionHandler) HandleCommand(ctx context.Context, cmd textapi.Command) error {
	proceed, err := resolveCommandSymbol(ctx, &cmd, h.wm, h.fs, h.notify, h.scheduleNextTick, h.parser,
		executeResolved("definition", h.notify, h.scheduleNextTick, h.execute))
	if !proceed || err != nil {
		return err
	}
	executeAtCursor("definition", h.notify, h.scheduleNextTick, cmd, h.execute)
	return nil
}

// execute performs the blocking LSP round trip. It is called off the event
// loop and schedules UI work onto it.
func (h *definitionHandler) execute(
	ctx context.Context, uri workspaceapi.URI, pos semanticapi.Position,
) error {
	params := semanticapi.DefinitionParams{
		TextDocument: TextDocID(uri),
		Position:     pos,
	}
	result, err := h.lsp.Definition(ctx, params)
	if err != nil {
		return err
	}
	entries := locationsFromResult(result)
	if len(entries) == 0 {
		return errNoLocations
	}
	entries = enrichEntries(entries, h.cfg.RootURI)
	presentLocations(
		"definition", entries, h.opener, h.wm, h.editor, h.notify,
		h.fs, h.scheduleNextTick, h.parser, h.cfg.ListConfig, h.log,
	)
	return nil
}

func (h *definitionHandler) Complete(
	ctx context.Context, _ string, args []string,
) (iterator.Iterator[string], error) {
	return completeReferencedSymbol(ctx, h.parser)
}
