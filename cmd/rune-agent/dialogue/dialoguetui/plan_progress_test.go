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

package dialoguetui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/component/comptest"
	"github.com/unstablebuild/rune-go-sdk/term"
)

var _ component.Responsive = (*PlanProgress)(nil)

func newTestPlanProgress() *PlanProgress {
	return NewPlanProgress(&ComponentConfig{}, nil)
}

func TestPlanProgressHeightMatchesTaskCount(t *testing.T) {
	p := newTestPlanProgress()
	assert.Equal(t, 0, p.Height(80))

	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "first", Status: "pending"})
	assert.Equal(t, 1, p.Height(80))

	p.UpdateTask(ProgressTaskEntry{ID: "2", Subject: "second", Status: "pending"})
	assert.Equal(t, 2, p.Height(80))
}

func TestPlanProgressHeightNoHeaderWhenInProgress(t *testing.T) {
	p := newTestPlanProgress()
	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "task", Status: "pending"})
	assert.Equal(t, 1, p.Height(80))

	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "task", ActiveForm: "Working", Status: "in_progress"})
	assert.Equal(t, 1, p.Height(80)) // no header, just 1 task

	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "task", Status: "completed"})
	assert.Equal(t, 1, p.Height(80))
}

func TestPlanProgressUpdateTaskUpserts(t *testing.T) {
	p := newTestPlanProgress()
	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "original", Status: "pending"})
	assert.Equal(t, 1, p.Height(80))

	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "updated", Status: "in_progress"})
	assert.Equal(t, "updated", p.tasks[0].Subject)
	assert.Equal(t, "in_progress", p.tasks[0].Status)
}

func TestPlanProgressDeletedRemovesTask(t *testing.T) {
	p := newTestPlanProgress()
	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "first", Status: "pending"})
	p.UpdateTask(ProgressTaskEntry{ID: "2", Subject: "second", Status: "pending"})

	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "first", Status: "deleted"})
	assert.Equal(t, 1, p.Height(80))
	assert.Equal(t, "second", p.tasks[0].Subject)
}

func TestPlanProgressActiveForm(t *testing.T) {
	p := newTestPlanProgress()
	assert.Equal(t, "", p.ActiveForm())

	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "task", Status: "pending"})
	assert.Equal(t, "", p.ActiveForm())

	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "task", ActiveForm: "Working", Status: "in_progress"})
	assert.Equal(t, "Working", p.ActiveForm())

	// Falls back to Subject when ActiveForm is empty.
	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "task", Status: "in_progress"})
	assert.Equal(t, "task", p.ActiveForm())

	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "task", Status: "completed"})
	assert.Equal(t, "", p.ActiveForm())
}

func TestPlanProgressDeletedNonExistent(t *testing.T) {
	p := newTestPlanProgress()
	p.UpdateTask(ProgressTaskEntry{ID: "999", Subject: "ghost", Status: "deleted"})
	assert.Equal(t, 0, p.Height(80))
}

func TestPlanProgressRenderMixedStatuses(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.Resize(30, 10)
	w := term.NewStringWriter(31, 11)

	tests := []comptest.TestCase{
		{
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "1", Subject: "Create store", Status: "completed",
				})
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "2", Subject: "Add tools", ActiveForm: "Adding tools", Status: "in_progress",
				})
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "3", Subject: "Write tests", Status: "pending",
				})
			},
			Expected: "󰗠 Create store                 \n" +
				"󰐌 Add tools                    \n" +
				"󰏥 Write tests                  \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
		{
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "2", Subject: "Add tools", Status: "completed",
				})
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "3", Subject: "Write tests", ActiveForm: "Writing tests", Status: "in_progress",
				})
			},
			Expected: "󰗠 Create store                 \n" +
				"󰗠 Add tools                    \n" +
				"󰐌 Write tests                  \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
	}
	comptest.TestComponent(t, comp, w, tests)
}

func TestPlanProgressRenderAllCompleted(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.Resize(30, 10)
	w := term.NewStringWriter(31, 11)

	tests := []comptest.TestCase{
		{
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "1", Subject: "First", Status: "completed",
				})
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "2", Subject: "Second", Status: "completed",
				})
			},
			Expected: "󰗠 First                        \n" +
				"󰗠 Second                       \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
	}
	comptest.TestComponent(t, comp, w, tests)
}

func TestPlanProgressRenderInProgressUsesSubjectWhenNoActiveForm(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.Resize(30, 10)
	w := term.NewStringWriter(31, 11)

	tests := []comptest.TestCase{
		{
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "1", Subject: "My task", Status: "in_progress",
				})
			},
			Expected: "󰐌 My task                      \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
	}
	comptest.TestComponent(t, comp, w, tests)
}

func TestPlanProgressClearedOnSendMessage(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.Resize(30, 10)
	w := term.NewStringWriter(31, 11)

	tests := []comptest.TestCase{
		{
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "1", Subject: "First", Status: "completed",
				})
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "2", Subject: "Second", Status: "pending",
				})
			},
			Expected: "󰗠 First                        \n" +
				"󰏥 Second                       \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
		{
			Action: func() {
				comp.AddSendMessage("next question")
			},
			Expected: "next question                  \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
	}
	comptest.TestComponent(t, comp, w, tests)
}

func TestPlanProgressLateTerminalEventsAfterClear(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.Resize(30, 10)
	w := term.NewStringWriter(31, 11)

	tests := []comptest.TestCase{
		{
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "1", Subject: "Build", Status: "in_progress",
				})
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "2", Subject: "Test", Status: "pending",
				})
			},
			Expected: "󰐌 Build                        \n" +
				"󰏥 Test                         \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
		{
			// User sends a message, clearing all tasks.
			Action: func() {
				comp.AddSendMessage("next question")
			},
			Expected: "next question                  \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
		{
			// Late "completed" for a cleared task recreates the
			// panel — no panic, the LLM decides what to show.
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "1", Subject: "Build", Status: "completed",
				})
			},
			Expected: "next question                  \n" +
				"󰗠 Build                        \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
		{
			// Late "deleted" for an unknown ID is a no-op on the
			// existing panel — no panic.
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "2", Subject: "Test", Status: "deleted",
				})
			},
			Expected: "next question                  \n" +
				"󰗠 Build                        \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
	}
	comptest.TestComponent(t, comp, w, tests)
}

func TestPlanProgressHeightIncludesDescription(t *testing.T) {
	p := newTestPlanProgress()
	p.UpdateTask(ProgressTaskEntry{ID: "1", Subject: "task", Description: "a short desc", Status: "pending"})
	// 1 subject line + 1 description line
	assert.Equal(t, 2, p.Height(80))

	// Long description wraps at width 20: "a longer description that wraps" → 2 lines at indent=2, usable=18
	p.UpdateTask(ProgressTaskEntry{ID: "2", Subject: "other", Description: "a longer description that wraps", Status: "pending"})
	// task1: 1+1=2, task2: 1 + ceil(31/18) = 1+2 = 3 → total 5
	assert.Equal(t, 5, p.Height(20))
}

func TestPlanProgressRenderDescription(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.Resize(30, 10)
	w := term.NewStringWriter(31, 11)

	tests := []comptest.TestCase{
		{
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "1", Subject: "First", Description: "Do the first thing", Status: "completed",
				})
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "2", Subject: "Second", Status: "pending",
				})
			},
			Expected: "󰗠 First                        \n" +
				"  Do the first thing           \n" +
				"󰏥 Second                       \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
	}
	comptest.TestComponent(t, comp, w, tests)
}

func TestPlanProgressDeletedTaskRemovedFromDisplay(t *testing.T) {
	comp := NewComponent(ComponentConfig{})
	comp.Resize(30, 10)
	w := term.NewStringWriter(31, 11)

	tests := []comptest.TestCase{
		{
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "1", Subject: "Keep", Status: "completed",
				})
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "2", Subject: "Remove", Status: "pending",
				})
			},
			Expected: "󰗠 Keep                         \n" +
				"󰏥 Remove                       \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
		{
			Action: func() {
				comp.UpdateTaskProgress(ProgressTaskEntry{
					ID: "2", Subject: "Remove", Status: "deleted",
				})
			},
			Expected: "󰗠 Keep                         \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"                               \n" +
				"  ┌───────────────────────┐    \n" +
				"  │                       │    \n" +
				"  └───────────────────────┘    \n" +
				"                               ",
		},
	}
	comptest.TestComponent(t, comp, w, tests)
}
