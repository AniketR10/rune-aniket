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
	"testing"

	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/repl"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler/command"
)

// stubEditor is a minimal command.Editor for tests. It records every
// event delivered to its EditHandler and lets the test invoke an
// optional handle function to mutate the buffer (e.g. append a rune)
// the way a real editor would.
type stubEditor struct {
	seen *[]term.Event
	// initialCursor is recorded by SetCursorAtScroll so tests can
	// assert the host seeded the cursor correctly.
	initialCursor *term.Coordinates
}

func (s stubEditor) Edit(buf *cell.Buffer) command.EditHandler {
	return &stubEditHandler{
		buf:           buf,
		seen:          s.seen,
		initialCursor: s.initialCursor,
	}
}

type stubEditHandler struct {
	buf           *cell.Buffer
	seen          *[]term.Event
	initialCursor *term.Coordinates
	width         int
}

func (s *stubEditHandler) Resize(width, _ int) { s.width = width }
func (s *stubEditHandler) Draw(w term.Writer) {
	x, y := 0, 0
	for _, r := range s.buf.String() {
		if s.width > 0 && x >= s.width {
			x = 0
			y++
		}
		w.SetCell(term.Coordinates{X: x, Y: y}, term.Cell{Ch: r})
		x++
	}
}
func (s *stubEditHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{X: s.buf.Columns(0)},
		term.CursorStyleSteadyBar, true
}
func (s *stubEditHandler) CursorAtScroll() term.Coordinates {
	return term.Coordinates{X: s.buf.Columns(0)}
}
func (s *stubEditHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	if s.initialCursor != nil {
		*s.initialCursor = pos
	}
	return true
}
func (s *stubEditHandler) Selection() (string, bool) { return "", false }
func (s *stubEditHandler) Handle(ev term.Event) (bool, bool) {
	if s.seen != nil {
		*s.seen = append(*s.seen, ev)
	}
	if ev.Type != term.EventKey {
		return false, false
	}
	// A real vi/modeless editor consumes <tab> (it inserts
	// indentation). The shell wrapper must therefore intercept <tab>
	// before delegating, the same way a VTE owns <tab> and never
	// forwards it to the running program.
	if ev.Key == term.KeyTab && ev.Mod == 0 {
		s.buf.WriteString("\t")
		return false, true
	}
	// vi's insert mode consumes <c-r> (insert register). The shell
	// wrapper must therefore intercept <c-r> for reverse-history
	// search before delegating, the same way a VTE owns <c-r>.
	if ev.Mod == term.ModCtrl && ev.Ch == 'r' {
		return false, true
	}
	// A real editor consumes arrow keys (and vi-style <c-j>/<c-k>) as
	// cursor motion. The shell wrapper must intercept these for
	// history cycling before the editor sees them.
	if ev.Mod == 0 &&
		(ev.Key == term.KeyArrowUp || ev.Key == term.KeyArrowDown) {
		return false, true
	}
	if ev.Mod == term.ModCtrl && (ev.Ch == 'j' || ev.Ch == 'k') {
		return false, true
	}
	switch ev.Key {
	case term.KeyBackspace:
		cols := s.buf.Columns(0)
		if cols > 0 {
			s.buf.DeleteCell(term.Coordinates{X: cols - 1})
		}
		return false, true
	case term.KeySpace:
		s.buf.WriteString(" ")
		return false, true
	}
	if ev.Ch != 0 {
		s.buf.WriteString(string(ev.Ch))
		return false, true
	}
	return false, false
}

var _ command.EditHandler = (*stubEditHandler)(nil)

func newEditTestHandler(t *testing.T, editor command.Editor) *Handler {
	t.Helper()
	h, _ := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
		editor,
		Config{
			MaxHistory: 100,
		},
	)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func feedRunes(h *Handler, seq string) {
	for _, c := range seq {
		h.Handle(term.Event{Type: term.EventKey, Ch: c})
	}
}

func TestEditorReceivesTypedRunes(t *testing.T) {
	var seen []term.Event
	h := newEditTestHandler(t, stubEditor{seen: &seen})
	h.Resize(testWidthH, testHeight)

	feedRunes(h, "hello")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeySpace})
	feedRunes(h, "world")

	// Every key reached the editor and mutated its buffer.
	assert.Equal(t, "hello world", h.editBuf.String())
	assert.Len(t, seen, 11)
}

func countInsertKeys(events []term.Event) int {
	n := 0
	for _, ev := range events {
		if ev.Type == term.EventKey && ev.Ch == 'i' && ev.Mod == 0 {
			n++
		}
	}
	return n
}

func TestModalStartInsertForwardsInsertKey(t *testing.T) {
	tests := []struct {
		name        string
		modal       bool
		startInsert bool
		wantInserts int
	}{
		{name: "modal start insert", modal: true, startInsert: true, wantInserts: 1},
		{name: "modal no start insert", modal: true, startInsert: false, wantInserts: 0},
		{name: "modeless start insert", modal: false, startInsert: true, wantInserts: 0},
		{name: "modeless no start insert", modal: false, startInsert: false, wantInserts: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var seen []term.Event
			h, _ := New(
				func(func()) bool { return false },
				term.NopInterrupter(),
				stubEditor{seen: &seen},
				Config{
					MaxHistory:       100,
					Modal:            tt.modal,
					ModalStartInsert: tt.startInsert,
				},
			)
			t.Cleanup(func() { _ = h.Close() })
			assert.Equal(t, tt.wantInserts, countInsertKeys(seen))
		})
	}
}

func TestEditorSubmitDispatchesAndClears(t *testing.T) {
	var dispatched []string
	h, registry := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
		stubEditor{},
		Config{MaxHistory: 100},
	)
	t.Cleanup(func() { _ = h.Close() })
	registry.Register("e", "echo", echoCmd{out: &dispatched})

	h.Resize(testWidthH, testHeight)
	feedRunes(h, "e")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})
	h.inner.Wait()

	require.Equal(t, []string{"e"}, dispatched)
	// Submit clears the editor buffer so the next prompt is empty.
	assert.Equal(t, "", h.editBuf.String())
}

func TestNewPanicsWithoutEditor(t *testing.T) {
	assert.Panics(t, func() {
		_, _ = New(
			func(func()) bool { return false },
			term.NopInterrupter(),
			nil,
			Config{},
		)
	})
}

// arrowUp/arrowDown/ctrlJ/ctrlK are the history-cycling keys the shell
// wrapper must own (the editor would otherwise consume them as cursor
// motion).
var (
	arrowUp   = term.Event{Type: term.EventKey, Key: term.KeyArrowUp}
	arrowDown = term.Event{Type: term.EventKey, Key: term.KeyArrowDown}
	ctrlJ     = term.Event{Type: term.EventKey, Ch: 'j', Mod: term.ModCtrl}
	ctrlK     = term.Event{Type: term.EventKey, Ch: 'k', Mod: term.ModCtrl}
)

// TestArrowUpCyclesHistoryIntoEditor verifies that <up> recalls
// successively older history entries into the editor buffer even though
// the editor consumes arrow keys as cursor motion.
func TestArrowUpCyclesHistoryIntoEditor(t *testing.T) {
	h := newTestHandler(t, []string{"oldest", "middle", "newest"})
	h.Resize(testWidthH, testHeight)

	h.Handle(arrowUp)
	assert.Equal(t, "newest", h.editBuf.String())
	h.Handle(arrowUp)
	assert.Equal(t, "middle", h.editBuf.String())
	h.Handle(arrowUp)
	assert.Equal(t, "oldest", h.editBuf.String())
}

// TestArrowDownCyclesHistoryIntoEditor verifies that <down> walks back
// toward the most recent entry after <up> has moved into history.
func TestArrowDownCyclesHistoryIntoEditor(t *testing.T) {
	h := newTestHandler(t, []string{"oldest", "middle", "newest"})
	h.Resize(testWidthH, testHeight)

	h.Handle(arrowUp)
	h.Handle(arrowUp)
	require.Equal(t, "middle", h.editBuf.String())

	h.Handle(arrowDown)
	assert.Equal(t, "newest", h.editBuf.String())
}

// TestCtrlKCtrlJCycleHistory verifies the vi-style <c-k>/<c-j> aliases
// move up/down through history identically to the arrow keys.
func TestCtrlKCtrlJCycleHistory(t *testing.T) {
	h := newTestHandler(t, []string{"oldest", "middle", "newest"})
	h.Resize(testWidthH, testHeight)

	h.Handle(ctrlK)
	assert.Equal(t, "newest", h.editBuf.String())
	h.Handle(ctrlK)
	assert.Equal(t, "middle", h.editBuf.String())
	h.Handle(ctrlJ)
	assert.Equal(t, "newest", h.editBuf.String())
}

// TestEditingDuringHistoryCycleKeepsPosition verifies that editing a
// recalled line and pressing <up> again walks to the entry above it —
// matching how a real shell treats an edited history line as an
// in-place modification rather than resetting to the newest entry.
func TestEditingDuringHistoryCycleKeepsPosition(t *testing.T) {
	h := newTestHandler(t, []string{"oldest", "middle", "newest"})
	h.Resize(testWidthH, testHeight)

	h.Handle(arrowUp)
	h.Handle(arrowUp)
	require.Equal(t, "middle", h.editBuf.String())

	// Edit the recalled line.
	feedRunes(h, "X")
	require.Equal(t, "middleX", h.editBuf.String())

	// <up> walks to the entry above the edited line.
	h.Handle(arrowUp)
	assert.Equal(t, "oldest", h.editBuf.String())
}

// TestHistoryDownRestoresLiveEdit verifies that after typing a fresh
// line and pressing <up>, pressing <down> back past the newest entry
// restores the in-progress line the user was typing.
func TestHistoryDownRestoresLiveEdit(t *testing.T) {
	h := newTestHandler(t, []string{"newest"})
	h.Resize(testWidthH, testHeight)

	feedRunes(h, "draft")
	h.Handle(arrowUp)
	require.Equal(t, "newest", h.editBuf.String())

	h.Handle(arrowDown)
	assert.Equal(t, "draft", h.editBuf.String())
}

// multilineStubEditor is a command.Editor whose handler tracks a cursor
// row and moves it within the buffer on <up>/<down>, mirroring how a
// real editor navigates a multi-row buffer. It lets tests assert that
// the shell forwards cursor motion to the editor until a vertical edge
// is reached, only then cycling history.
type multilineStubEditor struct{ h *multilineStubHandler }

func (s *multilineStubEditor) Edit(buf *cell.Buffer) command.EditHandler {
	s.h = &multilineStubHandler{buf: buf}
	return s.h
}

type multilineStubHandler struct {
	buf   *cell.Buffer
	cy    int
	width int
}

func (s *multilineStubHandler) Resize(width, _ int) { s.width = width }
func (s *multilineStubHandler) Draw(term.Writer)    {}
func (s *multilineStubHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{Y: s.cy}, term.CursorStyleSteadyBar, true
}
func (s *multilineStubHandler) CursorAtScroll() term.Coordinates {
	return term.Coordinates{Y: s.cy}
}
func (s *multilineStubHandler) SetCursorAtScroll(pos term.Coordinates) bool {
	s.cy = pos.Y
	return true
}
func (s *multilineStubHandler) Selection() (string, bool) { return "", false }
func (s *multilineStubHandler) Handle(ev term.Event) (bool, bool) {
	if ev.Type != term.EventKey {
		return false, false
	}
	switch {
	case ev.Mod == 0 && ev.Key == term.KeyArrowUp:
		if s.cy > 0 {
			s.cy--
		}
		return false, true
	case ev.Mod == 0 && ev.Key == term.KeyArrowDown:
		if s.cy < s.buf.Rows()-1 {
			s.cy++
		}
		return false, true
	}
	return false, false
}

var _ command.EditHandler = (*multilineStubHandler)(nil)

// TestArrowKeysMoveWithinMultilineItemBeforeCyclingHistory reproduces
// the bug where recalling a multi-line history entry trapped the cursor:
// the shell swallowed <up>/<down> for history cycling instead of letting
// the editor move the cursor between the entry's lines. Cursor motion
// must stay inside the buffer until a vertical edge is reached.
func TestArrowKeysMoveWithinMultilineItemBeforeCyclingHistory(t *testing.T) {
	editor := &multilineStubEditor{}
	h := newTestHandlerFull(t, []string{"old", "a\nb\nc"}, 100, nil)
	// Swap in the multiline-aware editor and rebind to the live buffer.
	h.editHandler = editor.Edit(h.editBuf)
	h.Resize(testWidthH, testHeight)

	// Recall the newest (multi-line) entry; cursor lands on its last row.
	h.Handle(arrowUp)
	require.Equal(t, "a\nb\nc", h.editBuf.String())
	require.Equal(t, 2, editor.h.cy)

	// <up> moves the cursor up within the recalled item, not into history.
	h.Handle(arrowUp)
	assert.Equal(t, 1, editor.h.cy)
	assert.Equal(t, "a\nb\nc", h.editBuf.String())

	h.Handle(arrowUp)
	assert.Equal(t, 0, editor.h.cy)
	assert.Equal(t, "a\nb\nc", h.editBuf.String())

	// At the top row, a further <up> cycles to the older entry.
	h.Handle(arrowUp)
	assert.Equal(t, "old", h.editBuf.String())
}

// TestArrowDownMovesWithinMultilineItemBeforeCyclingHistory is the
// downward counterpart: once the cursor has moved up inside a recalled
// multi-line entry, <down> walks back down its rows and only cycles to
// a newer history entry when the cursor reaches the bottom row.
func TestArrowDownMovesWithinMultilineItemBeforeCyclingHistory(t *testing.T) {
	editor := &multilineStubEditor{}
	h := newTestHandlerFull(t, []string{"a\nb\nc", "newest"}, 100, nil)
	h.editHandler = editor.Edit(h.editBuf)
	h.Resize(testWidthH, testHeight)

	// Recall "newest", then the older multi-line entry; cursor on row 2.
	h.Handle(arrowUp)
	require.Equal(t, "newest", h.editBuf.String())
	h.Handle(arrowUp)
	require.Equal(t, "a\nb\nc", h.editBuf.String())
	require.Equal(t, 2, editor.h.cy)

	// Climb to the top row of the recalled entry.
	h.Handle(arrowUp)
	h.Handle(arrowUp)
	require.Equal(t, 0, editor.h.cy)
	require.Equal(t, "a\nb\nc", h.editBuf.String())

	// <down> walks back down the entry's rows, not into newer history.
	h.Handle(arrowDown)
	assert.Equal(t, 1, editor.h.cy)
	assert.Equal(t, "a\nb\nc", h.editBuf.String())
	h.Handle(arrowDown)
	assert.Equal(t, 2, editor.h.cy)
	assert.Equal(t, "a\nb\nc", h.editBuf.String())

	// At the bottom row, a further <down> cycles to the newer entry.
	h.Handle(arrowDown)
	assert.Equal(t, "newest", h.editBuf.String())
}

func TestCtrlROpensReverseHistorySearch(t *testing.T) {
	h := newEditTestHandler(t, stubEditor{})
	h.Resize(testWidthH, testHeight)

	// Ordinary runes are consumed by the editor.
	feedRunes(h, "ab")
	assert.Equal(t, "ab", h.editBuf.String())
	assert.False(t, h.searching)

	// <c-r> is intercepted by the host (never delegated to the editor)
	// and opens the reverse-history search overlay.
	h.Handle(term.Event{Type: term.EventKey, Ch: 'r', Mod: term.ModCtrl})
	require.True(t, h.searching)
	assert.Equal(t, modeHistory, h.mode)
	h.cancelSearch()
}

// TestCtrlRNeverReachesEditor guards the VTE-like contract: the shell
// wrapper owns <c-r> and must intercept it for reverse-history search
// before the editor — which in vi insert mode would otherwise consume
// it to insert a register — ever sees it.
func TestCtrlRNeverReachesEditor(t *testing.T) {
	var seen []term.Event
	h := newEditTestHandler(t, stubEditor{seen: &seen})
	h.Resize(testWidthH, testHeight)

	feedRunes(h, "ab")
	h.Handle(term.Event{Type: term.EventKey, Ch: 'r', Mod: term.ModCtrl})

	for _, ev := range seen {
		require.False(t, ev.Mod == term.ModCtrl && ev.Ch == 'r',
			"editor must never receive <c-r>")
	}
	assert.True(t, h.searching)
	assert.Equal(t, modeHistory, h.mode)
	h.cancelSearch()
}

func TestTabTriggersCompletion(t *testing.T) {
	h, registry := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
		stubEditor{},
		Config{MaxHistory: 100},
	)
	t.Cleanup(func() { _ = h.Close() })
	registry.Register("foo", "foo", echoCmd{out: new([]string)})
	registry.Register("foobar", "foobar", echoCmd{out: new([]string)})

	h.Resize(testWidthH, testHeight)
	feedRunes(h, "foo")
	require.Equal(t, "foo", h.editBuf.String())

	// <tab> is intercepted by the host (never delegated to the editor)
	// and forwarded to the completion machinery, which opens the
	// overlay for the two matching commands.
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})
	require.True(t, h.searching)
	assert.Equal(t, modeCompletion, h.mode)
	h.cancelSearch()
}

// TestTabNeverReachesEditor guards the VTE-like contract: the shell
// wrapper owns <tab> and must intercept it for completion before the
// editor — which would otherwise consume it to insert indentation —
// ever sees it.
func TestTabNeverReachesEditor(t *testing.T) {
	var seen []term.Event
	h, registry := New(
		func(func()) bool { return false },
		term.NopInterrupter(),
		stubEditor{seen: &seen},
		Config{MaxHistory: 100},
	)
	t.Cleanup(func() { _ = h.Close() })
	registry.Register("foo", "foo", echoCmd{out: new([]string)})
	registry.Register("foobar", "foobar", echoCmd{out: new([]string)})

	h.Resize(testWidthH, testHeight)
	feedRunes(h, "foo")
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})

	for _, ev := range seen {
		require.False(t, ev.Key == term.KeyTab && ev.Mod == 0,
			"editor must never receive <tab>")
	}
	// The editor buffer keeps the typed prefix; no tab was inserted.
	assert.Equal(t, "foo", h.editBuf.String())
	assert.True(t, h.searching)
	h.cancelSearch()
}

// TestAcceptHistoryMatchReplacesEditorBuffer verifies that choosing a
// reverse-history result writes the chosen command into the editor
// buffer (the editor owns the input line), not the inner inputbox.
func TestAcceptHistoryMatchReplacesEditorBuffer(t *testing.T) {
	h := newTestHandler(t, []string{"deploy --prod"})
	h.Resize(testWidthH, testHeight)

	// Open reverse-history search; the single entry is focused.
	h.Handle(term.Event{Type: term.EventKey, Ch: 'r', Mod: term.ModCtrl})
	require.True(t, h.searching)
	require.Equal(t, modeHistory, h.mode)

	// Accept the focused match.
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	assert.False(t, h.searching)
	assert.Equal(t, "deploy --prod", h.editBuf.String())
}

// TestAcceptCompletionCandidateReplacesEditorBuffer verifies that
// accepting a tab-completion candidate rewrites the editor buffer with
// the completed command rather than mutating the inner inputbox.
func TestAcceptCompletionCandidateReplacesEditorBuffer(t *testing.T) {
	h := newTestHandlerFull(t, nil, 100, func(r *CommandRegistry) {
		r.Register("foobar", "foobar", stubCmd{})
		r.Register("foobaz", "foobaz", stubCmd{})
	})
	h.Resize(testWidthH, testHeight)

	feedRunes(h, "foob")
	require.Equal(t, "foob", h.editBuf.String())

	// <tab> opens the completion overlay with the two matches; the
	// first is focused.
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})
	require.True(t, h.searching)
	require.Equal(t, modeCompletion, h.mode)

	// Accept the focused candidate.
	h.Handle(term.Event{Type: term.EventKey, Key: term.KeyEnter})

	assert.False(t, h.searching)
	// The chosen candidate fully replaces the editor buffer.
	assert.Contains(t, []string{"foobar", "foobaz"}, h.editBuf.String())
}

// echoCmd is a CommandHandler that records its dispatched name into
// a caller-supplied slice. It is used to verify that edit-mode
// submit forwards the original <enter> through to the inner repl.
type echoCmd struct{ out *[]string }

func (e echoCmd) HandleCommand(
	_ context.Context, cmd repl.Command, _ repl.ProgressWriter,
) (iterator.Iterator[component.Responsive], error) {
	*e.out = append(*e.out, cmd.Name)
	return iterator.Empty[component.Responsive](), nil
}

func (e echoCmd) Complete(
	context.Context, string, []string,
) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

func (e echoCmd) Help(
	context.Context, []string,
) (iterator.Iterator[component.Responsive], error) {
	return iterator.Empty[component.Responsive](), nil
}
