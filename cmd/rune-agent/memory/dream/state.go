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

package dream

import (
	"context"
	"errors"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

const dreamStateID = "dream-state"

// DreamState tracks which dialogues have been processed by the dream system.
//
//nolint:revive // Preserved imported API name for compatibility and clarity.
type DreamState struct {
	SchemaVersion int              // template version memories were last processed under
	Dreamed       map[string]int64 // dialogue ID → version that was dreamed
	LastExtract   int64            // incremented when extraction produces new memories
	PhasesRun     map[string]int64 // phase name → LastExtract value it last ran at
}

type dreamStateStore struct {
	backend storageapi.Service
}

func newDreamState(backend storageapi.Service) *dreamStateStore {
	return &dreamStateStore{backend: backend}
}

func (s *dreamStateStore) load(ctx context.Context) (DreamState, error) {
	var state DreamState
	err := s.backend.Get(ctx, dreamStateID, &state)
	if err != nil {
		if errors.Is(err, storageapi.ErrNotFound) {
			return DreamState{
				Dreamed:   make(map[string]int64),
				PhasesRun: make(map[string]int64),
			}, nil
		}
		return DreamState{}, fmt.Errorf("load dream state: %w", err)
	}
	if state.SchemaVersion == 0 {
		state.SchemaVersion = 1
	}
	if state.Dreamed == nil {
		state.Dreamed = make(map[string]int64)
	}
	if state.PhasesRun == nil {
		state.PhasesRun = make(map[string]int64)
	}
	return state, nil
}

func (s *dreamStateStore) save(ctx context.Context, state DreamState) error {
	err := s.backend.Set(ctx, dreamStateID, &state)
	if err != nil {
		return fmt.Errorf("save dream state: %w", err)
	}
	return nil
}
