// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.

package agentshell

import (
	"context"

	"unstable.build/go-tui/cmd/rune-agent/configedit"
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
	return cfg.SetInt(ctx, maxTokensKey, n)
}

// addSkillDir appends dir to the global "skills" sequence. Returns
// configedit.ErrAlreadyPresent when dir is already configured.
func addSkillDir(ctx context.Context, cfg configedit.Setter, dir string) error {
	return cfg.AppendStringSlice(ctx, skillsKey, dir)
}

// removeSkillDir removes dir from the "skills" sequence. Returns
// configedit.ErrNotPresent when dir is not configured.
func removeSkillDir(ctx context.Context, cfg configedit.Setter, dir string) error {
	return cfg.RemoveStringSlice(ctx, skillsKey, dir)
}
