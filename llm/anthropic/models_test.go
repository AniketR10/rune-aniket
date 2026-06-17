// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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

package anthropic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlagshipModelInCatalog(t *testing.T) {
	_, ok := AvailableModels()[FlagshipModel()]
	assert.True(t, ok, "flagship %q must be in the catalog", FlagshipModel())
	assert.Equal(t, ClaudeOpus4Dot8, FlagshipModel())
}

func TestMaxOutputTokens(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{ClaudeFable5, 128000},
		{ClaudeSonnet4Dot5, 64000},
		{ClaudeOpus4, 32000},
		{ClaudeHaiku3, 4096},
		{"unknown-model", 0},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, MaxOutputTokens(tt.model))
		})
	}
}

func TestSupportsEffort(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{ClaudeOpus4Dot7, true},
		{ClaudeOpus4Dot6, true},
		{ClaudeSonnet4Dot6, true},
		{ClaudeHaiku4Dot5, true},
		{ClaudeOpus4Dot5, true},
		{ClaudeSonnet4Dot5, true},
		{ClaudeOpus4Dot1, true},
		{ClaudeSonnet4, true},
		{ClaudeOpus4, true},
		{ClaudeOpus3, false},
		{ClaudeSonnet3, false},
		{ClaudeHaiku3, false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, SupportsEffort(tt.model))
		})
	}
}

func TestSupportsAdaptiveThinking(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{ClaudeOpus4Dot6, true},
		{ClaudeSonnet4Dot6, true},
		{ClaudeHaiku4Dot5, false},
		{ClaudeOpus4Dot5, false},
		{ClaudeSonnet4Dot5, false},
		{ClaudeOpus4Dot1, false},
		{ClaudeOpus3, false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, SupportsAdaptiveThinking(tt.model))
		})
	}
}

func TestNormalizeEffort(t *testing.T) {
	tests := []struct {
		name       string
		model      string
		effort     string
		wantEffort string
		wantWarn   bool
	}{
		// Empty effort always passes through.
		{"empty effort", ClaudeOpus4Dot6, "", "", false},

		// Claude 3 models: effort is dropped with warning.
		{"claude-3 opus with high", ClaudeOpus3, "high", "", true},
		{"claude-3 sonnet with low", ClaudeSonnet3, "low", "", true},
		{"claude-3 haiku with medium", ClaudeHaiku3, "medium", "", true},

		// Claude 4.0–4.5: low/medium/high are supported.
		{"opus-4 low", ClaudeOpus4, "low", "low", false},
		{"opus-4 medium", ClaudeOpus4, "medium", "medium", false},
		{"opus-4 high", ClaudeOpus4, "high", "high", false},
		{"sonnet-4.5 low", ClaudeSonnet4Dot5, "low", "low", false},
		{"opus-4.5 high", ClaudeOpus4Dot5, "high", "high", false},
		{"haiku-4.5 medium", ClaudeHaiku4Dot5, "medium", "medium", false},

		// Claude 4.0–4.5: unsupported levels are dropped with warning.
		{"opus-4 none", ClaudeOpus4, "none", "", true},
		{"opus-4 minimal", ClaudeOpus4, "minimal", "", true},
		{"opus-4 xhigh", ClaudeOpus4, "xhigh", "", true},
		{"opus-4 max", ClaudeOpus4, "max", "", true},
		{"sonnet-4.5 xhigh", ClaudeSonnet4Dot5, "xhigh", "", true},
		{"haiku-4.5 max", ClaudeHaiku4Dot5, "max", "", true},

		// Claude 4.6+: low/medium/high/xhigh/max are all supported.
		{"opus-4.6 low", ClaudeOpus4Dot6, "low", "low", false},
		{"opus-4.6 medium", ClaudeOpus4Dot6, "medium", "medium", false},
		{"opus-4.6 high", ClaudeOpus4Dot6, "high", "high", false},
		{"opus-4.6 xhigh", ClaudeOpus4Dot6, "xhigh", "xhigh", false},
		{"opus-4.6 max", ClaudeOpus4Dot6, "max", "max", false},
		{"opus-4.7 low", ClaudeOpus4Dot7, "low", "low", false},
		{"opus-4.7 medium", ClaudeOpus4Dot7, "medium", "medium", false},
		{"opus-4.7 high", ClaudeOpus4Dot7, "high", "high", false},
		{"opus-4.7 xhigh", ClaudeOpus4Dot7, "xhigh", "xhigh", false},
		{"opus-4.7 max", ClaudeOpus4Dot7, "max", "max", false},
		{"opus-4.8 low", ClaudeOpus4Dot8, "low", "low", false},
		{"opus-4.8 high", ClaudeOpus4Dot8, "high", "high", false},
		{"opus-4.8 xhigh", ClaudeOpus4Dot8, "xhigh", "xhigh", false},
		{"opus-4.8 max", ClaudeOpus4Dot8, "max", "max", false},
		{"sonnet-4.6 max", ClaudeSonnet4Dot6, "max", "max", false},
		{"sonnet-4.6 xhigh", ClaudeSonnet4Dot6, "xhigh", "xhigh", false},
		{"fable-5 low", ClaudeFable5, "low", "low", false},
		{"fable-5 high", ClaudeFable5, "high", "high", false},
		{"fable-5 xhigh", ClaudeFable5, "xhigh", "xhigh", false},
		{"fable-5 max", ClaudeFable5, "max", "max", false},

		// Claude 4.6: unsupported levels are dropped with warning.
		{"opus-4.6 none", ClaudeOpus4Dot6, "none", "", true},
		{"opus-4.6 minimal", ClaudeOpus4Dot6, "minimal", "", true},
		{"sonnet-4.6 none", ClaudeSonnet4Dot6, "none", "", true},
		{"sonnet-4.7 none", ClaudeOpus4Dot7, "none", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, warning := NormalizeEffort(tt.model, tt.effort)
			assert.Equal(t, tt.wantEffort, normalized)
			if tt.wantWarn {
				assert.NotEmpty(t, warning, "expected a warning")
				assert.Contains(t, warning, tt.model)
				assert.Contains(t, warning, tt.effort)
			} else {
				assert.Empty(t, warning, "expected no warning")
			}
		})
	}
}
