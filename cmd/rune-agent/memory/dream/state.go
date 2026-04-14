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
	Dreamed       map[string]int   // dialogue ID → version that was dreamed
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
				Dreamed:   make(map[string]int),
				PhasesRun: make(map[string]int64),
			}, nil
		}
		return DreamState{}, fmt.Errorf("load dream state: %w", err)
	}
	if state.SchemaVersion == 0 {
		state.SchemaVersion = 1
	}
	if state.Dreamed == nil {
		state.Dreamed = make(map[string]int)
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
