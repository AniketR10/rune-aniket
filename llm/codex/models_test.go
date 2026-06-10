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


package codex

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlagshipModelInCatalog(t *testing.T) {
	_, ok := AvailableModels()[FlagshipModel()]
	assert.True(t, ok, "flagship %q must be in the catalog", FlagshipModel())
	assert.Equal(t, GPT5Dot5, FlagshipModel())
}

func TestMaxOutputTokens(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{GPT5Dot5, 128000},
		{GPT5Dot4, 128000},
		{GPT5Dot3Codex, 128000},
		{CodexAutoReview, 0},
		{"unknown-model", 0},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, MaxOutputTokens(tt.model))
		})
	}
}

func TestModelEntries(t *testing.T) {
	entries := ModelEntries()
	byName := make(map[string]int, len(entries))
	for _, e := range entries {
		byName[e.Name] = e.ContextWindow
		assert.Equal(t, LLMProvider, e.Provider)
		assert.Equal(t, OpenAICompatibleURL, e.BaseURL)
	}
	// Catalog names are bare upstream slugs; the router disambiguates
	// duplicates between providers via ModelEntry.Provider.
	assert.Equal(t, "gpt-5.4", GPT5Dot4)
	assert.Equal(t, 1000000, byName["gpt-5.4"])
	assert.Equal(t, 272000, byName["gpt-5.5"])
	assert.Equal(t, 1000000, byName["codex-auto-review"])
}

func TestUpstreamModelName(t *testing.T) {
	// On the rune side the catalog already holds upstream slugs so
	// UpstreamModelName is the identity function.
	assert.Equal(t, "gpt-5.4", UpstreamModelName("gpt-5.4"))
	assert.Equal(t, "unknown", UpstreamModelName("unknown"))
}
