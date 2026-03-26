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
		FixPrompt:   `You are a code migration agent. A new file claude.go was added to the workspace providing FetchClaudeDialogue. No existing files need changes — just verify everything compiles.

After checking:
1. Run: run_command go build ./...
2. Run: run_command go test ./...
3. Only report success when both pass.`,
		Description: "Added claude.go with FetchClaudeDialogue for Claude Code conversation sources",
	},
}

func init() {
	for v := 2; v <= templateVersion; v++ {
		if _, ok := migrations[v]; !ok {
			panic(fmt.Sprintf("dream: missing migration for version %d", v))
		}
	}
}
