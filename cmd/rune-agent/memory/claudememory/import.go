// Copyright (C) 2017-2026 The Rune Authors
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

package claudememory

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/iterator"
	"unstable.build/rune/cmd/rune-agent/memory/dream"
)

// Import scans Claude Code conversations at claudeHome and runs the
// dream memory-extraction pipeline on unprocessed sessions.
func Import(
	ctx context.Context, claudeHome string, deps dream.Deps,
) (iterator.Iterator[dream.Progress], error) {
	deps.Store = NewStore(claudeHome)
	deps.SourceDialoguePrompt = claudeSourceDialoguePrompt
	deps.FetchHelper = claudeFetchHelper
	return dream.Dream(ctx, deps)
}
