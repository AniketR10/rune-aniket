// Copyright (C) 2017-2026 The Rune Authors
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
