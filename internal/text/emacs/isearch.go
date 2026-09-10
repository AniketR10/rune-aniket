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

package emacs

import (
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// searchListID mirrors the unexported location-list ID that text.Cursor.Search
// populates. It is the list the incremental search walks and re-anchors.
const searchListID = "search"

// isearchState holds the transient state of an Emacs incremental search
// (C-s / C-r). The query is rebuilt into the cursor's search location list on
// every keystroke so matches update live; origin records point when the search
// began so C-g can restore it.
type isearchState struct {
	active  bool
	forward bool
	query   []rune
	origin  term.Coordinates
	// last holds the most recently accepted query so a repeat C-s / C-r
	// with an empty prompt resumes it, like the GNU search ring.
	last []rune
	// matchIdx is the index into the current search-match list of the match
	// point currently sits on, or -1 when the query has no match.
	matchIdx int
}

// searchLocations returns the current search-match locations in document
// order. Reading them directly (rather than through MoveToNextMatch) avoids
// the shared, stateful location-list iterator and lets the search anchor
// matches relative to an arbitrary position.
func (h *emacsHandler) searchLocations() []textapi.Location {
	for _, list := range h.cursor.LocationLists() {
		if list.ID == searchListID {
			return list.Locations
		}
	}
	return nil
}

// firstMatchFrom returns the index of the first match at or after pos when
// forward, or at or before pos when backward, wrapping around the ends. It
// returns false when locs is empty.
func firstMatchFrom(locs []textapi.Location, pos term.Coordinates, forward bool) (int, bool) {
	if len(locs) == 0 {
		return 0, false
	}
	if forward {
		for i, loc := range locs {
			if !coordLess(loc.From, pos) {
				return i, true
			}
		}
		return 0, true
	}
	for i := len(locs) - 1; i >= 0; i-- {
		if !coordLess(pos, locs[i].From) {
			return i, true
		}
	}
	return len(locs) - 1, true
}

// startIsearch begins (or, when already active, repeats) an incremental search
// in the given direction. The first invocation records point so C-g can
// restore it and shows the echo-area prompt; a repeat with an existing query
// advances to the next/previous match.
func (h *emacsHandler) startIsearch(forward bool) bool {
	if h.isearch.active {
		h.isearch.forward = forward
		if len(h.isearch.query) == 0 {
			if len(h.isearch.last) == 0 {
				// Nothing to resume: flipping direction is all that
				// happens until the user types a query.
				h.renderIsearch(true)
				return true
			}
			// C-s C-s / C-r C-r: resume the last accepted search, GNU
			// search-ring style.
			h.isearch.query = append(h.isearch.query, h.isearch.last...)
			h.isearchResearch()
			return true
		}
		h.isearchAdvance(forward)
		return true
	}
	h.isearch.active = true
	h.isearch.forward = forward
	h.isearch.query = h.isearch.query[:0]
	h.isearch.origin = h.cursor.CursorAtScroll()
	h.isearch.matchIdx = -1
	h.setTransientMode("ISEARCH")
	h.renderIsearch(true)
	return true
}

// isearchLand moves point to the GNU position for the given match: the end
// of the match when searching forward, its start when searching backward.
func (h *emacsHandler) isearchLand(loc textapi.Location) {
	if h.isearch.forward {
		h.cursor.MoveToScroll(loc.To)
		return
	}
	h.cursor.MoveToScroll(loc.From)
}

// isearchAdvance moves to the next match in the given direction from the match
// point currently sits on, wrapping around the ends.
func (h *emacsHandler) isearchAdvance(forward bool) {
	locs := h.searchLocations()
	if len(locs) == 0 {
		h.renderIsearch(false)
		return
	}
	idx := h.isearch.matchIdx
	if idx < 0 {
		idx, _ = firstMatchFrom(locs, h.isearch.origin, forward)
	} else if forward {
		idx = (idx + 1) % len(locs)
	} else {
		idx = (idx - 1 + len(locs)) % len(locs)
	}
	h.isearch.matchIdx = idx
	h.isearchLand(locs[idx])
	h.renderIsearch(true)
}

// isearchResearch rebuilds the search list for the current query and moves to
// the first match from the origin in the active direction. It is called after
// the query text changes.
func (h *emacsHandler) isearchResearch() {
	query := string(h.isearch.query)
	if query == "" {
		h.cursor.Search("")
		h.isearch.matchIdx = -1
		h.cursor.MoveToScroll(h.isearch.origin)
		h.renderIsearch(true)
		return
	}
	h.cursor.Search(query)
	locs := h.searchLocations()
	idx, ok := firstMatchFrom(locs, h.isearch.origin, h.isearch.forward)
	if !ok {
		h.isearch.matchIdx = -1
		h.cursor.MoveToScroll(h.isearch.origin)
		h.renderIsearch(false)
		return
	}
	h.isearch.matchIdx = idx
	h.isearchLand(locs[idx])
	h.renderIsearch(true)
}

// acceptIsearch exits the search leaving point at the current match. The
// search highlight is kept and the query is remembered so C-s C-s / C-r C-r
// resume it, matching Emacs leaving the last search available.
func (h *emacsHandler) acceptIsearch() {
	h.isearch.active = false
	if len(h.isearch.query) > 0 {
		h.isearch.last = append(h.isearch.last[:0], h.isearch.query...)
	}
	h.isearch.query = h.isearch.query[:0]
	h.less.SetMessage("")
	h.setTransientMode("")
}

// abortIsearch exits the search, restores point to the origin and clears the
// search highlight (C-g).
func (h *emacsHandler) abortIsearch() {
	h.isearch.active = false
	h.isearch.query = h.isearch.query[:0]
	h.cursor.Search("")
	h.cursor.MoveToScroll(h.isearch.origin)
	h.less.SetMessage("")
	h.setTransientMode("")
}

// renderIsearch shows the incremental-search prompt. failed toggles the
// "failing" hint Emacs shows when the query currently has no match.
func (h *emacsHandler) renderIsearch(matched bool) {
	label := "I-search: "
	if !h.isearch.forward {
		label = "I-search backward: "
	}
	if !matched {
		label = "Failing " + label
	}
	h.less.SetMessage("%s", label+string(h.isearch.query))
}

// handleIsearchKey consumes one key while an incremental search is active. It
// returns rehandle=true when the key should be reprocessed by the normal
// keymap after the search exits (e.g. an arrow key that both ends the search
// and performs its motion).
func (h *emacsHandler) handleIsearchKey(ev term.Event) (rehandle bool) {
	if ev.Type != term.EventKey {
		return false
	}

	switch ev.Mod {
	case term.ModCtrl:
		switch ev.Ch {
		case 's':
			h.startIsearch(true)
			return false
		case 'r':
			h.startIsearch(false)
			return false
		case 'g':
			h.abortIsearch()
			return false
		}
		// Any other control chord ends the search and is re-handled.
		h.acceptIsearch()
		return true
	case 0:
		switch ev.Key {
		case term.KeyEnter, term.KeyEsc:
			h.acceptIsearch()
			return false
		case term.KeyBackspace:
			if len(h.isearch.query) > 0 {
				h.isearch.query = h.isearch.query[:len(h.isearch.query)-1]
			}
			h.isearchResearch()
			return false
		case term.KeySpace:
			h.isearch.query = append(h.isearch.query, ' ')
			h.isearchResearch()
			return false
		case 0:
			if ev.Ch != 0 {
				h.isearch.query = append(h.isearch.query, ev.Ch)
				h.isearchResearch()
				return false
			}
			// A named key (arrow, home, …) ends the search and is re-handled.
			h.acceptIsearch()
			return true
		}
		// Any other named key ends the search and is re-handled.
		h.acceptIsearch()
		return true
	}
	// Meta or other modified chords end the search and are re-handled.
	h.acceptIsearch()
	return true
}
