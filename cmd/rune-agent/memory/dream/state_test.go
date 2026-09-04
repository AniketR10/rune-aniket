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

package dream

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
)

func TestDreamState(t *testing.T) {
	t.Run("load returns empty state when not found", func(t *testing.T) {
		storage := &mockStorage{}
		store := newDreamState(storage)

		state, err := store.load(context.Background())
		require.NoError(t, err)
		assert.NotNil(t, state.Dreamed)
		assert.Empty(t, state.Dreamed)
	})

	t.Run("save and load round-trip", func(t *testing.T) {
		storage := &mockStorage{data: make(map[string]any)}
		store := newDreamState(storage)
		ctx := context.Background()

		state := DreamState{
			Dreamed: map[string]int64{"d1": 1, "d2": 3},
		}
		require.NoError(t, store.save(ctx, state))

		loaded, err := store.load(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(1), loaded.Dreamed["d1"])
		assert.Equal(t, int64(3), loaded.Dreamed["d2"])
	})

	t.Run("load propagates backend error", func(t *testing.T) {
		storage := &mockStorage{getErr: errors.New("db down")}
		store := newDreamState(storage)

		_, err := store.load(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "db down")
	})

	t.Run("save propagates backend error", func(t *testing.T) {
		storage := &mockStorage{setErr: errors.New("write fail")}
		store := newDreamState(storage)

		err := store.save(context.Background(), DreamState{Dreamed: map[string]int64{}})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "write fail")
	})

	t.Run("SchemaVersion zero normalizes to 1 on load", func(t *testing.T) {
		storage := &mockStorage{data: make(map[string]any)}
		store := newDreamState(storage)

		storage.data[dreamStateID] = &DreamState{
			SchemaVersion: 0,
			Dreamed:       map[string]int64{"d1": 1},
		}

		loaded, err := store.load(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, loaded.SchemaVersion)
	})

	t.Run("SchemaVersion round-trips through save and load", func(t *testing.T) {
		storage := &mockStorage{data: make(map[string]any)}
		store := newDreamState(storage)
		ctx := context.Background()

		state := DreamState{
			SchemaVersion: 5,
			Dreamed:       map[string]int64{"d1": 1},
		}
		require.NoError(t, store.save(ctx, state))

		loaded, err := store.load(ctx)
		require.NoError(t, err)
		assert.Equal(t, 5, loaded.SchemaVersion)
	})

	t.Run("load initializes nil Dreamed map", func(t *testing.T) {
		storage := &mockStorage{data: make(map[string]any)}
		store := newDreamState(storage)
		ctx := context.Background()

		// Save a state with nil Dreamed to simulate stored data without the field.
		storage.data[dreamStateID] = &DreamState{}

		loaded, err := store.load(ctx)
		require.NoError(t, err)
		assert.NotNil(t, loaded.Dreamed)
	})

	t.Run("load initializes nil PhasesRun map", func(t *testing.T) {
		storage := &mockStorage{data: make(map[string]any)}
		store := newDreamState(storage)
		ctx := context.Background()

		storage.data[dreamStateID] = &DreamState{
			Dreamed: map[string]int64{"d1": 1},
		}

		loaded, err := store.load(ctx)
		require.NoError(t, err)
		assert.NotNil(t, loaded.PhasesRun)
	})

	t.Run("LastExtract round-trips through save and load", func(t *testing.T) {
		storage := &mockStorage{data: make(map[string]any)}
		store := newDreamState(storage)
		ctx := context.Background()

		state := DreamState{
			Dreamed:     map[string]int64{"d1": 1},
			LastExtract: 42,
			PhasesRun:   map[string]int64{},
		}
		require.NoError(t, store.save(ctx, state))

		loaded, err := store.load(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(42), loaded.LastExtract)
	})

	t.Run("PhasesRun round-trips through save and load", func(t *testing.T) {
		storage := &mockStorage{data: make(map[string]any)}
		store := newDreamState(storage)
		ctx := context.Background()

		state := DreamState{
			Dreamed:     map[string]int64{},
			LastExtract: 5,
			PhasesRun:   map[string]int64{"quality": 3, "deduplicate": 5},
		}
		require.NoError(t, store.save(ctx, state))

		loaded, err := store.load(ctx)
		require.NoError(t, err)
		assert.Equal(t, int64(3), loaded.PhasesRun["quality"])
		assert.Equal(t, int64(5), loaded.PhasesRun["deduplicate"])
	})
}

// mockStorage implements storageapi.Service for testing.
type mockStorage struct {
	data      map[string]any
	getErr    error
	setErr    error
	createErr error
	parts     map[string]*mockStorage
}

func (m *mockStorage) Create(_ context.Context, id string, doc any) error {
	if m.createErr != nil {
		return m.createErr
	}
	if m.data == nil {
		m.data = make(map[string]any)
	}
	if _, ok := m.data[id]; ok {
		return storageapi.ErrAlreadyExists
	}
	m.data[id] = doc
	return nil
}

func (m *mockStorage) Set(_ context.Context, id string, doc any) error {
	if m.setErr != nil {
		return m.setErr
	}
	if m.data == nil {
		m.data = make(map[string]any)
	}
	m.data[id] = doc
	return nil
}

func (m *mockStorage) Update(_ context.Context, _ string, _ []storageapi.Update, _ ...storageapi.Precondition) error {
	return nil
}

func (m *mockStorage) Get(_ context.Context, id string, doc any) error {
	if m.getErr != nil {
		return m.getErr
	}
	if m.data == nil {
		return storageapi.ErrNotFound
	}
	v, ok := m.data[id]
	if !ok {
		return storageapi.ErrNotFound
	}
	// Copy the stored state into doc.
	if dst, ok := doc.(*DreamState); ok {
		if src, ok := v.(*DreamState); ok {
			*dst = *src
		}
	}
	return nil
}

func (m *mockStorage) Delete(_ context.Context, id string) error {
	if m.data != nil {
		delete(m.data, id)
	}
	return nil
}

func (m *mockStorage) List(_ context.Context, _ []storageapi.Filter) (storageapi.Iterator, error) {
	return nil, nil
}

func (m *mockStorage) Partition(name string) (storageapi.Service, error) {
	if m.parts == nil {
		m.parts = make(map[string]*mockStorage)
	}
	if part, ok := m.parts[name]; ok {
		return part, nil
	}
	part := &mockStorage{
		data:      make(map[string]any),
		getErr:    m.getErr,
		setErr:    m.setErr,
		createErr: m.createErr,
	}
	m.parts[name] = part
	return part, nil
}

func (m *mockStorage) Close() error { return nil }
