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

package ideshell

import (
	"context"

	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/handler/search"
)

// Handler is the IDE companion shell handler. It wraps the SDK
// repl.Handler and overlays a fuzzy reverse-history search list at the
// bottom of the prompt area, triggered by <c-r>.
//
// The wrapper is needed because repl.Handler's input box and history
// slice are unexported in rune-go-sdk; this type owns both halves of
// the shell tab so it can read the persisted history and drive the
// inner inputbox by re-issuing key events.
type Handler struct {
	inner      *repl.Handler
	storage    storageapi.Service
	historyKey string
	maxHistory int

	list      *search.List
	width     int
	height    int
	searching bool
	// lastSearchH is the search overlay height most recently
	// applied via Resize. Tracking it lets Draw notice when the
	// match count changed and re-Resize the inner inputbox so it
	// re-anchors against the new vertical split.
	lastSearchH int
	// mode distinguishes the two overlay flavors so that
	// acceptSearch/cancelSearch know how to mutate (or not)
	// the underlying inputbox.
	mode searchMode
	// query mirrors what the user has typed since entering search mode.
	// It is what is forwarded to list.Buffer() to drive the fuzzy match.
	query []rune
	// compTyped tracks how many runes the user has typed (or
	// deleted) since opening the completion overlay. It is used
	// to decide when backspace has consumed the partial word and
	// the overlay should close.
	compTyped int

	// shim is consulted on tab completion: it captures the head/
	// candidates returned from the underlying repl.CommandHandler
	// so we can render them in a search.List instead of cycling
	// inline through inputbox completion.
	shim *completionShim
	// prompt mirrors what the inner repl was configured with so we
	// can paint the same prefix in front of the search overlay's
	// input bar — this is what makes the overlay's bar look like a
	// continuation of the shell prompt.
	prompt string
}

// Wait forwards to the underlying repl.Handler so callers (including
// ex_test) can drain in-flight commands.
func (h *Handler) Wait() {
	h.inner.Wait()
}

// Close releases both the inner repl handler and the search list.
func (h *Handler) Close() error {
	err := h.inner.Close()
	if h.list != nil {
		if cerr := h.list.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}

// Resize satisfies tui.Component. While the search overlay is open
// we shrink the inner repl so its prior output gets pushed up
// rather than overdrawn by the candidate band. When closed we
// delegate fully.
func (h *Handler) Resize(width, height int) {
	h.width = width
	h.height = height
	if !h.searching {
		h.inner.Resize(width, height)
		h.lastSearchH = 0
		return
	}
	searchH := h.searchHeight(height)
	listW := max(1, width-len(h.prompt))
	h.list.Resize(listW, searchH)
	innerSlice := searchH
	if h.mode == modeCompletion {
		// Completion mode keeps the inner's rl band visible
		// at the bottom of the screen, so the inner only
		// needs to give up the listRows that the candidate
		// band steals.
		innerSlice = max(0, searchH-h.list.InputHeight())
	}
	h.inner.Resize(width, max(0, height-innerSlice))
	h.lastSearchH = searchH
}

// Draw satisfies tui.Component.
func (h *Handler) Draw(w term.Writer) {
	if !h.searching {
		h.inner.Draw(w)
		return
	}
	h.drawSearch(w)
}

// Cursor satisfies tui.Handler. In completion mode the inner inputbox
// owns the cursor (the inner repl is anchored to the top, so its
// cursor coordinates are already correct). In history mode the
// cursor belongs to the search bar that we render on the bottom
// row.
func (h *Handler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	if !h.searching {
		return h.inner.Cursor()
	}
	if h.mode == modeCompletion {
		coords, style, ok := h.inner.Cursor()
		if !ok {
			return coords, style, ok
		}
		// drawCompletion shifts the inner's rl band down by
		// listRows when compositing onto the screen, so the
		// cursor (which the inner places inside its rl band)
		// must be translated by the same amount.
		searchH := h.searchHeight(h.height)
		listRows := max(0, searchH-h.list.InputHeight())
		coords.Y += listRows
		return coords, style, true
	}
	return term.Coordinates{
		X: len(h.prompt) + len(h.query),
		Y: max(0, h.height-1),
	}, term.CursorStyleSteadyBar, true
}

// Selection satisfies tui.Handler.
func (h *Handler) Selection() (string, bool) {
	return h.inner.Selection()
}

// Handle satisfies tui.Handler. <c-r> opens the reverse-history search
// overlay; while open, the overlay consumes keys for navigation /
// acceptance / dismissal and otherwise mirrors typed runes back into
// the inner inputbox so the prompt and search query stay in sync.
func (h *Handler) Handle(ev term.Event) (exit, handled bool) {
	if ev.Type != term.EventKey {
		return h.inner.Handle(ev)
	}

	if !h.searching {
		if ev.Mod == term.ModCtrl && ev.Ch == 'r' {
			h.openSearch()
			return false, true
		}
		if ev.Key == term.KeyTab && ev.Mod == 0 {
			return h.handleTabClosed(ev)
		}
		return h.inner.Handle(ev)
	}

	return h.handleSearch(ev)
}

// historyDoc mirrors the on-disk shape that repl.Handler persists via
// repl.WithStorage. It is duplicated here because the field is
// unexported in the SDK; we must read the same document to seed the
// reverse-history search overlay.
type historyDoc struct {
	Items   []string
	Version int64
}

// searchOverlayMaxRows bounds how many candidate rows the search list
// renders at once. The full overlay height is this plus a fixed
// allowance for the search bar and the match-count bar
// (searchOverlayChromeRows). This ceiling matters only when there is
// more vertical room than candidates; small windows use whatever is
// available.
const (
	searchOverlayMaxRows    = 10
	searchOverlayChromeRows = 2
)

// searchMode is the overlay flavor.
type searchMode int

const (
	// modeHistory is the reverse-history search overlay opened
	// with <c-r>. Accepting replaces the entire prompt.
	modeHistory searchMode = iota
	// modeCompletion is the tab-completion overlay opened
	// with <tab> when the underlying handler returns more than
	// one completion candidate. Accepting replaces only the
	// currently-completed word.
	modeCompletion
)

// drawSearch renders the search overlay shared by both reverse-
// history and tab-completion modes. Both modes shrink the inner
// repl so prior output is pushed up (instead of being overdrawn
// by the candidate band), draw the search list immediately below
// the inner repl, and put a prompt-style row at the very bottom
// of the screen. They differ only in what owns the bottom row:
//
//   - history mode: the inner repl's bottom row (which would only
//     hold an empty prompt) is blanked out and the search list's
//     own BottomSearchBar lands on the screen's bottom row instead,
//     with the shell prompt prefix painted in front of it. The
//     cursor lives on that row and the user types into the search
//     query.
//   - completion mode: the inner repl's rl band stays visible at
//     the bottom of the screen so the user continues editing the
//     partial word in the inputbox. To keep the inputbox anchored
//     to row height-1 (rather than row height-listRows-1, where
//     the inner would naturally place it after our shrink), we
//     render the inner to an offscreen buffer at height-listRows
//     and shift its rl band down by listRows when copying to the
//     real writer. The search list is then drawn just above the
//     relocated rl band, clipped to its candidate rows.
func (h *Handler) drawSearch(w term.Writer) {
	if h.mode == modeCompletion {
		h.drawCompletion(w)
		return
	}
	h.drawHistory(w)
}

// drawHistory implements the reverse-history overlay. See drawSearch.
func (h *Handler) drawHistory(w term.Writer) {
	searchH := h.searchHeight(h.height)
	listW := max(1, h.width-len(h.prompt))
	innerH := max(0, h.height-searchH)
	if searchH != h.lastSearchH {
		h.inner.Resize(h.width, innerH)
		h.list.Resize(listW, searchH)
		h.lastSearchH = searchH
	}
	innerW := &component.VirtualWriter{
		Writer: w,
		Width:  h.width,
		Height: innerH,
	}
	h.inner.Draw(innerW)
	if innerH > 0 {
		// Blank out the inner's bottom row so its (often
		// empty) inputbox prompt doesn't appear in addition
		// to the search bar at the bottom of the screen.
		// Multi-row wrapped input above this row stays
		// visible.
		clearY := innerH - 1
		for x := range h.width {
			w.SetCell(term.Coordinates{X: x, Y: clearY},
				term.Cell{Ch: ' '})
		}
	}
	barY := h.height - 1
	for i, r := range h.prompt {
		w.SetCell(term.Coordinates{X: i, Y: barY},
			term.Cell{Ch: r})
	}
	overlayW := &component.VirtualWriter{
		Writer: w,
		Offset: term.Coordinates{X: len(h.prompt), Y: innerH},
		Width:  listW,
		Height: searchH,
	}
	h.list.Draw(overlayW)
}

// drawCompletion implements the tab-completion overlay. See drawSearch.
func (h *Handler) drawCompletion(w term.Writer) {
	searchH := h.searchHeight(h.height)
	listW := max(1, h.width-len(h.prompt))
	listRows := max(0, searchH-h.list.InputHeight())
	innerH := max(0, h.height-listRows)
	if searchH != h.lastSearchH {
		h.inner.Resize(h.width, innerH)
		h.list.Resize(listW, searchH)
		h.lastSearchH = searchH
	}
	buf := term.NewStringWriter(h.width, innerH)
	h.inner.Draw(buf)
	rlH, _ := h.inner.LayoutHeights()
	outH := innerH - rlH
	cells := buf.Cells()
	for y := range outH {
		for x := range h.width {
			w.SetCell(term.Coordinates{X: x, Y: y},
				cells[y*h.width+x])
		}
	}
	for y := range rlH {
		srcY := outH + y
		dstY := h.height - rlH + y
		for x := range h.width {
			w.SetCell(term.Coordinates{X: x, Y: dstY},
				cells[srcY*h.width+x])
		}
	}
	overlayW := &component.VirtualWriter{
		Writer: w,
		Offset: term.Coordinates{X: len(h.prompt), Y: outH},
		Width:  listW,
		Height: listRows,
	}
	h.list.Draw(overlayW)
}

// handleTabClosed forwards <tab> to the inner repl.Handler so its
// inputbox calls our completion shim. If the shim captured more than
// one candidate, we open the completion overlay seeded with them.
// Otherwise we let the inner handler's response stand (zero or one
// candidate is best handled inline, exactly like the SDK default).
func (h *Handler) handleTabClosed(ev term.Event) (exit, handled bool) {
	h.shim.reset()
	exit, handled = h.inner.Handle(ev)
	if exit {
		return exit, handled
	}
	captured, ok := h.shim.consume()
	if !ok || len(captured.candidates) <= 1 {
		return false, handled
	}
	h.openCompletion(captured)
	return false, true
}

func (h *Handler) handleSearch(ev term.Event) (exit, handled bool) {
	if h.mode == modeCompletion {
		return h.handleCompletion(ev)
	}
	return h.handleHistory(ev)
}

func (h *Handler) handleHistory(ev term.Event) (exit, handled bool) {
	switch ev.Mod {
	case 0:
		switch ev.Key {
		case term.KeyEnter, term.KeyTab:
			h.acceptSearch()
			return false, true
		case term.KeyEsc:
			h.cancelSearch()
			return false, true
		case term.KeyArrowUp:
			h.list.FocusUp()
			return false, true
		case term.KeyArrowDown:
			h.list.FocusDown()
			return false, true
		case term.KeyBackspace:
			h.shrinkQuery()
			return false, true
		case term.KeySpace:
			h.appendQuery(' ')
			return false, true
		}
		if ev.Ch != 0 {
			h.appendQuery(ev.Ch)
			return false, true
		}
	case term.ModCtrl:
		switch ev.Ch {
		case 'r':
			h.list.FocusDown()
			return false, true
		case 'j':
			h.list.FocusDown()
			return false, true
		case 'k':
			h.list.FocusUp()
			return false, true
		case 'c', 'g':
			h.cancelSearch()
			return false, true
		}
	}
	// Swallow unhandled events while searching so they don't bypass the
	// overlay and reach the underlying inputbox.
	return false, true
}

// handleCompletion handles events while the tab-completion overlay
// is open. The inputbox stays visible and is the source of truth for
// the partial word being completed; we mirror it into the search
// list's filter buffer.
func (h *Handler) handleCompletion(ev term.Event) (exit, handled bool) {
	switch ev.Mod {
	case 0:
		switch ev.Key {
		case term.KeyEnter, term.KeyTab:
			h.acceptSearch()
			return false, true
		case term.KeyEsc:
			h.cancelSearch()
			return false, true
		case term.KeyArrowUp:
			h.list.FocusUp()
			return false, true
		case term.KeyArrowDown:
			h.list.FocusDown()
			return false, true
		case term.KeyBackspace:
			return h.completionBackspace()
		case term.KeySpace:
			// space ends the current argument: close the
			// overlay and forward the space to the inputbox.
			h.cancelSearch()
			return h.inner.Handle(ev)
		}
		if ev.Ch != 0 {
			h.completionAppend(ev.Ch)
			return false, true
		}
	case term.ModCtrl:
		switch ev.Ch {
		case 'j':
			h.list.FocusDown()
			return false, true
		case 'k':
			h.list.FocusUp()
			return false, true
		case 'c', 'g':
			h.cancelSearch()
			return false, true
		}
	}
	return false, false
}

// completionAppend forwards a printable rune to the inner inputbox
// so the prompt visually grows, then mirrors the same character into
// the list's filter buffer so the candidate set narrows.
func (h *Handler) completionAppend(r rune) {
	_, _ = h.inner.Handle(term.Event{Type: term.EventKey, Ch: r})
	h.compTyped++
	h.appendQuery(r)
}

// completionBackspace either shrinks the typed-after-tab portion of
// the partial word (forwarded to the inputbox AND mirrored into the
// filter buffer), or closes the overlay when the user has back-
// spaced past everything they typed since opening it. The backspace
// itself is forwarded to the inputbox in either case so the prompt
// keeps shrinking.
func (h *Handler) completionBackspace() (exit, handled bool) {
	if h.compTyped <= 0 {
		// Backspaces have caught up to the original partial
		// word boundary; further deletions should affect the
		// underlying line, not the overlay.
		h.cancelSearch()
		return h.inner.Handle(
			term.Event{Type: term.EventKey, Key: term.KeyBackspace})
	}
	_, _ = h.inner.Handle(
		term.Event{Type: term.EventKey, Key: term.KeyBackspace})
	h.compTyped--
	h.shrinkQuery()
	return false, true
}

func (h *Handler) openSearch() {
	h.list.DataReset()
	for _, item := range h.loadHistory() {
		h.list.PushSync([]byte(item))
	}
	h.query = h.query[:0]
	h.list.Buffer().Replace("")
	h.list.FocusStart()
	h.searching = true
	h.mode = modeHistory
	h.Resize(h.width, h.height)
}

// openCompletion opens the tab-completion overlay seeded with the
// captured candidates, sharing the search.List rendering and
// navigation behavior with reverse-history search.
func (h *Handler) openCompletion(c capturedCompletion) {
	h.list.DataReset()
	for _, item := range c.candidates {
		h.list.PushSync([]byte(item))
	}
	h.query = h.query[:0]
	h.list.Buffer().Replace("")
	h.list.FocusStart()
	h.searching = true
	h.mode = modeCompletion
	h.shim.lastPrefix = c.prefix
	h.Resize(h.width, h.height)
}

func (h *Handler) cancelSearch() {
	h.list.Cancel()
	h.list.Wait()
	h.searching = false
	h.query = h.query[:0]
	h.compTyped = 0
	h.list.Buffer().Replace("")
	h.Resize(h.width, h.height)
}

func (h *Handler) acceptSearch() {
	h.list.Cancel()
	h.list.Wait()
	match, ok := h.list.Focus()
	mode := h.mode
	prefix := h.shim.lastPrefix
	typed := h.compTyped
	h.searching = false
	h.query = h.query[:0]
	h.compTyped = 0
	h.Resize(h.width, h.height)
	if !ok {
		return
	}
	text := string(match.Data())
	switch mode {
	case modeHistory:
		h.replaceInputText(text)
	case modeCompletion:
		// The inputbox currently contains the original
		// prefix plus everything the user typed after <tab>;
		// delete both, then insert the chosen candidate.
		h.deleteAndType(len([]rune(prefix))+typed, text)
	}
}

func (h *Handler) appendQuery(r rune) {
	h.query = append(h.query, r)
	h.list.Buffer().Replace(string(h.query))
}

func (h *Handler) shrinkQuery() {
	if len(h.query) == 0 {
		return
	}
	h.query = h.query[:len(h.query)-1]
	h.list.Buffer().Replace(string(h.query))
}

// replaceInputText drives the inner inputbox to contain text by first
// clearing the line (<c-u>) and then re-issuing each rune. The inner
// repl.Handler's inputbox state is private, so re-issuing key events
// is the supported way to mutate it from outside.
func (h *Handler) replaceInputText(text string) {
	clear := term.Event{Type: term.EventKey, Ch: 'u', Mod: term.ModCtrl}
	_, _ = h.inner.Handle(clear)
	for _, r := range text {
		ev := term.Event{Type: term.EventKey, Ch: r}
		_, _ = h.inner.Handle(ev)
	}
}

// deleteAndType deletes n runes to the left of the cursor and then
// types text. It is used to swap the partial word at the cursor for
// a chosen completion candidate while preserving everything else on
// the line.
func (h *Handler) deleteAndType(n int, text string) {
	bs := term.Event{Type: term.EventKey, Key: term.KeyBackspace}
	for i := 0; i < n; i++ {
		_, _ = h.inner.Handle(bs)
	}
	for _, r := range text {
		ev := term.Event{Type: term.EventKey, Ch: r}
		_, _ = h.inner.Handle(ev)
	}
}

// loadHistory reads the persisted shell history from storage. It
// returns the entries newest-first so that the search list focuses on
// the most recent commands when opened.
func (h *Handler) loadHistory() []string {
	if h.storage == nil || h.historyKey == "" {
		return nil
	}
	var doc historyDoc
	if err := h.storage.Get(context.Background(), h.historyKey, &doc); err != nil {
		return nil
	}
	items := doc.Items
	if h.maxHistory > 0 && len(items) > h.maxHistory {
		items = items[len(items)-h.maxHistory:]
	}
	out := make([]string, len(items))
	for i, it := range items {
		out[len(items)-1-i] = it
	}
	return out
}

func (h *Handler) searchHeight(total int) int {
	if total <= 1 {
		return 0
	}
	// Reserve at least one row for the inner inputbox so the
	// caller still sees the prompt and any partial input above the
	// overlay.
	maxOverlay := total - 1
	rows := h.list.MatchCount()
	if rows < 1 {
		// Always render at least one candidate row, even if the
		// list is empty: this keeps the overlay's "0/0" affordance
		// stable instead of collapsing the bottom block.
		rows = 1
	}
	if rows > searchOverlayMaxRows {
		rows = searchOverlayMaxRows
	}
	want := rows + searchOverlayChromeRows
	if want > maxOverlay {
		want = maxOverlay
	}
	return want
}
