// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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
	"context"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/go-tui/cmd/rune-agent/agentshell"
)

// NewExtension returns an extension and its metadata.
func NewExtension() (extensionapi.WorkspaceExtension, extensionapi.Metadata) {
	return &workspaceExtension{}, extensionapi.Metadata{
			DeveloperID:    "Unstable Build",
			DeveloperEmail: "it@unstable.build",
			DeveloperKey:   "064D4ABCFA6D9338",
			ExtensionID:    "rune-agent",
			ExtensionName:  "Rune Agent",
			Permissions: extensionapi.NewPermissions(
				permissions...,
			),
		}
}

var (
	commands = []textapi.CommandManual{
		{
			Name: commandQuery,
			Summary: "Send a coding question to your AI assistant. " +
				"The current active file is loaded and available in the model's context. " +
				"The default coding model is configured via extension configuration.",
			Synopsis: "[<message>]",
		},
		{
			Name: commandChat,
			Summary: "Open a new conversation tab with your AI assistant. " +
				"If no dialogue ID is provided, a new conversation is started. " +
				"If no model is provided, the default model configured via extension " +
				"configuration is used.",
			Synopsis: "[<dialogue_id> [<model>]]",
		},
		{
			Name: commandModel,
			Summary: "Show or switch the model of the focused agent chat. " +
				"Run from an open agent chat tab.",
			Synopsis: "[model]",
		},
		{
			Name: commandEffort,
			Summary: "Show or set the reasoning effort of the focused agent chat. " +
				"Run from an open agent chat tab.",
			Synopsis: "[none|minimal|low|medium|high|xhigh|max]",
		},
		{
			Name: commandMaxTokens,
			Summary: "Show or set the max output tokens of the focused agent chat. " +
				"Run from an open agent chat tab.",
			Synopsis: "[tokens]",
		},
		{
			Name: commandSkill,
			Summary: "Load a skill into the focused agent chat. " +
				"Run from an open agent chat tab.",
			Synopsis: "<skill> [args]",
		},
		{
			Name: commandClear,
			Summary: "Clear the focused agent chat and archive its previous " +
				"contents. Run from an open agent chat tab.",
			Synopsis: "",
		},
		{
			Name: commandCompact,
			Summary: "Compact the focused agent chat into a summarized copy. " +
				"Run from an open agent chat tab.",
			Synopsis: "",
		},
		{
			Name: commandFork,
			Summary: "Open a picker to fork the focused agent chat at a selected " +
				"message. Run from an open agent chat tab.",
			Synopsis: "",
		},
		{
			Name: commandExport,
			Summary: "Export the focused agent chat or its audit log to a temp " +
				"file. Run from an open agent chat tab.",
			Synopsis: "[--audit]",
		},
		{
			Name: commandLog,
			Summary: "Show the LLM token audit log for the focused agent chat. " +
				"Run from an open agent chat tab.",
			Synopsis: "",
		},
	}
	events = []textapi.EventType{
		textapi.EventTypeOpen, textapi.EventTypeFocus,
		textapi.EventTypeUnfocus, textapi.EventTypeFlush,
		textapi.EventTypeClose,
	}
	permissions = []extensionapi.Permission{
		extensionapi.PermissionBrowserWindowManager,
		extensionapi.PermissionBrowserResourceOpener,
		extensionapi.PermissionNotifications,
		extensionapi.PermissionInterrupt,
		extensionapi.PermissionStorage,
		extensionapi.PermissionEditor,
		extensionapi.PermissionCommands,
		extensionapi.PermissionConfig,
		extensionapi.PermissionFileSystem,
		extensionapi.PermissionExecute,
		extensionapi.PermissionTerminal,
		extensionapi.PermissionSyntaxTree,
		extensionapi.PermissionLSP,
		extensionapi.PermissionLLM,
	}
)

type workspaceExtension struct{}

func (e *workspaceExtension) ExtendWorkspace(
	ctx context.Context, w *extensionapi.Workspace, cfg config.Config,
) error {
	h, err := newCommandEventHandler(ctx, w.Editor(ctx), w, cfg)
	if err != nil {
		return err
	}

	for _, cmd := range commands {
		err := w.RegisterCommand(cmd, h)
		if err != nil {
			return fmt.Errorf("register command %q: %w", cmd.Name, err)
		}
	}

	if err := w.RegisterREPLCommand(agentshell.Manual(), h.newAgentShell()); err != nil {
		return fmt.Errorf("register repl command %q: %w", agentshell.CommandName, err)
	}

	ed := w.Editor(ctx)
	if err := ed.SubscribeEvents(events, h); err != nil {
		return fmt.Errorf("subscribe events: %w", err)
	}

	return nil
}
