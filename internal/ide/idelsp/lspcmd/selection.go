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

package lspcmd

import (
	"context"
	"sync"

	"github.com/unstablebuild/rune-go-sdk/api/semanticapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

var _ textapi.EventHandler = (*SelectionTracker)(nil)

// SelectionTracker tracks active text selections by subscribing to
// EventTypeSelection and EventTypeCursor events.
type SelectionTracker struct {
	mu   sync.Mutex
	sels map[string]semanticapi.Range
}

// NewSelectionTracker creates a new SelectionTracker.
func NewSelectionTracker() *SelectionTracker {
	return &SelectionTracker{
		sels: make(
			map[string]semanticapi.Range,
		),
	}
}

// Handle processes a text event and updates the selection state.
func (s *SelectionTracker) Handle(
	_ context.Context, ev textapi.Event,
) bool {
	uri := URIToLSP(ev.URI)

	s.mu.Lock()
	defer s.mu.Unlock()

	switch ev.Type {
	case textapi.EventTypeSelection:
		start, end := CoordToPos(ev.Start), CoordToPos(ev.End)
		// LSP requires Range.Start <= Range.End; the editor may
		// report a backward selection (user dragged upward).
		if start.Line > end.Line ||
			(start.Line == end.Line && start.Character > end.Character) {
			start, end = end, start
		}
		s.sels[uri] = semanticapi.Range{Start: start, End: end}
	case textapi.EventTypeCursor:
		delete(s.sels, uri)
	}
	return false
}

// Get returns the tracked selection range for the given URI.
func (s *SelectionTracker) Get(
	uri workspaceapi.URI,
) (semanticapi.Range, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.sels[URIToLSP(uri)]
	return r, ok
}

// ClampRange clamps Start and End character offsets so they do not
// exceed the actual line lengths in the document obtained from the
// editor's CellView. If the CellView is nil or RawCells fails,
// the range is returned unchanged.
func ClampRange(
	rng semanticapi.Range, editor textapi.Editor, resource textapi.Handler,
) semanticapi.Range {
	cv := editor.CellView(resource)
	if cv == nil {
		return rng
	}
	cells, err := cv.RawCells()
	if err != nil || len(cells) == 0 {
		return rng
	}

	clamp := func(p semanticapi.Position) semanticapi.Position {
		line := int(p.Line)
		if line >= len(cells) {
			return p
		}
		if lineLen := uint32(len(cells[line])); p.Character > lineLen {
			p.Character = lineLen
		}
		return p
	}
	rng.Start = clamp(rng.Start)
	rng.End = clamp(rng.End)
	return rng
}
