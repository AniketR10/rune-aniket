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

package vte

import (
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/text"
)

const searchListID = "search"

// searchController searches the primary buffer's scrollback on behalf of
// the floating search box. It drives a separate view of the shared buffer,
// so the live terminal view and its parser offsets stay untouched and are
// restored as soon as the results view is dismissed.
type searchController struct {
	vi  *viHandler
	cfg SearchConfig
	// viModeActive reports whether the terminal is currently in actual
	// modal (vi) navigation, as opposed to the live view. It is nil when
	// modal mode is disabled.
	viModeActive func() bool
	// viewing reports whether the results view is rendered instead of the
	// live terminal view. It outlives the search box: closing the box parks
	// the reader on the current match until they dismiss it or type.
	viewing  bool
	query    []rune
	last     []rune
	origin   term.Coordinates
	matchIdx int
}

// SearchOrigin satisfies searchbox.Controller.
func (c *searchController) SearchOrigin() term.Coordinates {
	if c.viewing || (c.viModeActive != nil && c.viModeActive()) {
		// the vi cursor, not the live pty cursor, is where the reader is
		// actually looking once they have navigated into modal mode
		return c.vi.cursorAtScroll()
	}
	return c.vi.searchOrigin()
}

// SearchLast satisfies searchbox.Controller.
func (c *searchController) SearchLast() string { return string(c.last) }

// Selection satisfies searchbox.Controller. Searching selects nothing,
// it only highlights, but the current match reads as selected and
// copying it is the whole point of finding it, so it stands in until
// the reader selects something else.
func (c *searchController) Selection() (string, bool) {
	if selected, ok := c.vi.Selection(); ok {
		return selected, true
	}
	locs := c.vi.searchLocations()
	if c.matchIdx < 0 || c.matchIdx >= len(locs) {
		return "", false
	}
	return c.vi.textAt(locs[c.matchIdx].From, locs[c.matchIdx].To)
}

// BeginSearch satisfies searchbox.Controller.
func (c *searchController) BeginSearch(query []rune, origin term.Coordinates) {
	c.origin = origin
	c.query = append(c.query[:0], query...)
	c.matchIdx = -1
	if !c.viewing {
		c.viewing = true
		c.vi.enterSearchMode(origin)
	}
	c.research()
}

// SetSearchQuery satisfies searchbox.Controller.
func (c *searchController) SetSearchQuery(query string) {
	c.query = append(c.query[:0], []rune(query)...)
	c.matchIdx = -1
	c.research()
}

// AdvanceSearch satisfies searchbox.Controller.
func (c *searchController) AdvanceSearch() {
	locs := c.vi.searchLocations()
	if len(locs) == 0 {
		return
	}
	idx := c.matchIdx
	if idx < 0 {
		idx = firstMatchFrom(locs, c.origin)
	} else {
		idx = (idx + 1) % len(locs)
	}
	c.land(locs, idx)
}

// FinishSearch satisfies searchbox.Controller.
func (c *searchController) FinishSearch() {
	if len(c.query) > 0 {
		c.last = append(c.last[:0], c.query...)
	}
	c.query = c.query[:0]
	if c.matchIdx < 0 {
		c.exitView()
	}
}

// exitView drops the highlights and hands the screen back to the live
// terminal view.
func (c *searchController) exitView() {
	c.matchIdx = -1
	c.viewing = false
	c.vi.clearSearch()
}

func (c *searchController) research() {
	query := string(c.query)
	if query == "" {
		c.vi.clearSearch()
		c.vi.moveSearchCursor(c.origin)
		return
	}
	locs := c.vi.search(query)
	if len(locs) == 0 {
		c.vi.moveSearchCursor(c.origin)
		return
	}
	c.land(locs, firstMatchFrom(locs, c.origin))
}

func (c *searchController) land(locs []textapi.Location, idx int) {
	c.matchIdx = idx
	for i := range locs {
		locs[i].Attr = c.cfg.MatchAttr
	}
	locs[idx].Attr = c.cfg.CurrentMatchAttr
	c.vi.setSearchLocations(locs)
	c.vi.moveSearchCursor(locs[idx].From)
}

// firstMatchFrom returns the index of the first location at or after pos,
// wrapping around to the first location when pos is past every match.
func firstMatchFrom(locs []textapi.Location, pos term.Coordinates) int {
	for i, loc := range locs {
		if loc.From.Y > pos.Y || (loc.From.Y == pos.Y && loc.From.X >= pos.X) {
			return i
		}
	}
	return 0
}

func (v *viHandler) enterSearchMode(pos term.Coordinates) {
	v.sync.scroll.SetOffset(term.Coordinates{})
	v.sync.scroll.Attributes = v.sync.vteScroll.Attributes

	v.sync.mu.Lock()
	defer v.sync.mu.Unlock()
	v.viSetCursorAtScroll(pos)
}

func (v *viHandler) searchOrigin() term.Coordinates {
	v.sync.mu.Lock()
	defer v.sync.mu.Unlock()

	return v.comp.cursorAtScroll()
}

// clearSearch drops the highlights; vi.Search("") also replaces the
// search location list with an empty one.
func (v *viHandler) clearSearch() {
	v.sync.mu.Lock()
	defer v.sync.mu.Unlock()

	v.sync.vi.Search("")
	v.sync.vi.Unselect()
}

func (v *viHandler) search(query string) []textapi.Location {
	v.sync.mu.Lock()
	defer v.sync.mu.Unlock()

	v.sync.vi.Search(query)
	return v.locations()
}

func (v *viHandler) searchLocations() []textapi.Location {
	v.sync.mu.Lock()
	defer v.sync.mu.Unlock()

	return v.locations()
}

func (v *viHandler) locations() []textapi.Location {
	for _, list := range v.sync.vi.LocationLists() {
		if list.ID == searchListID {
			return list.Locations
		}
	}
	return nil
}

func (v *viHandler) setSearchLocations(locs []textapi.Location) {
	v.sync.mu.Lock()
	defer v.sync.mu.Unlock()

	v.sync.vi.SetLocationList(
		textapi.LocationPriorityCritical, searchListID, text.LocationSlice(locs))
}

func (v *viHandler) moveSearchCursor(pos term.Coordinates) {
	v.sync.mu.Lock()
	defer v.sync.mu.Unlock()

	v.viSetCursorAtScroll(pos)
}

// textAt returns the buffer contents of the half-open range [from, to).
func (v *viHandler) textAt(from, to term.Coordinates) (string, bool) {
	v.sync.mu.Lock()
	defer v.sync.mu.Unlock()

	cells, _, ok := v.sync.selector.Select(from, to)
	if !ok {
		return "", false
	}
	return term.CellsToString(cells), true
}
