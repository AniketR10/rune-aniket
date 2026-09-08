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

package standard

import (
	"context"
	"fmt"

	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/internal/text"
)

const searchListID = "search"

type findState struct {
	active       bool
	legacyPrompt bool
	query        []rune
	origin       term.Coordinates
	last         []rune
	matchIdx     int
}

func (h *standardHandler) searchLocations() []textapi.Location {
	for _, list := range h.cursor.LocationLists() {
		if list.ID == searchListID {
			return list.Locations
		}
	}
	return nil
}

func firstMatchFrom(locs []textapi.Location, pos term.Coordinates) (int, bool) {
	if len(locs) == 0 {
		return 0, false
	}
	for i, loc := range locs {
		if !coordLess(loc.From, pos) {
			return i, true
		}
	}
	return 0, true
}

func coordLess(a, b term.Coordinates) bool {
	if a.Y != b.Y {
		return a.Y < b.Y
	}
	return a.X < b.X
}

func orderedCoordinates(a, b term.Coordinates) (term.Coordinates, term.Coordinates) {
	if coordLess(b, a) {
		return b, a
	}
	return a, b
}

// BeginSearch satisfies searchbox.Controller.
func (h *standardHandler) BeginSearch(query []rune, origin term.Coordinates) {
	h.find.active = true
	h.find.legacyPrompt = false
	h.find.origin = origin
	h.find.query = append(h.find.query[:0], query...)
	h.find.matchIdx = -1
	h.researchFind()
}

// SetSearchQuery satisfies searchbox.Controller.
func (h *standardHandler) SetSearchQuery(query string) {
	h.find.query = append(h.find.query[:0], []rune(query)...)
	h.find.matchIdx = -1
	h.researchFind()
}

// AdvanceSearch satisfies searchbox.Controller.
func (h *standardHandler) AdvanceSearch() {
	h.advanceFind(true)
}

// SearchLast satisfies searchbox.Controller.
func (h *standardHandler) SearchLast() string {
	return string(h.find.last)
}

// SearchOrigin satisfies searchbox.Controller.
func (h *standardHandler) SearchOrigin() term.Coordinates {
	return h.cursor.CursorAtScroll()
}

// FinishSearch satisfies searchbox.Controller.
func (h *standardHandler) FinishSearch() {
	if !h.find.active {
		return
	}
	if len(h.find.query) == 0 {
		h.cursor.Search("")
		h.cursor.Unselect()
		h.cursor.MoveToScroll(h.find.origin)
	}
	h.acceptFind()
}

// ReplaceNext satisfies searchbox.ReplaceController.
func (h *standardHandler) ReplaceNext(replacement string) {
	if len(h.find.query) == 0 {
		return
	}
	locs := h.searchLocations()
	if h.find.matchIdx < 0 || h.find.matchIdx >= len(locs) {
		return
	}
	loc := locs[h.find.matchIdx]
	from, to := orderedCoordinates(loc.From, loc.To)
	h.buf.MarkStartUndo()
	_, boundary, _ := h.CellEditor().Edit(context.Background(), from, to, replacement)
	h.buf.GroupUndo()
	h.find.origin = boundary
	h.find.matchIdx = -1
	h.researchFind()
}

// ReplaceAll satisfies searchbox.ReplaceController.
func (h *standardHandler) ReplaceAll(replacement string) {
	if len(h.find.query) == 0 {
		return
	}
	locs := append([]textapi.Location(nil), h.searchLocations()...)
	if len(locs) == 0 {
		return
	}
	h.buf.MarkStartUndo()
	ed := h.CellEditor()
	for i := len(locs) - 1; i >= 0; i-- {
		from, to := orderedCoordinates(locs[i].From, locs[i].To)
		ed.Edit(context.Background(), from, to, replacement)
	}
	h.buf.GroupUndo()
	h.find.matchIdx = -1
	h.researchFind()
}

func (h *standardHandler) startFind() bool {
	if h.find.active {
		if len(h.find.query) == 0 {
			if len(h.find.last) == 0 {
				h.renderFind(true)
				return true
			}
			h.find.query = append(h.find.query, h.find.last...)
			h.researchFind()
			return true
		}
		h.advanceFind(true)
		return true
	}

	h.find.active = true
	h.find.legacyPrompt = true
	h.find.query = h.find.query[:0]
	h.find.origin = h.cursor.CursorAtScroll()
	h.find.matchIdx = -1
	h.renderFind(true)
	return true
}

func (h *standardHandler) landFind(loc textapi.Location) {
	h.cursor.Unselect()
	h.cursor.MoveToScroll(loc.From)
	h.cursor.Select()
	h.cursor.MoveToScroll(loc.To)
}

func (h *standardHandler) styleFindLocations(locs []textapi.Location) {
	for i := range locs {
		locs[i].Attr = h.cfg.search.MatchAttr
	}
	if h.find.matchIdx >= 0 && h.find.matchIdx < len(locs) {
		locs[h.find.matchIdx].Attr = h.cfg.search.CurrentMatchAttr
	}
	h.cursor.SetLocationList(
		textapi.LocationPriorityCritical, searchListID, text.LocationSlice(locs))
}

func (h *standardHandler) advanceFind(forward bool) {
	locs := h.searchLocations()
	if len(locs) == 0 {
		h.renderFind(false)
		return
	}

	idx := h.find.matchIdx
	if idx < 0 {
		idx, _ = firstMatchFrom(locs, h.find.origin)
	} else if forward {
		idx = (idx + 1) % len(locs)
	} else {
		idx = (idx - 1 + len(locs)) % len(locs)
	}
	h.find.matchIdx = idx
	h.styleFindLocations(locs)
	h.landFind(locs[idx])
	h.renderFind(true)
}

func (h *standardHandler) researchFind() {
	query := string(h.find.query)
	if query == "" {
		h.cursor.Search("")
		h.cursor.Unselect()
		h.find.matchIdx = -1
		h.cursor.MoveToScroll(h.find.origin)
		h.renderFind(true)
		return
	}

	h.cursor.Search(query)
	locs := h.searchLocations()
	idx, ok := firstMatchFrom(locs, h.find.origin)
	if !ok {
		h.cursor.Unselect()
		h.find.matchIdx = -1
		h.cursor.MoveToScroll(h.find.origin)
		h.renderFind(false)
		return
	}
	h.find.matchIdx = idx
	h.styleFindLocations(locs)
	h.landFind(locs[idx])
	h.renderFind(true)
}

func (h *standardHandler) acceptFind() {
	h.find.active = false
	if len(h.find.query) > 0 {
		h.find.last = append(h.find.last[:0], h.find.query...)
	}
	h.find.query = h.find.query[:0]
	h.setFindPrompt("")
	h.find.legacyPrompt = false
}

func (h *standardHandler) renderFind(matched bool) {
	if !h.find.legacyPrompt {
		return
	}
	if !matched {
		h.setFindPrompt(fmt.Sprintf("Find: %s  no matches", string(h.find.query)))
		return
	}
	if locs := h.searchLocations(); len(locs) > 0 && h.find.matchIdx >= 0 {
		h.setFindPrompt(fmt.Sprintf(
			"Find: %s  %d/%d", string(h.find.query), h.find.matchIdx+1, len(locs)))
		return
	}
	h.setFindPrompt(fmt.Sprintf("Find: %s", string(h.find.query)))
}

func (h *standardHandler) setFindPrompt(prompt string) {
	if !h.find.legacyPrompt && prompt != "" {
		return
	}
	// When status_attr is unset the zero value hands the slot back to the
	// status bar layout; the editor attributes must not leak into it.
	h.statusBar.SetStatus(prompt, h.cfg.search.StatusAttr)
}

func (h *standardHandler) handleFindKey(ev term.Event) (rehandle bool) {
	if ev.Type != term.EventKey {
		return false
	}

	if ev.Mod == term.ModShift && ev.Key == term.KeyEnter {
		h.advanceFind(false)
		return false
	}
	if (ev.Mod == term.ModMeta || ev.Mod == term.ModCtrl) && ev.Ch == 'f' {
		h.startFind()
		return false
	}
	if ev.Mod != 0 {
		h.acceptFind()
		return true
	}

	switch ev.Key {
	case term.KeyEnter:
		h.advanceFind(true)
		return false
	case term.KeyEsc:
		h.acceptFind()
		return false
	case term.KeyBackspace:
		if len(h.find.query) > 0 {
			h.find.query = h.find.query[:len(h.find.query)-1]
		}
		h.researchFind()
		return false
	case term.KeySpace:
		h.find.query = append(h.find.query, ' ')
		h.researchFind()
		return false
	case 0:
		if ev.Ch != 0 {
			h.find.query = append(h.find.query, ev.Ch)
			h.researchFind()
			return false
		}
	}

	h.acceptFind()
	return true
}
