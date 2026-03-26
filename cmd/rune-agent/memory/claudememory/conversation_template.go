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
