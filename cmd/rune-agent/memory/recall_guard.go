// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package memory

import (
	"context"
	"errors"
	"log/slog"

	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/configedit"
)

// memoryRecallEnabledKey is the configedit key consulted by the
// GuardedRecaller. true = always allow memory recall; false = always
// skip memory recall; absent = ask the user the first time a recall
// would surface at least one memory.
const memoryRecallEnabledKey = "memory_recall_enabled"

// GuardedRecaller wraps an inner agent.MemoryRecaller with a
// configurable on/off switch and a one-time prompt that records the
// user's preference.
//
// Behavior on each Recall call:
//   - If memory_recall_enabled is true: delegate to the inner recaller.
//   - If memory_recall_enabled is false: return (nil, nil) without
//     calling the inner recaller.
//   - If absent: call the inner recaller; if it returns no memories
//     or an error, return as-is without prompting. Otherwise prompt
//     the user with Yes / Always / No / Never. Always/Never are
//     persisted to .rune/config.yaml; Yes/No are one-shot decisions.
//
// enabled is the deferred configedit.Bool view of memory_recall_enabled.
// It is built once at construction time and resolved per call so writes
// through cfg.SetBool (Always/Never choices) are observed immediately in
// the same session.
type GuardedRecaller struct {
	inner   agent.MemoryRecaller
	cfg     configedit.Config
	enabled configedit.Bool
}

// NewGuardedRecaller returns a GuardedRecaller wired to inner and cfg.
// Panics if inner or cfg is nil.
func NewGuardedRecaller(
	inner agent.MemoryRecaller, cfg configedit.Config,
) *GuardedRecaller {
	if inner == nil {
		panic("memory: GuardedRecaller inner must not be nil")
	}
	if cfg == nil {
		panic("memory: GuardedRecaller cfg must not be nil")
	}
	return &GuardedRecaller{
		inner:   inner,
		cfg:     cfg,
		enabled: cfg.GetBool(memoryRecallEnabledKey),
	}
}

// Recall implements agent.MemoryRecaller.
func (g *GuardedRecaller) Recall(
	ctx context.Context, files []string, task string, workspaceRoot string,
) ([]agent.Memory, error) {
	v, err := g.enabled.Resolve(ctx)
	switch {
	case err == nil:
		if v {
			return g.inner.Recall(ctx, files, task, workspaceRoot)
		}
		return nil, nil
	case errors.Is(err, configedit.ErrNotFound):
		// fall through to prompt
	default:
		slog.Warn("get memory_recall_enabled from config", "error", err)
		// fall through; default to attempting recall
	}

	mems, err := g.inner.Recall(ctx, files, task, workspaceRoot)
	if err != nil || len(mems) == 0 {
		return mems, err
	}

	prompter := agent.PrompterFromContext(ctx)
	if prompter == nil {
		// No prompter configured (tests, e2e tools): default-allow so
		// memories are not silently dropped.
		return mems, nil
	}

	resp, err := prompter.Prompt(ctx, agent.PromptRequest{
		Title:  "Allow memory recall?",
		Header: "memory",
		Body: "Rune Agent has compiled memories from past sessions in " +
			"this workspace. Memories surface architecture decisions, " +
			"user preferences, and episodic context that may be " +
			"relevant to your request. Include them in this turn?",
		Options: []agent.PromptOption{
			{Value: "yes", Label: "Yes",
				Description: "Use memories for this call"},
			{Value: "always", Label: "Always",
				Description: "Always use memories; remember the choice"},
			{Value: "no", Label: "No",
				Description: "Skip memories for this call"},
			{Value: "never", Label: "Never",
				Description: "Always skip memories; remember the choice"},
		},
	})
	if err != nil {
		// Dismissed: treat as "no" for this call; do not persist.
		return nil, nil
	}
	var choice string
	if len(resp.Values) > 0 {
		choice = resp.Values[0]
	}
	switch choice {
	case "yes":
		return mems, nil
	case "always":
		if err := setMemoryRecallEnabled(ctx, g.cfg, true, false); err != nil {
			slog.Warn("persist memory_recall_enabled=true", "error", err)
		}
		return mems, nil
	case "no":
		return nil, nil
	case "never":
		if err := setMemoryRecallEnabled(ctx, g.cfg, false, false); err != nil {
			slog.Warn("persist memory_recall_enabled=false", "error", err)
		}
		return nil, nil
	default:
		// Unknown response: treat as "no" for this call.
		return nil, nil
	}
}

// setMemoryRecallEnabled updates memory_recall_enabled via cfg.
// When ephemeral is false the value is persisted to .rune/config.yaml;
// when true it is only written to the in-memory overlay and lost on
// restart. Subsequent Resolve calls on the corresponding
// configedit.Bool observe the new value either way.
func setMemoryRecallEnabled(
	ctx context.Context, cfg configedit.Setter, v, ephemeral bool,
) error {
	return cfg.SetBool(ctx, memoryRecallEnabledKey, v, ephemeral)
}
