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

import "fmt"

type migration struct {
	From        int
	To          int
	FixPrompt   string
	Description string
}

var migrations = map[int]migration{
	2: {
		From: 1, To: 2,
		FixPrompt: `You are a code migration agent. The Memory interface in the workspace was updated to include:

    FetchConversation(ctx context.Context) (Dialogue, error)

Existing memory files need to implement this method. For each memory struct that
doesn't have it, add:

    func (MyStruct) FetchConversation(ctx context.Context) (Dialogue, error) {
        return Dialogue{}, nil
    }

Add "context" to the import block if not already present.

After fixing all files:
1. Run: run_command go build ./...
2. Run: run_command go test ./...
3. Only report success when both pass.`,
		Description: "Added FetchConversation to Memory interface, conversation.go, rune-go-sdk dep",
	},
	3: {
		From: 2, To: 3,
		FixPrompt: `You are a code migration agent. A new file claude.go was added to the workspace providing FetchClaudeDialogue. No existing files need changes — just verify everything compiles.

After checking:
1. Run: run_command go build ./...
2. Run: run_command go test ./...
3. Only report success when both pass.`,
		Description: "Added claude.go with FetchClaudeDialogue for Claude Code conversation sources",
	},
	4: {
		From: 3, To: 4,
		FixPrompt: `You are a code migration agent. The workspace gained an optional Scope dimension:

    type Scope struct {
        Workspaces []string
        Repos      []string
        Languages  []string
        PathGlobs  []string
    }

    type ScopedMemory interface {
        Scope() Scope
    }

The change is additive: memories that do not implement ScopedMemory are unscoped
and surface anywhere. main.go now accepts -workspace-root and -workspaces flags
and filters scoped memories whose Workspaces are disjoint from the caller's set.

No memory file requires changes for the workspace to compile. As an optional
improvement, you MAY add a Scope() method to existing memories when the scope
can be inferred from the memory's Content() or its TriggerOn().Files (e.g.
memories about repository-specific code paths). Do NOT modify Content() or ID().

After any edits:
1. Run: run_command go build ./...
2. Run: run_command go test ./...
3. Only report success when both pass.`,
		Description: "Added optional Scope dimension and workspace-aware recall filter",
	},
}

func init() {
	for v := 2; v <= templateVersion; v++ {
		if _, ok := migrations[v]; !ok {
			panic(fmt.Sprintf("dream: missing migration for version %d", v))
		}
	}
}
