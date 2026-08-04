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

const zigResolutionTimeout = 5 * time.Second

// installer resolves executables the host provisioned alongside this
// extension on the workspace host. It is satisfied by
// *extensionapi.Workspace.
type installer interface {
	FindInstalledExecutable(ctx context.Context, name string) (string, error)
}

var wellKnownZlsPaths = []string{
	"~/.rune/bin/zls",
	"/opt/homebrew/bin/zls",
	"/usr/local/bin/zls",
}

var wellKnownZigPaths = []string{
	"~/.rune/bin/zig",
	"/opt/homebrew/bin/zig",
	"/usr/local/bin/zig",
}

// readLspPath reads the optional extensions.zig.config.lsp_path
// override. A missing key is not an error; a non-string value warns and
// is ignored. The returned bool reports whether a non-empty override was
// supplied.
func readLspPath(cfg config.Config, notify browserapi.Notifications) (string, bool) {
	return readPathKey(cfg, notify, "lsp_path")
}

// readZigPath reads the optional extensions.zig.config.zig_path
// override for the zig compiler binary.
func readZigPath(cfg config.Config, notify browserapi.Notifications) (string, bool) {
	return readPathKey(cfg, notify, "zig_path")
}

func readPathKey(
	cfg config.Config, notify browserapi.Notifications, key string,
) (string, bool) {
	if cfg == nil {
		return "", false
	}
	v, err := cfg.GetString(key)
	if err != nil {
		if errors.Is(err, config.ErrNotFound) {
			return "", false
		}
		if notify != nil {
			_, _ = notify.Notify(browserapi.LevelWarn,
				"extensions.zig.config.%s must be a string: %v", key, err)
		}
		return "", false
	}
	return v, v != ""
}

// resolveZls returns the zls language server path. A configured lsp_path
// overrides discovery; otherwise the host is probed: the provisioned
// binary first, then well-known install locations, then a login-shell
// `command -v` lookup. A miss warns and returns "" so the caller
// surfaces the initialization error.
func resolveZls(
	ctx context.Context,
	cfg config.Config,
	notify browserapi.Notifications,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	inst installer,
) string {
	if p, ok := readLspPath(cfg, notify); ok {
		return p
	}
	bin, err := resolveBinary(ctx, fs, exec, inst, "zls", wellKnownZlsPaths)
	if err == nil {
		return bin
	}
	if notify != nil {
		_, _ = notify.Notify(browserapi.LevelWarn,
			"We could not locate the zls executable, please set the "+
				"extensions.zig.config.lsp_path property in your config and "+
				"reload the workspace")
	}
	return ""
}

// resolveZig returns the zig compiler path forwarded to zls as
// zig_exe_path. A configured zig_path overrides discovery. An empty
// result is acceptable: zls then falls back to its own PATH lookup.
func resolveZig(
	ctx context.Context,
	cfg config.Config,
	notify browserapi.Notifications,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	inst installer,
) string {
	if p, ok := readZigPath(cfg, notify); ok {
		return p
	}
	bin, err := resolveBinary(ctx, fs, exec, inst, "zig", wellKnownZigPaths)
	if err != nil {
		return ""
	}
	return bin
}

// resolveBinary probes for name on the workspace host: the provisioned
// install first, then the well-known locations, then a shell lookup.
func resolveBinary(
	ctx context.Context,
	fs workspaceapi.FileSystem,
	exec workspaceapi.Executor,
	inst installer,
	name string,
	wellKnown []string,
) (string, error) {
	// A miss (os.ErrNotExist) or a probe failure both fall through to the
	// well-known and shell candidates below, which is the whole point of
	// this resolver having fallbacks.
	if inst != nil {
		if bin, err := inst.FindInstalledExecutable(ctx, name); err == nil {
			return bin, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			slog.Debug("probe provisioned binary failed", "name", name, "error", err)
		}
	}

	if bin, ok := probeWellKnown(fs, wellKnown); ok {
		return bin, nil
	}

	ctx, cancel := context.WithTimeout(ctx, zigResolutionTimeout)
	defer cancel()
	if bin, err := probeShellLookup(ctx, exec, name); err == nil {
		return bin, nil
	}

	return "", fmt.Errorf("%s binary not found on workspace host", name)
}

func probeWellKnown(fs workspaceapi.FileSystem, candidates []string) (string, bool) {
	for _, candidate := range candidates {
		uri, err := fs.URI(candidate)
		if err != nil {
			continue
		}
		full := uri.Path()
		info, err := fs.Stat(full)
		if err != nil || info == nil || info.IsDir() {
			continue
		}
		return full, true
	}
	return "", false
}

func probeShellLookup(
	ctx context.Context, exec workspaceapi.Executor, name string,
) (string, error) {
	out, err := commandOutput(ctx, exec, "sh", "-lc", "command -v "+name)
	if err != nil {
		return "", err
	}
	out = strings.TrimSpace(out)
	// command -v may print multiple lines for aliases/functions; take
	// the first absolute path it printed.
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/") {
			return line, nil
		}
	}
	return "", fmt.Errorf("shell probe produced no absolute path: %q", out)
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
