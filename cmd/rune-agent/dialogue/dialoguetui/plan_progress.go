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

package dialoguetui

import (
	"slices"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// ProgressTaskEntry represents a single task for TUI rendering.
type ProgressTaskEntry struct {
	ID          string
	Subject     string
	Description string
	ActiveForm  string
	Status      string // "pending", "in_progress", "completed"
}

// PlanProgress renders a bottom-pinned task checklist.
// It satisfies component.Responsive.
type PlanProgress struct {
	tasks     []ProgressTaskEntry
	taskIndex map[string]int // ID → index into tasks
	cfg       *ComponentConfig
	width     int
	height    int
	drawCount int
}

var _ component.Responsive = (*PlanProgress)(nil)

var (
	defaultInProgressAttr = term.Attributes{Fg: term.ColorTeal}
	defaultPendingAttr    = term.Attributes{Fg: term.ColorYellow}
	defaultDescAttr       = term.Attributes{Fg: term.ColorGray}
)

// NewPlanProgress creates a new PlanProgress component.
func NewPlanProgress(cfg *ComponentConfig, _ term.Interrupter) *PlanProgress {
	return &PlanProgress{
		cfg:       cfg,
		taskIndex: make(map[string]int),
	}
}

// UpdateTask upserts a task by ID. New tasks are appended;
// existing tasks are updated in place. Tasks with status "deleted"
// are removed.
func (p *PlanProgress) UpdateTask(entry ProgressTaskEntry) {
	if idx, ok := p.taskIndex[entry.ID]; ok {
		if entry.Status == "deleted" {
			// slices.Delete clears the tail so the removed entry's strings
			// are not retained in the backing array past len.
			p.tasks = slices.Delete(p.tasks, idx, idx+1)
			delete(p.taskIndex, entry.ID)
			for i := idx; i < len(p.tasks); i++ {
				p.taskIndex[p.tasks[i].ID] = i
			}
		} else {
			p.tasks[idx] = entry
		}
	} else if entry.Status != "deleted" {
		p.taskIndex[entry.ID] = len(p.tasks)
		p.tasks = append(p.tasks, entry)
	}
}

// Height satisfies component.Responsive.
func (p *PlanProgress) Height(width int) int {
	h := 0
	for _, task := range p.tasks {
		h++ // subject line
		if task.Description != "" && width > 2 {
			h += len(wrapText(task.Description, width, 2))
		}
	}
	return h
}

// Resize satisfies tui.Component.
func (p *PlanProgress) Resize(width, height int) {
	p.width = width
	p.height = height
}

// Draw satisfies tui.Component.
func (p *PlanProgress) Draw(w term.Writer) {
	p.drawCount++

	successAttr := p.successAttr()
	inProgressAttr := p.inProgressAttr()
	pendingAttr := p.pendingAttr()
	descAttr := defaultDescAttr

	y := 0
	for _, task := range p.tasks {
		if y >= p.height {
			break
		}
		var x int
		switch task.Status {
		case "completed":
			x = writeRuneLineAttr(w, 0, y, "󰗠 ", p.width, successAttr)
			writeRuneLineAttr(w, x, y, task.Subject, p.width, term.Attributes{})
		case "in_progress":
			x = writeRuneLineAttr(w, 0, y, "󰐌 ", p.width, inProgressAttr)
			writeRuneLineAttr(w, x, y, task.Subject, p.width, term.Attributes{Attrs: term.AttrBold})
		default: // pending
			x = writeRuneLineAttr(w, 0, y, "󰏥 ", p.width, pendingAttr)
			writeRuneLineAttr(w, x, y, task.Subject, p.width, term.Attributes{})
		}
		y++

		// Indent descriptions to the subject column so both stay aligned
		// regardless of how many columns the status icon occupies.
		if task.Description != "" && p.width > x {
			for _, line := range wrapText(task.Description, p.width, x) {
				if y >= p.height {
					break
				}
				writeRuneLineAttr(w, 0, y, line, p.width, descAttr)
				y++
			}
		}
	}
}

// ActiveForm returns the ActiveForm of the first in_progress task,
// or the subject if ActiveForm is empty, or empty string if none.
// It is used by the status hint to display the current task phase.
func (p *PlanProgress) ActiveForm() string {
	for _, t := range p.tasks {
		if t.Status == "in_progress" {
			if t.ActiveForm != "" {
				return t.ActiveForm
			}
			return t.Subject
		}
	}
	return ""
}

// Close releases resources associated with this PlanProgress.
func (p *PlanProgress) Close() {}

func (p *PlanProgress) successAttr() term.Attributes {
	if p.cfg.CollapsedSuccessAttr != (term.Attributes{}) {
		return p.cfg.CollapsedSuccessAttr
	}
	return defaultSuccessAttr
}

func (p *PlanProgress) inProgressAttr() term.Attributes {
	if p.cfg.CollapsedErrorAttr != (term.Attributes{}) {
		return p.cfg.CollapsedErrorAttr
	}
	return defaultInProgressAttr
}

func (p *PlanProgress) pendingAttr() term.Attributes {
	return defaultPendingAttr
}
