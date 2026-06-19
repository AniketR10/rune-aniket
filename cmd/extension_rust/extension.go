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
	"os"
	"path"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// NewExtension returns the Rust extension and its metadata.
func NewExtension() (extensionapi.WorkspaceExtension, extensionapi.Metadata) {
	ext := &rustExtension{}
	meta := extensionapi.Metadata{
		DeveloperID:    "Unstable Build",
		DeveloperEmail: "it@unstable.build",
		DeveloperKey:   "064D4ABCFA6D9338",
		ExtensionID:    "rust",
		ExtensionName:  "Rust Language Extension",
		Permissions: extensionapi.NewPermissions(
			extensionapi.PermissionLSP,
			extensionapi.PermissionEditor,
			extensionapi.PermissionCommands,
			extensionapi.PermissionConfig,
			extensionapi.PermissionNotifications,
			extensionapi.PermissionExecute,
			extensionapi.PermissionFileSystem,
			extensionapi.PermissionSyntaxTree,
			extensionapi.PermissionDebugger,
		),
	}
	return ext, meta
}

type rustExtension struct{}

func (e *rustExtension) ExtendWorkspace(
	ctx context.Context, w *extensionapi.Workspace, cfg config.Config,
) error {
	return e.extendWorkspaceWith(ctx,
		w.FileSystem(ctx),
		w.Executor(ctx),
		w.Notifications(ctx),
		w.LSP(ctx),
		w.DataDir(ctx),
		os.Getenv("RUSTUP_HOME"),
		os.Getenv("CARGO_HOME"),
		cfg,
		w.RegisterREPLCommand,
	)
}

// extendWorkspaceWith performs the workspace bring-up against an explicit
// set of dependencies. ExtendWorkspace supplies them from a real
// *extensionapi.Workspace; the e2e harness supplies real FileSystem and
// Executor with fake Notifications and LSP so the full path runs without
// a live host. The dependencies are passed positionally so the compiler
// flags a missing one at every call site.
func (e *rustExtension) extendWorkspaceWith(
	ctx context.Context,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	notify browserapi.Notifications,
	lsp semanticapi.LSP,
	dataDir, rustupHome, cargoHome string,
	cfg config.Config,
	registerREPL func(textapi.CommandManual, textapi.REPLHandler) error,
) error {
	if !detectRustProject(ctx, fs) {
		return nil
	}

	cwd, err := fs.URI(".")
	if err != nil {
		return fmt.Errorf("resolve cwd uri: %w", err)
	}
	rootURI := fmt.Sprintf("file://%s", cwd.Path())

	rustupBin := resolveRustup(ctx, fs, dataDir)

	initLSP := func(ctx context.Context) error {
		command := resolveRustAnalyzer(cfg, notify, dataDir)
		sysroot := resolveSysroot(ctx, exec)
		params, err := rustInitializeParams(rootURI, command, sysroot)
		if err != nil {
			return fmt.Errorf("build init params: %w", err)
		}
		if _, err := lsp.Initialize(ctx, params); err != nil {
			return fmt.Errorf("initialize rust lsp: %w", err)
		}
		slog.Info("rust lsp initialized", "command", command)
		return nil
	}

	if err := bootstrapRustup(
		ctx, rustupBin, exec, notify, fs, rustupHome, cargoHome, dataDir,
	); err != nil {
		_, _ = notify.Notify(browserapi.LevelWarn,
			"Rust toolchain setup failed, continuing without a managed toolchain: %v", err)
		slog.Warn("rust toolchain setup failed", "error", err)
	}
	if err := initLSP(ctx); err != nil {
		_, _ = notify.Notify(browserapi.LevelWarn, "Rust language server setup failed: %v", err)
		slog.Warn("rust lsp setup failed", "error", err)
	}

	reload := func(ctx context.Context) error {
		if err := refreshSymlinks(ctx, exec, fs, cargoHome, dataDir); err != nil {
			return err
		}
		return initLSP(ctx)
	}
	manual, handler := newRustHandler(exec, notify, cwd.Path(), rustupBin, reload)
	if err := registerREPL(manual, handler); err != nil {
		return fmt.Errorf("register rust command: %w", err)
	}
	return nil
}

// resolveRustup locates the bundled rustup at <dataDir>/bin/rustup,
// returning its path or "" so callers fall back to the bare command name.
func resolveRustup(_ context.Context, fs workspaceapi.FileSystem, dataDir string) string {
	candidate := path.Join(dataDir, "bin", "rustup")
	if info, err := fs.Stat(candidate); err == nil && info != nil && !info.IsDir() {
		return candidate
	}
	return ""
}
