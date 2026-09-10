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

package taskstore

import (
	"fmt"
	"slices"
	"strconv"
	"sync"
)

// Task represents a tracked task in the agent's work plan.
type Task struct {
	ID          string
	Subject     string
	Description string
	ActiveForm  string
	Status      string // "pending", "in_progress", "completed"
	Blocks      []string
	BlockedBy   []string
	Owner       string
	Metadata    map[string]any
}

// UpdateOpts holds optional fields for updating a task.
// Pointer fields are only applied when non-nil.
type UpdateOpts struct {
	Subject      *string
	Description  *string
	ActiveForm   *string
	Status       *string
	AddBlocks    []string
	AddBlockedBy []string
	Owner        *string
	Metadata     map[string]any
}

// Store is a thread-safe in-memory task store.
// It is scoped to a single chat session.
type Store struct {
	mu     sync.Mutex
	tasks  []*Task
	byID   map[string]*Task
	nextID int
}

// New creates a new empty Store.
func New() *Store {
	return &Store{
		byID: make(map[string]*Task),
	}
}

// Create adds a new task with status "pending" and returns a snapshot.
func (s *Store) Create(subject, description, activeForm string, metadata map[string]any) Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	t := &Task{
		ID:          strconv.Itoa(s.nextID),
		Subject:     subject,
		Description: description,
		ActiveForm:  activeForm,
		Status:      "pending",
		Metadata:    metadata,
	}
	s.tasks = append(s.tasks, t)
	s.byID[t.ID] = t
	return *t
}

// Update modifies a task. If Status is "deleted", the task is removed.
// Returns a snapshot of the task after the update.
func (s *Store) Update(id string, opts UpdateOpts) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.byID[id]
	if !ok {
		return Task{}, fmt.Errorf("task %q not found", id)
	}
	if opts.Subject != nil {
		t.Subject = *opts.Subject
	}
	if opts.Description != nil {
		t.Description = *opts.Description
	}
	if opts.ActiveForm != nil {
		t.ActiveForm = *opts.ActiveForm
	}
	if opts.Status != nil {
		if *opts.Status == "deleted" {
			delete(s.byID, id)
			for i, task := range s.tasks {
				if task.ID == id {
					// slices.Delete clears the tail so the removed *Task is
					// not retained in the backing array past len.
					s.tasks = slices.Delete(s.tasks, i, i+1)
					break
				}
			}
			snapshot := *t
			snapshot.Status = "deleted"
			return snapshot, nil
		}
		t.Status = *opts.Status
	}
	if len(opts.AddBlocks) > 0 {
		t.Blocks = append(t.Blocks, opts.AddBlocks...)
	}
	if len(opts.AddBlockedBy) > 0 {
		t.BlockedBy = append(t.BlockedBy, opts.AddBlockedBy...)
	}
	if opts.Owner != nil {
		t.Owner = *opts.Owner
	}
	if opts.Metadata != nil {
		if t.Metadata == nil {
			t.Metadata = make(map[string]any)
		}
		for k, v := range opts.Metadata {
			t.Metadata[k] = v
		}
	}
	return *t, nil
}

// Get returns a snapshot of a task by ID.
func (s *Store) Get(id string) (Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.byID[id]
	if !ok {
		return Task{}, fmt.Errorf("task %q not found", id)
	}
	return *t, nil
}

// List returns all tasks in creation order.
func (s *Store) List() []*Task {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*Task, len(s.tasks))
	copy(result, s.tasks)
	return result
}
