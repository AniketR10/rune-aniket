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
	"context"
	"fmt"
	"log/slog"

	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
)

// NewExtension returns the Go extension and its metadata.
func NewExtension() (extensionapi.WorkspaceExtension, extensionapi.Metadata) {
	ext := &goExtension{}
	meta := extensionapi.Metadata{
		DeveloperID:    "Unstable Build",
		DeveloperEmail: "it@unstable.build",
		DeveloperKey:   "064D4ABCFA6D9338",
		ExtensionID:    "go",
		ExtensionName:  "Go Language Extension",
		Permissions: extensionapi.NewPermissions(
			extensionapi.PermissionLSP,
			extensionapi.PermissionEditor,
			extensionapi.PermissionCommands,
			extensionapi.PermissionConfig,
			extensionapi.PermissionBrowserWindowManager,
			extensionapi.PermissionNotifications,
			extensionapi.PermissionInterrupt,
			extensionapi.PermissionExecute,
			extensionapi.PermissionFileSystem,
			extensionapi.PermissionBrowserResourceOpener,
			extensionapi.PermissionSyntaxTree,
		),
	}
	return ext, meta
}

type goExtension struct{}

func (e *goExtension) ExtendWorkspace(
	ctx context.Context, w *extensionapi.Workspace, cfg config.Config,
) error {
	lsp := w.LSP(ctx)
	editor := w.Editor(ctx)
	wm := w.WindowManager(ctx)
	notify := w.Notifications(ctx)

	cwd, err := w.FileSystem(ctx).URI(".")
	if err != nil {
		return fmt.Errorf("resolve cwd uri: %w", err)
	}
	// gopls runs on the same host as the workspace files (locally for a
	// local workspace, or on the remote host for a remote one), so rewrite
	// the URI to the file:// scheme expected by the language server.
	rootURI := fmt.Sprintf("file://%s", cwd.Path())

	params, err := goplsInitializeParams(rootURI)
	if err != nil {
		return fmt.Errorf("build init params: %w", err)
	}
	_, err = lsp.Initialize(ctx, params)
	if err != nil {
		return fmt.Errorf("initialize gopls: %w", err)
	}
	slog.Info("gopls initialized")

	parser := w.Parser(ctx)
	executor := w.Executor(ctx)
	manual, handler, err := newGoHandler(lsp, editor, wm, notify, parser, executor)
	if err != nil {
		return fmt.Errorf("create handler: %w", err)
	}
	if err := w.RegisterCommand(manual, handler); err != nil {
		return fmt.Errorf("register command: %w", err)
	}
	return nil
}
