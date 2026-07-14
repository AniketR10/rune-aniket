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

// TestEmacsKeymapAdditions covers the Emacs parity chords added for RUNE-274:
// M-m back-to-indentation, M-^ delete-indentation, M-\ delete-horizontal-space,
// C-o open-line, C-j newline-and-indent, and C-M-f/C-M-b bracket sexp motion.
func TestEmacsKeymapAdditions(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///keymap.go")
	require.NoError(t, err)

	cases := []struct {
		name    string
		content string
		start   term.Coordinates
		events  []term.Event
		want    *string
		at      term.Coordinates
	}{
		{
			name:    "M-m back-to-indentation",
			content: "  ab",
			start:   term.Coordinates{X: 4},
			events:  []term.Event{{Type: term.EventKey, Mod: term.ModAlt, Ch: 'm'}},
			at:      term.Coordinates{X: 2},
		},
		{
			name:    "M-^ delete-indentation joins next line",
			content: "a\nb",
			events:  []term.Event{{Type: term.EventKey, Mod: term.ModAlt, Ch: '^'}},
			want:    new("ab"),
			at:      term.Coordinates{X: 1},
		},
		{
			name:    "M-backslash delete-horizontal-space",
			content: "a   b",
			start:   term.Coordinates{X: 2},
			events:  []term.Event{{Type: term.EventKey, Mod: term.ModAlt, Ch: '\\'}},
			want:    new("ab"),
			at:      term.Coordinates{X: 1},
		},
		{
			name:    "C-o open-line keeps point before newline",
			content: "ab",
			start:   term.Coordinates{X: 1},
			events:  []term.Event{{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'o'}},
			want:    new("a\nb"),
			at:      term.Coordinates{X: 1},
		},
		{
			name:    "C-j newline-and-indent",
			content: "ab",
			start:   term.Coordinates{X: 1},
			events:  []term.Event{{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'j'}},
			want:    new("a\nb"),
			at:      term.Coordinates{Y: 1, X: 0},
		},
		{
			name:    "C-M-f forward-sexp over brackets",
			content: "(x)y",
			events:  []term.Event{{Type: term.EventKey, Mod: term.ModCtrlAlt, Ch: 'f'}},
			at:      term.Coordinates{X: 3},
		},
		{
			name:    "C-M-f forward-sexp scans to next opener",
			content: "a(x)y",
			events:  []term.Event{{Type: term.EventKey, Mod: term.ModCtrlAlt, Ch: 'f'}},
			at:      term.Coordinates{X: 4},
		},
		{
			name:    "C-M-b backward-sexp over brackets",
			content: "(x)y",
			start:   term.Coordinates{X: 4},
			events:  []term.Event{{Type: term.EventKey, Mod: term.ModCtrlAlt, Ch: 'b'}},
			at:      term.Coordinates{X: 0},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			buf := cell.NewBuffer()
			buf.ReadFrom(strings.NewReader(tc.content))
			h := NewHandler(buf, uri, text.IndentRuneTab, 0)
			h.Resize(80, 10)
			if tc.start != (term.Coordinates{}) {
				require.True(t, h.SetCursorAtScroll(tc.start))
			}

			for _, ev := range tc.events {
				_, handled := h.Handle(ev)
				require.True(t, handled, "event %v", ev)
			}

			if tc.want != nil {
				assert.Equal(t, *tc.want, buf.String())
			}
			assert.Equal(t, tc.at, h.CursorAtScroll())
		})
	}
}

// TestEmacsKeyboardQuit verifies C-g (keyboard-quit) clears the active
// selection, the search highlight and any pending C-SPC mark.
func TestEmacsKeyboardQuit(t *testing.T) {
	uri, err := workspaceapi.ParseURI("memory:///quit.go")
	require.NoError(t, err)

	buf := cell.NewBuffer()
	buf.ReadFrom(strings.NewReader("alpha beta"))
	h := NewHandler(buf, uri, text.IndentRuneTab, 0)
	h.Resize(80, 10)

	run := func(keys string) {
		seq, err := term.ParseKeys(keys)
		require.NoError(t, err)
		for _, key := range seq {
			h.Handle(term.Event{
				Type: term.EventKey, Key: key.Key, Mod: key.Mod, Ch: key.Ch,
			})
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

	// Set a mark and grow a selection towards it.
	run("<ctrl-space><shift-right><shift-right>")
	selection, ok := h.Selection()
	require.True(t, ok)
	require.Equal(t, "al", selection)
	require.Len(t, markLocations(), 1)

	// C-g cancels: selection gone, mark cleared.
	run("<ctrl-g>")
	_, ok = h.Selection()
	assert.False(t, ok)
	assert.Empty(t, markLocations())
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

// The tests below form the emacs edge-case battery. They mirror the
// robustness bar set by the vi (modal) handler tests: every functional
// area is exercised at the buffer boundaries (empty buffer, empty lines,
// first/last line and column), with null cells, wide runes, stale
// cursors after external edits, and no-op safety for keys that cannot
// act. They intentionally assert observed handler behavior rather than
// idealized Emacs semantics.

// emacsTestURI is the shared resource URI used by the edge-case tests.
// A .go suffix keeps comment/language wiring consistent with the other
// suites in this file.
func emacsTestURI(t *testing.T) workspaceapi.URI {
	t.Helper()
	uri, err := workspaceapi.ParseURI("memory:///edgecase.go")
	require.NoError(t, err)
	return uri
}

// newEmacsHandler builds an emacs handler over content with a generous
// window so scrolling never interferes with boundary assertions.
func newEmacsHandler(t *testing.T, content string, opts ...Option) (text.Handler, *cell.Buffer) {
	t.Helper()
	buf := cell.NewBuffer()
	_, err := buf.ReadFrom(strings.NewReader(content))
	require.NoError(t, err)
	h := NewHandler(buf, emacsTestURI(t), text.IndentRuneTab, 0, opts...)
	h.Resize(80, 20)
	return h, buf
}

// key builds a plain key event.
func key(k term.Key) term.Event { return term.Event{Type: term.EventKey, Key: k} }

// key2 builds a modified special-key event (e.g. M-DEL is
// key2(term.KeyBackspace, term.ModAlt)).
func key2(k term.Key, mod term.Modifier) term.Event {
	return term.Event{Type: term.EventKey, Key: k, Mod: mod}
}

// ctrl builds a Control-modified character event (e.g. C-a).
func ctrl(ch rune) term.Event {
	return term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: ch}
}

// alt builds a Meta-modified character event (e.g. M-f).
func alt(ch rune) term.Event {
	return term.Event{Type: term.EventKey, Mod: term.ModAlt, Ch: ch}
}

// ctrlAlt builds a Control-Meta-modified character event (e.g. C-M-f).
func ctrlAlt(ch rune) term.Event {
	return term.Event{Type: term.EventKey, Mod: term.ModCtrlAlt, Ch: ch}
}

// runEvent feeds one event and returns whether it was handled.
func runEvent(h text.Handler, ev term.Event) bool {
	_, handled := h.Handle(ev)
	return handled
}

// emacsMarks returns the current emacs mark locations.
func emacsMarks(h text.Handler) []textapi.Location {
	for _, list := range h.LocationLists() {
		if list.ID == emacsMarkLocationListID {
			return list.Locations
		}
	}
	return nil
}

// TestEmacsMotionEdgeCases exercises every cursor-motion binding at the
// boundaries of the buffer: first/last line, first/last column, empty
// buffer and single-character buffer. A motion that cannot move must be
// a safe no-op and leave the cursor where it was.
func TestEmacsMotionEdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		content string
		start   term.Coordinates
		event   term.Event
		at      term.Coordinates
	}{
		// Arrow keys at the edges.
		{"left at buffer start", "abc", term.Coordinates{}, key(term.KeyArrowLeft), term.Coordinates{}},
		{"right at buffer end", "abc", term.Coordinates{X: 3}, key(term.KeyArrowRight), term.Coordinates{X: 3}},
		{"up at first line", "a\nb", term.Coordinates{}, key(term.KeyArrowUp), term.Coordinates{}},
		{"down at last line", "a\nb", term.Coordinates{Y: 1}, key(term.KeyArrowDown), term.Coordinates{Y: 1}},
		{"left at column zero stays on line", "ab\ncd", term.Coordinates{Y: 1}, key(term.KeyArrowLeft), term.Coordinates{Y: 1}},
		{"right at line end stays on line", "ab\ncd", term.Coordinates{Y: 0, X: 2}, key(term.KeyArrowRight), term.Coordinates{Y: 0, X: 2}},

		// Ctrl motion aliases (C-f/C-b/C-n/C-p/C-a/C-e).
		{"C-f at end is no-op", "abc", term.Coordinates{X: 3}, ctrl('f'), term.Coordinates{X: 3}},
		{"C-b at start is no-op", "abc", term.Coordinates{}, ctrl('b'), term.Coordinates{}},
		{"C-p at first line is no-op", "a\nb", term.Coordinates{}, ctrl('p'), term.Coordinates{}},
		{"C-n at last line is no-op", "a\nb", term.Coordinates{Y: 1}, ctrl('n'), term.Coordinates{Y: 1}},
		{"C-a already at line start", "abc", term.Coordinates{}, ctrl('a'), term.Coordinates{}},
		{"C-e already at line end", "abc", term.Coordinates{X: 3}, ctrl('e'), term.Coordinates{X: 3}},
		{"C-a from mid line", "abc", term.Coordinates{X: 2}, ctrl('a'), term.Coordinates{}},
		{"C-e from mid line", "abc", term.Coordinates{X: 1}, ctrl('e'), term.Coordinates{X: 3}},

		// Home/End.
		{"Home at start", "abc", term.Coordinates{}, key(term.KeyHome), term.Coordinates{}},
		{"End at end", "abc", term.Coordinates{X: 3}, key(term.KeyEnd), term.Coordinates{X: 3}},

		// Buffer ends (M-< / M->).
		{"M-< at buffer start", "a\nb\nc", term.Coordinates{}, alt(','), term.Coordinates{}},
		{"M-> from top reaches last line", "a\nb\nc", term.Coordinates{}, alt('.'), term.Coordinates{Y: 2}},
		{"M-< from bottom reaches first line", "a\nb\nc", term.Coordinates{Y: 2}, alt(','), term.Coordinates{}},

		// Word motion (M-f / M-b) at the ends.
		{"M-f at buffer end", "ab", term.Coordinates{X: 2}, alt('f'), term.Coordinates{X: 2}},
		{"M-b at buffer start", "ab", term.Coordinates{}, alt('b'), term.Coordinates{}},
		{"M-f over single word", "word", term.Coordinates{}, alt('f'), term.Coordinates{X: 4}},
		{"M-b from end of single word", "word", term.Coordinates{X: 4}, alt('b'), term.Coordinates{}},

		// back-to-indentation (M-m).
		{"M-m from end of indented line", "\t\tab", term.Coordinates{X: 4}, alt('m'), term.Coordinates{X: 2}},
		{"M-m already at indentation is no-op", "\t\tab", term.Coordinates{X: 2}, alt('m'), term.Coordinates{X: 2}},
		{"M-m on unindented line", "abc", term.Coordinates{X: 2}, alt('m'), term.Coordinates{}},

		// Empty buffer: no motion can move.
		{"left on empty buffer", "", term.Coordinates{}, key(term.KeyArrowLeft), term.Coordinates{}},
		{"right on empty buffer", "", term.Coordinates{}, key(term.KeyArrowRight), term.Coordinates{}},
		{"up on empty buffer", "", term.Coordinates{}, key(term.KeyArrowUp), term.Coordinates{}},
		{"down on empty buffer", "", term.Coordinates{}, key(term.KeyArrowDown), term.Coordinates{}},
		{"C-e on empty buffer", "", term.Coordinates{}, ctrl('e'), term.Coordinates{}},
		{"M-f on empty buffer", "", term.Coordinates{}, alt('f'), term.Coordinates{}},
		{"M-> on empty buffer", "", term.Coordinates{}, alt('.'), term.Coordinates{}},

		// Single character buffer.
		{"right past only char", "a", term.Coordinates{X: 1}, key(term.KeyArrowRight), term.Coordinates{X: 1}},
		{"C-a on single char", "a", term.Coordinates{X: 1}, ctrl('a'), term.Coordinates{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newEmacsHandler(t, tc.content)
			if tc.start != (term.Coordinates{}) {
				require.True(t, h.SetCursorAtScroll(tc.start))
			}
			require.NotPanics(t, func() { h.Handle(tc.event) })
			assert.Equal(t, tc.at, h.CursorAtScroll())
		})
	}
}

// TestEmacsEditingEdgeCases exercises insertion, deletion, backspace and
// the kill commands at buffer boundaries and on an empty buffer. Deletes
// and backspaces that have nothing to remove must be safe no-ops.
func TestEmacsEditingEdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		content string
		start   term.Coordinates
		event   term.Event
		want    string
		at      term.Coordinates
	}{
		// Character insertion.
		{"insert into empty buffer", "", term.Coordinates{}, term.Event{Type: term.EventKey, Ch: 'x'}, "x", term.Coordinates{X: 1}},
		{"insert at end of line", "ab", term.Coordinates{X: 2}, term.Event{Type: term.EventKey, Ch: 'c'}, "abc", term.Coordinates{X: 3}},
		{"insert at start of line", "ab", term.Coordinates{}, term.Event{Type: term.EventKey, Ch: 'z'}, "zab", term.Coordinates{X: 1}},

		// Backspace (also C-h).
		{"backspace at buffer start is no-op", "ab", term.Coordinates{}, key(term.KeyBackspace), "ab", term.Coordinates{}},
		{"backspace on empty buffer is no-op", "", term.Coordinates{}, key(term.KeyBackspace), "", term.Coordinates{}},
		{"backspace mid line", "abc", term.Coordinates{X: 2}, key(term.KeyBackspace), "ac", term.Coordinates{X: 1}},
		{"backspace joins with previous line", "a\nb", term.Coordinates{Y: 1}, key(term.KeyBackspace), "ab", term.Coordinates{X: 1}},
		{"C-h at buffer start is no-op", "ab", term.Coordinates{}, ctrl('h'), "ab", term.Coordinates{}},
		{"C-h mid line", "abc", term.Coordinates{X: 2}, ctrl('h'), "ac", term.Coordinates{X: 1}},

		// Forward delete (Delete / C-d).
		{"delete at buffer end is no-op", "ab", term.Coordinates{X: 2}, key(term.KeyDelete), "ab", term.Coordinates{X: 2}},
		{"delete on empty buffer is no-op", "", term.Coordinates{}, key(term.KeyDelete), "", term.Coordinates{}},
		{"delete mid line", "abc", term.Coordinates{X: 1}, key(term.KeyDelete), "ac", term.Coordinates{X: 1}},
		// Forward delete does not join the following line (asymmetric with
		// backspace, which does join the previous line).
		{"delete at line end is no-op", "a\nb", term.Coordinates{X: 1}, key(term.KeyDelete), "a\nb", term.Coordinates{X: 1}},
		{"C-d at buffer end is no-op", "ab", term.Coordinates{X: 2}, ctrl('d'), "ab", term.Coordinates{X: 2}},
		{"C-d mid line", "abc", term.Coordinates{X: 1}, ctrl('d'), "ac", term.Coordinates{X: 1}},

		// Kill to end of line (C-k).
		{"C-k from start of line", "abc", term.Coordinates{}, ctrl('k'), "", term.Coordinates{}},
		{"C-k from mid line", "abc", term.Coordinates{X: 1}, ctrl('k'), "a", term.Coordinates{X: 1}},
		{"C-k at end of line is no-op", "abc", term.Coordinates{X: 3}, ctrl('k'), "abc", term.Coordinates{X: 3}},
		{"C-k on empty buffer is no-op", "", term.Coordinates{}, ctrl('k'), "", term.Coordinates{}},
		{"C-k on empty first line", "\nb", term.Coordinates{}, ctrl('k'), "\nb", term.Coordinates{}},

		// Word kill forward (M-d).
		{"M-d kill first word", "alpha beta", term.Coordinates{}, alt('d'), " beta", term.Coordinates{}},
		{"M-d at buffer end is no-op", "alpha", term.Coordinates{X: 5}, alt('d'), "alpha", term.Coordinates{X: 5}},
		{"M-d on empty buffer is no-op", "", term.Coordinates{}, alt('d'), "", term.Coordinates{}},

		// Backward word kill (M-DEL).
		{"M-DEL kill previous word", "alpha beta", term.Coordinates{X: 10}, key2(term.KeyBackspace, term.ModAlt), "alpha ", term.Coordinates{X: 6}},
		{"M-DEL at buffer start is no-op", "alpha", term.Coordinates{}, key2(term.KeyBackspace, term.ModAlt), "alpha", term.Coordinates{}},

		// Forward word kill via Delete+Alt (M-Delete).
		{"M-Delete kill next word", "alpha beta", term.Coordinates{}, key2(term.KeyDelete, term.ModAlt), " beta", term.Coordinates{}},

		// delete-indentation / join (M-^).
		{"M-^ joins with next line", "a\nb", term.Coordinates{}, alt('^'), "ab", term.Coordinates{X: 1}},
		{"M-^ on last line is no-op", "a\nb", term.Coordinates{Y: 1}, alt('^'), "a\nb", term.Coordinates{Y: 1, X: 0}},
		{"M-^ on single line is no-op", "abc", term.Coordinates{}, alt('^'), "abc", term.Coordinates{}},

		// delete-horizontal-space (M-\).
		{"M-backslash collapses interior spaces", "a   b", term.Coordinates{X: 2}, alt('\\'), "ab", term.Coordinates{X: 1}},
		{"M-backslash with no spaces is no-op", "ab", term.Coordinates{X: 1}, alt('\\'), "ab", term.Coordinates{X: 1}},
		{"M-backslash trailing spaces", "ab   ", term.Coordinates{X: 5}, alt('\\'), "ab", term.Coordinates{X: 2}},
		{"M-backslash leading spaces", "   ab", term.Coordinates{}, alt('\\'), "ab", term.Coordinates{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, buf := newEmacsHandler(t, tc.content)
			if tc.start != (term.Coordinates{}) {
				require.True(t, h.SetCursorAtScroll(tc.start))
			}
			require.NotPanics(t, func() { h.Handle(tc.event) })
			assert.Equal(t, tc.want, buf.String())
			assert.Equal(t, tc.at, h.CursorAtScroll())
		})
	}
}

// TestEmacsCursorRobustnessAfterExternalEdit drives external buffer
// mutations (as if another writer changed the file underneath the
// cursor) and then continues interacting with the handler. The cursor
// must stay within bounds and subsequent keys must not panic. This is
// the emacs analogue of vi's stale-cursor coverage.
func TestEmacsCursorRobustnessAfterExternalEdit(t *testing.T) {
	edit := func(h text.Handler, from, to term.Coordinates, str string) {
		h.CellEditor().Edit(context.Background(), from, to, str)
	}

	t.Run("external delete of whole buffer snaps cursor to origin", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a\nb\nc\nd\ne")
		for range 4 {
			require.True(t, runEvent(h, key(term.KeyArrowDown)))
		}
		require.Equal(t, term.Coordinates{Y: 4}, h.CursorAtScroll())

		edit(h, term.Coordinates{Y: 0}, term.Coordinates{Y: 5}, "")
		assert.Equal(t, "", buf.String())
		assert.Equal(t, term.Coordinates{}, h.CursorAtScroll())

		// Further motion is a safe no-op; insertion still works.
		assert.False(t, runEvent(h, key(term.KeyArrowRight)))
		assert.Equal(t, term.Coordinates{}, h.CursorAtScroll())
		require.True(t, runEvent(h, term.Event{Type: term.EventKey, Ch: 'z'}))
		assert.Equal(t, "z", buf.String())
		assert.Equal(t, term.Coordinates{X: 1}, h.CursorAtScroll())
	})

	t.Run("external delete of lines below leaves cursor in place", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a\nb\nc\nd\ne")
		require.True(t, runEvent(h, key(term.KeyArrowDown)))
		require.Equal(t, term.Coordinates{Y: 1}, h.CursorAtScroll())

		edit(h, term.Coordinates{Y: 3}, term.Coordinates{Y: 5}, "")
		assert.Equal(t, "a\nb\nc\n", buf.String())
		assert.Equal(t, term.Coordinates{Y: 1}, h.CursorAtScroll())
	})

	t.Run("external delete of lines above shifts cursor up", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "a\nb\nc\nd\ne")
		for range 3 {
			require.True(t, runEvent(h, key(term.KeyArrowDown)))
		}
		require.Equal(t, term.Coordinates{Y: 3}, h.CursorAtScroll())

		edit(h, term.Coordinates{Y: 0}, term.Coordinates{Y: 2}, "")
		assert.Equal(t, term.Coordinates{Y: 1}, h.CursorAtScroll())
	})

	t.Run("external truncation of current line clamps column", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "abcdef\ngh")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 5}))

		edit(h, term.Coordinates{X: 2}, term.Coordinates{X: 6}, "")
		assert.Equal(t, "ab\ngh", buf.String())
		// Column clamps back to the new (shorter) line end.
		assert.Equal(t, term.Coordinates{X: 2}, h.CursorAtScroll())
		// Editing continues without panic.
		require.True(t, runEvent(h, term.Event{Type: term.EventKey, Ch: 'X'}))
		assert.Equal(t, "abX\ngh", buf.String())
	})

	t.Run("external edit then kill line stays valid", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "one\ntwo\nthree")
		require.True(t, runEvent(h, key(term.KeyArrowDown)))
		edit(h, term.Coordinates{Y: 0}, term.Coordinates{Y: 1}, "")
		assert.Equal(t, "two\nthree", buf.String())
		require.NotPanics(t, func() { h.Handle(ctrl('k')) })
	})

	t.Run("external replace growing the buffer keeps cursor logical", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a\nb\nc")
		for range 2 {
			require.True(t, runEvent(h, key(term.KeyArrowDown)))
		}
		require.Equal(t, term.Coordinates{Y: 2}, h.CursorAtScroll())

		edit(h, term.Coordinates{Y: 0}, term.Coordinates{Y: 0}, "x\ny\n")
		assert.Equal(t, "x\ny\na\nb\nc", buf.String())
		assert.Equal(t, term.Coordinates{Y: 4}, h.CursorAtScroll())
	})

	t.Run("external shrink to empty leaves navigation panic-free", func(t *testing.T) {
		// An external edit that empties the buffer moves the cursor row to
		// the only remaining line, but the column recorded before the edit
		// can outlive the content it referred to (external edits do not run
		// through SetCursorAtScroll's clamp). The handler must stay
		// panic-free; End collapses the stale column back to zero.
		h, buf := newEmacsHandler(t, "line1\nline2\nline3")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{Y: 2, X: 4}))
		edit(h, term.Coordinates{Y: 0}, term.Coordinates{Y: 3}, "")
		require.Equal(t, "", buf.String())
		assert.Equal(t, 0, h.CursorAtScroll().Y)

		for _, k := range []term.Key{term.KeyArrowUp, term.KeyArrowDown, term.KeyArrowLeft, term.KeyArrowRight, term.KeyEnd, term.KeyHome} {
			require.NotPanics(t, func() { h.Handle(key(k)) })
			assert.Equal(t, 0, h.CursorAtScroll().Y, "row must stay on the only line for %v", k)
		}
		// End on the empty line resolves the column to the true line end.
		assert.Equal(t, term.Coordinates{}, h.CursorAtScroll())

		// Insertion after all of that lands at the origin and works.
		require.True(t, runEvent(h, term.Event{Type: term.EventKey, Ch: 'q'}))
		assert.Equal(t, "q", buf.String())
	})
}

// TestEmacsNullCellHandling exercises buffers that contain NUL (\x00)
// cells, which are treated as blank characters. Navigation, editing and
// kill commands must handle them without panicking.
func TestEmacsNullCellHandling(t *testing.T) {
	t.Run("arrow-right traverses null cells", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "a\x00b")
		for _, want := range []int{1, 2, 3, 3} {
			require.NotPanics(t, func() { h.Handle(key(term.KeyArrowRight)) })
			assert.Equal(t, want, h.CursorAtScroll().X)
		}
	})

	t.Run("C-e moves to end past null cells", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "a\x00b")
		require.True(t, runEvent(h, ctrl('e')))
		assert.Equal(t, term.Coordinates{X: 3}, h.CursorAtScroll())
	})

	t.Run("kill to end of line removes null cells", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a\x00b\nc")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 1}))
		require.True(t, runEvent(h, ctrl('k')))
		assert.Equal(t, "a\nc", buf.String())
	})

	t.Run("join line preserves interior null cell", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a\x00\nb")
		require.True(t, runEvent(h, alt('^')))
		assert.Equal(t, "a\x00b", buf.String())
	})

	t.Run("delete-horizontal-space removes null cells around point", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a\x00\x00b")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 1}))
		require.NotPanics(t, func() { h.Handle(alt('\\')) })
		assert.Equal(t, "ab", buf.String())
	})

	t.Run("backspace over null cell", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a\x00b")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 2}))
		require.True(t, runEvent(h, key(term.KeyBackspace)))
		assert.Equal(t, "ab", buf.String())
	})

	t.Run("null-only line navigation and edits are panic-free", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "\x00\x00\x00")
		for _, ev := range []term.Event{
			key(term.KeyArrowRight), ctrl('e'), ctrl('a'), alt('f'), alt('b'),
			ctrl('k'), alt('\\'), key(term.KeyBackspace), key(term.KeyDelete),
		} {
			require.NotPanics(t, func() { h.Handle(ev) })
		}
	})
}

// TestEmacsWideRuneHandling exercises buffers containing wide (CJK) and
// multi-byte (emoji) runes. Motion, insertion, deletion and word
// commands must treat them as single logical cells and never panic.
func TestEmacsWideRuneHandling(t *testing.T) {
	t.Run("arrow-right advances one cell per wide rune", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "世界")
		require.True(t, runEvent(h, key(term.KeyArrowRight)))
		assert.Equal(t, term.Coordinates{X: 1}, h.CursorAtScroll())
		require.True(t, runEvent(h, key(term.KeyArrowRight)))
		assert.Equal(t, term.Coordinates{X: 2}, h.CursorAtScroll())
		assert.False(t, runEvent(h, key(term.KeyArrowRight)))
	})

	t.Run("C-e reaches end of wide-rune line", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "世界")
		require.True(t, runEvent(h, ctrl('e')))
		assert.Equal(t, term.Coordinates{X: 2}, h.CursorAtScroll())
	})

	t.Run("M-f crosses a wide word", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "世界 foo")
		require.True(t, runEvent(h, alt('f')))
		assert.Equal(t, term.Coordinates{X: 2}, h.CursorAtScroll())
	})

	t.Run("insert a wide rune into an empty buffer", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "")
		require.True(t, runEvent(h, term.Event{Type: term.EventKey, Ch: '世'}))
		assert.Equal(t, "世", buf.String())
		assert.Equal(t, term.Coordinates{X: 1}, h.CursorAtScroll())
	})

	t.Run("backspace removes a whole wide rune", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a世b")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 2}))
		require.True(t, runEvent(h, key(term.KeyBackspace)))
		assert.Equal(t, "ab", buf.String())
		assert.Equal(t, term.Coordinates{X: 1}, h.CursorAtScroll())
	})

	t.Run("forward delete removes a whole wide rune", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a世b")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 1}))
		require.True(t, runEvent(h, key(term.KeyDelete)))
		assert.Equal(t, "ab", buf.String())
	})

	t.Run("emoji rune insertion and deletion", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "")
		require.True(t, runEvent(h, term.Event{Type: term.EventKey, Ch: '🚀'}))
		assert.Equal(t, "🚀", buf.String())
		require.True(t, runEvent(h, key(term.KeyBackspace)))
		assert.Equal(t, "", buf.String())
	})

	t.Run("kill to end of line over wide runes", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "世界世\nx")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 1}))
		require.True(t, runEvent(h, ctrl('k')))
		assert.Equal(t, "世\nx", buf.String())
	})
}

// TestEmacsSelectionEdgeCases covers shift-selection lifecycle: growing,
// shrinking, replacing with edits, and clearing via a plain motion, Esc
// or C-g. It also confirms selection cannot grow past the buffer end.
func TestEmacsSelectionEdgeCases(t *testing.T) {
	shiftRight := key2(term.KeyArrowRight, term.ModShift)
	shiftLeft := key2(term.KeyArrowLeft, term.ModShift)

	t.Run("shift-right grows then plain motion clears", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "abcde")
		require.True(t, runEvent(h, shiftRight))
		sel, ok := h.Selection()
		require.True(t, ok)
		assert.Equal(t, "a", sel)

		require.True(t, runEvent(h, key(term.KeyArrowRight)))
		_, ok = h.Selection()
		assert.False(t, ok)
		assert.Equal(t, term.Coordinates{X: 2}, h.CursorAtScroll())
	})

	t.Run("shift-left shrinks a growing selection", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "abcde")
		require.True(t, runEvent(h, shiftRight))
		require.True(t, runEvent(h, shiftRight))
		sel, _ := h.Selection()
		require.Equal(t, "ab", sel)

		require.True(t, runEvent(h, shiftLeft))
		sel, ok := h.Selection()
		require.True(t, ok)
		assert.Equal(t, "a", sel)
	})

	t.Run("Esc clears the selection", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "abcde")
		require.True(t, runEvent(h, shiftRight))
		require.True(t, runEvent(h, shiftRight))
		require.True(t, runEvent(h, key(term.KeyEsc)))
		_, ok := h.Selection()
		assert.False(t, ok)
		assert.Equal(t, term.Coordinates{X: 2}, h.CursorAtScroll())
	})

	t.Run("Delete removes the selection", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "abcde")
		require.True(t, runEvent(h, shiftRight))
		require.True(t, runEvent(h, shiftRight))
		require.True(t, runEvent(h, key(term.KeyDelete)))
		assert.Equal(t, "cde", buf.String())
		_, ok := h.Selection()
		assert.False(t, ok)
	})

	t.Run("Backspace removes the selection", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "abcde")
		require.True(t, runEvent(h, shiftRight))
		require.True(t, runEvent(h, shiftRight))
		require.True(t, runEvent(h, key(term.KeyBackspace)))
		assert.Equal(t, "cde", buf.String())
	})

	t.Run("Space over a selection replaces it with a space", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "abcde")
		require.True(t, runEvent(h, shiftRight))
		require.True(t, runEvent(h, shiftRight))
		require.True(t, runEvent(h, key(term.KeySpace)))
		assert.Equal(t, " cde", buf.String())
		assert.Equal(t, term.Coordinates{X: 1}, h.CursorAtScroll())
	})

	t.Run("shift-right at buffer end cannot select", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "ab")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 2}))
		require.NotPanics(t, func() { h.Handle(shiftRight) })
		_, ok := h.Selection()
		assert.False(t, ok)
		assert.Equal(t, term.Coordinates{X: 2}, h.CursorAtScroll())
	})

	t.Run("shift-select on empty buffer is a safe no-op", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "")
		require.NotPanics(t, func() { h.Handle(shiftRight) })
		_, ok := h.Selection()
		assert.False(t, ok)
	})
}

// allEmacsBindings returns one event for every key binding the handler
// recognizes across all modifier layers. It is used to prove that no
// binding panics regardless of the buffer or cursor state.
func allEmacsBindings() []term.Event {
	var evs []term.Event
	// Plain keys.
	for _, k := range []term.Key{
		term.KeyEsc, term.KeyEnd, term.KeyHome, term.KeyPgup, term.KeyPgdn,
		term.KeyArrowLeft, term.KeyArrowRight, term.KeyArrowUp, term.KeyArrowDown,
		term.KeyEnter, term.KeySpace, term.KeyTab, term.KeyBackspace, term.KeyDelete,
	} {
		evs = append(evs, key(k))
	}
	evs = append(evs, term.Event{Type: term.EventKey, Ch: 'a'}) // plain insert

	// Meta (M-) layer.
	for _, ch := range []rune{'f', 'b', 'd', 'w', ',', '.', 'm', '^', '\\', 'u', 'l', 'c', ';', 'y', 'q', '{', '}'} {
		evs = append(evs, alt(ch))
	}
	evs = append(evs,
		key2(term.KeyArrowDown, term.ModAlt), key2(term.KeyArrowUp, term.ModAlt),
		key2(term.KeyArrowLeft, term.ModAlt), key2(term.KeyArrowRight, term.ModAlt),
		key2(term.KeyBackspace, term.ModAlt), key2(term.KeyDelete, term.ModAlt),
	)

	// Meta-Shift line duplication.
	evs = append(evs,
		key2(term.KeyArrowDown, term.ModAltShift), key2(term.KeyArrowUp, term.ModAltShift),
	)

	// Control (C-) layer.
	for _, ch := range []rune{
		'c', 'y', 'l', 'v', 'g', 'o', 'j', 'm', 'M', 'd', 'h', 'a', 'p', 'n',
		'q', 'Q', 'e', 'z', 'Z', 'A', 'f', 'b', 'k', '-', '_', 't', 'K', 'w', '=',
	} {
		evs = append(evs, ctrl(ch))
	}
	evs = append(evs, key2(term.KeyEnter, term.ModCtrl), key2(term.KeySpace, term.ModCtrl))

	// Control-Shift layer.
	evs = append(evs,
		key2(term.KeyEnter, term.ModCtrlShift),
		term.Event{Type: term.EventKey, Mod: term.ModCtrlShift, Ch: 'Q'},
		term.Event{Type: term.EventKey, Mod: term.ModCtrlShift, Ch: 'M'},
		term.Event{Type: term.EventKey, Mod: term.ModCtrlShift, Ch: 'W'},
	)

	// Control-Meta layer.
	for _, ch := range []rune{'h', 'v', 'f', 'b'} {
		evs = append(evs, ctrlAlt(ch))
	}
	evs = append(evs,
		key2(term.KeyArrowUp, term.ModCtrlAlt), key2(term.KeyArrowDown, term.ModCtrlAlt),
	)
	return evs
}

// TestEmacsNoOpSafety drives every binding against pathological buffer
// and cursor states. No binding may panic, and the handler must remain
// interactive (a subsequent insertion still works) afterwards. This is
// the emacs equivalent of vi's out-of-bounds and stale-state coverage.
func TestEmacsNoOpSafety(t *testing.T) {
	fixtures := []struct {
		name    string
		content string
		arrange func(t *testing.T, h text.Handler)
	}{
		{name: "empty buffer", content: ""},
		{name: "single character", content: "a"},
		{name: "single blank line", content: " "},
		{name: "trailing newline", content: "a\n"},
		{name: "null cells only", content: "\x00\x00"},
		{
			name:    "cursor past last column",
			content: "abc",
			arrange: func(t *testing.T, h text.Handler) {
				h.SetCursorAtScroll(term.Coordinates{X: 99})
			},
		},
		{
			name:    "cursor past last line",
			content: "a\nb",
			arrange: func(t *testing.T, h text.Handler) {
				h.SetCursorAtScroll(term.Coordinates{Y: 99})
			},
		},
		{
			name:    "cursor stranded after external shrink",
			content: "a\nb\nc\nd",
			arrange: func(t *testing.T, h text.Handler) {
				require.True(t, h.SetCursorAtScroll(term.Coordinates{Y: 3, X: 1}))
				h.CellEditor().Edit(context.Background(), term.Coordinates{Y: 0}, term.Coordinates{Y: 4}, "")
			},
		},
	}

	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			for _, ev := range allEmacsBindings() {
				h, _ := newEmacsHandler(t, fx.content)
				if fx.arrange != nil {
					fx.arrange(t, h)
				}
				require.NotPanicsf(t, func() { h.Handle(ev) },
					"binding %+v panicked on %q", ev, fx.content)
				// The handler must still accept input afterwards.
				require.NotPanics(t, func() {
					h.Handle(term.Event{Type: term.EventKey, Ch: 'z'})
				})
			}
		})
	}
}

// TestEmacsResizeRobustness verifies that the scroll cursor position is
// preserved across window resizes, including collapsing to a 1x1 window
// and a deferred set through a zero-sized window. Mirrors the modal
// handler's resize-robustness coverage.
func TestEmacsResizeRobustness(t *testing.T) {
	t.Run("cursor survives shrink and grow", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "a\nb\nc\nd\ne")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{Y: 3}))
		require.Equal(t, term.Coordinates{Y: 3}, h.CursorAtScroll())

		for _, dim := range []struct{ w, h int }{{1, 1}, {80, 20}, {2, 2}, {40, 3}} {
			h.Resize(dim.w, dim.h)
			assert.Equal(t, term.Coordinates{Y: 3}, h.CursorAtScroll(),
				"cursor must survive resize to %dx%d", dim.w, dim.h)
		}
	})

	t.Run("set through zero window applies after resize", func(t *testing.T) {
		buf := cell.NewBuffer()
		_, err := buf.ReadFrom(strings.NewReader("a\nb\nc\nd\ne"))
		require.NoError(t, err)
		h := NewHandler(buf, emacsTestURI(t), text.IndentRuneTab, 0)
		h.Resize(0, 0)

		// A zero window defers the set; the pending position is applied
		// once the window has real dimensions.
		assert.False(t, h.SetCursorAtScroll(term.Coordinates{Y: 3}))
		h.Resize(80, 20)
		assert.Equal(t, term.Coordinates{Y: 3}, h.CursorAtScroll())
	})

	t.Run("out-of-range set through zero window is clamped", func(t *testing.T) {
		buf := cell.NewBuffer()
		_, err := buf.ReadFrom(strings.NewReader("a\nb"))
		require.NoError(t, err)
		h := NewHandler(buf, emacsTestURI(t), text.IndentRuneTab, 0)
		h.Resize(0, 0)

		assert.False(t, h.SetCursorAtScroll(term.Coordinates{Y: 99}))
		h.Resize(80, 20)
		assert.Equal(t, term.Coordinates{Y: 1}, h.CursorAtScroll())
	})

	t.Run("editing after collapse and grow is panic-free", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "hello")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{X: 5}))
		h.Resize(1, 1)
		h.Resize(80, 20)
		require.NotPanics(t, func() {
			h.Handle(term.Event{Type: term.EventKey, Ch: '!'})
		})
		assert.Equal(t, "hello!", buf.String())
	})
}

// TestEmacsMarkRobustness covers the C-SPC mark ring: setting marks,
// clearing them with C-g, and their interaction with external buffer
// mutations. Marks that become stale after an external delete must not
// crash a subsequent kill-region.
func TestEmacsMarkRobustness(t *testing.T) {
	t.Run("C-g clears a pending mark", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "abcde")
		require.True(t, runEvent(h, key2(term.KeySpace, term.ModCtrl)))
		require.Len(t, emacsMarks(h), 1)

		require.NotPanics(t, func() { h.Handle(ctrl('g')) })
		assert.Empty(t, emacsMarks(h))
	})

	t.Run("multiple set-mark commands accumulate", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "a\nb\nc")
		require.True(t, runEvent(h, key2(term.KeySpace, term.ModCtrl)))
		require.True(t, runEvent(h, key(term.KeyArrowDown)))
		require.True(t, runEvent(h, key2(term.KeySpace, term.ModCtrl)))
		marks := emacsMarks(h)
		require.Len(t, marks, 2)
		assert.Equal(t, term.Coordinates{}, marks[0].From)
		assert.Equal(t, term.Coordinates{Y: 1}, marks[1].From)
	})

	t.Run("external delete of the whole buffer keeps mark until C-g", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "a\nb\nc")
		require.True(t, runEvent(h, key(term.KeyArrowDown)))
		require.True(t, runEvent(h, key2(term.KeySpace, term.ModCtrl)))
		require.Len(t, emacsMarks(h), 1)

		h.CellEditor().Edit(context.Background(), term.Coordinates{Y: 0}, term.Coordinates{Y: 3}, "")
		require.Equal(t, "", buf.String())
		// The mark location survives the external delete (its stored
		// coordinates are remapped by the edit, not dropped).
		require.Len(t, emacsMarks(h), 1)
		// C-g then clears it, and is safe even though the mark is stale.
		require.NotPanics(t, func() { h.Handle(ctrl('g')) })
		assert.Empty(t, emacsMarks(h))
	})

	t.Run("kill-region after external shrink is panic-free", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "alpha\nbeta\ngamma")
		require.True(t, runEvent(h, key(term.KeyArrowDown)))
		require.True(t, runEvent(h, key2(term.KeySpace, term.ModCtrl)))
		// Shrink the buffer out from under the mark and point.
		h.CellEditor().Edit(context.Background(), term.Coordinates{Y: 0}, term.Coordinates{Y: 2}, "")
		require.NotPanics(t, func() { h.Handle(ctrl('w')) })
		_ = buf
	})

	t.Run("set-mark on empty buffer then C-g", func(t *testing.T) {
		h, _ := newEmacsHandler(t, "")
		require.NotPanics(t, func() { h.Handle(key2(term.KeySpace, term.ModCtrl)) })
		require.NotPanics(t, func() { h.Handle(ctrl('g')) })
		assert.Empty(t, emacsMarks(h))
	})
}

// newEmacsHandlerWithHistory builds a handler backed by a history-aware
// clipboard so kill-ring and yank-pop behavior can be exercised.
func newEmacsHandlerWithHistory(t *testing.T, content string) (text.Handler, *cell.Buffer, clipboard.Register) {
	t.Helper()
	clip := clipboard.NewInMemory()
	reg := registerhistory.NewClipboard(registerset.New(clip))
	buf := cell.NewBuffer()
	_, err := buf.ReadFrom(strings.NewReader(content))
	require.NoError(t, err)
	h := NewHandler(buf, emacsTestURI(t), text.IndentRuneTab, 0, WithClipboard(reg))
	h.Resize(80, 20)
	return h, buf, reg
}

// TestEmacsKillRingEdgeCases exercises the yank / kill-ring commands at
// their edges: yanking with an empty clipboard, yank-pop without a prior
// yank, and copy/kill followed by yank.
func TestEmacsKillRingEdgeCases(t *testing.T) {
	t.Run("C-y with empty clipboard is a safe no-op", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "abc")
		require.NotPanics(t, func() { h.Handle(ctrl('y')) })
		assert.Equal(t, "abc", buf.String())
		assert.Equal(t, term.Coordinates{}, h.CursorAtScroll())
	})

	t.Run("M-y without a prior yank is not handled", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "abc")
		assert.False(t, runEvent(h, alt('y')))
		assert.Equal(t, "abc", buf.String())
	})

	t.Run("C-w deletes the region", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "alpha beta")
		// Mark at start, select "alpha", kill it.
		require.True(t, runEvent(h, key2(term.KeySpace, term.ModCtrl)))
		for range 5 {
			require.True(t, runEvent(h, key2(term.KeyArrowRight, term.ModShift)))
		}
		require.True(t, runEvent(h, ctrl('w')))
		assert.Equal(t, " beta", buf.String())
		assert.Equal(t, term.Coordinates{}, h.CursorAtScroll())
	})

	t.Run("M-w copy then C-y yank duplicates the region", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "alpha beta")
		require.True(t, runEvent(h, key2(term.KeySpace, term.ModCtrl)))
		for range 5 {
			require.True(t, runEvent(h, key2(term.KeyArrowRight, term.ModShift)))
		}
		require.True(t, runEvent(h, alt('w')))
		require.True(t, runEvent(h, key(term.KeyEnd)))
		require.True(t, runEvent(h, ctrl('y')))
		assert.Equal(t, "alpha betaalpha", buf.String())
	})

	t.Run("yank on empty buffer with empty clipboard is safe", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "")
		require.NotPanics(t, func() { h.Handle(ctrl('y')) })
		require.NotPanics(t, func() { h.Handle(alt('y')) })
		assert.Equal(t, "", buf.String())
	})

	t.Run("C-y yank-pop cycle through kill ring", func(t *testing.T) {
		h, buf, reg := newEmacsHandlerWithHistory(t, "z")
		for _, entry := range []string{"a", "b", "c"} {
			require.NoError(t, reg.Copy(clipboard.DefaultRegisterID,
				clipboard.Data{Text: entry, Metadata: text.StandardSelection}))
		}
		require.True(t, runEvent(h, key(term.KeyEnd)))
		require.True(t, runEvent(h, ctrl('y')))
		assert.Equal(t, "zc", buf.String())
		require.True(t, runEvent(h, alt('y')))
		assert.Equal(t, "zb", buf.String())
		require.True(t, runEvent(h, alt('y')))
		assert.Equal(t, "za", buf.String())
	})
}

// TestEmacsSexpMotionEdgeCases exercises the bracket-based forward and
// backward sexp motions (C-M-f / C-M-b) over nested, mismatched,
// unbalanced and multi-line brackets. A motion that cannot find a
// matching bracket must be a no-op.
func TestEmacsSexpMotionEdgeCases(t *testing.T) {
	cases := []struct {
		name    string
		content string
		start   term.Coordinates
		event   term.Event
		handled bool
		at      term.Coordinates
	}{
		{"forward over nested from outer opener", "((a))b", term.Coordinates{}, ctrlAlt('f'), true, term.Coordinates{X: 5}},
		{"forward over nested from inner opener", "((a))b", term.Coordinates{X: 1}, ctrlAlt('f'), true, term.Coordinates{X: 4}},
		{"forward scans to next opener", "a(x)y", term.Coordinates{}, ctrlAlt('f'), true, term.Coordinates{X: 4}},
		{"forward on closer is a no-op", "(a)", term.Coordinates{X: 2}, ctrlAlt('f'), false, term.Coordinates{X: 2}},
		{"forward with mismatched brackets is a no-op", "(a]b", term.Coordinates{}, ctrlAlt('f'), false, term.Coordinates{}},
		{"forward with unclosed bracket is a no-op", "(ab", term.Coordinates{}, ctrlAlt('f'), false, term.Coordinates{}},
		{"forward across a line boundary", "(a\nb)", term.Coordinates{}, ctrlAlt('f'), true, term.Coordinates{Y: 1, X: 2}},
		{"forward on empty buffer is a no-op", "", term.Coordinates{}, ctrlAlt('f'), false, term.Coordinates{}},
		{"backward from after a closer", "(a)", term.Coordinates{X: 3}, ctrlAlt('b'), true, term.Coordinates{}},
		{"backward over nested closers", "((a))", term.Coordinates{X: 5}, ctrlAlt('b'), true, term.Coordinates{}},
		{"backward with unopened bracket is a no-op", "ab)", term.Coordinates{X: 3}, ctrlAlt('b'), false, term.Coordinates{X: 3}},
		{"backward on empty buffer is a no-op", "", term.Coordinates{}, ctrlAlt('b'), false, term.Coordinates{}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, _ := newEmacsHandler(t, tc.content)
			if tc.start != (term.Coordinates{}) {
				require.True(t, h.SetCursorAtScroll(tc.start))
			}
			var handled bool
			require.NotPanics(t, func() { _, handled = h.Handle(tc.event) })
			assert.Equal(t, tc.handled, handled)
			assert.Equal(t, tc.at, h.CursorAtScroll())
		})
	}
}

// TestEmacsLineManipulationEdgeCases covers move-line, duplicate-line and
// jump-to-matching-bracket at buffer boundaries and on degenerate input.
// Move-line depends on the configured clipboard, so its cases use a
// history-aware clipboard (as the editor does in practice) and multi-line
// content with a trailing newline where a swap is expected.
func TestEmacsLineManipulationEdgeCases(t *testing.T) {
	t.Run("move line up at the top keeps the top line first", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "a\nb\nc")
		require.NotPanics(t, func() { h.Handle(key2(term.KeyArrowUp, term.ModAlt)) })
		// The first line cannot move above itself; "a" stays on top.
		assert.True(t, strings.HasPrefix(buf.String(), "a\n"), "got %q", buf.String())
	})

	t.Run("move line down at the bottom is panic-free", func(t *testing.T) {
		h, _, _ := newEmacsHandlerWithHistory(t, "a\nb\nc")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{Y: 2}))
		require.NotPanics(t, func() { h.Handle(key2(term.KeyArrowDown, term.ModAlt)) })
	})

	t.Run("move first line down swaps with the next", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "a\nb\nc")
		require.True(t, runEvent(h, key2(term.KeyArrowDown, term.ModAlt)))
		assert.Equal(t, "b\na\nc", buf.String())
	})

	t.Run("move line up swaps with the previous", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "a\nb\nc")
		require.True(t, h.SetCursorAtScroll(term.Coordinates{Y: 1}))
		require.True(t, runEvent(h, key2(term.KeyArrowUp, term.ModAlt)))
		assert.Equal(t, "b\na\nc", buf.String())
	})

	t.Run("move on empty buffer is panic-free", func(t *testing.T) {
		h, _, _ := newEmacsHandlerWithHistory(t, "")
		require.NotPanics(t, func() { h.Handle(key2(term.KeyArrowUp, term.ModAlt)) })
		require.NotPanics(t, func() { h.Handle(key2(term.KeyArrowDown, term.ModAlt)) })
	})

	t.Run("duplicate only line", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "solo")
		require.True(t, runEvent(h, key2(term.KeyArrowDown, term.ModAltShift)))
		assert.Equal(t, "solo\nsolo", buf.String())
	})

	t.Run("duplicate empty buffer is a no-op", func(t *testing.T) {
		h, buf, _ := newEmacsHandlerWithHistory(t, "")
		require.NotPanics(t, func() { h.Handle(key2(term.KeyArrowDown, term.ModAltShift)) })
		assert.Equal(t, "", buf.String())
	})

	t.Run("jump to matching bracket forward", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "(ab)")
		require.True(t, runEvent(h, ctrl('m')))
		assert.Equal(t, "(ab)", buf.String())
		assert.Equal(t, term.Coordinates{X: 3}, h.CursorAtScroll())
	})

	t.Run("jump to matching bracket with no bracket is a no-op", func(t *testing.T) {
		h, buf := newEmacsHandler(t, "abc")
		require.False(t, runEvent(h, ctrl('m')))
		assert.Equal(t, "abc", buf.String())
		assert.Equal(t, term.Coordinates{}, h.CursorAtScroll())
	})
}
