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

package emacs

import (
	"context"
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

type testMacroRecorder struct {
	started   []string
	stopped   int
	recording bool
}

func (r *testMacroRecorder) Start(registerID string) {
	r.started = append(r.started, registerID)
	r.recording = true
}

func (r *testMacroRecorder) Stop() {
	r.stopped++
	r.recording = false
}

func (r *testMacroRecorder) IsRecording() bool {
	return r.recording
}

type testMacroPlayer struct {
	plays   []testMacroPlay
	err     error
	playing bool
}

type testMacroPlay struct {
	registerID string
	count      int
}

func (p *testMacroPlayer) Play(registerID string, count int) error {
	p.plays = append(p.plays, testMacroPlay{registerID: registerID, count: count})
	return p.err
}

func (p *testMacroPlayer) IsPlaying() bool {
	return p.playing
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

// SelectionExpand lets the emacs handler tests exercise wiring that
// targets a selection service. The default behavior expands an empty
// caret range by one column so the tests do not need a full syntactic
// selection implementation.
func (s testCommentService) SelectionExpand(rng term.Range) (term.Range, bool) {
	if rng.Start == rng.End {
		end := rng.End
		end.X++
		return term.Range{Start: rng.Start, End: end}, true
	}
	return term.Range{}, false
}

func (s testCommentService) SelectionShrink(
	rng term.Range, caret term.Coordinates,
) (term.Range, bool) {
	return term.Range{Start: caret, End: caret}, true
}

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
		vi := NewHandler(buf, uri, text.IndentRuneTab, 0)
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
		vi := NewHandler(buf, uri, text.IndentRuneTab, 0)
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
		vi := NewHandler(buf, uri, text.IndentRuneTab, 0)
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
		vi := NewHandler(buf, uri, text.IndentRuneTab, 0)
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
		vi := NewHandler(buf, uri, text.IndentRuneTab, 0)
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
		vi := NewHandler(buf, uri, text.IndentRuneTab, 0)
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
		vi := NewHandler(buf, uri, text.IndentRuneTab, 0)
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
		vi := NewHandler(buf, uri, text.IndentRuneTab, 0)
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
		handler := NewHandler(buf, uri, text.IndentRuneTab, 0)
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

	h := NewHandler(buf, uri, text.IndentRuneTab, 0)
	h.Resize(80, 10)
	require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 6}))

	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: '='})
	require.True(t, handled)
	selection, ok := h.Selection()
	require.True(t, ok)
	assert.Equal(t, "beta", selection)

	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: '='})
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

func TestEmacsKeyBindingsMacOS(t *testing.T) {
	const snippet = "a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"

	suite := []struct {
		description string
		keycomb     string
		result      *string
		coordinates term.Coordinates
		clipboard   *string
	}{
		// General editing
		// C-y yanks the most recently copied region (C-c copies).
		{"Copy+Paste", "<shift-right><ctrl-c><ctrl-y>", new("aa\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Undo", "<ctrl-shift-k><ctrl-z>", new("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Insert completion/snippet or indent", "<tab>", new("\ta\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Previous snippet field or unindent", "<tab><shift-tab>", new("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},

		// Line manipulation
		{"Insert line after current line", "<ctrl-enter>", new("a\n\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 0}, nil},
		{"Insert line before current line", "<ctrl-shift-enter>", new("\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Move line/selection up", "<down><alt-up>", new("b\na\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Move line/selection down", "<alt-down>", new("b\na\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 0}, nil},
		{"Duplicate line(s)", "<alt-shift-down>", new("a\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 0}, nil},
		{"Delete entire line", "<ctrl-shift-k>", new("b\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Kill to end of line", "<ctrl-k>", new("\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Set mark at cursor position", "<ctrl-space>", nil, term.Coordinates{}, nil},
		{"Copy region from mark to point (M-w)", "<ctrl-space><down><down><alt-w>", nil, term.Coordinates{Y: 2, X: 0}, new("a\nb\n")},
		{"Kill region from mark to point (C-w)", "<ctrl-space><down><down><ctrl-w>", new("c\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Transpose (swap adjacent characters)", "<down><right><ctrl-t>", new("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 1}, nil},

		// Comments - depends on language/syntax (assuming C-style). M-; is
		// comment-dwim.
		{"Toggle line comment (M-;)", "<alt-;>", new("// a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 3}, nil},

		// Text transformation. M-q is fill-paragraph.
		{"Wrap paragraph at ruler (M-q)", "<alt-q>", new("alpha beta\ngamma delta\nepsilon zeta\neta theta\n\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},

		// Expand selection
		{"Expand selection to brackets", "{abc}<left><left><shift-left><ctrl-shift-m>", nil, term.Coordinates{X: 4}, nil},

		// Navigation and movement
		{"Move cursor to beginning of line", "<right><ctrl-a>", nil, term.Coordinates{Y: 0, X: 0}, nil},
		{"Move cursor to end of line", "<ctrl-e>", nil, term.Coordinates{Y: 0, X: 1}, nil},
		{"Move up one line", "<down><ctrl-p>", nil, term.Coordinates{Y: 0, X: 0}, nil},
		{"Move down one line", "<ctrl-n>", nil, term.Coordinates{Y: 1, X: 0}, nil},
		{"Move right one character", "<ctrl-f>", nil, term.Coordinates{Y: 0, X: 1}, nil},
		{"Move left one character", "<right><ctrl-b>", nil, term.Coordinates{Y: 0, X: 0}, nil},
		{"Jump to matching bracket", "{}<left><left><ctrl-m>", nil, term.Coordinates{X: 1}, nil},
		{"Move to start of buffer (M-<)", "<down><down><alt-,>", nil, term.Coordinates{Y: 0, X: 0}, nil},
		{"Move to end of buffer (M->)", "<alt-.>", nil, term.Coordinates{Y: 10, X: 0}, nil},

		// Scrolling
		{"Center current line in view", "<alt-.>z<enter>z<enter>z<enter>z<enter>z<enter>z<enter><up><up><up><up><ctrl-l>",
			nil, term.Coordinates{Y: 12}, nil},
		{"Scroll down one page", "<ctrl-v>", nil, term.Coordinates{Y: 3, X: 0}, nil},
		{"Scroll view up one line", "<alt-.><up><ctrl-alt-up>", nil, term.Coordinates{Y: 9}, nil},
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

		// Macros
		{"Start/stop recording macro", "<ctrl-q>", nil, term.Coordinates{}, nil},
		{"Playback recorded macro", "<ctrl-q><right><ctrl-q><ctrl-shift-q>", nil, term.Coordinates{Y: 0, X: 1}, nil},

		// Build system
		// {"Build (run default build system)", "<meta-b>", nil, term.Coordinates{}},
		// {"Build with... (select build system)", "<shift-meta-b>", nil, term.Coordinates{}},
		// {"Cancel current build", "<ctrl-c>", nil, term.Coordinates{}},

		// Spell check
		//{"Toggle spell check", "<f6>", nil, term.Coordinates{}},
		//{"Jump to next misspelling", "<ctrl-f6>", nil, term.Coordinates{}},
		//{"Jump to previous misspelling", "<ctrl-shift-f6>", nil, term.Coordinates{}},

		// Auto-pairing (context-dependent) - these insert characters
		{"Auto-pair double quotes", "\"", new("\"\"a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Auto-pair single quotes", "'", new("''a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Auto-pair parentheses", "(", new("()a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Auto-pair square brackets", "[", new("[]a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Auto-pair curly braces", "{", new("{}a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 1}, nil},
		{"Delete matching pair (when between paired characters)", "(<backspace>", new("a\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 0, X: 0}, nil},
		{"Add line between paired braces", "{<enter>", new("{\n\n}\na\nb\nc\nd\ne\nf\ng\nh\ni\nj\nk"), term.Coordinates{Y: 1, X: 0}, nil},
	}

	uri, err := workspaceapi.ParseURI("memory:///myfile.go")
	require.NoError(t, err)

	for _, test := range suite {
		t.Run(test.description, func(t *testing.T) {
			seq, err := term.ParseKeys(test.keycomb)
			require.NoError(t, err)

			clip := clipboard.NewInMemory()
			reg := registerhistory.NewClipboard(registerset.New(clip))
			content := snippet
			if test.description == "Wrap paragraph at ruler (M-q)" {
				content = "alpha beta gamma delta epsilon zeta eta theta\n\ni\nj\nk"
			}
			buf := cell.NewBuffer()
			buf.ReadFrom(strings.NewReader(content))
			buf.WithView(testCommentService{
				view:  buf.View(),
				line:  []string{"//"},
				block: []string{"/*", "*/"},
			})
			recorder := new(testMacroRecorder)
			player := new(testMacroPlayer)
			opts := []Option{
				WithClipboard(reg),
				WithMacroRecorder(recorder),
				WithMacroPlayer(player),
				WithRuler(12),
				WithComments(text.CommentConfig{
					"go": {
						Line:  []string{"//"},
						Block: []text.CommentBlock{{Start: "/*", End: "*/"}},
					},
				}),
			}
			if strings.HasPrefix(test.description, "Auto-pair") ||
				test.description == "Delete matching pair (when between paired characters)" ||
				test.description == "Add line between paired braces" {
				opts = append(opts, WithAutoPair(true))
			}
			handler := NewHandler(buf, uri, text.IndentRuneTab, 0, opts...)
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
			switch test.description {
			case "Start/stop recording macro":
				assert.Equal(t, []string{registerset.UnnamedRegisterID}, recorder.started)
				assert.True(t, recorder.recording)
				assert.Zero(t, recorder.stopped)
				assert.Empty(t, player.plays)
			case "Playback recorded macro":
				assert.Equal(t, []string{registerset.UnnamedRegisterID}, recorder.started)
				assert.False(t, recorder.recording)
				assert.Equal(t, 1, recorder.stopped)
				assert.Equal(t, []testMacroPlay{{
					registerID: registerset.UnnamedRegisterID,
					count:      1,
				}}, player.plays)
			}
			assert.Equal(t, test.coordinates, handler.CursorAtScroll())
		})
	}
}

func TestAutoPairOption(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///myfile.go")
	require.NoError(t, err)

	run := func(t *testing.T, keycomb string, opts ...Option) (*cell.Buffer, *emacsHandler) {
		t.Helper()
		seq, err := term.ParseKeys(keycomb)
		require.NoError(t, err)

		buf := cell.NewBuffer()
		buf.ReadFrom(strings.NewReader("a"))
		h := NewHandler(buf, uri, text.IndentRuneTab, 0, opts...)
		h.Resize(10, 3)
		handler := h.(*emacsHandler)
		for _, key := range seq {
			_, handled := handler.Handle(term.Event{
				Type: term.EventKey,
				Key:  key.Key,
				Mod:  key.Mod,
				Ch:   key.Ch,
			})
			require.True(t, handled)
		}
		return buf, handler
	}

	t.Run("disabled by default", func(t *testing.T) {
		buf, h := run(t, "(")

		assert.Equal(t, "(a", buf.String())
		assert.Equal(t, term.Coordinates{X: 1}, h.CursorAtScroll())
	})

	t.Run("opening delimiters insert matching pair when enabled", func(t *testing.T) {
		buf, h := run(t, "(", WithAutoPair(true))

		assert.Equal(t, "()a", buf.String())
		assert.Equal(t, term.Coordinates{X: 1}, h.CursorAtScroll())
	})

	t.Run("backspace removes matching pair when enabled", func(t *testing.T) {
		buf, h := run(t, "(<backspace>", WithAutoPair(true))

		assert.Equal(t, "a", buf.String())
		assert.Equal(t, term.Coordinates{}, h.CursorAtScroll())
	})

	t.Run("enter between braces creates blank line when enabled", func(t *testing.T) {
		buf, h := run(t, "{<enter>", WithAutoPair(true))

		assert.Equal(t, "{\n\n}\na", buf.String())
		assert.Equal(t, term.Coordinates{Y: 1}, h.CursorAtScroll())
	})
}

func TestSetMarkUsesSharedLocationList(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///myfile.go")
	require.NoError(t, err)

	buf := cell.NewBuffer()
	buf.ReadFrom(strings.NewReader("a\nb\nc"))
	h := NewHandler(buf, uri, text.IndentRuneTab, 0)
	h.Resize(10, 3)

	run := func(keys string) {
		seq, err := term.ParseKeys(keys)
		require.NoError(t, err)
		for _, key := range seq {
			_, handled := h.Handle(term.Event{
				Type: term.EventKey,
				Key:  key.Key,
				Mod:  key.Mod,
				Ch:   key.Ch,
			})
			require.True(t, handled)
		}
	}
	markLocations := func() []textapi.Location {
		for _, list := range h.LocationLists() {
			if list.ID == emacsMarkLocationListID {
				return list.Locations
			}
		}
		return nil
	}

	// C-SPC is set-mark-command.
	run("<ctrl-space>")
	require.Len(t, markLocations(), 1)
	assert.Equal(t, term.Coordinates{}, markLocations()[0].From)

	run("<down><down><ctrl-space>")
	require.Len(t, markLocations(), 2)
	assert.Equal(t, term.Coordinates{}, markLocations()[0].From)
	assert.Equal(t, term.Coordinates{Y: 2}, markLocations()[1].From)
}

// TestEmacsMetaWordEditing covers the authentic M- word-motion, word-kill and
// word-case commands that live on the <alt> (Meta) layer.
func TestEmacsMetaWordEditing(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///words.go")
	require.NoError(t, err)

	cases := []struct {
		name    string
		content string
		keys    string
		want    *string
		at      term.Coordinates
	}{
		{"M-f forward-word", "alpha beta", "<alt-f>", nil, term.Coordinates{X: 5}},
		{"M-f twice crosses space", "alpha beta gamma", "<alt-f><alt-f>", nil, term.Coordinates{X: 10}},
		{"M-b backward-word", "alpha beta", "<end><alt-b>", nil, term.Coordinates{X: 6}},
		{"M-d kill-word forward", "alpha beta", "<alt-d>", new(" beta"), term.Coordinates{}},
		{"M-DEL backward-kill-word", "alpha beta", "<end><alt-backspace>", new("alpha "), term.Coordinates{X: 6}},
		{"M-u upcase-word", "alpha beta", "<alt-u>", new("ALPHA beta"), term.Coordinates{X: 5}},
		{"M-l downcase-word", "ALPHA beta", "<alt-l>", new("alpha beta"), term.Coordinates{X: 5}},
		{"M-c capitalize-word", "alpha beta", "<alt-c>", new("Alpha beta"), term.Coordinates{X: 5}},
		{"M-c capitalize lowercases tail", "aLPHA beta", "<alt-c>", new("Alpha beta"), term.Coordinates{X: 5}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := cell.NewBuffer()
			buf.ReadFrom(strings.NewReader(tc.content))
			h := NewHandler(buf, uri, text.IndentRuneTab, 0)
			h.Resize(80, 3)

			seq, err := term.ParseKeys(tc.keys)
			require.NoError(t, err)
			for _, key := range seq {
				_, handled := h.Handle(term.Event{
					Type: term.EventKey, Key: key.Key, Mod: key.Mod, Ch: key.Ch,
				})
				require.True(t, handled, "key %v", key)
			}

			if tc.want != nil {
				assert.Equal(t, *tc.want, buf.String())
			}
			assert.Equal(t, tc.at, h.CursorAtScroll())
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
				{input: "<end><ctrl-y>", wantHandled: true, wantContent: "zc", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-y>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-y>", wantHandled: true, wantContent: "za", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-y>", wantHandled: true, wantContent: "za", wantCursor: coords(term.Coordinates{X: 2})},
			},
		},
		{
			name:        "regular paste restarts history cycle",
			content:     "z",
			history:     []string{"a", "b", "c"},
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<end><ctrl-y>", wantHandled: true, wantContent: "zc"},
				{input: "<alt-y>", wantHandled: true, wantContent: "zb"},
				{input: "<ctrl-y>", wantHandled: true, wantContent: "zbc"},
				{input: "<alt-y>", wantHandled: true, wantContent: "zbb"},
			},
		},
		{
			name:        "typing after paste starts a new history paste",
			content:     "z",
			history:     []string{"a", "b"},
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<end><ctrl-y>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "x", wantHandled: true, wantContent: "zbx", wantCursor: coords(term.Coordinates{X: 3})},
				{input: "<alt-y>", wantHandled: true, wantContent: "zbxb", wantCursor: coords(term.Coordinates{X: 4})},
				{input: "<alt-y>", wantHandled: true, wantContent: "zbxa", wantCursor: coords(term.Coordinates{X: 4})},
			},
		},
		{
			name:        "cursor movement after paste starts a new history paste",
			content:     "z",
			history:     []string{"a", "b"},
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<end><ctrl-y>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<left>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 1})},
				{input: "<alt-y>", wantHandled: true, wantContent: "zbb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-y>", wantHandled: true, wantContent: "zab", wantCursor: coords(term.Coordinates{X: 2})},
			},
		},
		{
			name:        "clipboard without history support consumes in paste context",
			content:     "z",
			history:     []string{"a", "b"},
			wrapHistory: false,
			steps: []pasteHistoryStep{
				{input: "<end><ctrl-y>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-y>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
			},
		},
		{
			name:        "M-y before paste initiates history paste",
			content:     "z",
			history:     []string{"a", "b", "c"},
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<end><alt-y>", wantHandled: true, wantContent: "zc", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-y>", wantHandled: true, wantContent: "zb", wantCursor: coords(term.Coordinates{X: 2})},
				{input: "<alt-y>", wantHandled: true, wantContent: "za", wantCursor: coords(term.Coordinates{X: 2})},
			},
		},
		{
			name:        "M-y without history support before paste is not handled",
			content:     "z",
			history:     []string{"a", "b"},
			wrapHistory: false,
			steps: []pasteHistoryStep{
				{input: "<alt-y>", wantHandled: false, wantContent: "z", wantCursor: coords(term.Coordinates{})},
			},
		},
		{
			name:        "emacs copy operations populate shared history",
			content:     "ab\ncd",
			wrapHistory: true,
			steps: []pasteHistoryStep{
				{input: "<shift-right><ctrl-c><right><shift-right><ctrl-c><end><ctrl-y>", wantHandled: true, wantContent: "abb\ncd"},
				{input: "<alt-y>", wantHandled: true, wantContent: "aba\ncd"},
				{input: "<alt-y>", wantHandled: true, wantContent: "aba\ncd"},
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
			h := NewHandler(buf, uri, text.IndentRuneTab, 0, WithClipboard(reg))
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

// TestEmacsTabAtTargetInsertsFullIndentLevel reproduces RUNE-121 for the
// emacs handler: when the line is already at the syntax target indent, a
// <tab> keypress must add a full indent level rather than a single space, and
// a second <tab> must add another level rather than dedenting.
func TestEmacsTabAtTargetInsertsFullIndentLevel(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///indent.yaml")
	require.NoError(t, err)

	buf := cell.NewBuffer()
	buf.Init()
	_, err = buf.ReadFrom(strings.NewReader("  "))
	require.NoError(t, err)
	buf.WithView(testIndentView{View: buf.View(), indents: map[int]int{0: 1}})

	h := NewHandler(buf, uri, text.IndentRuneSpace, 2, WithTabspaces(2))
	h.Resize(20, 10)
	require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 2, Y: 0}))

	_, handled := h.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})
	require.True(t, handled)
	assert.Equal(t, "    ", buf.String())
	assert.Equal(t, term.Coordinates{X: 4, Y: 0}, h.CursorAtScroll())

	_, handled = h.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})
	require.True(t, handled)
	assert.Equal(t, "      ", buf.String())
	assert.Equal(t, term.Coordinates{X: 6, Y: 0}, h.CursorAtScroll())
}

// TestEmacsShiftTabFallsThroughWhenNoDedent verifies that <shift-tab>
// is reported as unhandled when there is no indentation to remove, so the
// event can fall through to outer command keybindings (e.g. the file
// explorer toggle) instead of being silently swallowed.
func TestEmacsShiftTabFallsThroughWhenNoDedent(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///dedent.txt")
	require.NoError(t, err)

	buf := cell.NewBuffer()
	buf.Init()
	_, err = buf.ReadFrom(strings.NewReader("hello"))
	require.NoError(t, err)

	h := NewHandler(buf, uri, text.IndentRuneSpace, 2, WithTabspaces(2))
	h.Resize(20, 10)

	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModShift, Key: term.KeyTab})
	assert.False(t, handled, "shift-tab with nothing to dedent must fall through")
	assert.Equal(t, "hello", buf.String())
}

// TestEmacsShiftTabDedentsWhenIndented verifies that <shift-tab> still
// dedents an indented line and reports the event as handled.
func TestEmacsShiftTabDedentsWhenIndented(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///dedent.txt")
	require.NoError(t, err)

	buf := cell.NewBuffer()
	buf.Init()
	_, err = buf.ReadFrom(strings.NewReader("  hello"))
	require.NoError(t, err)

	h := NewHandler(buf, uri, text.IndentRuneSpace, 2, WithTabspaces(2))
	h.Resize(20, 10)
	require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 2, Y: 0}))

	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModShift, Key: term.KeyTab})
	assert.True(t, handled, "shift-tab with indentation must be handled")
	assert.Equal(t, "hello", buf.String())
}
