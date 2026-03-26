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

package taskstore

import (
	"fmt"
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
	Subject     *string
	Description *string
	ActiveForm  *string
	Status      *string
	AddBlocks   []string
	AddBlockedBy []string
	Owner       *string
	Metadata    map[string]any
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
					s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
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
