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

package agentools

import (
	"context"
	"fmt"
	"runtime"

	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/workspace/walkdir"
)

const maxAgentWalkdirWorkers = 4

func boundedWalkdirContext(ctx context.Context) context.Context {
	workers := max(min(runtime.NumCPU(), maxAgentWalkdirWorkers), 1)
	return walkdir.ContextWithWorkerCount(ctx, workers)
}

// listToolFiles lists the files a search-style tool should traverse
// under root, relative to the workspace root cwd. walkdir.ListFiles
// replaces a file path with its nearest parent directory, which path
// completion relies on but which would make a tool search a whole tree
// the caller never named.
func listToolFiles(
	ctx context.Context, fs workspaceapi.FileSystem, cwd workspaceapi.URI,
	root string, filter walkdir.Filter,
) (iterator.Iterator[string], error) {
	info, err := fs.Stat(root)
	if err != nil || !info.Mode().IsRegular() {
		return walkdir.ListFiles(ctx, fs, root)
	}
	rootURI, err := fs.URI(root)
	if err != nil {
		return nil, fmt.Errorf("URI: %v", err)
	}
	rel := workspaceapi.RelPath(cwd, rootURI)
	if filter != nil && filter.MatchRelPath(rel, false) {
		return iterator.Empty[string](), nil
	}
	return iterator.FromSlice([]string{rel}), nil
}

// walkIterErr classifies an error surfaced by a walkdir iterator.
// Cancellation is always fatal: results collected before the abort must
// not render as a successful response. Per-file and per-directory
// failures fail the call only when nothing was found, otherwise they
// are reported next to the partial output.
func walkIterErr(ctx context.Context, err error, results int) (msg string, fatal bool) {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return fmt.Sprintf("error: canceled before completion: %v", ctxErr), true
	}
	if err == nil {
		return "", false
	}
	if results == 0 {
		return fmt.Sprintf("error: %v", err), true
	}
	return fmt.Sprintf("(warning: some paths could not be read: %v)", err), false
}
