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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSystemPrompt(t *testing.T) {
	t.Run("contains data path", func(t *testing.T) {
		prompt := systemPrompt("/tmp/memories", nil, "", "")
		assert.Contains(t, prompt, "/tmp/memories")
	})

	t.Run("contains key instructions", func(t *testing.T) {
		prompt := systemPrompt("/tmp/memories", nil, "", "")
		assert.Contains(t, prompt, "go build")
		assert.Contains(t, prompt, "go test")
		assert.Contains(t, prompt, "ID()")
		assert.Contains(t, prompt, "Content()")
		assert.Contains(t, prompt, "Register")
		assert.Contains(t, prompt, "init()")
	})

	t.Run("includes existing categories", func(t *testing.T) {
		cats := []string{"GoIdiomMemory", "BugFixMemory"}
		prompt := systemPrompt("/tmp/memories", cats, "", "")
		assert.Contains(t, prompt, "GoIdiomMemory")
		assert.Contains(t, prompt, "BugFixMemory")
		assert.Contains(t, prompt, "Existing Categories")
	})

	t.Run("omits categories section when empty", func(t *testing.T) {
		prompt := systemPrompt("/tmp/memories", nil, "", "")
		assert.NotContains(t, prompt, "Existing Categories")
	})

	t.Run("uses default source dialogue section", func(t *testing.T) {
		prompt := systemPrompt("/tmp/memories", nil, "", "")
		assert.Contains(t, prompt, "## Source Dialogue Tracking")
		assert.Contains(t, prompt, "FetchDialogue(ctx,")
	})

	t.Run("replaces source dialogue section", func(t *testing.T) {
		custom := "## Custom Source\n\nUse FetchCustom(ctx, id)."
		prompt := systemPrompt("/tmp/memories", nil, custom, "")
		assert.Contains(t, prompt, "## Custom Source")
		assert.NotContains(t, prompt, "## Source Dialogue Tracking")
	})

	t.Run("replaces fetch helper in examples", func(t *testing.T) {
		prompt := systemPrompt("/tmp/memories", nil, "", "FetchClaudeDialogue")
		assert.Contains(t, prompt, "FetchClaudeDialogue(ctx,")
		assert.NotContains(t, prompt, "FetchDialogue(ctx,")
	})

	t.Run("custom source and helper together", func(t *testing.T) {
		custom := "## Claude Source\n\nUse FetchClaudeDialogue."
		prompt := systemPrompt("/tmp/memories", nil, custom, "FetchClaudeDialogue")
		assert.Contains(t, prompt, "## Claude Source")
		assert.Contains(t, prompt, "FetchClaudeDialogue(ctx,")
		assert.NotContains(t, prompt, "FetchDialogue(ctx,")
	})
}
