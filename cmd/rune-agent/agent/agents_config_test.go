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
