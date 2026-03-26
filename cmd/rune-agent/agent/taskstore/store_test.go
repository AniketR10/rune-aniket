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
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateAssignsIncrementingIDs(t *testing.T) {
	s := New()
	t1 := s.Create("first", "desc1", "", nil)
	t2 := s.Create("second", "desc2", "", nil)
	t3 := s.Create("third", "desc3", "", nil)

	assert.Equal(t, "1", t1.ID)
	assert.Equal(t, "2", t2.ID)
	assert.Equal(t, "3", t3.ID)
}

func TestCreateDefaultsPending(t *testing.T) {
	s := New()
	task := s.Create("test", "desc", "Testing", nil)
	assert.Equal(t, "pending", task.Status)
	assert.Equal(t, "test", task.Subject)
	assert.Equal(t, "desc", task.Description)
	assert.Equal(t, "Testing", task.ActiveForm)
}

func TestCreateWithMetadata(t *testing.T) {
	s := New()
	meta := map[string]any{"key": "value"}
	task := s.Create("test", "desc", "", meta)
	assert.Equal(t, meta, task.Metadata)
}

func TestUpdateChangesFields(t *testing.T) {
	s := New()
	s.Create("original", "desc", "", nil)

	newSubject := "updated"
	newStatus := "in_progress"
	newActiveForm := "Updating"
	updated, err := s.Update("1", UpdateOpts{
		Subject:    &newSubject,
		Status:     &newStatus,
		ActiveForm: &newActiveForm,
	})
	require.NoError(t, err)
	assert.Equal(t, "updated", updated.Subject)
	assert.Equal(t, "in_progress", updated.Status)
	assert.Equal(t, "Updating", updated.ActiveForm)
}

func TestUpdateDeletedRemovesTask(t *testing.T) {
	s := New()
	s.Create("first", "desc1", "", nil)
	s.Create("second", "desc2", "", nil)

	deleted := "deleted"
	_, err := s.Update("1", UpdateOpts{Status: &deleted})
	require.NoError(t, err)

	tasks := s.List()
	assert.Len(t, tasks, 1)
	assert.Equal(t, "2", tasks[0].ID)

	_, err = s.Get("1")
	assert.Error(t, err)
}

func TestUpdateAddsBlocksAndBlockedBy(t *testing.T) {
	s := New()
	s.Create("task", "desc", "", nil)

	updated, err := s.Update("1", UpdateOpts{
		AddBlocks:    []string{"2", "3"},
		AddBlockedBy: []string{"4"},
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"2", "3"}, updated.Blocks)
	assert.Equal(t, []string{"4"}, updated.BlockedBy)
}

func TestUpdateMergesMetadata(t *testing.T) {
	s := New()
	s.Create("task", "desc", "", map[string]any{"a": 1})

	updated, err := s.Update("1", UpdateOpts{
		Metadata: map[string]any{"b": 2},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, updated.Metadata["a"])
	assert.Equal(t, 2, updated.Metadata["b"])
}

func TestUpdateOwner(t *testing.T) {
	s := New()
	s.Create("task", "desc", "", nil)

	owner := "alice"
	updated, err := s.Update("1", UpdateOpts{Owner: &owner})
	require.NoError(t, err)
	assert.Equal(t, "alice", updated.Owner)
}

func TestGetReturnsErrorForNonExistentID(t *testing.T) {
	s := New()
	_, err := s.Get("999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "999")
}

func TestGetReturnsTask(t *testing.T) {
	s := New()
	s.Create("test", "desc", "", nil)

	task, err := s.Get("1")
	require.NoError(t, err)
	assert.Equal(t, "test", task.Subject)
}

func TestUpdateNonExistentID(t *testing.T) {
	s := New()
	_, err := s.Update("999", UpdateOpts{})
	assert.Error(t, err)
}

func TestListReturnsOrderedTasks(t *testing.T) {
	s := New()
	s.Create("alpha", "desc1", "", nil)
	s.Create("beta", "desc2", "", nil)
	s.Create("gamma", "desc3", "", nil)

	tasks := s.List()
	require.Len(t, tasks, 3)
	assert.Equal(t, "alpha", tasks[0].Subject)
	assert.Equal(t, "beta", tasks[1].Subject)
	assert.Equal(t, "gamma", tasks[2].Subject)
}

func TestListReturnsEmptySlice(t *testing.T) {
	s := New()
	tasks := s.List()
	assert.Empty(t, tasks)
}

func TestThreadSafety(t *testing.T) {
	s := New()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Create("task", "desc", "", nil)
		}()
	}
	wg.Wait()

	tasks := s.List()
	assert.Len(t, tasks, 100)
}
