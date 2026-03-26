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
