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

package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentsConfig(t *testing.T) {
	tests := []struct {
		name     string
		defs     []Definition
		assertFn func(t *testing.T, cfg *Cfg)
	}{
		{
			name: "Get returns existing definition",
			defs: []Definition{
				{ID: "coder", Name: "Coder Agent"},
			},
			assertFn: func(t *testing.T, cfg *Cfg) {
				d, ok := cfg.Get("coder")
				require.True(t, ok)
				assert.Equal(t, "Coder Agent", d.Name)
			},
		},
		{
			name: "Get returns false for unknown ID",
			defs: []Definition{
				{ID: "coder", Name: "Coder Agent"},
			},
			assertFn: func(t *testing.T, cfg *Cfg) {
				_, ok := cfg.Get("unknown")
				assert.False(t, ok)
			},
		},
		{
			name: "AllowedAgents with explicit allowlist",
			defs: []Definition{
				{
					ID:         "parent",
					Name:       "Parent",
					AllowSpawn: []string{"child_a", "child_b"},
				},
				{ID: "child_a", Name: "Child A"},
				{ID: "child_b", Name: "Child B"},
				{ID: "child_c", Name: "Child C"},
			},
			assertFn: func(t *testing.T, cfg *Cfg) {
				allowed := cfg.AllowedAgents("parent")
				require.Len(t, allowed, 2)
				assert.Equal(t, "child_a", allowed[0].ID)
				assert.Equal(t, "child_b", allowed[1].ID)
			},
		},
		{
			name: "AllowedAgents with wildcard",
			defs: []Definition{
				{
					ID:       "parent",
					Name:     "Parent",
					AllowAny: true,
				},
				{ID: "alpha", Name: "Alpha"},
				{ID: "beta", Name: "Beta"},
			},
			assertFn: func(t *testing.T, cfg *Cfg) {
				allowed := cfg.AllowedAgents("parent")
				require.Len(t, allowed, 3)
				// Requester first, then sorted by ID
				assert.Equal(t, "parent", allowed[0].ID)
				assert.Equal(t, "alpha", allowed[1].ID)
				assert.Equal(t, "beta", allowed[2].ID)
			},
		},
		{
			name: "AllowedAgents for unknown requester returns nil",
			defs: []Definition{
				{ID: "coder", Name: "Coder"},
			},
			assertFn: func(t *testing.T, cfg *Cfg) {
				allowed := cfg.AllowedAgents("unknown")
				assert.Nil(t, allowed)
			},
		},
		{
			name: "AllowedAgents includes self when in allowlist",
			defs: []Definition{
				{
					ID:         "coder",
					Name:       "Coder",
					AllowSpawn: []string{"coder", "helper"},
				},
				{ID: "helper", Name: "Helper"},
			},
			assertFn: func(t *testing.T, cfg *Cfg) {
				allowed := cfg.AllowedAgents("coder")
				require.Len(t, allowed, 2)
				assert.Equal(t, "coder", allowed[0].ID)
				assert.Equal(t, "helper", allowed[1].ID)
			},
		},
		{
			name: "AllowedAgents skips non-existent IDs in allowlist",
			defs: []Definition{
				{
					ID:         "parent",
					Name:       "Parent",
					AllowSpawn: []string{"exists", "ghost"},
				},
				{ID: "exists", Name: "Exists"},
			},
			assertFn: func(t *testing.T, cfg *Cfg) {
				allowed := cfg.AllowedAgents("parent")
				require.Len(t, allowed, 1)
				assert.Equal(t, "exists", allowed[0].ID)
			},
		},
		{
			name: "empty config",
			defs: nil,
			assertFn: func(t *testing.T, cfg *Cfg) {
				_, ok := cfg.Get("any")
				assert.False(t, ok)
				assert.Nil(t, cfg.AllowedAgents("any"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewConfig(tt.defs)
			tt.assertFn(t, cfg)
		})
	}
}
