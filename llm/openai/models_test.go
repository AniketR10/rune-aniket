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


package openai

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
		{O1, 100000},
		{O3Mini, 65536},
		{GPT4o, 16384},
		{GPT4, 8192},
		{"unknown-model", 0},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, MaxOutputTokens(tt.model))
		})
	}
}

func TestIsReasoningModel(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		{"o1", true},
		{"o1-mini", true},
		{"o1-preview", true},
		{"o3", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"gpt-4", false},
		{"gpt-4-turbo", false},
		{"gpt-5", false},
		{"gpt-5-mini", false},
		{"gpt-3.5-turbo", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, IsReasoningModel(tt.model))
		})
	}
}

func TestSupportsReasoning(t *testing.T) {
	tests := []struct {
		model string
		want  bool
	}{
		// O-series models support reasoning.
		{"o1", true},
		{"o1-mini", true},
		{"o3", true},
		{"o3-mini", true},
		{"o3-pro", true},
		{"o4-mini", true},
		// GPT-5.x models support reasoning.
		{"gpt-5", true},
		{"gpt-5-mini", true},
		{"gpt-5-nano", true},
		{"gpt-5.1", true},
		{"gpt-5.2", true},
		{"gpt-5.2-pro", true},
		{"gpt-5.3-codex", true},
		{"gpt-5.3-instant", true},
		{"gpt-5.4", true},
		{"gpt-5.4-pro", true},
		{"gpt-5.4-mini", true},
		{"gpt-5.4-nano", true},
		// Older models do not support reasoning.
		{"gpt-4", false},
		{"gpt-4-turbo", false},
		{"gpt-4o", false},
		{"gpt-4.1", false},
		{"gpt-4.1-mini", false},
		{"gpt-3.5-turbo", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, SupportsReasoning(tt.model))
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
		{"empty effort", "o4-mini", "", "", false},

		// Non-reasoning models: effort is dropped silently.
		{"gpt-4 with high", "gpt-4", "high", "", false},
		{"gpt-4o with low", "gpt-4o", "low", "", false},

		// O-series: low/medium/high are supported.
		{"o4-mini low", "o4-mini", "low", "low", false},
		{"o4-mini medium", "o4-mini", "medium", "medium", false},
		{"o4-mini high", "o4-mini", "high", "high", false},
		{"o3 low", "o3", "low", "low", false},
		{"o1-mini medium", "o1-mini", "medium", "medium", false},

		// O-series: unsupported levels are dropped with warning.
		{"o4-mini none", "o4-mini", "none", "", true},
		{"o4-mini minimal", "o4-mini", "minimal", "", true},
		{"o4-mini xhigh", "o4-mini", "xhigh", "", true},
		{"o4-mini max", "o4-mini", "max", "", true},
		{"o3 max", "o3", "max", "", true},
		{"o1 xhigh", "o1", "xhigh", "", true},

		// GPT-5.4: none/low/medium/high/xhigh are supported.
		{"gpt-5.4 none", "gpt-5.4", "none", "none", false},
		{"gpt-5.4 low", "gpt-5.4", "low", "low", false},
		{"gpt-5.4 medium", "gpt-5.4", "medium", "medium", false},
		{"gpt-5.4 high", "gpt-5.4", "high", "high", false},
		{"gpt-5.4 xhigh", "gpt-5.4", "xhigh", "xhigh", false},

		// GPT-5.4: unsupported levels are dropped with warning.
		{"gpt-5.4 max", "gpt-5.4", "max", "", true},
		{"gpt-5.4 minimal", "gpt-5.4", "minimal", "", true},

		// GPT-5.4-mini: none/low/medium/high/xhigh (same as gpt-5.4).
		{"gpt-5.4-mini none", "gpt-5.4-mini", "none", "none", false},
		{"gpt-5.4-mini low", "gpt-5.4-mini", "low", "low", false},
		{"gpt-5.4-mini xhigh", "gpt-5.4-mini", "xhigh", "xhigh", false},
		{"gpt-5.4-mini max", "gpt-5.4-mini", "max", "", true},
		{"gpt-5.4-mini minimal", "gpt-5.4-mini", "minimal", "", true},

		// GPT-5.4-nano: none/low/medium/high/xhigh (same as gpt-5.4).
		{"gpt-5.4-nano none", "gpt-5.4-nano", "none", "none", false},
		{"gpt-5.4-nano xhigh", "gpt-5.4-nano", "xhigh", "xhigh", false},
		{"gpt-5.4-nano max", "gpt-5.4-nano", "max", "", true},

		// GPT-5.4-pro: medium/high/xhigh only.
		{"gpt-5.4-pro medium", "gpt-5.4-pro", "medium", "medium", false},
		{"gpt-5.4-pro xhigh", "gpt-5.4-pro", "xhigh", "xhigh", false},
		{"gpt-5.4-pro low", "gpt-5.4-pro", "low", "", true},
		{"gpt-5.4-pro none", "gpt-5.4-pro", "none", "", true},

		// GPT-5.2: none/low/medium/high/xhigh are supported.
		{"gpt-5.2 none", "gpt-5.2", "none", "none", false},
		{"gpt-5.2 xhigh", "gpt-5.2", "xhigh", "xhigh", false},
		{"gpt-5.2 max", "gpt-5.2", "max", "", true},

		// GPT-5.3-codex: low/medium/high/xhigh.
		{"gpt-5.3-codex low", "gpt-5.3-codex", "low", "low", false},
		{"gpt-5.3-codex xhigh", "gpt-5.3-codex", "xhigh", "xhigh", false},
		{"gpt-5.3-codex none", "gpt-5.3-codex", "none", "", true},

		// GPT-5.1: none/low/medium/high.
		{"gpt-5.1 none", "gpt-5.1", "none", "none", false},
		{"gpt-5.1 high", "gpt-5.1", "high", "high", false},
		{"gpt-5.1 xhigh", "gpt-5.1", "xhigh", "", true},

		// GPT-5 base: minimal/low/medium/high.
		{"gpt-5 minimal", "gpt-5", "minimal", "minimal", false},
		{"gpt-5 high", "gpt-5", "high", "high", false},
		{"gpt-5 xhigh", "gpt-5", "xhigh", "", true},
		{"gpt-5 none", "gpt-5", "none", "", true},
		{"gpt-5 max", "gpt-5", "max", "", true},

		// GPT-5-mini: minimal/low/medium/high.
		{"gpt-5-mini minimal", "gpt-5-mini", "minimal", "minimal", false},
		{"gpt-5-mini high", "gpt-5-mini", "high", "high", false},
		{"gpt-5-mini xhigh", "gpt-5-mini", "xhigh", "", true},

		// GPT-5-nano: minimal/low/medium/high.
		{"gpt-5-nano minimal", "gpt-5-nano", "minimal", "minimal", false},
		{"gpt-5-nano none", "gpt-5-nano", "none", "", true},
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
