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

package bedrock

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModelEntries_CoverCatalog(t *testing.T) {
	entries := ModelEntries()
	require.Len(t, entries, len(AvailableModels()))
	for _, e := range entries {
		assert.Equal(t, LLMProvider, e.Provider)
		assert.Positive(t, e.ContextWindow, "model %s", e.Name)
	}
}

func TestFlagshipModel_IsInCatalog(t *testing.T) {
	_, ok := AvailableModels()[FlagshipModel()]
	assert.True(t, ok)
}

func TestNormalizeEffort(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		effort   string
		want     string
		wantWarn bool
	}{
		{"unset", ClaudeSonnet45, "", "", false},
		{"claude high", ClaudeSonnet45, "high", "high", false},
		{"claude unsupported level", ClaudeSonnet45, "ultra", "", true},
		{"nova rejects effort", NovaPro, "high", "", true},
		{"claude 3 predates thinking", "us.anthropic.claude-3-haiku-20240307-v1:0", "low", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, warn := NormalizeEffort(tt.model, tt.effort)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantWarn, warn != "", "warning: %q", warn)
		})
	}
}

func TestIsClaudeModel(t *testing.T) {
	assert.True(t, IsClaudeModel(ClaudeOpus45))
	assert.False(t, IsClaudeModel(NovaPro))
	assert.False(t, IsClaudeModel(DeepSeekR1))
}

func TestVerificationModelForRegion(t *testing.T) {
	tests := []struct {
		region string
		want   string
	}{
		{"", FlagshipModel()},
		{"us-east-1", FlagshipModel()},
		{"eu-west-1", "eu.anthropic.claude-opus-4-5-20251101-v1:0"},
		{"ap-southeast-2", "apac.anthropic.claude-opus-4-5-20251101-v1:0"},
		{"us-gov-west-1", "us-gov.anthropic.claude-opus-4-5-20251101-v1:0"},
	}
	for _, tt := range tests {
		t.Run("region "+tt.region, func(t *testing.T) {
			assert.Equal(t, tt.want, VerificationModelForRegion(tt.region))
		})
	}
}
