// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
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


package fileexplorer

import (
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// Entry represents a file or directory in the explorer.
// This is the primary data structure for oil.nvim-like editing.
// Entries are identified by their ID (from Unicode Private Use Area),
// which persists across edits allowing move/rename detection.
type Entry struct {
	ID       rune             // Unique identifier (0 for new entries)
	Name     string           // Filename without path
	URI      workspaceapi.URI // Full URI
	IsDir    bool             // True if directory
	Expanded bool             // True if directory is expanded
	Depth    int              // Nesting level (0 = root children)
}

// ParentURI returns the URI of this entry's parent directory.
func (e Entry) ParentURI() workspaceapi.URI {
	return workspaceapi.Dir(e.URI)
}

// EntryMap provides O(1) lookup of entries by ID.
type EntryMap map[rune]*Entry

// Add adds an entry to the map. If ID is 0, it's ignored (new entries).
func (m EntryMap) Add(e *Entry) {
	if e.ID != 0 {
		m[e.ID] = e
	}
}

// Get returns the entry with the given ID, or nil if not found.
func (m EntryMap) Get(id rune) *Entry {
	return m[id]
}

// collectEntries extracts all entries from a node tree into a flat list.
//
// All directories with loaded children are traversed, regardless of
// their "expanded" flag (which is a UI concern). This ensures that
// files inside collapsed directories — which are still part of the
// model — are included when diffing view vs base for change
// detection.
func collectEntries(root *node) []*Entry {
	var entries []*Entry
	var walk func(n *node)
	walk = func(n *node) {
		for _, child := range n.children {
			entries = append(entries, &Entry{
				ID:       child.id,
				Name:     child.name,
				URI:      child.uri,
				IsDir:    child.isDir,
				Expanded: child.expanded,
				Depth:    child.depth,
			})
			if child.isDir {
				walk(child)
			}
		}
	}
	walk(root)
	return entries
}

// buildEntryMap creates a map of ID -> Entry from a list of entries.
func buildEntryMap(entries []*Entry) EntryMap {
	m := make(EntryMap)
	for _, e := range entries {
		m.Add(e)
	}
	return m
}
