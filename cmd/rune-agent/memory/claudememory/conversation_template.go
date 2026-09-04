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

package claudememory

// claudeFetchHelper is the helper function name used in FetchConversation
// implementations for Claude Code-sourced memories.
const claudeFetchHelper = "FetchClaudeDialogue"

// claudeSourceDialoguePrompt is the "Source Dialogue Tracking" section
// for conversations imported from Claude Code history.
var claudeSourceDialoguePrompt = `## Source Dialogue Tracking

Every memory MUST implement FetchConversation to retrieve the originating
conversation. The conversations were imported from Claude Code history.
The source dialogue ID is provided at the top of the transcript (format: "project/session").

Use the helper function FetchClaudeDialogue:

` + "```go" + `
func (m MyMemory) FetchConversation(ctx context.Context) (Dialogue, error) {
	return FetchClaudeDialogue(ctx, "<source-dialogue-id>")
}
` + "```" + `

Replace <source-dialogue-id> with the actual source dialogue ID from the transcript header.`
