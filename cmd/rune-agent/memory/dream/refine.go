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

package dream

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/rune/cmd/rune-agent/dialogue/dialoguemanager"
)

const (
	memoryContextOpenTag  = "<memory-context>"
	memoryContextCloseTag = "</memory-context>"
)

// buildRefineUserPrompt collects the dialogues processed in this epoch
// that contain at least one <memory-context> block and packages them
// for the refine agent. Returns "" when there is no evidence to act on,
// causing NewAgentPhase to skip the phase silently.
func buildRefineUserPrompt(ctx context.Context, deps Deps) (string, error) {
	stateStore := newDreamState(deps.Storage)
	state, err := stateStore.load(ctx)
	if err != nil {
		return "", fmt.Errorf("load state: %w", err)
	}

	ids := make([]string, 0, len(state.Dreamed))
	for id := range state.Dreamed {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	var b strings.Builder
	evidence := 0
	for _, id := range ids {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		d, err := deps.Store.Get(ctx, id)
		if errors.Is(err, storageapi.ErrNotFound) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("get dialogue %s: %w", id, err)
		}
		if !dialogueHasMemoryContext(d) {
			continue
		}
		fmt.Fprintf(&b, "### Dialogue %s\n\n%s\n", d.ID, formatTranscript(d))
		evidence++
	}

	if evidence == 0 {
		return "", nil
	}

	var prompt strings.Builder
	fmt.Fprintf(&prompt,
		"Review the following %d dialogues. For each, locate the "+
			"<memory-context> block(s) in the user messages and examine how "+
			"the surfaced memories were actually used in the rest of the "+
			"conversation. Then improve the recall machinery (main.go, "+
			"categories.go, per-memory predicates) following the rules in "+
			"your system prompt.\n\n",
		evidence)
	prompt.WriteString(b.String())
	return prompt.String(), nil
}

// dialogueHasMemoryContext reports whether any user message in the
// dialogue carries a <memory-context> block.
func dialogueHasMemoryContext(d dialoguemanager.Dialogue) bool {
	for _, msg := range d.Messages {
		if msg.Role != llmapi.RoleUser {
			continue
		}
		if strings.Contains(msg.Content, memoryContextOpenTag) &&
			strings.Contains(msg.Content, memoryContextCloseTag) {
			return true
		}
	}
	return false
}
