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

package agentshell

import (
	"context"

	"unstable.build/rune/cmd/rune-agent/configedit"
)

// Domain-specific configuration keys owned by agentshell. The
// configedit package is intentionally key-agnostic; these helpers
// translate shell commands into generic configedit.Setter calls.
const (
	maxTokensKey = "max_tokens"
	skillsKey    = "skills"
)

// setMaxTokens persists the global max_tokens setting via cfg and
// updates the in-memory overlay so subsequent
// cfg.GetInt(maxTokensKey).Resolve(ctx) calls observe the new value.
func setMaxTokens(ctx context.Context, cfg configedit.Setter, n int) error {
	return cfg.SetInt(ctx, maxTokensKey, n, false)
}

// addSkillDir appends dir to the global "skills" sequence. Returns
// configedit.ErrAlreadyPresent when dir is already configured.
func addSkillDir(ctx context.Context, cfg configedit.Setter, dir string) error {
	return cfg.AppendStringSlice(ctx, skillsKey, dir, false)
}

// removeSkillDir removes dir from the "skills" sequence. Returns
// configedit.ErrNotPresent when dir is not configured.
func removeSkillDir(ctx context.Context, cfg configedit.Setter, dir string) error {
	return cfg.RemoveStringSlice(ctx, skillsKey, dir, false)
}
