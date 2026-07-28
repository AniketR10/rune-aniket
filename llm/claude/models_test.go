// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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

package claude

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/llm/anthropic"
)

func TestFlagshipModelIsOpus5(t *testing.T) {
	assert.Equal(t, anthropic.ClaudeOpus5, FlagshipModel())
}

func TestMaxOutputTokensMirrorsAnthropic(t *testing.T) {
	model := anthropic.ClaudeOpus5
	assert.Equal(t, anthropic.MaxOutputTokens(model), MaxOutputTokens(model))
	assert.Equal(t, 128000, MaxOutputTokens(model))
	assert.Equal(t, 0, MaxOutputTokens("unknown-model"))
}

func TestModelEntriesStampClaudeProvider(t *testing.T) {
	entries := ModelEntries()
	require.NotEmpty(t, entries)

	gotSlugs := make(map[string]bool, len(entries))
	var opus5ContextWindow int
	var sonnet5ContextWindow int
	for _, e := range entries {
		assert.Equal(t, LLMProvider, e.Provider, "entry %q not stamped claude", e.Name)
		gotSlugs[e.Name] = true
		if e.Name == anthropic.ClaudeOpus5 {
			opus5ContextWindow = e.ContextWindow
		}
		if e.Name == anthropic.ClaudeSonnet5 {
			sonnet5ContextWindow = e.ContextWindow
		}
	}
	assert.Equal(t, 1000000, opus5ContextWindow)
	assert.Equal(t, 1000000, sonnet5ContextWindow)

	wantSlugs := make(map[string]bool)
	for _, e := range anthropic.ModelEntries() {
		wantSlugs[e.Name] = true
	}
	assert.Equal(t, wantSlugs, gotSlugs, "claude slugs must equal anthropic slugs")
}
