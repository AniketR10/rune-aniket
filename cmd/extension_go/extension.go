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
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/extensionapi"
	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
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

	dbg := readGoplsDebugOptions(cfg)
	goplsBin := resolveGoplsForWorkspace(ctx, w, cfg, notify, cwd.Scheme())
	params, err := goplsInitializeParams(rootURI, dbg, goplsBin)
	if err != nil {
		return fmt.Errorf("build init params: %w", err)
	}
	_, err = lsp.Initialize(ctx, params)
	if err != nil {
		return fmt.Errorf("initialize gopls: %w", err)
	}
	slog.Info("gopls initialized",
		"rpc_trace", dbg.RPCTrace,
		"logfile", dbg.LogFile,
		"debug_addr", dbg.DebugAddr,
		"trace", string(dbg.Trace),
	)

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

// readGoplsDebugOptions extracts the optional `debug` sub-config of the
// Go extension. Missing keys leave the corresponding field at its zero
// value (debugging disabled). Unknown values for `trace` fall back to
// "off" so a typo cannot crash the workspace bring-up.
//
// Example rune.star:
//
//	"extensions": {
//	    "go": {
//	        "path": "extension_go",
//	        "config": {
//	            "lsp_path": "/usr/local/bin/gopls",
//	            "debug": {
//	                "rpc_trace": True,
//	                "logfile":   "~/.rune/logs/lsp/gopls.log",
//	                "addr":      "localhost:6060",
//	                "trace":     "verbose",
//	            },
//	        },
//	    },
//	}
func readGoplsDebugOptions(cfg config.Config) goplsDebugOptions {
	var opts goplsDebugOptions
	if cfg == nil {
		return opts
	}
	dbg, err := cfg.GetConfig("debug")
	if err != nil || dbg == nil {
		return opts
	}
	if v, err := dbg.GetBool("rpc_trace"); err == nil {
		opts.RPCTrace = v
	}
	if v, err := dbg.GetString("logfile"); err == nil {
		resolved, rerr := resolveLogFile(v)
		if rerr != nil {
			slog.Warn("gopls debug logfile disabled",
				"logfile", v, "error", rerr)
		} else {
			opts.LogFile = resolved
		}
	}
	if v, err := dbg.GetString("addr"); err == nil {
		opts.DebugAddr = v
	}
	if v, err := dbg.GetString("trace"); err == nil {
		switch semanticapi.TraceValue(v) {
		case semanticapi.TraceValueOff,
			semanticapi.TraceValueMessages,
			semanticapi.TraceValueVerbose:
			opts.Trace = semanticapi.TraceValue(v)
		}
	}
	return opts
}

// resolveLogFile expands a leading "~" / "~/" against $HOME and ensures
// the parent directory exists. gopls exits with status 2 when it cannot
// create the file passed to `-logfile`, so doing this here keeps a
// missing directory from bricking workspace bring-up. The literal
// "auto" is passed through unchanged; gopls itself maps it to a
// per-pid file under $TMPDIR.
func resolveLogFile(path string) (string, error) {
	if path == "" || path == "auto" {
		return path, nil
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~"))
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create logfile dir: %w", err)
	}
	return path, nil
}

// readGoplsLspPath reads the optional top-level `lsp_path` config key.
// Validation: must be a non-empty string with no spaces (the LSP
// command is split on space, so spaces would corrupt argv). Failures
// surface as a warn notification so the user gets a clear error
// instead of an obscure runtime failure later on.
func readGoplsLspPath(cfg config.Config, notify browserapi.Notifications) string {
	if cfg == nil {
		return ""
	}
	v, err := cfg.GetString("lsp_path")
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return ""
		}
		if notify != nil {
			_, _ = notify.Notify(browserapi.LevelWarn,
				"extensions.go.config.lsp_path must be a string: %v", err)
		}
		return ""
	}
	if v == "" {
		return ""
	}
	if strings.ContainsRune(v, ' ') {
		if notify != nil {
			_, _ = notify.Notify(browserapi.LevelWarn,
				"extensions.go.config.lsp_path must be an absolute path "+
					"without spaces, got %q", v)
		}
		return ""
	}
	return v
}

// resolveGoplsForWorkspace orchestrates gopls binary resolution on the
// workspace host. It returns an absolute path on success or the empty
// string when the workspace has no Go project files (in which case
// gopls is never started) or every resolution strategy failed (in
// which case a warn notification has been emitted and the caller
// falls back to the bare "gopls" command).
func resolveGoplsForWorkspace(
	ctx context.Context,
	w *extensionapi.Workspace,
	cfg config.Config,
	notify browserapi.Notifications,
	scheme string,
) string {
	fs := w.FileSystem(ctx)
	if !hasGoProjectFiles(ctx, fs) {
		return ""
	}
	lspPath := readGoplsLspPath(cfg, notify)
	bin, err := resolveGoplsBinary(ctx, fs, w.Executor(ctx), lspPath)
	if err == nil {
		return bin
	}
	msg := "We could not locate the gopls executable, please set the " +
		"extensions.go.config.lsp_path property in your config and " +
		"reload the workspace"
	if scheme == "file" {
		msg = "We could not locate the gopls executable, please " +
			"reinstall the go extension"
	}
	if notify != nil {
		_, _ = notify.Notify(browserapi.LevelWarn, msg)
	}
	return ""
}
