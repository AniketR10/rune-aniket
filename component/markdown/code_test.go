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

package markdown

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// mockParser implements syntaxapi.Parser for testing. Only Highlight is used.
type mockParser struct {
	highlights []textapi.Location
	err        error
}

func (m *mockParser) Highlight(_ workspaceapi.URI, _ string) (
	iterator.Iterator[textapi.Location], error,
) {
	if m.err != nil {
		return nil, m.err
	}
	return iterator.FromSlice(m.highlights), nil
}

func (m *mockParser) Search(string, []string, ...string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (m *mockParser) ResolveSymbol(context.Context, string, syntaxapi.Progress) (
	iterator.Iterator[syntaxapi.Match], error,
) {
	return iterator.Empty[syntaxapi.Match](), nil
}

func (m *mockParser) ListReferencedSymbols(context.Context) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

func (m *mockParser) SearchNode(syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (m *mockParser) Query(workspaceapi.URI, string, []string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (m *mockParser) QueryNode(workspaceapi.URI, syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

// captureSchedule overrides cfg.ScheduleNextTick to capture the goroutine's
// callback via a channel. The returned function blocks until the callback
// arrives and then runs it on the caller's goroutine, eliminating races.
func captureSchedule(cfg *Config) func() {
	ch := make(chan func(), 1)
	cfg.ScheduleNextTick = func(fn func()) bool {
		ch <- fn
		return true
	}
	return func() {
		fn := <-ch
		fn()
	}
}

func TestCodeBlockHighlights(t *testing.T) {
	green := term.Attributes{Fg: term.ColorGreen}
	blue := term.Attributes{Fg: term.ColorBlue}

	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: term.ColorSilver}
	cfg.Parser = &mockParser{
		highlights: []textapi.Location{
			{
				From: term.Coordinates{X: 0, Y: 0},
				To:   term.Coordinates{X: 4, Y: 0},
				Attr: green,
			},
			{
				From: term.Coordinates{X: 5, Y: 0},
				To:   term.Coordinates{X: 9, Y: 0},
				Attr: blue,
			},
		},
	}
	drain := captureSchedule(&cfg)

	cb := newCodeBlock("go", "func main\n", &cfg)
	drain()

	require.Len(t, cb.cells, 1)
	for x := 0; x < 4; x++ {
		assert.Equal(t, green, cb.cells[0][x].Attributes,
			"cell [0][%d] should be green", x)
	}
	for x := 5; x < 9; x++ {
		assert.Equal(t, blue, cb.cells[0][x].Attributes,
			"cell [0][%d] should be blue", x)
	}
}

func TestCodeBlockHighlightsPreserveBg(t *testing.T) {
	green := term.Attributes{Fg: term.ColorGreen}
	bgColor := term.ColorGray

	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: term.ColorSilver, Bg: bgColor}
	cfg.Parser = &mockParser{
		highlights: []textapi.Location{
			{
				From: term.Coordinates{X: 0, Y: 0},
				To:   term.Coordinates{X: 2, Y: 0},
				Attr: green,
			},
		},
	}
	drain := captureSchedule(&cfg)

	cb := newCodeBlock("go", "hi\n", &cfg)
	drain()

	require.Len(t, cb.cells, 1)
	assert.Equal(t, term.ColorGreen, cb.cells[0][0].Fg)
	assert.Equal(t, bgColor, cb.cells[0][0].Bg)
}

func TestCodeBlockDrawWithHighlights(t *testing.T) {
	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: term.ColorSilver}
	cfg.Parser = &mockParser{
		highlights: []textapi.Location{
			{
				From: term.Coordinates{X: 0, Y: 0},
				To:   term.Coordinates{X: 5, Y: 0},
				Attr: term.Attributes{Fg: term.ColorRed},
			},
		},
	}
	drain := captureSchedule(&cfg)

	cb := newCodeBlock("go", "hello\n", &cfg)
	drain()

	w := term.NewStringWriter(10, 2)
	require.NoError(t, w.Clear(term.Attributes{}))

	cb.w = 10
	cb.Draw(w)
	require.NoError(t, w.Flush())

	assert.Equal(t, "hello     \n          ", w.String())
}

func TestCodeBlockDrawWithoutParser(t *testing.T) {
	cfg := DefaultConfig()
	cb := newCodeBlock("go", "hello\n", &cfg)

	w := term.NewStringWriter(10, 2)
	require.NoError(t, w.Clear(term.Attributes{}))

	cb.w = 10
	cb.Draw(w)
	require.NoError(t, w.Flush())

	assert.Equal(t, "hello     \n          ", w.String())

	for x := range 5 {
		assert.Equal(t, cfg.CodeBlock, cb.cells[0][x].Attributes,
			"cell [0][%d] should have CodeBlock attrs", x)
	}
}

func TestCodeBlockDrawTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		width    int
		height   int
		expected string
	}{
		{
			name:     "short line no wrap",
			code:     "hello\n",
			width:    10,
			height:   2,
			expected: "hello     \n          ",
		},
		{
			name:     "exact width fit",
			code:     "12345\n",
			width:    5,
			height:   2,
			expected: "12345\n     ",
		},
		{
			name:     "single line wraps once",
			code:     "abcdef\n",
			width:    3,
			height:   3,
			expected: "abc\ndef\n   ",
		},
		{
			name:     "multiple lines wrap independently",
			code:     "0123456789\nabcdefghij\n",
			width:    5,
			height:   5,
			expected: "01234\n56789\nabcde\nfghij\n     ",
		},
		{
			name:     "empty source line is preserved",
			code:     "ab\n\ncdef\n",
			width:    2,
			height:   5,
			expected: "ab\n  \ncd\nef\n  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cb := newCodeBlock("", tt.code, &cfg)

			w := term.NewStringWriter(tt.width, tt.height)
			require.NoError(t, w.Clear(term.Attributes{}))

			cb.w = tt.width
			cb.Draw(w)
			require.NoError(t, w.Flush())

			assert.Equal(t, tt.expected, w.String())
		})
	}
}

func TestCodeBlockDrawEmptyLanguage(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Parser = &mockParser{}

	cb := newCodeBlock("", "hello\n", &cfg)

	for x := range 5 {
		assert.Equal(t, cfg.CodeBlock, cb.cells[0][x].Attributes)
	}
}

func TestCodeBlockSpanAtTableDriven(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		width        int
		x            int
		y            int
		expectedText string
		expectedOK   bool
	}{
		{
			name:         "first row returns full line",
			code:         "hello world\n",
			width:        20,
			x:            3,
			y:            0,
			expectedText: "hello world",
			expectedOK:   true,
		},
		{
			name:         "wrapped continuation returns full original line",
			code:         "abcdef\n",
			width:        3,
			x:            1,
			y:            1,
			expectedText: "abcdef",
			expectedOK:   true,
		},
		{
			name:       "x past visible width fails",
			code:       "abcdef\n",
			width:      3,
			x:          3,
			y:          0,
			expectedOK: false,
		},
		{
			name:       "blank area on short wrapped row fails",
			code:       "abcd\n",
			width:      3,
			x:          2,
			y:          1,
			expectedOK: false,
		},
		{
			name:       "y out of range fails",
			code:       "abc\n",
			width:      3,
			x:          0,
			y:          5,
			expectedOK: false,
		},
		{
			name:       "empty source line has no span",
			code:       "\n",
			width:      4,
			x:          0,
			y:          0,
			expectedOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cb := newCodeBlock("", tt.code, &cfg)
			cb.w = tt.width

			text, url, ok := cb.SpanAt(tt.x, tt.y)
			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expectedText, text)
			assert.Empty(t, url)
		})
	}
}

func TestCodeBlockCharAtTableDriven(t *testing.T) {
	tests := []struct {
		name       string
		code       string
		width      int
		x          int
		y          int
		expected   rune
		expectedOK bool
	}{
		{
			name:       "simple first cell",
			code:       "abc\n",
			width:      10,
			x:          0,
			y:          0,
			expected:   'a',
			expectedOK: true,
		},
		{
			name:       "simple last cell",
			code:       "abc\n",
			width:      10,
			x:          2,
			y:          0,
			expected:   'c',
			expectedOK: true,
		},
		{
			name:       "wrapped row start",
			code:       "abcdef\n",
			width:      3,
			x:          0,
			y:          1,
			expected:   'd',
			expectedOK: true,
		},
		{
			name:       "wrapped row end",
			code:       "abcdef\n",
			width:      3,
			x:          2,
			y:          1,
			expected:   'f',
			expectedOK: true,
		},
		{
			name:       "partial wrapped row blank cell",
			code:       "abcd\n",
			width:      3,
			x:          2,
			y:          1,
			expectedOK: false,
		},
		{
			name:       "x past visible width fails",
			code:       "abcdef\n",
			width:      3,
			x:          3,
			y:          0,
			expectedOK: false,
		},
		{
			name:       "negative x fails",
			code:       "abc\n",
			width:      3,
			x:          -1,
			y:          0,
			expectedOK: false,
		},
		{
			name:       "negative y fails",
			code:       "abc\n",
			width:      3,
			x:          0,
			y:          -1,
			expectedOK: false,
		},
		{
			name:       "y out of range fails",
			code:       "abc\n",
			width:      3,
			x:          0,
			y:          4,
			expectedOK: false,
		},
		{
			name:       "empty line has no character",
			code:       "\n",
			width:      4,
			x:          0,
			y:          0,
			expectedOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cb := newCodeBlock("", tt.code, &cfg)
			cb.w = tt.width

			ch, ok := cb.CharAt(tt.x, tt.y)
			assert.Equal(t, tt.expectedOK, ok)
			assert.Equal(t, tt.expected, ch)
		})
	}
}

func TestCodeBlockMaxLineWidth(t *testing.T) {
	cfg := DefaultConfig()
	cb := newCodeBlock("", "a\nlonger\nmid\n", &cfg)

	assert.Equal(t, 6, cb.maxLineWidth())
}

func TestCodeBlockHeightTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		width    int
		expected int
	}{
		{name: "zero width returns zero", code: "abc\n", width: 0, expected: 0},
		{name: "negative width returns zero", code: "abc\n", width: -1, expected: 0},
		{name: "empty code block keeps empty row and spacing", code: "\n", width: 4, expected: 2},
		{name: "short lines no wrap", code: "a\nb\nc\n", width: 20, expected: 4},
		{name: "exact width fit", code: "123\nabc\n", width: 3, expected: 3},
		{name: "single line wraps to two rows", code: "123456\n", width: 3, expected: 3},
		{name: "mixed lines wrap independently", code: "123456\nabc\n", width: 3, expected: 4},
		{name: "empty source line counts as one row", code: "ab\n\ncd\n", width: 2, expected: 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cb := newCodeBlock("", tt.code, &cfg)
			assert.Equal(t, tt.expected, cb.Height(tt.width))
		})
	}
}

func TestCodeBlockWrappedLineCountTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		row      string
		width    int
		expected int
	}{
		{name: "zero width", row: "abc", width: 0, expected: 0},
		{name: "empty row", row: "", width: 4, expected: 1},
		{name: "short row", row: "abc", width: 5, expected: 1},
		{name: "exact fit", row: "abc", width: 3, expected: 1},
		{name: "one overflow chunk", row: "abcd", width: 3, expected: 2},
		{name: "multiple chunks", row: "abcdefg", width: 3, expected: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cb := newCodeBlock("", tt.row+"\n", &cfg)
			var row []term.Cell
			if len(cb.cells) > 0 {
				row = cb.cells[0]
			}
			assert.Equal(t, tt.expected, cb.wrappedLineCount(row, tt.width))
		})
	}
}

func TestCodeBlockDimensionsTableDriven(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		resizeWidth    int
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "short lines use longest source line",
			code:           "hello\nworld\n",
			expectedWidth:  5,
			expectedHeight: 3,
		},
		{
			name:           "wrapped rendering does not change ideal dimensions",
			code:           "123456\nab\n",
			resizeWidth:    3,
			expectedWidth:  6,
			expectedHeight: 3,
		},
		{
			name:           "empty source line contributes height but not width",
			code:           "\n",
			expectedWidth:  0,
			expectedHeight: 2,
		},
		{
			name:           "mixed line lengths keep maximum width",
			code:           "a\nlonger-line\nmid\n",
			resizeWidth:    4,
			expectedWidth:  11,
			expectedHeight: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cb := newCodeBlock("", tt.code, &cfg)
			if tt.resizeWidth != 0 {
				cb.Resize(tt.resizeWidth, cb.Height(tt.resizeWidth))
			}

			w, h := cb.Dimensions()
			assert.Equal(t, tt.expectedWidth, w)
			assert.Equal(t, tt.expectedHeight, h)
		})
	}
}

func TestCodeBlockHighlightsAsync(t *testing.T) {
	green := term.Attributes{Fg: term.ColorGreen}

	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: term.ColorSilver}
	cfg.Parser = &mockParser{
		highlights: []textapi.Location{
			{
				From: term.Coordinates{X: 0, Y: 0},
				To:   term.Coordinates{X: 2, Y: 0},
				Attr: green,
			},
		},
	}
	drain := captureSchedule(&cfg)

	cb := newCodeBlock("go", "hi\n", &cfg)

	// Before the callback runs, cells should have base style.
	require.Len(t, cb.cells, 1)
	assert.Equal(t, cfg.CodeBlock, cb.cells[0][0].Attributes)

	// Run the scheduled callback.
	drain()

	// Now cells should reflect the highlight.
	assert.Equal(t, green, cb.cells[0][0].Attributes)
	assert.Equal(t, green, cb.cells[0][1].Attributes)
}

// blockingIterator yields one Location then blocks on Next until the
// context is canceled. When unblocked it closes the done channel.
type blockingIterator struct {
	first    textapi.Location
	yielded  bool
	done     chan struct{}
	closedCh chan struct{}
}

func (it *blockingIterator) Next(ctx context.Context) (textapi.Location, bool) {
	if !it.yielded {
		it.yielded = true
		return it.first, true
	}
	// Block until ctx is canceled.
	<-ctx.Done()
	close(it.done)
	return textapi.Location{}, false
}

func (it *blockingIterator) Err() error { return nil }

func (it *blockingIterator) Close() error {
	close(it.closedCh)
	return nil
}

// blockingParser returns a blockingIterator from Highlight.
type blockingParser struct {
	iter *blockingIterator
}

func (p *blockingParser) Highlight(_ workspaceapi.URI, _ string) (
	iterator.Iterator[textapi.Location], error,
) {
	return p.iter, nil
}

func (p *blockingParser) Search(string, []string, ...string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (p *blockingParser) ResolveSymbol(context.Context, string, syntaxapi.Progress) (
	iterator.Iterator[syntaxapi.Match], error,
) {
	return iterator.Empty[syntaxapi.Match](), nil
}

func (p *blockingParser) ListReferencedSymbols(context.Context) (iterator.Iterator[string], error) {
	return iterator.Empty[string](), nil
}

func (p *blockingParser) SearchNode(syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (p *blockingParser) Query(workspaceapi.URI, string, []string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func (p *blockingParser) QueryNode(workspaceapi.URI, syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.FromSlice[syntaxapi.Result](nil), nil
}

func TestCodeBlockCloseCancelsIterator(t *testing.T) {
	it := &blockingIterator{
		first: textapi.Location{
			From: term.Coordinates{X: 0, Y: 0},
			To:   term.Coordinates{X: 2, Y: 0},
			Attr: term.Attributes{Fg: term.ColorGreen},
		},
		done:     make(chan struct{}),
		closedCh: make(chan struct{}),
	}

	cfg := DefaultConfig()
	cfg.CodeBlock = term.Attributes{Fg: term.ColorSilver}
	cfg.Parser = &blockingParser{iter: it}

	// Use a ScheduleNextTick that never runs the callback,
	// so we can observe the goroutine's lifecycle independently.
	cfg.ScheduleNextTick = func(func()) bool { return true }

	cb := newCodeBlock("go", "hi\n", &cfg)

	// The goroutine is now blocked inside iter.Next waiting
	// for the second element. Cancel via close.
	cb.close()

	// The done channel is closed once the blocked Next returns,
	// proving that Close short-circuited iterator consumption.
	select {
	case <-it.done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("close did not cancel the iterator within 2s")
	}
}

func TestCodeBlockHeightBasic(t *testing.T) {
	cfg := DefaultConfig()
	cb := newCodeBlock("", "a\nb\nc\n", &cfg)

	assert.Equal(t, 4, cb.Height(20))
	assert.Equal(t, 0, cb.Height(0))
}
