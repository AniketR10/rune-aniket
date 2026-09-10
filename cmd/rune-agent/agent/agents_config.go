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
	"cmp"
	"slices"
)

// Definition describes a named agent that can be spawned.
type Definition struct {
	ID           string
	Name         string
	SystemPrompt string
	Model        string
	AllowSpawn   []string // agent IDs this agent can spawn
	AllowAny     bool     // bypass AllowSpawn and allow all agents
}

// Cfg holds named agent definitions and resolves
// spawn allowlists.
type Cfg struct {
	defs map[string]Definition
}

// NewConfig creates an AgentsConfig from the given
// definitions.
func NewConfig(defs []Definition) *Cfg {
	m := make(map[string]Definition, len(defs))
	for _, d := range defs {
		m[d.ID] = d
	}
	return &Cfg{defs: m}
}

// Get returns the definition for the given agent ID.
func (c *Cfg) Get(id string) (Definition, bool) {
	d, ok := c.defs[id]
	return d, ok
}

// AllowedAgents returns the agent definitions that the
// requester is permitted to spawn, sorted by ID with the
// requester listed first.
func (c *Cfg) AllowedAgents(requesterID string) []Definition {
	requester, ok := c.defs[requesterID]
	if !ok {
		return nil
	}

	if requester.AllowAny {
		return c.allSorted(requesterID)
	}

	allowed := make(map[string]bool, len(requester.AllowSpawn))
	for _, id := range requester.AllowSpawn {
		allowed[id] = true
	}

	var result []Definition
	// Add requester first if it's in its own allowlist.
	if allowed[requesterID] {
		result = append(result, requester)
	}

	// Collect the rest sorted by ID.
	var others []Definition
	for id, def := range c.defs {
		if id == requesterID || !allowed[id] {
			continue
		}
		others = append(others, def)
	}
	slices.SortFunc(others, func(a, b Definition) int {
		return cmp.Compare(a.ID, b.ID)
	})
	result = append(result, others...)
	return result
}

func (c *Cfg) allSorted(requesterID string) []Definition {
	var result []Definition
	if d, ok := c.defs[requesterID]; ok {
		result = append(result, d)
	}

	var others []Definition
	for id, def := range c.defs {
		if id == requesterID {
			continue
		}
		others = append(others, def)
	}
	slices.SortFunc(others, func(a, b Definition) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return append(result, others...)
}
