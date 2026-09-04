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

package ide

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
)

// watchedFilesTelemetryLSP reports the number of files carried by each
// workspace/didChangeWatchedFiles notification before delegating. Only the
// agent-facing RPC seam is wrapped, so filesystem watcher events — which
// reach the language servers through idelsp's own event handler — are never
// reported.
type watchedFilesTelemetryLSP struct {
	semanticapi.LSP
	onChange func(n int)
}

func (w watchedFilesTelemetryLSP) DidChangeWatchedFiles(
	ctx context.Context, params semanticapi.DidChangeWatchedFilesParams,
) error {
	if w.onChange != nil {
		w.onChange(len(params.Changes))
	}
	return w.LSP.DidChangeWatchedFiles(ctx, params)
}
