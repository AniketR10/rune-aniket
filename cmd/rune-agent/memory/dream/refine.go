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

package dream

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguemanager"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
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
