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

	"unstable.build/go-tui/cmd/rune-agent/llm/llmregistry"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"unstable.build/go-tui/debug"
)

// NewExtension returns an extension and its metadata.
func NewExtension(
	registry llmregistry.Registry,
	defaultModel string,
) (extensionapi.WorkspaceExtension, extensionapi.Metadata) {
	return &workspaceExtension{
			defaultModel: defaultModel,
			registry:     registry,
		}, extensionapi.Metadata{
			DeveloperID:      "ernestrc",
			DeveloperEmail:   "ernest@unstable.build",
			DeveloperKey:     "064D4ABCFA6D9338",
			ExtensionID:      "rune-agent",
			ExtensionName:    "Rune Agent",
			ExtensionVersion: debug.Tag,
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
				"The default coding model used is configured via extension configuration. ",
			Synopsis: "[message]",
		},
		{
			Name: commandChat,
			Summary: "Open a new conversation tab with your AI assistant. " +
				"If no dialogue ID is provided, a new conversation is started. " +
				"If not passed, the default model used is configured via extension configuration. ",
			Synopsis: "[dialogue_id [model]]",
		},
		{
			Name: commandResetChat, Summary: "Clear all current chat's history.",
			Synopsis: "[dialogue_id]",
		},
		{
			Name: commandShell,
			Summary: "Open an interactive shell for inspecting and managing the agent extension internals. " +
				"Browse available models, registered tools, agent definitions, and LLM configuration. " +
				"List, inspect, and delete saved conversation sessions. " +
				"View system prompts and per-session model assignments.",
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
	}
)

type workspaceExtension struct {
	defaultModel string
	registry     llmregistry.Registry
}

func (e *workspaceExtension) ExtendWorkspace(
	ctx context.Context, w *extensionapi.Workspace, cfg config.Config,
) error {
	h, err := newCommandEventHandler(ctx, w.Editor(ctx), w, cfg,
		e.registry, e.defaultModel)
	if err != nil {
		return err
	}

	for _, cmd := range commands {
		err := w.RegisterCommand(cmd, h)
		if err != nil {
			return fmt.Errorf("register command %q: %w", cmd.Name, err)
		}
	}

	ed := w.Editor(ctx)
	if err := ed.SubscribeEvents(events, h); err != nil {
		return fmt.Errorf("subscribe events: %w", err)
	}

	return nil
}
