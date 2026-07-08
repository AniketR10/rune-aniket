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
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/extension/langext"
)

// rustMarkers are the project-root markers that drive nested discovery.
// A stray .rs file with no enclosing Cargo.toml does not spawn a server;
// the bare-.rs fallback stays a workspace-root concern (see
// detectRustProject).
var rustMarkers = []string{"Cargo.toml"}

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
		w.Editor(ctx),
		w.DataDir(ctx),
		os.Getenv("RUSTUP_HOME"),
		os.Getenv("CARGO_HOME"),
		cfg,
		w.RegisterREPLCommand,
	)
}

// extendWorkspaceWith wires Rust project discovery to per-root language
// server bring-up. It registers the REPL command once for the workspace,
// subscribes for opened .rs files so a server is initialized rooted at
// each file's nearest Cargo.toml project, and eagerly initializes the
// workspace-root project when one is present. ExtendWorkspace supplies
// the dependencies from a real *extensionapi.Workspace; the e2e harness
// supplies real FileSystem and Executor with fake Notifications, LSP and
// Editor so the full path runs without a live host. The dependencies are
// passed positionally so the compiler flags a missing one at every call
// site.
func (e *rustExtension) extendWorkspaceWith(
	ctx context.Context,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	notify browserapi.Notifications,
	lsp semanticapi.LSP,
	editor textapi.Editor,
	dataDir, rustupHome, cargoHome string,
	cfg config.Config,
	registerREPL func(textapi.CommandManual, textapi.REPLHandler) error,
) error {
	cwd, err := fs.URI(".")
	if err != nil {
		return fmt.Errorf("resolve cwd uri: %w", err)
	}
	rustupBin := resolveRustup(ctx, fs, dataDir)

	init := langext.NewInitializer(ctx, fs, editor, langext.ProjectConfig{
		LanguageID: "rust",
		Markers:    rustMarkers,
		FileMatch:  isRustFile,
		InitRoot: func(ctx context.Context, root langext.Root) error {
			return initializeRustRoot(ctx,
				fs, exec, notify, lsp, dataDir, rustupHome, cargoHome, rustupBin, cfg, root)
		},
	})
	if err := init.Start(); err != nil {
		return fmt.Errorf("subscribe rust open events: %w", err)
	}

	// The REPL command's cwd is always the workspace root, independent of
	// any nested project, so register it once up front. A reload (or a
	// toolchain-mutating subcommand) must rebuild every server brought up
	// so far, which Reinitialize does across all discovered roots.
	reload := func(ctx context.Context) error {
		if err := refreshSymlinks(ctx, exec, fs, cargoHome, dataDir, cwd.Path()); err != nil {
			return err
		}
		return init.Reinitialize(ctx)
	}
	manual, handler := newRustHandler(exec, notify, cwd.Path(), rustupBin, reload)
	if err := registerREPL(manual, handler); err != nil {
		return fmt.Errorf("register rust command: %w", err)
	}

	// Preserve the eager workspace-root behavior: if the workspace root
	// is itself a Rust project (including the bare-.rs fallback), bring
	// it up immediately rather than waiting for the first open. Nested
	// discovery still requires a Cargo.toml.
	if detectRustProject(ctx, fs) {
		root := langext.Root{Dir: cwd.Path(), URI: fmt.Sprintf("file://%s", cwd.Path())}
		if err := init.InitializeAt(ctx, root); err != nil {
			return err
		}
	}
	return nil
}

// initializeRustRoot performs the language-specific bring-up for a
// discovered project root: it bootstraps the rustup toolchain rooted
// there (best-effort), resolves rust-analyzer and the sysroot, and
// initializes the language server with the nested root URI.
func initializeRustRoot(
	ctx context.Context,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	notify browserapi.Notifications,
	lsp semanticapi.LSP,
	dataDir, rustupHome, cargoHome, rustupBin string,
	cfg config.Config,
	root langext.Root,
) error {
	if err := bootstrapRustup(
		ctx, rustupBin, exec, notify, fs, rustupHome, cargoHome, dataDir, root.Dir,
	); err != nil {
		_, _ = notify.Notify(browserapi.LevelWarn,
			"Rust toolchain setup failed, continuing without a managed toolchain: %v", err)
		slog.Warn("rust toolchain setup failed", "root", root.Dir, "error", err)
	}

	command := resolveRustAnalyzer(cfg, notify, dataDir)
	sysroot := resolveSysroot(ctx, exec)
	params, err := rustInitializeParams(root.URI, command, sysroot)
	if err != nil {
		return fmt.Errorf("build init params: %w", err)
	}
	if _, err := lsp.Initialize(ctx, params); err != nil {
		return fmt.Errorf("initialize rust lsp: %w", err)
	}
	slog.Info("rust lsp initialized", "root", root.Dir, "command", command)
	return nil
}

// isRustFile reports whether uri names a Rust source file.
func isRustFile(uri workspaceapi.URI) bool {
	return strings.HasSuffix(uri.Path(), ".rs")
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
