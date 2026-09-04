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
	"context"
	"errors"
	"log/slog"
	"os"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// installer resolves executables the host provisioned alongside this
// extension on the workspace host. It is satisfied by
// *extensionapi.Workspace.
type installer interface {
	FindInstalledExecutable(ctx context.Context, name string) (string, error)
}

// resolvePyTool locates a Python toolchain binary (ty, ruff, uv)
// provisioned under the install root's bin/ on the workspace host,
// returning its path or "" when absent. Unlike the Go extension, it does
// not probe well-known dirs or the shell: Python tooling lacks the
// cross-version compatibility guarantees that make a loosely-resolved
// binary safe, so the caller falls back to the bare command name and
// lets the executor resolve it through $PATH. Resolution goes through
// the installer so file:// and ssh:// workspaces both find the
// provisioned binary.
func resolvePyTool(
	ctx context.Context,
	_ workspaceapi.FileSystem,
	_ workspaceapi.Executor,
	inst installer, name string,
) string {
	bin, err := inst.FindInstalledExecutable(ctx, name)
	if err == nil {
		return bin
	}
	if !errors.Is(err, os.ErrNotExist) {
		slog.Warn("probe provisioned tool failed", "tool", name, "error", err)
	}
	return ""
}
