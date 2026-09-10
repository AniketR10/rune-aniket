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

package gemini

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFlagshipModelInCatalog(t *testing.T) {
	_, ok := AvailableModels()[FlagshipModel()]
	assert.True(t, ok, "flagship %q must be in the catalog", FlagshipModel())
	assert.Equal(t, Gemini_3_1_Pro_Preview, FlagshipModel())
}

func TestCurrentFlashModelsInCatalog(t *testing.T) {
	models := AvailableModels()
	assert.Equal(t, 1_048_576, models[Gemini_3_6_Flash])
	assert.Equal(t, 1_048_576, models[Gemini_3_5_FlashLite])
}

func TestMaxOutputTokens(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{Gemini_3_6_Flash, 65536},
		{Gemini_3_5_FlashLite, 65536},
		{Gemini_3_1_Pro_Preview, 65536},
		{Gemini_2_5_Flash, 65536},
		{Gemini_2_0_Flash, 8192},
		{"unknown-model", 0},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			assert.Equal(t, tt.want, MaxOutputTokens(tt.model))
		})
	}
}
