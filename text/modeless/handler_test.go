// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2024 Unstable Build, All Rights Reserved.
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

package modeless

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/clipboard"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/registerhistory"
	"unstable.build/go-tui/text/registerset"
)

type testSelectionService struct {
	view   cell.View
	expand map[term.Range]term.Range
	shrink map[term.Range]term.Range
}

type testCommentService struct {
	view  cell.View
	line  []string
	block []string
}

func (s testSelectionService) Rows() int { return s.view.Rows() }

func (s testSelectionService) Columns(row int) int { return s.view.Columns(row) }

func (s testSelectionService) Cell(at term.Coordinates) (term.Cell, bool) {
	return s.view.Cell(at)
}

func (s testSelectionService) RawCells() [][]term.Cell { return s.view.RawCells() }

func (s testSelectionService) String() string { return s.view.String() }

func (s testSelectionService) SelectionExpand(rng term.Range) (term.Range, bool) {
	next, ok := s.expand[rng]
	return next, ok
}

func (s testSelectionService) SelectionShrink(rng term.Range, caret term.Coordinates) (term.Range, bool) {
	next, ok := s.shrink[rng]
	return next, ok
}

func (s testCommentService) Rows() int { return s.view.Rows() }

func (s testCommentService) Columns(row int) int { return s.view.Columns(row) }

func (s testCommentService) Cell(at term.Coordinates) (term.Cell, bool) { return s.view.Cell(at) }

func (s testCommentService) RawCells() [][]term.Cell { return s.view.RawCells() }

func (s testCommentService) String() string { return s.view.String() }

func (s testCommentService) CommentCoverage(rng term.Range) ([]term.Range, bool) {
	start, end := term.CoordinatesSort(rng.Start, rng.End)
	var ranges []term.Range
	for y := start.Y; y <= end.Y; y++ {
		line := term.CellsToString([][]term.Cell{s.view.RawCells()[y]})
		trimmed := strings.TrimLeft(line, " \t")
		indent := len(line) - len(trimmed)
		for _, prefix := range s.line {
			if strings.HasPrefix(trimmed, prefix) {
				ranges = append(ranges, term.Range{
					Start: term.Coordinates{Y: y, X: indent},
					End:   term.Coordinates{Y: y, X: len(line)},
				})
				goto nextLine
			}
			if strings.HasPrefix(trimmed, prefix+" ") {
				ranges = append(ranges, term.Range{
					Start: term.Coordinates{Y: y, X: indent},
					End:   term.Coordinates{Y: y, X: len(line)},
				})
				goto nextLine
			}
		}
		for i := 0; i+1 < len(s.block); i += 2 {
			open, close := s.block[i], s.block[i+1]
			openIdx := strings.Index(line, open)
			closeIdx := strings.LastIndex(line, close)
			if openIdx >= 0 && closeIdx >= openIdx+len(open) {
				ranges = append(ranges, term.Range{
					Start: term.Coordinates{Y: y, X: openIdx},
					End:   term.Coordinates{Y: y, X: closeIdx + len(close)},
				})
				goto nextLine
			}
		}
		return nil, false
	nextLine:
	}
	if len(ranges) == 0 {
		return nil, false
	}
	return ranges, true
}

func TestCursorExternalEdit(t *testing.T) {
	uri, err := workspaceapi.ParseURI("test:///")
	require.NoError(t, err)

	t.Run("if external insert above, moves cursor to keep cursor in current logical line", func(t *testing.T) {
		width, height := 20, 10

		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(snippet))
		vi := NewHandler(buf, uri, text.IndentRuneTab)
		vi.Resize(width, height)

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)

		assert.Equal(t, term.Coordinates{Y: 2}, vi.CursorAtScroll())
		at := term.Coordinates{}
		vi.CellEditor().Edit(context.Background(), at, at, "a\nb")
		assert.Equal(t, term.Coordinates{Y: 3}, vi.CursorAtScroll())
	})

	t.Run("if external insert below, it does nothing", func(t *testing.T) {
		width, height := 20, 10

		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(snippet))
		vi := NewHandler(buf, uri, text.IndentRuneTab)
		vi.Resize(width, height)

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)

		assert.Equal(t, term.Coordinates{Y: 2}, vi.CursorAtScroll())
		at := term.Coordinates{Y: 3}
		vi.CellEditor().Edit(context.Background(), at, at, "a\nb")
		assert.Equal(t, term.Coordinates{Y: 2}, vi.CursorAtScroll())
	})

	t.Run("if external delete below, it does nothing", func(t *testing.T) {
		width, height := 20, 10

		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(snippet))
		vi := NewHandler(buf, uri, text.IndentRuneTab)
		vi.Resize(width, height)

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)

		assert.Equal(t, term.Coordinates{Y: 2}, vi.CursorAtScroll())
		from := term.Coordinates{Y: 3}
		to := term.Coordinates{Y: 4}
		vi.CellEditor().Edit(context.Background(), from, to, "")
		assert.Equal(t, term.Coordinates{Y: 2}, vi.CursorAtScroll())
	})

	t.Run("if external delete above, it keeps cursor at logical line", func(t *testing.T) {
		width, height := 20, 10

		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(snippet))
		vi := NewHandler(buf, uri, text.IndentRuneTab)
		vi.Resize(width, height)

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)

		assert.Equal(t, term.Coordinates{Y: 2}, vi.CursorAtScroll())
		from := term.Coordinates{Y: 0}
		to := term.Coordinates{Y: 1}
		vi.CellEditor().Edit(context.Background(), from, to, "")
		assert.Equal(t, term.Coordinates{Y: 1}, vi.CursorAtScroll())
	})

	t.Run("if external delete to current line, it keeps cursor at logical line", func(t *testing.T) {
		width, height := 20, 10

		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(snippet))
		vi := NewHandler(buf, uri, text.IndentRuneTab)
		vi.Resize(width, height)

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
		require.True(t, handled)

		assert.Equal(t, term.Coordinates{Y: 2, X: 1}, vi.CursorAtScroll())
		from := term.Coordinates{Y: 0}
		to := term.Coordinates{Y: 2, X: 5}
		vi.CellEditor().Edit(context.Background(), from, to, "")
		assert.Equal(t, term.Coordinates{Y: 0, X: 0}, vi.CursorAtScroll())
	})

	t.Run("if external insert to current line, it keeps cursor at logical line", func(t *testing.T) {
		width, height := 20, 10

		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(snippet))
		vi := NewHandler(buf, uri, text.IndentRuneTab)
		vi.Resize(width, height)

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
		require.True(t, handled)

		assert.Equal(t, term.Coordinates{Y: 2, X: 1}, vi.CursorAtScroll())
		at := term.Coordinates{Y: 2, X: 0}
		vi.CellEditor().Edit(context.Background(), at, at, "a\nbbb")
		assert.Equal(t, term.Coordinates{Y: 3, X: 3}, vi.CursorAtScroll())
	})

	t.Run("if external replace to current line, it keeps cursor at logical line", func(t *testing.T) {
		width, height := 20, 10

		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(snippet))
		vi := NewHandler(buf, uri, text.IndentRuneTab)
		vi.Resize(width, height)

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
		require.True(t, handled)

		assert.Equal(t, term.Coordinates{Y: 2, X: 1}, vi.CursorAtScroll())
		start := term.Coordinates{Y: 1, X: 0}
		end := term.Coordinates{Y: 2, X: 1}
		vi.CellEditor().Edit(context.Background(), start, end, "a\nbbb")
		assert.Equal(t, term.Coordinates{Y: 2, X: 3}, vi.CursorAtScroll())
	})

	t.Run("if external replace only cols to current line, it keeps cursor at logical position", func(t *testing.T) {
		width, height := 20, 10

		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader(snippet))
		vi := NewHandler(buf, uri, text.IndentRuneTab)
		vi.Resize(width, height)

		_, handled := vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowDown})
		require.True(t, handled)
		for range 3 {
			_, handled = vi.Handle(term.Event{Type: term.EventKey, Key: term.KeyArrowRight})
			require.True(t, handled)
		}

		assert.Equal(t, term.Coordinates{Y: 2, X: 3}, vi.CursorAtScroll())
		start := term.Coordinates{Y: 2, X: 0}
		end := term.Coordinates{Y: 2, X: 4}
		vi.CellEditor().Edit(context.Background(), start, end, "bbb")
		assert.Equal(t, term.Coordinates{Y: 2, X: 3}, vi.CursorAtScroll())
	})
}

func TestLocationMessage(t *testing.T) {
	cases := []handlertest.SequenceTestCase{
		{"<down>",
			`a                   
▐                   
c                   
                    
                    
                    
                    
                    
                    
            durrdurr`},
		{"<down><down>",
			`a                   
b                   
▐                   
                    
                    
                    
                    
                    
                    
                    `},
		{"<down><down><up>",
			`a                   
▐                   
c                   
                    
                    
                    
                    
                    
                    
            durrdurr`},
	}

	newVi := func(t *testing.T) tui.Handler {
		uri, err := workspaceapi.ParseURI("memory:///myfile")
		require.NoError(t, err)
		buf := cell.NewBuffer()
		_, _ = buf.ReadFrom(strings.NewReader("a\nb\nc"))
		handler := NewHandler(buf, uri, text.IndentRuneTab)
		handler.Resize(20, 10)
		handler.SetLocationList(textapi.LocationPriorityInfo, "id",
			textapi.LocationSlice([]textapi.Location{
				{
					Message: "durrdurr",
					From:    term.Coordinates{Y: 1},
					To:      term.Coordinates{Y: 1, X: 5},
				},
			}))
		return handler
	}
	handlertest.RunHandlerIsolated(t, newVi, 20, 10, cases)
}

func TestSyntacticSelectionKeyBindings(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///selection.go")
	require.NoError(t, err)

	buf := cell.NewBuffer()
	_, _ = buf.ReadFrom(strings.NewReader("alpha beta gamma"))

	view := testSelectionService{
		view: buf.View(),
		expand: map[term.Range]term.Range{
			{Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 6}}:  {Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}},
			{Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}}: {Start: term.Coordinates{}, End: term.Coordinates{X: 10}},
		},
		shrink: map[term.Range]term.Range{
			{Start: term.Coordinates{}, End: term.Coordinates{X: 10}}:     {Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}},
			{Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 10}}: {Start: term.Coordinates{X: 6}, End: term.Coordinates{X: 6}},
		},
	}
	buf.WithView(view)

	h := NewHandler(buf, uri, text.IndentRuneTab)
	h.Resize(80, 10)
	require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 6}))

	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'w'})
	require.True(t, handled)
	selection, ok := h.Selection()
	require.True(t, ok)
	assert.Equal(t, "beta", selection)

	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'w'})
	require.True(t, handled)
	selection, ok = h.Selection()
	require.True(t, ok)
	assert.Equal(t, "alpha beta", selection)

	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl | term.ModShift, Ch: 'W'})
	require.True(t, handled)
	selection, ok = h.Selection()
	require.True(t, ok)
	assert.Equal(t, "beta", selection)

	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl | term.ModShift, Ch: 'W'})
	require.True(t, handled)
	selection, ok = h.Selection()
	assert.False(t, ok)
	assert.Equal(t, "", selection)
	assert.Equal(t, term.Coordinates{X: 6}, h.CursorAtScroll())

	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl | term.ModShift, Ch: 'W'})
	assert.False(t, handled)
}

func TestSublimeKeyBindingsMacOS(t *testing.T) {
	const snippet = "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"
	spAll := func(add string) *string {
		return new(snippet + add)
	}

	suite := []struct {
		description string
		keycomb     string
		result      *string
		coordinates term.Coordinates
		clipboard   *string
	}{
		// General editing
		{"Cut (cuts entire line when nothing selected)", "<meta-x>", new("b\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Copy+Paste", "<shift-right><meta-c><meta-v>", new("aa\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Copy+Paste and indent correctly", "<shift-right><meta-c><down><shift-meta-v>", new("a\nab\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 1}, nil},
		{"Paste from clipboard history", "<shift-right><meta-c><meta-v><meta-v><alt-meta-v>", new("aaa\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 2}, nil},
		{"Undo", "<meta-x><meta-z>", new("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Redo", "<meta-x><meta-z><shift-meta-z>", new("b\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Redo or repeat last command", "<meta-x><meta-z><meta-y>", new("b\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		// {"Soft undo (undo cursor movement without undoing edit)", "<right><right><meta-u>", nil, term.Coordinates{Y: 0, X: 0}},
		// {"Soft redo", "<right><right><meta-u><shift-meta-u>", nil, term.Coordinates{Y: 0, X: 1}},
		// {"Trigger auto-complete", "<ctrl-space>", nil, term.Coordinates{}},
		{"Insert completion/snippet or indent", "<tab>", new("\ta\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Previous snippet field or unindent", "<tab><shift-tab>", new("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},

		// Line manipulation
		{"Insert line after current line", "<ctrl-enter>", new("a\n\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 0}, nil},
		{"Insert line before current line", "<ctrl-shift-enter>", new("\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Move line/selection up", "<down><ctrl-meta-up>", new("b\na\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Move line/selection down", "<ctrl-meta-down>", new("b\na\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 0}, nil},
		{"Duplicate line(s)", "<shift-meta-d>", new("a\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 0}, nil},
		{"Delete entire line", "<ctrl-shift-k>", new("b\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Join line below to end of current line", "<meta-j>", new("ab\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Indent current line(s)", "<meta-]>", new("\ta\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Unindent current line(s)", "<meta-]><meta-[>", new("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Delete from cursor to end of line", "<meta-k><meta-k>", new("\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Delete to beginning of line", "<right><meta-k><meta-backspace>", new("\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Delete to end of line", "<meta-delete>", new("\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Transpose (swap adjacent characters)", "<down><right><ctrl-t>", new("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 1}, nil},
		//{"Sort lines alphabetically", "<f5>", nil, term.Coordinates{}}, // Already sorted a-k
		//{"Sort lines (case sensitive)", "<ctrl-f5>", nil, term.Coordinates{}},

		// Comments - depends on language/syntax (assuming C-style)
		{"Toggle line comment", "<meta-/>", new("// a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 3}, nil},
		{"Toggle block comment", "<shift-right><alt-meta-/>", new("/*a*/\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 2}, nil},

		// Text transformation - require selection
		{"Transform selection to UPPERCASE", "<shift-right><meta-k><meta-u>", new("A\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Transform selection to lowercase", "<shift-right><meta-k><meta-u><home><shift-right><meta-k><meta-l>", new("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		//{"Wrap paragraph at ruler", "<alt-meta-q>", nil, term.Coordinates{}}, // no effect on single character line
		//{"Wrap selection in HTML tag", "<shift-right><ctrl-shift-w>", sp("<p>a</p>\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 3}},
		//{"Close current HTML/XML tag", "<alt-meta-.>", nil, term.Coordinates{}}, // No open tag

		// Selection
		{"Select all", "<meta-a><m-c>", nil, term.Coordinates{}, spAll("\n")},
		{"Select entire line (repeat to select additional lines)", "<down><meta-l><meta-l><m-c>", nil, term.Coordinates{Y: 1, X: 0}, new("b\nc\n")},
		{"Select word at cursor (repeat to select next occurrence)", "<meta-down>a<enter><meta-up><meta-d><meta-d><meta-d><meta-c>", nil, term.Coordinates{}, new("a")},
		//{"Select word at cursor (repeat to select next occurrence, multi cursor edits all)", "<meta-down>a<meta-up><meta-d><meta-c>", nil, term.Coordinates{Y: 10, X: 0}, sp("a")},
		//{"Skip current selection, find and select next occurrence", "<meta-d><meta-k><meta-d>", nil, term.Coordinates{X: 0}, sp("a")},
		//{"Select all occurrences of current selection", "<meta-d><ctrl-meta-g>", nil, term.Coordinates{}},
		//{"Split selection into multiple cursors (one per line)", "<meta-a><shift-meta-l>", nil, term.Coordinates{}},
		//{"Add cursor on previous line (column selection up)", "<down><ctrl-shift-up>", nil, term.Coordinates{Y: 0, X: 0}},
		//{"Add cursor on next line (column selection down)", "<ctrl-shift-down>", nil, term.Coordinates{Y: 1, X: 0}},
		//{"Add cursor at click location", "<meta-click>", nil, term.Coordinates{}},
		//{"Exit multiple selections (single selection mode)", "<ctrl-shift-down><escape>", nil, term.Coordinates{Y: 0, X: 0}},

		// Expand selection
		//{"Expand selection to brackets", "{abc}<left><left><shift-left><ctrl-shift-m>", nil, term.Coordinates{X: 4}}, // No brackets
		//{"Expand selection to HTML/XML tag", "<shift-meta-a>", nil, term.Coordinates{}}, // No tags
		//{"Expand selection to scope", "<shift-meta-space>", nil, term.Coordinates{}},
		//{"Expand selection to indentation level", "<shift-meta-j>", nil, term.Coordinates{}},

		// Navigation and movement
		{"Move cursor to beginning of line", "<right><ctrl-a>", nil, term.Coordinates{Y: 0, X: 0}, nil},
		{"Move cursor to end of line", "<ctrl-e>", nil, term.Coordinates{Y: 0, X: 1}, nil},
		{"Move to beginning of text on line", "<space><right><meta-left>", nil, term.Coordinates{Y: 0, X: 1}, nil},
		{"Move to end of line", "<meta-right>", nil, term.Coordinates{Y: 0, X: 1}, nil},
		{"Move up one line", "<down><ctrl-p>", nil, term.Coordinates{Y: 0, X: 0}, nil},
		{"Move down one line", "<ctrl-n>", nil, term.Coordinates{Y: 1, X: 0}, nil},
		{"Move right one character", "<ctrl-f>", nil, term.Coordinates{Y: 0, X: 1}, nil},
		{"Move left one character", "<right><ctrl-b>", nil, term.Coordinates{Y: 0, X: 0}, nil},
		{"Jump to matching bracket", "{}<left><left><ctrl-m>", nil, term.Coordinates{X: 1}, nil},
		{"Move to start of file", "<down><down><meta-up>", nil, term.Coordinates{Y: 0, X: 0}, nil},
		{"Move to end of file", "<meta-down>", nil, term.Coordinates{Y: 10, X: 0}, nil},
		//{"Jump back (previous location)", "<meta-down><ctrl-->", nil, term.Coordinates{Y: 0, X: 0}, nil},
		//{"Jump forward (next location)", "<meta-down><ctrl--><ctrl-shift-->", nil, term.Coordinates{Y: 10, X: 1}, nil},

		// Scrolling
		{"Center current line in view", "<meta-down>z<enter>z<enter>z<enter>z<enter>z<enter>z<enter><up><up><up><up><ctrl-l>",
			nil, term.Coordinates{Y: 12}, nil},
		{"Scroll down one page", "<ctrl-v>", nil, term.Coordinates{Y: 3, X: 0}, nil},
		{"Scroll view up one line", "<meta-down><up><ctrl-alt-up>", nil, term.Coordinates{Y: 9}, nil},
		{"Scroll view down one line", "<ctrl-alt-down>", nil, term.Coordinates{Y: 1}, nil},

		// Search and replace
		//{"Find", "<meta-f>", nil, term.Coordinates{}},
		//{"Find next", "<meta-f><meta-g>", nil, term.Coordinates{}},
		//{"Find previous", "<meta-f><shift-meta-g>", nil, term.Coordinates{}},
		//{"Incremental find", "<meta-i>", nil, term.Coordinates{}},
		//{"Find and replace", "<alt-meta-f>", nil, term.Coordinates{}},
		//{"Replace next", "<alt-meta-f><alt-meta-e>", nil, term.Coordinates{}},
		//{"Replace all", "<alt-meta-f><ctrl-meta-e>", nil, term.Coordinates{}},
		//{"Find in files", "<shift-meta-f>", nil, term.Coordinates{}},
		//{"Next result in file search", "<f4>", nil, term.Coordinates{}},
		//{"Previous result in file search", "<shift-f4>", nil, term.Coordinates{}},
		//{"Use selection for find", "<shift-right><meta-e>", nil, term.Coordinates{Y: 0, X: 1}},
		//{"Use selection for replace", "<shift-right><shift-meta-e>", nil, term.Coordinates{Y: 0, X: 1}},
		//{"Quick find (select word under cursor)", "<alt-meta-g>", nil, term.Coordinates{}},
		//{"Quick find all (select all occurrences of word)", "<ctrl-meta-g>", nil, term.Coordinates{}},

		// Bookmarks (done via command.key_bindings)
		//{"Toggle bookmark on current line", "<meta-f2>", nil, term.Coordinates{}},
		//{"Jump to next bookmark", "<meta-f2><f2>", nil, term.Coordinates{Y: 0, X: 0}},
		//{"Jump to previous bookmark", "<meta-f2><shift-f2>", nil, term.Coordinates{Y: 0, X: 0}},
		//{"Select all bookmarks", "<meta-f2><alt-f2>", nil, term.Coordinates{}},
		//{"Clear all bookmarks", "<meta-f2><shift-meta-f2>", nil, term.Coordinates{}},

		// Marks (advanced bookmarking)
		//{"Set mark at cursor position", "<meta-k><meta-space>", nil, term.Coordinates{}},
		//{"Select from cursor to mark", "<meta-k><meta-space><down><down><meta-k><meta-a>", nil, term.Coordinates{Y: 0, X: 0}},
		//{"Delete from cursor to mark", "<meta-k><meta-space><down><down><meta-k><meta-w>", sp("c\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}},
		//{"Swap cursor position with mark", "<meta-k><meta-space><down><down><meta-k><meta-x>", nil, term.Coordinates{Y: 0, X: 0}},
		//{"Clear mark", "<meta-k><meta-space><meta-k><meta-g>", nil, term.Coordinates{}},

		// Macros
		//{"Start/stop recording macro", "<ctrl-q>", nil, term.Coordinates{}},
		//{"Playback recorded macro", "<ctrl-q><right><ctrl-q><ctrl-shift-q>", nil, term.Coordinates{Y: 0, X: 1}},

		// Build system
		// {"Build (run default build system)", "<meta-b>", nil, term.Coordinates{}},
		// {"Build with... (select build system)", "<shift-meta-b>", nil, term.Coordinates{}},
		// {"Cancel current build", "<ctrl-c>", nil, term.Coordinates{}},

		// Spell check
		//{"Toggle spell check", "<f6>", nil, term.Coordinates{}},
		//{"Jump to next misspelling", "<ctrl-f6>", nil, term.Coordinates{}},
		//{"Jump to previous misspelling", "<ctrl-shift-f6>", nil, term.Coordinates{}},

		// Auto-pairing (context-dependent) - these insert characters
		// {"Auto-pair double quotes", "\"", sp("\"\"\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}},
		// {"Auto-pair single quotes", "'", sp("''\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}},
		// {"Auto-pair parentheses", "(", sp("()\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}},
		// {"Auto-pair square brackets", "[", sp("[]\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}},
		// {"Auto-pair curly braces", "{", sp("{}\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}},
		// {"Delete matching pair (when between paired characters)", "(<backspace>", sp("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}},
		// {"Add line between paired braces", "{<enter>", sp("{\n\n}\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 0}},
	}

	uri, err := workspaceapi.ParseURI("memory:///myfile.go")
	require.NoError(t, err)

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			seq, err := term.ParseKeys(test.keycomb)
			require.NoError(t, err)

			clip := clipboard.NewInMemory()
			reg := registerhistory.NewClipboard(registerset.New(clip))
			buf := cell.NewBuffer()
			buf.ReadFrom(strings.NewReader(snippet))
			buf.WithView(testCommentService{
				view:  buf.View(),
				line:  []string{"//"},
				block: []string{"/*", "*/"},
			})
			handler := NewHandler(buf, uri, text.IndentRuneTab, WithClipboard(reg))
			handler = NewHandler(buf, uri,
				text.IndentRuneTab,
				WithClipboard(reg),
				WithComments(text.CommentConfig{
					"go": {
						Line:  []string{"//"},
						Block: []text.CommentBlock{{Start: "/*", End: "*/"}},
					},
				}),
			)
			handler.Resize(10, 3)
			for _, key := range seq {
				ev := term.Event{Type: term.EventKey, Key: key.Key, Mod: key.Mod, Ch: key.Ch}
				_, ok := handler.Handle(ev)
				require.True(t, ok)
			}
			if test.result != nil {
				assert.Equal(t, *test.result, buf.String())
			}
			if test.clipboard != nil {
				paste, err := clip.Paste(clipboard.DefaultRegisterID)
				require.NoError(t, err)
				assert.Equal(t, *test.clipboard, paste.Text)
			}
			assert.Equal(t, test.coordinates, handler.CursorAtScroll())
		})
	}
}

func TestPasteFromClipboardHistory(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///myfile")
	require.NoError(t, err)

	type pasteHistoryStep struct {
		input       string
		wantHandled bool
		wantContent string
		wantCursor  *term.Coordinates
	}

	type pasteHistoryTest struct {
		name        string
		content     string
		history     []string
		wrapHistory bool
		steps       []pasteHistoryStep
	}

	coords := func(pos term.Coordinates) *term.Coordinates { return &pos }

	runKeys := func(t *testing.T, h text.Handler, keys string, wantHandled bool) {
		t.Helper()
		seq, err := term.ParseKeys(keys)
		require.NoError(t, err)
		for _, key := range seq {
			ev := term.Event{Type: term.EventKey, Key: key.Key, Mod: key.Mod, Ch: key.Ch}
			_, ok := h.Handle(ev)
			require.Equal(t, wantHandled, ok, "input %q", keys)
		}
	}

	tests := []pasteHistoryTest{
		{
			name:        "cycles through multiple history entries",
			content:     "z",
			history:     []string{"a", "b", "c"},
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<end><meta-v>", wantHandled: true, wantContent: "zc", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "za", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "za", wantCursor: coords(term.Coordinates{X: 2})},
			},
		},
		{
			name:        "regular paste restarts history cycle",
			content:     "z",
			history:     []string{"a", "b", "c"},
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<end><meta-v>", wantHandled: true, wantContent: "zc"},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "zb"},
				{input: "<meta-v>", wantHandled: true, wantContent: "zbc"},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "zbb"},
			},
		},
		{
			name:        "typing after paste starts a new history paste",
			content:     "z",
			history:     []string{"a", "b"},
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<end><meta-v>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "x", wantHandled: true, wantContent: "zbx", wantCursor: coords(term.Coordinates{X: 3})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "zbxb", wantCursor: coords(term.Coordinates{X: 4})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "zbxa", wantCursor: coords(term.Coordinates{X: 4})},
			},
		},
		{
			name:        "cursor movement after paste starts a new history paste",
			content:     "z",
			history:     []string{"a", "b"},
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<end><meta-v>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<left>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 1})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "zbb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "zab", wantCursor: coords(term.Coordinates{X: 2})},
			},
		},
		{
			name:        "clipboard without history support consumes in paste context",
			content:     "z",
			history:     []string{"a", "b"},
			wrapHistory: false,
			steps: []pasteHistoryStep{
				{input: "<end><meta-v>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
			},
		},
		{
			name:        "alt-meta-v before paste initiates history paste",
			content:     "z",
			history:     []string{"a", "b", "c"},
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<end><alt-meta-v>", wantHandled: true, wantContent: "zc", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "za", wantCursor: coords(term.Coordinates{X: 2})},
			},
		},
		{
			name:        "alt-meta-v without history support before paste is not handled",
			content:     "z",
			history:     []string{"a", "b"},
			wrapHistory: false,
			steps: []pasteHistoryStep{
				{input: "<alt-meta-v>", wantHandled: false, wantContent: "z", wantCursor: coords(term.Coordinates{})},
			},
		},
		{
			name:        "modeless copy operations populate shared history",
			content:     "ab\ncd",
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<shift-right><meta-c><right><shift-right><meta-c><end><meta-v>", wantHandled: true, wantContent: "abb\ncd"},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "aba\ncd"},
				{input: "<alt-meta-v>", wantHandled: true, wantContent: "aba\ncd"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clip := clipboard.NewInMemory()
			var reg clipboard.Register = registerset.New(clip)
			if test.wrapHistory {
				reg = registerhistory.NewClipboard(reg)
			}
			for _, entry := range test.history {
				require.NoError(t, reg.Copy(clipboard.DefaultRegisterID, clipboard.Data{Text: entry, Metadata: text.StandardSelection}))
			}

			buf := cell.NewBuffer()
			buf.ReadFrom(strings.NewReader(test.content))
			h := NewHandler(buf, uri, text.IndentRuneTab, WithClipboard(reg))
			h.Resize(10, 3)

			for _, step := range test.steps {
				runKeys(t, h, step.input, step.wantHandled)
				assert.Equal(t, step.wantContent, buf.String(), "input %q", step.input)
				if step.wantCursor != nil {
					assert.Equal(t, *step.wantCursor, h.CursorAtScroll(), "input %q", step.input)
				}
			}
		})
	}
}

// testIndentView wraps a cell.View and provides an IndentationAt method
// so that ReindentSelection actually adjusts indentation in tests.
type testIndentView struct {
	cell.View
	indents map[int]int // line -> target indentation level
}

func (v testIndentView) IndentationAt(line int) (int, bool) {
	target, ok := v.indents[line]
	return target, ok
}

// errorClipboard is a clipboard.Register that always returns an error on Paste.
type errorClipboard struct{}

func (errorClipboard) Paste(string) (clipboard.Data, error) {
	return clipboard.Data{}, errors.New("clipboard error")
}

func (errorClipboard) Copy(string, clipboard.Data) error {
	return errors.New("clipboard error")
}

func TestPasteAndReindent(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///reindent.go")
	require.NoError(t, err)

	tests := []struct {
		name         string
		content      string           // initial buffer content
		clipText     string           // text to place in clipboard before paste
		clipMeta     any              // metadata for clipboard data
		indents      map[int]int      // per-line target indentation (nil = no indent view)
		cursorAt     term.Coordinates // cursor position before paste
		wantHandled  bool
		wantContent  string
		wantCursor   term.Coordinates
		useErrorClip bool // use errorClipboard instead of normal one
	}{
		{
			name:        "under-indented adds tab",
			content:     "hello",
			clipText:    "x",
			clipMeta:    text.StandardSelection,
			indents:     map[int]int{0: 1},
			cursorAt:    term.Coordinates{},
			wantHandled: true,
			wantContent: "\txhello",
			wantCursor:  term.Coordinates{X: 1},
		},
		{
			name:        "over-indented removes tab",
			content:     "\t\thello",
			clipText:    "x",
			clipMeta:    text.StandardSelection,
			indents:     map[int]int{0: 1},
			cursorAt:    term.Coordinates{X: 2},
			wantHandled: true,
			wantContent: "\txhello",
			wantCursor:  term.Coordinates{X: 3},
		},
		{
			name:        "already indented is unchanged",
			content:     "\thello",
			clipText:    "x",
			clipMeta:    text.StandardSelection,
			indents:     map[int]int{0: 1},
			cursorAt:    term.Coordinates{X: 1},
			wantHandled: true,
			wantContent: "\txhello",
			wantCursor:  term.Coordinates{X: 2},
		},
		{
			name:        "metadata is not SelectMode defaults to StandardSelection",
			content:     "hello\nworld",
			clipText:    "x",
			clipMeta:    "not a SelectMode",
			indents:     nil,
			cursorAt:    term.Coordinates{},
			wantHandled: true,
			wantContent: "xhello\nworld",
			wantCursor:  term.Coordinates{X: 1},
		},
		{
			name:        "startY at last line",
			content:     "a\nb\nc",
			clipText:    "x",
			clipMeta:    text.StandardSelection,
			indents:     map[int]int{2: 1},
			cursorAt:    term.Coordinates{Y: 2},
			wantHandled: true,
			wantContent: "a\nb\n\txc",
			wantCursor:  term.Coordinates{Y: 2, X: 1},
		},
		{
			name:         "clipboard error returns false",
			content:      "hello\nworld",
			useErrorClip: true,
			wantHandled:  false,
			wantContent:  "hello\nworld",
			wantCursor:   term.Coordinates{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := cell.NewBuffer()
			buf.ReadFrom(strings.NewReader(tt.content))

			var clip clipboard.Register
			if tt.useErrorClip {
				clip = errorClipboard{}
			} else {
				clip = clipboard.NewInMemory()
				require.NoError(t, clip.Copy(
					clipboard.DefaultRegisterID,
					clipboard.Data{Text: tt.clipText, Metadata: tt.clipMeta},
				))
			}

			handler := NewHandler(buf, uri,
				text.IndentRuneTab,
				WithClipboard(clip),
				WithTabspaces(1),
			)
			handler.Resize(80, 10)

			if tt.indents != nil {
				buf.WithView(testIndentView{
					View:    buf.View(),
					indents: tt.indents,
				})
			}

			if tt.cursorAt.X != 0 || tt.cursorAt.Y != 0 {
				require.True(t, handler.SetCursorAtScroll(tt.cursorAt))
			}

			ev := term.Event{
				Type: term.EventKey,
				Mod:  term.ModMeta,
				Ch:   'V',
			}
			_, handled := handler.Handle(ev)

			assert.Equal(t, tt.wantHandled, handled, "handled")
			assert.Equal(t, tt.wantContent, buf.String(), "buffer content")
			assert.Equal(t, tt.wantCursor, handler.CursorAtScroll(), "cursor position")
		})
	}
}
