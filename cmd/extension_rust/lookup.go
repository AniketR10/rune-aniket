// Copyright (C) 2017-2026 Unstable Build, LLC
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

const rustResolutionTimeout = 5 * time.Second

// installer resolves executables the host provisioned alongside this
// extension on the workspace host. It is satisfied by
// *extensionapi.Workspace.
type installer interface {
	FindInstalledExecutable(ctx context.Context, name string) (string, error)
}

// readLspPath reads the optional extensions.rust.config.lsp_path
// override. A missing key is not an error; a non-string value warns and
// is ignored. The returned bool reports whether a non-empty override was
// supplied.
func readLspPath(cfg config.Config, notify browserapi.Notifications) (string, bool) {
	if cfg == nil {
		return "", false
	}
	v, err := cfg.GetString("lsp_path")
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return "", false
		}
		if notify != nil {
			_, _ = notify.Notify(browserapi.LevelWarn,
				"extensions.rust.config.lsp_path must be a string: %v", err)
		}
		return "", false
	}
	return v, v != ""
}

// readExperimental reports whether extensions.rust.config.experimental is
// set. It gates rust-analyzer's experimental LSP extension subcommands and
// the matching client capabilities, so they stay off by default. A missing
// key or non-bool value resolves to false.
func readExperimental(cfg config.Config, notify browserapi.Notifications) bool {
	if cfg == nil {
		return false
	}
	v, err := cfg.GetBool("experimental")
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) && notify != nil {
			_, _ = notify.Notify(browserapi.LevelWarn,
				"extensions.rust.config.experimental must be a bool: %v", err)
		}
		return false
	}
	return v
}

// readMemoryUsage reports whether extensions.rust.config.debug.memory_usage
// is set. It gates the `memory-usage` subcommand, which only a
// rust-analyzer built with `--features dhat --profile dev-rel` answers;
// every other build rejects the request. A missing key or non-bool value
// resolves to false.
func readMemoryUsage(cfg config.Config, notify browserapi.Notifications) bool {
	if cfg == nil {
		return false
	}
	debug, err := cfg.GetConfig("debug")
	if err != nil || debug == nil {
		return false
	}
	v, err := debug.GetBool("memory_usage")
	if err != nil {
		if !errors.Is(err, config.ErrNotFound) && notify != nil {
			_, _ = notify.Notify(browserapi.LevelWarn,
				"extensions.rust.config.debug.memory_usage must be a bool: %v", err)
		}
		return false
	}
	return v
}

// resolveRustAnalyzer returns the rust-analyzer language server path. A
// configured lsp_path overrides the bundled binary, which the package
// ships under the install root's bin/ on the workspace host. Resolution
// goes through the installer so file:// and ssh:// workspaces both find
// the provisioned binary; a miss returns "" and the caller surfaces the
// initialization error.
func resolveRustAnalyzer(
	ctx context.Context,
	cfg config.Config,
	notify browserapi.Notifications,
	inst installer,
) string {
	if p, ok := readLspPath(cfg, notify); ok {
		return p
	}
	bin, err := inst.FindInstalledExecutable(ctx, "rust-analyzer")
	if err == nil {
		return bin
	}
	if !errors.Is(err, os.ErrNotExist) {
		slog.Warn("probe provisioned rust-analyzer failed", "error", err)
	}
	return ""
}

// resolveSysroot returns the active toolchain sysroot via
// `<rustcBin> --print sysroot`, or "" when the probe fails. rustcBin
// must be the absolute path to the installation-owned rustc
// ($CARGO_HOME/bin/rustc); a bare `rustc` is never used, since it could
// resolve to a system toolchain unrelated to the one our install
// manages.
func resolveSysroot(ctx context.Context, exec workspaceapi.Executor, rustcBin string) string {
	lookupCtx, cancel := context.WithTimeout(ctx, rustResolutionTimeout)
	defer cancel()
	out, err := commandOutput(lookupCtx, exec, rustcBin, "--print", "sysroot")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// commandOutput runs bin with args and returns its stdout, draining
// stderr so the process never blocks on a full pipe.
func commandOutput(
	ctx context.Context,
	exec workspaceapi.Executor,
	bin string,
	args ...string,
) (string, error) {
	var stdout, stderr bytes.Buffer
	ch := make(chan error, 1)
	cmd := workspaceapi.Cmd{
		Path:    bin,
		Args:    args,
		Stdout:  &stdout,
		Stderr:  &stderr,
		Watcher: workspaceapi.ChanProcessWatcher(ch),
	}
	if _, err := exec.Start(ctx, cmd); err != nil {
		return "", fmt.Errorf("start %s: %w", bin, err)
	}
	var runErr error
	select {
	case runErr = <-ch:
	case <-ctx.Done():
		runErr = ctx.Err()
	}
	if runErr != nil {
		return "", fmt.Errorf("%s %s: %w", bin, strings.Join(args, " "), runErr)
	}
	return stdout.String(), nil
}
