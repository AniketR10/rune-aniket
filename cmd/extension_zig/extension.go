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
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/extension/langext"
	"unstable.build/go-tui/ide/idelsp/lspcmd"
)

// zigMarkers are the project-root markers that drive nested discovery.
// A stray .zig file with no enclosing build.zig does not spawn a
// server; the bare-.zig fallback stays a workspace-root concern (see
// detectZigProject).
var zigMarkers = []string{"build.zig", "build.zig.zon"}

// NewExtension returns the Zig extension and its metadata.
func NewExtension() (extensionapi.WorkspaceExtension, extensionapi.Metadata) {
	ext := &zigExtension{}
	meta := extensionapi.Metadata{
		DeveloperID:    "Unstable Build",
		DeveloperEmail: "it@unstable.build",
		DeveloperKey:   "064D4ABCFA6D9338",
		ExtensionID:    "zig",
		ExtensionName:  "Zig Language Extension",
		Permissions: extensionapi.NewPermissions(
			extensionapi.PermissionLSP,
			extensionapi.PermissionEditor,
			extensionapi.PermissionCommands,
			extensionapi.PermissionConfig,
			extensionapi.PermissionBrowserWindowManager,
			extensionapi.PermissionNotifications,
			extensionapi.PermissionExecute,
			extensionapi.PermissionFileSystem,
		),
	}
	return ext, meta
}

type zigExtension struct{}

func (e *zigExtension) ExtendWorkspace(
	ctx context.Context, w *extensionapi.Workspace, cfg config.Config,
) error {
	return e.extendWorkspaceWith(ctx,
		w.FileSystem(ctx),
		w.Executor(ctx),
		w.Notifications(ctx),
		w.LSP(ctx),
		w.Editor(ctx),
		w.WindowManager(ctx),
		w,
		cfg,
		w.RegisterREPLCommand,
		w.RegisterCommand,
	)
}

func (e *zigExtension) extendWorkspaceWith(
	ctx context.Context,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	notify browserapi.Notifications,
	lsp semanticapi.LSP,
	editor textapi.Editor,
	wm browserapi.WindowManager,
	inst installer,
	cfg config.Config,
	registerREPL func(textapi.CommandManual, textapi.REPLHandler) error,
	registerCommand func(textapi.CommandManual, textapi.CommandHandler) error,
) error {
	cwd, err := fs.URI(".")
	if err != nil {
		return fmt.Errorf("resolve cwd uri: %w", err)
	}

	init := langext.NewInitializer(ctx, fs, editor, langext.ProjectConfig{
		LanguageID: "zig",
		Markers:    zigMarkers,
		FileMatch:  isZigFile,
		InitRoot: func(ctx context.Context, root langext.Root) error {
			return initializeZigRoot(ctx, fs, exec, notify, lsp, inst, cfg, root)
		},
	})
	if err := init.Start(); err != nil {
		return fmt.Errorf("subscribe zig open events: %w", err)
	}

	// The REPL command's cwd is always the workspace root, independent
	// of any nested project, so register it once up front. A reload
	// must rebuild every server brought up so far, which Reinitialize
	// does across all discovered roots.
	reload := func(ctx context.Context) error {
		return init.Reinitialize(ctx)
	}
	resolveBin := func(ctx context.Context) string {
		return resolveZig(ctx, cfg, notify, fs, exec, inst)
	}
	manual, handler := newZigHandler(exec, notify, cwd.Path(), resolveBin, reload)
	if err := registerREPL(manual, handler); err != nil {
		return fmt.Errorf("register zig command: %w", err)
	}

	// The `zig` command-prompt handler exposes zls's code actions. Its
	// selection/cursor subscriptions and lifetime are workspace-scoped,
	// not project-scoped, so register it once up front regardless of
	// whether a Zig project is ever discovered.
	sel := lspcmd.NewSelectionTracker()
	evs := []textapi.EventType{textapi.EventTypeSelection, textapi.EventTypeCursor}
	if err := editor.SubscribeEvents(evs, sel); err != nil {
		return fmt.Errorf("subscribe selection events: %w", err)
	}
	actionManual, actionHandler := newZigActionHandler(lsp, editor, wm, notify, sel)
	if err := registerCommand(actionManual, actionHandler); err != nil {
		return fmt.Errorf("register zig action command: %w", err)
	}

	// Preserve the eager workspace-root behavior: if the workspace root
	// is itself a Zig project (including the bare-.zig fallback), bring
	// it up immediately rather than waiting for the first open. Nested
	// discovery still requires a build.zig or build.zig.zon.
	if detectZigProject(ctx, fs) {
		root := langext.Root{Dir: cwd.Path(), URI: fmt.Sprintf("file://%s", cwd.Path())}
		if err := init.InitializeAt(ctx, root); err != nil {
			return err
		}
	}
	return nil
}

func initializeZigRoot(
	ctx context.Context,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	notify browserapi.Notifications,
	lsp semanticapi.LSP,
	inst installer,
	cfg config.Config,
	root langext.Root,
) error {
	command := resolveZls(ctx, cfg, notify, fs, exec, inst)
	zigBin := resolveZig(ctx, cfg, notify, fs, exec, inst)
	bos := readBuildOnSave(cfg, notify)
	warnMissingCheckStep(fs, notify, root, bos)

	params, err := zlsInitializeParams(
		root.URI, filepath.Base(root.Dir), command, zigBin,
		zlsLogLevel(cfg, notify), bos)
	if err != nil {
		return fmt.Errorf("build init params: %w", err)
	}
	if _, err := lsp.Initialize(ctx, params); err != nil {
		return fmt.Errorf("initialize zig lsp: %w", err)
	}
	slog.Info("zig lsp initialized", "root", root.Dir, "command", command, "zig", zigBin)
	return nil
}

func zlsLogLevel(cfg config.Config, notify browserapi.Notifications) string {
	const defaultLevel = "info"
	if cfg == nil {
		return defaultLevel
	}
	debug, err := cfg.GetConfig("debug")
	if err != nil || debug == nil {
		return defaultLevel
	}
	level, err := debug.GetString("log_level")
	if err != nil || level == "" {
		return defaultLevel
	}
	switch level {
	case "err", "warn", "info", "debug":
		return level
	default:
		if notify != nil {
			_, _ = notify.Notify(browserapi.LevelWarn,
				"extensions.zig.config.debug.log_level must be one of "+
					"\"err\", \"warn\", \"info\", \"debug\"; got %q", level)
		}
		return defaultLevel
	}
}

func isZigFile(uri workspaceapi.URI) bool {
	return strings.HasSuffix(uri.Path(), ".zig")
}

// detectZigProject reports whether the workspace root looks like a Zig
// project: a build.zig/build.zig.zon manifest or any top-level .zig
// source file.
func detectZigProject(_ context.Context, fs workspaceapi.FileSystem) bool {
	for _, marker := range zigMarkers {
		if info, err := fs.Stat(marker); err == nil && info != nil && !info.IsDir() {
			return true
		}
	}
	if entries, err := fs.ReadDir("."); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".zig") {
				return true
			}
		}
	}
	return false
}

// readBuildOnSave reads the optional extensions.zig.config.build_on_save
// and build_on_save_args keys forwarded to zls. Enable stays nil when
// the key is absent, preserving zls's default of enabling build-on-save
// only when build.zig declares a "check" step.
func readBuildOnSave(cfg config.Config, notify browserapi.Notifications) buildOnSaveOptions {
	var bos buildOnSaveOptions
	if cfg == nil {
		return bos
	}
	if v, err := cfg.GetBool("build_on_save"); err == nil {
		bos.Enable = &v
	} else if !errors.Is(err, config.ErrNotFound) && notify != nil {
		_, _ = notify.Notify(browserapi.LevelWarn,
			"extensions.zig.config.build_on_save must be a bool: %v", err)
	}
	if vs, err := cfg.GetSlice("build_on_save_args"); err == nil {
		for _, v := range vs {
			s, ok := v.(string)
			if !ok {
				if notify != nil {
					_, _ = notify.Notify(browserapi.LevelWarn,
						"extensions.zig.config.build_on_save_args must be a list of strings")
				}
				bos.Args = nil
				break
			}
			bos.Args = append(bos.Args, s)
		}
	}
	return bos
}

// warnMissingCheckStep surfaces why build-on-save diagnostics are
// inactive: with no explicit build_on_save setting, zls only runs the
// build after saves when build.zig declares a "check" step. An explicit
// setting (either way) silences the hint, as does a missing build.zig
// (the bare-.zig fallback has no build graph to run).
func warnMissingCheckStep(
	fs workspaceapi.FileSystem,
	notify browserapi.Notifications,
	root langext.Root,
	bos buildOnSaveOptions,
) {
	if bos.Enable != nil || notify == nil {
		return
	}
	content, err := readWorkspaceFile(fs, filepath.Join(root.Dir, "build.zig"))
	if err != nil {
		return
	}
	if hasCheckStep(content) {
		return
	}
	_, _ = notify.NotifyOnce(browserapi.LevelInfo,
		"zls build-on-save diagnostics are inactive: declare a \"check\" step "+
			"in build.zig (b.step(\"check\", ...)) or set "+
			"extensions.zig.config.build_on_save")
}

// hasCheckStep reports whether build.zig content declares a top-level
// "check" step, which zls uses to auto-enable build-on-save.
func hasCheckStep(buildZig []byte) bool {
	return strings.Contains(string(buildZig), `step("check"`)
}

func readWorkspaceFile(fs workspaceapi.FileSystem, path string) ([]byte, error) {
	f, err := fs.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}
