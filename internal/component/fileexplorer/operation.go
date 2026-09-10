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

package fileexplorer

import (
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// OperationType describes the kind of filesystem operation.
type OperationType int

const (
	// OpCreate indicates a new file should be created.
	OpCreate OperationType = iota
	// OpMkdir indicates a new directory should be created.
	OpMkdir
	// OpDelete indicates a file or directory should be deleted.
	OpDelete
	// OpRename indicates a file or directory should be renamed (same parent).
	OpRename
	// OpMove indicates a file or directory should be moved (different parent).
	OpMove
	// OpCopy indicates a file or directory should be copied.
	OpCopy
)

// String returns the string representation of the operation type.
func (t OperationType) String() string {
	switch t {
	case OpCreate:
		return "create"
	case OpMkdir:
		return "mkdir"
	case OpDelete:
		return "delete"
	case OpRename:
		return "rename"
	case OpMove:
		return "move"
	case OpCopy:
		return "copy"
	default:
		return "unknown"
	}
}

// Operation represents a single filesystem change detected by
// comparing the view tree against the base tree.
type Operation struct {
	Type   OperationType
	URI    workspaceapi.URI // Source URI (for delete, rename, move, copy)
	NewURI workspaceapi.URI // Destination URI (for create, mkdir, rename, move, copy)
}

// String returns a human-readable description of the operation.
func (o Operation) String() string {
	switch o.Type {
	case OpCreate:
		return fmt.Sprintf("create %s", o.NewURI.Name())
	case OpMkdir:
		return fmt.Sprintf("mkdir %s", o.NewURI.Name())
	case OpDelete:
		return fmt.Sprintf("delete %s", o.URI.Name())
	case OpRename:
		return fmt.Sprintf("rename %s -> %s", o.URI.Name(), o.NewURI.Name())
	case OpMove:
		return fmt.Sprintf("move %s -> %s", o.URI.Path(), o.NewURI.Path())
	case OpCopy:
		return fmt.Sprintf("copy %s -> %s", o.URI.Path(), o.NewURI.Path())
	default:
		return "unknown operation"
	}
}

// ChangeSet holds the result of change detection with additional metadata.
type ChangeSet struct {
	Operations []Operation
	Conflicts  []Conflict
}

// Conflict describes a conflict between two operations.
type Conflict struct {
	Path    string // The conflicting path
	Entry1  rune   // ID of first entry
	Entry2  rune   // ID of second entry (0 if conflict with existing file)
	Message string
}

// HasConflicts returns true if there are any conflicts.
func (cs *ChangeSet) HasConflicts() bool {
	return len(cs.Conflicts) > 0
}

// computeChangeSet compares the view tree against the base tree
// and returns a ChangeSet with operations and any detected conflicts.
func computeChangeSet(baseTree, viewTree *node) *ChangeSet {
	cs := &ChangeSet{}

	// Collect entries from both trees
	baseEntries := collectEntries(baseTree)
	viewEntries := collectEntries(viewTree)

	baseMap := buildEntryMap(baseEntries)
	viewMap := buildEntryMap(viewEntries)

	// Track target paths to detect conflicts
	targetPaths := make(map[string]rune) // path -> entry ID

	// Find IDs that appear multiple times in view (copies)
	idCounts := make(map[rune]int)
	for _, e := range viewEntries {
		if e.ID != 0 {
			idCounts[e.ID]++
		}
	}

	// Process view entries to detect operations
	for _, viewEntry := range viewEntries {
		// Check for path conflicts
		if existingID, exists := targetPaths[viewEntry.URI.String()]; exists {
			cs.Conflicts = append(cs.Conflicts, Conflict{
				Path:    viewEntry.URI.Path(),
				Entry1:  existingID,
				Entry2:  viewEntry.ID,
				Message: fmt.Sprintf("path conflict: %s", viewEntry.URI.Path()),
			})
			continue
		}
		targetPaths[viewEntry.URI.String()] = viewEntry.ID

		// New entry (no ID)
		if viewEntry.ID == 0 {
			if viewEntry.IsDir {
				cs.Operations = append(cs.Operations, Operation{
					Type:   OpMkdir,
					NewURI: viewEntry.URI,
				})
			} else {
				cs.Operations = append(cs.Operations, Operation{
					Type:   OpCreate,
					NewURI: viewEntry.URI,
				})
			}
			continue
		}

		// Look up in base
		baseEntry := baseMap.Get(viewEntry.ID)
		if baseEntry == nil {
			// ID exists in view but not base - treat as new
			if viewEntry.IsDir {
				cs.Operations = append(cs.Operations, Operation{
					Type:   OpMkdir,
					NewURI: viewEntry.URI,
				})
			} else {
				cs.Operations = append(cs.Operations, Operation{
					Type:   OpCreate,
					NewURI: viewEntry.URI,
				})
			}
			continue
		}

		// Check if this is a copy (same ID appears multiple times)
		if idCounts[viewEntry.ID] > 1 {
			// First occurrence is the original, subsequent are copies
			// We track which we've already processed
			idCounts[viewEntry.ID]--
			if idCounts[viewEntry.ID] > 0 {
				cs.Operations = append(cs.Operations, Operation{
					Type:   OpCopy,
					URI:    baseEntry.URI,
					NewURI: viewEntry.URI,
				})
				continue
			}
		}

		// Check if renamed or moved
		if baseEntry.URI.String() != viewEntry.URI.String() {
			baseParent := baseEntry.ParentURI().String()
			viewParent := viewEntry.ParentURI().String()

			if baseParent == viewParent {
				// Same parent = rename
				cs.Operations = append(cs.Operations, Operation{
					Type:   OpRename,
					URI:    baseEntry.URI,
					NewURI: viewEntry.URI,
				})
			} else {
				// Different parent = move
				cs.Operations = append(cs.Operations, Operation{
					Type:   OpMove,
					URI:    baseEntry.URI,
					NewURI: viewEntry.URI,
				})
			}
		}
	}

	// Find deleted entries (in base but not in view)
	for _, baseEntry := range baseEntries {
		if baseEntry.ID == 0 {
			continue
		}
		if viewMap.Get(baseEntry.ID) == nil {
			cs.Operations = append(cs.Operations, Operation{
				Type: OpDelete,
				URI:  baseEntry.URI,
			})
		}
	}

	return cs
}
