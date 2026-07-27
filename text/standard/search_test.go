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

package standard

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
)

type searchTestWindow uint64

func (w searchTestWindow) WindowID() uint64 { return uint64(w) }

type searchTestWindowManager struct {
	floating     browserapi.Floating
	config       browserapi.FloatingConfig
	closed       int
	err          error
	closeErr     error
	closeContent bool
}

type searchSequenceHarness struct {
	owner         *searchHandler
	width, height int
	closeCalls    int
	closeErr      error
}

func newSearchSequenceHarness(t *testing.T, content string, mode searchMode) *searchSequenceHarness {
	t.Helper()
	buf := cell.NewBuffer()
	buf.WriteString(content)
	root := NewHandler(buf, workspaceapi.URI{}, '\t', 0).(*standardHandler)
	h := &searchSequenceHarness{}
	cfg := defaultConfig().search
	cfg.WindowManager = h
	h.owner = &searchHandler{Handler: root, controller: root, config: cfg}
	if mode == searchModeReplace {
		h.owner.open(mode)
	}
	return h
}

func (h *searchSequenceHarness) Floating(
	f browserapi.Floating, _ browserapi.FloatingConfig,
) (browserapi.Window, error) {
	h.owner.floating = f.(*searchFloating)
	h.owner.floating.Resize(h.width, h.height)
	return searchTestWindow(1), nil
}

func (h *searchSequenceHarness) CloseWindow(browserapi.Window) error {
	h.closeCalls++
	if h.owner.floating != nil {
		return h.owner.floating.Close()
	}
	return nil
}

func (h *searchSequenceHarness) Resize(width, height int) {
	h.width, h.height = width, height
	h.owner.Handler.Resize(width, height)
	if h.owner.floating != nil {
		h.owner.floating.Resize(width, height)
	}
}

func (h *searchSequenceHarness) Draw(w term.Writer) {
	if h.owner.floating != nil {
		h.owner.floating.Draw(w)
		return
	}
	h.owner.Handler.Draw(w)
}

func (h *searchSequenceHarness) Handle(ev term.Event) (bool, bool) {
	if h.owner.floating == nil {
		return h.owner.Handle(ev)
	}
	exit, handled := h.owner.floating.Handle(ev)
	if exit {
		h.closeErr = errors.Join(h.closeErr, h.owner.floating.Close())
	}
	return false, handled
}

func (h *searchSequenceHarness) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	if h.owner.floating != nil {
		return h.owner.floating.Cursor()
	}
	return h.owner.Handler.Cursor()
}

func (h *searchSequenceHarness) Selection() (string, bool) {
	if h.owner.floating != nil {
		return h.owner.floating.Selection()
	}
	return h.owner.Handler.Selection()
}

func (h *searchSequenceHarness) Close() error { return h.owner.Close() }

func golden(width int, lines ...string) string {
	for i, line := range lines {
		lines[i] = line + strings.Repeat(" ", max(0, width-utf8.RuneCountInString(line)))
	}
	return strings.Join(lines, "\n")
}

func TestSearchFloatingKeyboardStateDiagram(t *testing.T) {
	tests := []struct {
		name    string
		content string
		cases   []handlertest.SequenceTestCase
	}{
		{
			name:    "find refinement navigation recovery and reopen",
			content: "one two one",
			cases: []handlertest.SequenceTestCase{
				{InputSequence: "<meta-f>", Expected: golden(48,
					"", " ┌──────────────────────────────┐", " │▐ind                          │  Replace ",
					" └──────────────────────────────┘", "")},
				{InputSequence: "one", Expected: golden(48,
					"", " ┌──────────────────────────────┐", " │one▐                          │  Replace ",
					" └──────────────────────────────┘", "")},
				{InputSequence: "<enter>", Expected: golden(48,
					"", " ┌──────────────────────────────┐", " │one▐                          │  Replace ",
					" └──────────────────────────────┘", "")},
				{InputSequence: "z", Expected: golden(48,
					"", " ┌──────────────────────────────┐", " │onez▐                         │  Replace ",
					" └──────────────────────────────┘", "")},
				{InputSequence: "<backspace>", Expected: golden(48,
					"", " ┌──────────────────────────────┐", " │one▐                          │  Replace ",
					" └──────────────────────────────┘", "")},
				{InputSequence: "<meta-f>", Expected: golden(48,
					"", " ┌──────────────────────────────┐", " │one▐                          │  Replace ",
					" └──────────────────────────────┘", "")},
				{InputSequence: "<esc>", Expected: golden(48,
					"one two one▐", "", "", "", "")},
				{InputSequence: "<ctrl-f>", Expected: golden(48,
					"", " ┌──────────────────────────────┐", " │one▐                          │  Replace ",
					" └──────────────────────────────┘", "")},
			},
		},
		{
			name:    "replace upgrade field cycles spaces and wide input",
			content: "界 one 界 one",
			cases: []handlertest.SequenceTestCase{
				{InputSequence: "<meta-f>界<space>one<meta-r>", Expected: golden(48,
					"", " ┌────────────────────────────┐", " │界  one▐                     │",
					" └────────────────────────────┘", "", " ┌────────────────────────────┐",
					" │Replace with                │  Replace   All ",
					" └────────────────────────────┘", "")},
				{InputSequence: "<tab>x界", Expected: golden(48,
					"", " ┌────────────────────────────┐", " │界  one                      │",
					" └────────────────────────────┘", "", " ┌────────────────────────────┐",
					" │x界 ▐                        │  Replace   All ",
					" └────────────────────────────┘", "")},
				{InputSequence: "<shift-tab>q", Expected: golden(48,
					"", " ┌────────────────────────────┐", " │界  oneq▐                    │",
					" └────────────────────────────┘", "", " ┌────────────────────────────┐",
					" │x界                          │  Replace   All ",
					" └────────────────────────────┘", "")},
				{InputSequence: "<backspace><enter><tab><enter>", Expected: golden(48,
					"", " ┌────────────────────────────┐", " │界  one                      │",
					" └────────────────────────────┘", "", " ┌────────────────────────────┐",
					" │x界 ▐                        │  Replace   All ",
					" └────────────────────────────┘", "")},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newSearchSequenceHarness(t, tt.content, searchModeFind)
			handlertest.RunHandlerSequence(t, h, 48, len(strings.Split(tt.cases[0].Expected, "\n")), tt.cases)
			require.NoError(t, h.closeErr)
		})
	}
}

func TestSearchFloatingDirectReplaceSequence(t *testing.T) {
	h := newSearchSequenceHarness(t, "alpha alpha", searchModeFind)
	handlertest.RunHandlerSequence(t, h, 48, 9, []handlertest.SequenceTestCase{
		{InputSequence: "<meta-r>alpha<tab>beta", Expected: golden(48,
			"", " ┌────────────────────────────┐", " │alpha                       │",
			" └────────────────────────────┘", "", " ┌────────────────────────────┐",
			" │beta▐                       │  Replace   All ",
			" └────────────────────────────┘", "")},
		{InputSequence: "<meta-r>", Expected: golden(48,
			"", " ┌────────────────────────────┐", " │alpha                       │",
			" └────────────────────────────┘", "", " ┌────────────────────────────┐",
			" │beta▐                       │  Replace   All ",
			" └────────────────────────────┘", "")},
	})
}

func TestSearchFloatingUsesStandardInputEditing(t *testing.T) {
	h := newSearchSequenceHarness(t, "one Xone", searchModeFind)
	handlertest.RunHandlerSequence(t, h, 48, 5, []handlertest.SequenceTestCase{
		{InputSequence: "<meta-f>one<meta-left>X", Expected: golden(48,
			"", " ┌──────────────────────────────┐", " │X▐ne                          │  Replace ",
			" └──────────────────────────────┘", "")},
	})
}

type searchBrowserWindowManager struct {
	browser *browser.Component
}

func (m searchBrowserWindowManager) Floating(
	h browserapi.Floating, cfg browserapi.FloatingConfig,
) (browserapi.Window, error) {
	return m.browser.Floating(h.(browser.Floating), cfg), nil
}

func (m searchBrowserWindowManager) CloseWindow(win browserapi.Window) error {
	return win.(browser.Window).Close()
}

func TestSearchFloatingRealBrowserLifecycle(t *testing.T) {
	cfg := browser.DefaultConfig()
	cfg.WindowManagerConfig.NoMaxSize = false
	b := browser.NewComponent(cfg)
	t.Cleanup(func() { require.NoError(t, b.Close()) })

	buf := cell.NewBuffer()
	buf.WriteString("one two one")
	ed := Editor(WithSearchConfig(SearchConfig{WindowManager: searchBrowserWindowManager{browser: b}}))
	h, err := ed.Edit(context.Background(), workspaceapi.URI{}, buf, false, false)
	require.NoError(t, err)
	require.NoError(t, b.Focus().SetContent(h))

	handlertest.RunHandlerSequence(t, b, 56, 14, []handlertest.SequenceTestCase{
		{InputSequence: "<ctrl-f>one", Expected: golden(56,
			"┌──────────────────────────────────────────────────────┐",
			"│                                                      │",
			"├──────────────────────────────────────────────────────┤",
			"│one two one                                           │",
			"│    █●██████████████ Find / Replace ██████████████    │",
			"│    │                                            │    │",
			"│    │ ┌──────────────────────────────┐           │    │",
			"│    │ │one▐                          │  Replace  │    │",
			"│    │ └──────────────────────────────┘           │    │",
			"│    └────────────────────────────────────────────┘    │",
			"│                                                      │",
			"│                                                      │",
			"│                                                      │",
			"└──────────────────────────────────────────────────────┘")},
		{InputSequence: "<esc>", Expected: golden(56,
			"┌──────────────────────────────────────────────────────┐",
			"│                                                      │",
			"├──────────────────────────────────────────────────────┤",
			"│one▐two one                                           │",
			"│                                                      │",
			"│                                                      │",
			"│                                                      │",
			"│                                                      │",
			"│                                                      │",
			"│                                                      │",
			"│                                                      │",
			"│                                                      │",
			"│                                                      │",
			"└──────────────────────────────────────────────────────┘")},
	})
	assert.Zero(t, b.FloatingWindows())
}

func TestSearchFloatingConstrainedKeyboardSequence(t *testing.T) {
	tests := []struct {
		name          string
		width, height int
		input         string
		want          string
	}{
		{name: "narrow wide find wraps and scrolls", width: 8, height: 3,
			input: "<meta-f>界界界界", want: golden(8, " │界│", " │界│", " │▐│")},
		{name: "short replace viewport follows replacement cursor", width: 12, height: 4,
			input: "<meta-r>abcdef<tab>123456", want: golden(12, " │4│", " │5│", " │6│", " │▐│")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newSearchSequenceHarness(t, "abcdef abcdef", searchModeFind)
			handlertest.RunHandlerSequence(t, h, tt.width, tt.height, []handlertest.SequenceTestCase{
				{InputSequence: tt.input, Expected: tt.want},
			})
		})
	}
}

func TestSearchFloatingDimensionsAreIdeal(t *testing.T) {
	tests := []struct {
		name, query, replacement string
		mode                     searchMode
		wantWidth, wantHeight    int
	}{
		{name: "empty find", mode: searchModeFind, wantWidth: 44, wantHeight: 4},
		{name: "empty replace", mode: searchModeReplace, wantWidth: 50, wantHeight: 8},
		{name: "long find", mode: searchModeFind, query: strings.Repeat("q", 80), wantWidth: 95, wantHeight: 4},
		{name: "wide replace", mode: searchModeReplace, query: "界界", replacement: strings.Repeat("界", 30), wantWidth: 81, wantHeight: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner := &searchHandler{config: defaultConfig().search}
			f := newSearchFloating(owner, tt.mode, tt.query)
			if f.replacement != nil {
				for _, ch := range tt.replacement {
					f.replacement.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
			}
			beforeW, beforeH := f.Dimensions()
			assert.Equal(t, tt.wantWidth, beforeW)
			assert.Equal(t, tt.wantHeight, beforeH)
			f.Resize(beforeW, beforeH)
			assert.Equal(t, searchPaddingX, f.layout.query.x)
			assert.Equal(t, beforeH, f.layout.contentHeight)
			if tt.mode == searchModeFind {
				assert.Equal(t, beforeW-searchPaddingX,
					f.layout.upgrade.x+f.layout.upgrade.width)
			} else {
				assert.Equal(t, beforeW-searchPaddingX,
					f.layout.all.x+f.layout.all.width)
			}
			f.Resize(13, 3)
			w := term.NewStringWriter(13, 3)
			f.Draw(w)
			require.NoError(t, w.Flush())
			afterW, afterH := f.Dimensions()
			assert.Equal(t, beforeW, afterW)
			assert.Equal(t, beforeH, afterH)
		})
	}
}

func TestSearchFloatingConstrainedResizeDrawAndSeek(t *testing.T) {
	tests := []struct {
		name          string
		mode          searchMode
		query         string
		replacement   string
		width, height int
		wantHeight    int
		want          string
	}{
		{name: "find wraps wide query", mode: searchModeFind, query: "界界界界", width: 8, height: 3,
			wantHeight: 8, want: golden(8, " │界│", " │▐│", " └─┘")},
		{name: "replace scrolls focused replacement", mode: searchModeReplace, query: "abcdef", replacement: "123456", width: 12, height: 4,
			wantHeight: 20, want: golden(12, " │5│", " │6│", " │▐│", " └─┘")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newSearchFloating(&searchHandler{config: defaultConfig().search}, tt.mode, tt.query)
			if f.replacement != nil {
				for _, ch := range tt.replacement {
					f.replacement.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
				f.focus = searchFocusReplacement
			}
			assert.Equal(t, tt.wantHeight, f.Height(tt.width))
			f.Resize(tt.width, tt.height)
			assert.LessOrEqual(t, f.SeekOffset(), f.MaxSeekOffset())
			assert.GreaterOrEqual(t, f.SeekOffset(), 0)
			assert.Equal(t, tt.want, handlertest.DrawHandler(f, tt.width, tt.height))
			for f.SeekDown() {
			}
			assert.Equal(t, f.MaxSeekOffset(), f.SeekOffset())
			assert.False(t, f.SeekDown())
			for f.SeekUp() {
			}
			assert.Zero(t, f.SeekOffset())
			assert.False(t, f.SeekUp())
		})
	}
}

func TestSearchReplacementSemantics(t *testing.T) {
	tests := []struct {
		name, content, query, replacement, want string
		all                                     bool
	}{
		{name: "no matches", content: "abc", query: "z", replacement: "x", want: "abc"},
		{name: "empty query", content: "abc", replacement: "x", want: "abc"},
		{name: "shorter next", content: "one one", query: "one", replacement: "x", want: "x one"},
		{name: "longer all", content: "a a", query: "a", replacement: "alpha", all: true, want: "alpha alpha"},
		{name: "empty all", content: "a-a-a", query: "a", all: true, want: "--"},
		{name: "contains query all", content: "a a", query: "a", replacement: "aa", all: true, want: "aa aa"},
		{name: "unicode wide", content: "界x界", query: "界", replacement: "語", all: true, want: "語x語"},
		{name: "multiline", content: "one\none", query: "one", replacement: "x\ny", all: true, want: "x\ny\nx\ny"},
		{name: "nul buffer", content: "a\x00a", query: "a", replacement: "b", all: true, want: "b\x00b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := cell.NewBuffer()
			buf.WriteString(tt.content)
			h := NewHandler(buf, workspaceapi.URI{}, '\t', 0).(*standardHandler)
			h.Resize(40, 10)
			h.beginSearch([]rune(tt.query), term.Coordinates{})
			if tt.all {
				h.replaceAll(tt.replacement)
			} else {
				h.replaceNext(tt.replacement)
			}
			assert.Equal(t, tt.want, buf.View().String())
			if tt.want != tt.content {
				undone, _ := buf.Undo()
				require.True(t, undone)
				assert.Equal(t, tt.content, buf.View().String())
			}
		})
	}
}

func TestSearchFloatingMouseStateAndActions(t *testing.T) {
	tests := []struct {
		name   string
		button searchButton
		action func(*searchFloating, searchRect)
		want   string
		mode   searchMode
	}{
		{name: "upgrade", mode: searchModeFind, button: searchButtonUpgrade,
			action: clickSearchButton, want: "replace"},
		{name: "replace next", mode: searchModeReplace, button: searchButtonNext,
			action: clickSearchButton, want: "x one"},
		{name: "replace all", mode: searchModeReplace, button: searchButtonAll,
			action: clickSearchButton, want: "x x"},
		{name: "drag off", mode: searchModeReplace, button: searchButtonNext,
			action: func(f *searchFloating, r searchRect) {
				f.Handle(mouseEvent(term.MouseLeft, r.x, r.y))
				f.Handle(mouseEvent(term.MouseRelease, r.x-1, r.y))
			}, want: "one one"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wm := new(searchTestWindowManager)
			buf := cell.NewBuffer()
			buf.WriteString("one one")
			root := NewHandler(buf, workspaceapi.URI{}, '\t', 0).(*standardHandler)
			cfg := defaultConfig().search
			cfg.WindowManager = wm
			owner := &searchHandler{Handler: root, controller: root, config: cfg}
			owner.open(tt.mode)
			f := owner.floating
			require.NotNil(t, f)
			f.Resize(48, 9)
			f.owner.controller.setSearchQuery("one")
			if f.replacement != nil {
				f.replacement.Handle(term.Event{Type: term.EventKey, Ch: 'x'})
			}
			r := map[searchButton]searchRect{
				searchButtonUpgrade: f.layout.upgrade,
				searchButtonNext:    f.layout.next,
				searchButtonAll:     f.layout.all,
			}[tt.button]
			_, handled := f.Handle(mouseEvent(0, r.x, r.y))
			assert.True(t, handled)
			assert.Equal(t, tt.button, f.hover)
			w := term.NewStringWriter(48, 9)
			f.Draw(w)
			assert.Equal(t, cfg.ButtonHoverAttr, w.Cells()[r.y*48+r.x].Attributes)
			f.Handle(mouseEvent(0, 47, 0))
			assert.Equal(t, searchButtonNone, f.hover)

			tt.action(f, r)
			if tt.want == "replace" {
				assert.Equal(t, searchModeReplace, f.mode)
			} else {
				assert.Equal(t, tt.want, buf.View().String())
			}
		})
	}
}

func TestSearchFloatingMouseInputFocusAndHitboxEdges(t *testing.T) {
	h := newSearchSequenceHarness(t, "one one", searchModeReplace)
	h.Resize(48, 9)
	f := h.owner.floating
	require.NotNil(t, f)

	x, y := f.layout.replacement.x+1, f.layout.replacement.y+1
	_, handled := f.Handle(mouseEvent(term.MouseLeft, x, y))
	assert.True(t, handled)
	f.Handle(mouseEvent(term.MouseRelease, x, y))
	assert.Equal(t, searchFocusReplacement, f.focus)
	f.Handle(term.Event{Type: term.EventKey, Ch: 'x'})
	assert.Equal(t, "x", f.replacement.text())
	assert.Empty(t, f.query.text())

	for _, r := range []searchRect{f.layout.next, f.layout.all} {
		assert.NotEqual(t, searchButtonNone, f.buttonAt(r.x, r.y))
		assert.NotEqual(t, searchButtonNone, f.buttonAt(r.x+r.width-1, r.y))
		assert.Equal(t, searchButtonNone, f.buttonAt(r.x+r.width, r.y))
	}
}

func TestSearchFloatingMouseSelection(t *testing.T) {
	tests := []struct {
		name         string
		mode         searchMode
		query        string
		replacement  string
		selectQuery  bool
		displayStart int
		displayWidth int
		drag         bool
		height       int
		want         string
		frame        string
	}{
		{
			name: "double-click query word", mode: searchModeFind, query: "one two",
			selectQuery: true, displayStart: 4, displayWidth: 3, height: 5, want: "two",
			frame: golden(48,
				"", " ┌──────────────────────────────┐", " │one two▐                      │  Replace ",
				" └──────────────────────────────┘", ""),
		},
		{
			name: "double-click replacement word", mode: searchModeReplace,
			query: "needle", replacement: "red blue", displayStart: 4, displayWidth: 4, height: 9,
			want: "blue",
			frame: golden(48,
				"", " ┌────────────────────────────┐", " │needle                      │",
				" └────────────────────────────┘", "", " ┌────────────────────────────┐",
				" │red blue▐                   │  Replace   All  ",
				" └────────────────────────────┘", ""),
		},
		{
			name: "drag query word", mode: searchModeFind, query: "one two",
			selectQuery: true, displayStart: 4, displayWidth: 3, drag: true, height: 5, want: "two",
			frame: golden(48,
				"", " ┌──────────────────────────────┐", " │one two▐                      │  Replace ",
				" └──────────────────────────────┘", ""),
		},
		{
			name: "wide character before query word", mode: searchModeFind, query: "界 two",
			selectQuery: true, displayStart: 3, displayWidth: 3, height: 5, want: "two",
			frame: golden(48,
				"", " ┌──────────────────────────────┐", " │界  two▐                       │  Replace ",
				" └──────────────────────────────┘", ""),
		},
		{
			name: "double-click wide query word", mode: searchModeFind, query: "one 界",
			selectQuery: true, displayStart: 4, displayWidth: 2, height: 5, want: "界",
			frame: golden(48,
				"", " ┌──────────────────────────────┐", " │one 界 ▐                       │  Replace ",
				" └──────────────────────────────┘", ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newSearchSequenceHarness(t, "one two", tt.mode)
			if tt.mode == searchModeFind {
				h.owner.open(searchModeFind)
			}
			f := h.owner.floating
			require.NotNil(t, f)
			f.Resize(48, tt.height)
			for _, ch := range tt.query {
				f.Handle(term.Event{Type: term.EventKey, Ch: ch})
			}
			input, rect := f.replacement, f.layout.replacement
			if tt.selectQuery {
				input, rect = f.query, f.layout.query
			} else {
				f.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})
				for _, ch := range tt.replacement {
					f.Handle(term.Event{Type: term.EventKey, Ch: ch})
				}
			}
			require.NotNil(t, input)
			x, y := rect.x+1+tt.displayStart, rect.y+1
			f.Handle(mouseEvent(term.MouseLeft, x, y))
			if tt.drag {
				f.Handle(mouseEvent(term.MouseLeft, x+tt.displayWidth, y))
			} else {
				f.Handle(mouseEvent(term.MouseRelease, x, y))
				f.Handle(mouseEvent(term.MouseLeft, x, y))
			}
			releaseX := x
			if tt.drag {
				releaseX += tt.displayWidth
			}
			f.Handle(mouseEvent(term.MouseRelease, releaseX, y))

			selected, ok := f.Selection()
			require.True(t, ok)
			assert.Equal(t, tt.want, selected)

			w := term.NewStringWriter(48, tt.height)
			handlertest.RunHandlerSequenceWriter(t, w, f, 48, tt.height, []handlertest.SequenceTestCase{
				{Expected: tt.frame},
			})
			for cellX := x; cellX < x+tt.displayWidth; cellX++ {
				assert.NotZero(t, w.Cells()[y*48+cellX].Attrs&term.AttrReverse)
			}
		})
	}
}

func clickSearchButton(f *searchFloating, r searchRect) {
	f.Handle(mouseEvent(term.MouseLeft, r.x, r.y))
	f.Handle(mouseEvent(term.MouseRelease, r.x+r.width-1, r.y))
}

func mouseEvent(key term.Key, x, y int) term.Event {
	return term.Event{Type: term.EventMouse, Key: key, MouseX: x, MouseY: y}
}

func TestSearchFloatingConfiguredAttributesAndPureDraw(t *testing.T) {
	cfg := defaultConfig().search
	cfg.Attr = term.Attributes{Bg: term.ColorBlue}
	cfg.InputAttr = term.Attributes{Fg: term.ColorGreen}
	cfg.PlaceholderAttr = term.Attributes{Fg: term.ColorYellow}
	cfg.FrameAttr = term.Attributes{Fg: term.ColorRed}
	cfg.FocusFrameAttr = term.Attributes{Fg: term.ColorPurple}
	cfg.ButtonAttr = term.Attributes{Fg: term.ColorTeal}
	cfg.ButtonHoverAttr = term.Attributes{Bg: term.ColorWhite}
	f := newSearchFloating(&searchHandler{config: cfg}, searchModeFind, "")
	f.Resize(48, 5)
	f.hover = searchButtonUpgrade
	before := *f
	w := term.NewStringWriter(48, 5)
	f.Draw(w)
	assert.Equal(t, before, *f)
	cells := w.Cells()
	assert.Equal(t, cfg.Attr, cells[0].Attributes)
	assert.Equal(t, cfg.FocusFrameAttr, cells[48+1].Attributes)
	assert.Equal(t, cfg.PlaceholderAttr, cells[2*48+2].Attributes)
	assert.Equal(t, cfg.ButtonHoverAttr, cells[f.layout.upgrade.y*48+f.layout.upgrade.x].Attributes)
}

func TestSearchFloatingDefaultPaddingAndAttributes(t *testing.T) {
	cfg := defaultConfig().search
	f := newSearchFloating(&searchHandler{config: cfg}, searchModeFind, "")
	f.Resize(48, 5)

	assert.Equal(t, 1, f.layout.query.x)
	assert.Equal(t, term.Attributes{Fg: term.ColorSilver}, cfg.FocusFrameAttr)
	assert.Equal(t, term.Attributes{Bg: term.ColorGray}, cfg.ButtonAttr)
	assert.Equal(t, term.Attributes{Bg: term.ColorBlue}, cfg.ButtonHoverAttr)

	w := term.NewStringWriter(48, 5)
	f.Draw(w)
	cells := w.Cells()
	assert.Equal(t, cfg.Attr, cells[48].Attributes)
	assert.Equal(t, cfg.FocusFrameAttr, cells[48+1].Attributes)
	assert.Equal(t, cfg.ButtonAttr,
		cells[f.layout.upgrade.y*48+f.layout.upgrade.x].Attributes)

	f.hover = searchButtonUpgrade
	w = term.NewStringWriter(48, 5)
	f.Draw(w)
	cells = w.Cells()
	assert.Equal(t, cfg.ButtonHoverAttr,
		cells[f.layout.upgrade.y*48+f.layout.upgrade.x].Attributes)
}

func TestSearchFloatingReplacementEditRelayout(t *testing.T) {
	h := newSearchSequenceHarness(t, "one", searchModeReplace)
	h.Resize(12, 4)
	f := h.owner.floating
	require.NotNil(t, f)
	f.Handle(term.Event{Type: term.EventKey, Key: term.KeyTab})
	before := f.layout.contentHeight
	for _, ch := range "replacement" {
		f.Handle(term.Event{Type: term.EventKey, Ch: ch})
	}
	assert.Greater(t, f.layout.contentHeight, before)
	pos, _, show := f.Cursor()
	assert.True(t, show)
	assert.GreaterOrEqual(t, pos.Y, 0)
	assert.Less(t, pos.Y, f.layout.height)
}

func TestSearchCustomTriggersAndLifecycle(t *testing.T) {
	wm := &searchTestWindowManager{closeErr: errors.New("close")}
	cfg := defaultConfig().search
	cfg.WindowManager = wm
	cfg.FindKey = term.KeyComb{Mod: term.ModCtrl, Ch: 's'}
	cfg.ReplaceKey = term.KeyComb{Mod: term.ModCtrl, Ch: 'h'}
	buf := cell.NewBuffer()
	buf.WriteString("one")
	root := NewHandler(buf, workspaceapi.URI{}, '\t', 0).(*standardHandler)
	h := &searchHandler{Handler: root, controller: root, config: cfg}

	_, handled := h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'f'})
	assert.True(t, handled)
	assert.Nil(t, h.floating)
	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 's'})
	assert.True(t, handled)
	require.NotNil(t, h.floating)
	h.floating.Handle(term.Event{Type: term.EventKey, Ch: 'o'})
	assert.False(t, root.find.legacyPrompt)

	err := h.finish(true)
	assert.ErrorIs(t, err, wm.closeErr)
	assert.Equal(t, 1, wm.closed)
	assert.Nil(t, h.floating)
	assert.Nil(t, h.window)
	assert.False(t, root.find.active)
	assert.NotEmpty(t, root.searchLocations(), "accepted search highlights must remain")

	_, handled = h.Handle(term.Event{Type: term.EventKey, Mod: term.ModCtrl, Ch: 'h'})
	assert.True(t, handled)
	require.NotNil(t, h.floating)
	assert.Equal(t, searchModeReplace, h.floating.mode)
}

func (m *searchTestWindowManager) Floating(
	f browserapi.Floating, cfg browserapi.FloatingConfig,
) (browserapi.Window, error) {
	m.floating, m.config = f, cfg
	if m.err != nil {
		return nil, m.err
	}
	return searchTestWindow(1), nil
}

func (m *searchTestWindowManager) CloseWindow(browserapi.Window) error {
	m.closed++
	if m.closeContent && m.floating != nil {
		return errors.Join(m.closeErr, m.floating.Close())
	}
	return m.closeErr
}

func TestWithSearchConfigRequiresWindowManager(t *testing.T) {
	assert.PanicsWithValue(t, "standard: SearchConfig.WindowManager must not be nil", func() {
		WithSearchConfig(SearchConfig{})
	})
}

func TestSearchCompatibilityOptions(t *testing.T) {
	match := term.Attributes{Fg: term.ColorYellow}
	status := term.Attributes{Bg: term.ColorPurple}
	cfg := defaultConfig()
	WithResAttr(match)(&cfg)
	WithBarAttr(status)(&cfg)
	assert.Equal(t, match, cfg.search.MatchAttr)
	assert.Equal(t, status, cfg.search.StatusAttr)
}

func TestEditorInstallsConfiguredSearchWrapper(t *testing.T) {
	wm := new(searchTestWindowManager)
	ed := Editor(WithSearchConfig(SearchConfig{WindowManager: wm}))
	buf := cell.NewBuffer()
	buf.WriteString("one two one")
	h, err := ed.Edit(context.Background(), workspaceapi.URI{}, buf, false, false)
	require.NoError(t, err)

	wrapped, ok := h.(*searchHandler)
	require.True(t, ok)
	exit, handled := wrapped.Handle(term.Event{Type: term.EventKey, Mod: term.ModMeta, Ch: 'f'})
	assert.False(t, exit)
	assert.True(t, handled)
	require.NotNil(t, wm.floating)
	assert.Equal(t, componentTopCenter(), wm.config.Alignment)
	assert.Equal(t, term.Coordinates{Y: 2}, wm.config.Offset)
	assert.False(t, wm.config.NoWindowBar)
	assert.Equal(t, "Find / Replace", wm.config.Title)
}

func componentTopCenter() component.Alignment {
	return component.AlignmentTop | component.AlignmentHorizontallyCentered
}

func TestEditorWithoutSearchConfigPreservesStandaloneHandler(t *testing.T) {
	ed := Editor()
	h, err := ed.Edit(context.Background(), workspaceapi.URI{}, cell.NewBuffer(), false, false)
	require.NoError(t, err)
	_, wrapped := h.(*searchHandler)
	assert.False(t, wrapped)
}

func TestSearchFloatingDimensionsAndWideWrapping(t *testing.T) {
	owner := &searchHandler{config: SearchConfig{}}
	f := newSearchFloating(owner, searchModeFind, "界界界界")
	w, h := f.Dimensions()
	assert.Equal(t, 44, w)
	assert.Equal(t, 4, h)

	f.Resize(8, 3)
	assert.Equal(t, w, func() int { got, _ := f.Dimensions(); return got }())
	assert.Greater(t, f.Height(8), 5)
	assert.Greater(t, f.MaxSeekOffset(), 0)
	for f.SeekUp() {
	}
	assert.True(t, f.SeekDown())
	assert.Equal(t, 1, f.SeekOffset())

	f.upgrade()
	w, h = f.Dimensions()
	assert.Equal(t, 50, w)
	assert.Equal(t, 8, h)
}

func TestSearchFloatingButtonPointerAndIdempotentClose(t *testing.T) {
	wm := &searchTestWindowManager{closeContent: true}
	buf := cell.NewBuffer()
	buf.WriteString("one one")
	root := NewHandler(buf, workspaceapi.URI{}, '\t', 0).(*standardHandler)
	root.Resize(20, 4)
	owner := &searchHandler{Handler: root, controller: root,
		config: SearchConfig{WindowManager: wm}}
	owner.open(searchModeFind)
	f := owner.floating
	require.NotNil(t, f)
	f.Resize(48, 5)

	r := f.layout.upgrade
	_, handled := f.Handle(term.Event{Type: term.EventMouse, MouseX: r.x, MouseY: r.y})
	assert.True(t, handled)
	assert.Equal(t, searchButtonUpgrade, f.hover)
	f.Handle(term.Event{Type: term.EventMouse, Key: term.MouseLeft, MouseX: r.x, MouseY: r.y})
	f.Handle(term.Event{Type: term.EventMouse, Key: term.MouseRelease, MouseX: r.x, MouseY: r.y})
	assert.Equal(t, searchModeReplace, f.mode)

	require.NoError(t, f.Close())
	require.NoError(t, f.Close())
	assert.Equal(t, 1, wm.closed)
	assert.Nil(t, owner.floating)
}

func TestSearchOwnerCloseClosesFloatingWindowOnce(t *testing.T) {
	wm := &searchTestWindowManager{closeContent: true}
	root := NewHandler(cell.NewBuffer(), workspaceapi.URI{}, '\t', 0).(*standardHandler)
	owner := &searchHandler{Handler: root, controller: root,
		config: SearchConfig{WindowManager: wm}}
	owner.open(searchModeFind)

	require.NoError(t, owner.Close())
	assert.Equal(t, 1, wm.closed)
	assert.Nil(t, owner.floating)
}

func TestSearchReplacementContainingQueryTerminates(t *testing.T) {
	tests := []struct {
		name string
		all  bool
		want string
	}{
		{name: "next", want: "aa a"},
		{name: "all", all: true, want: "aa aa"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := cell.NewBuffer()
			buf.WriteString("a a")
			h := NewHandler(buf, workspaceapi.URI{}, '\t', 0).(*standardHandler)
			h.Resize(20, 4)
			h.beginSearch([]rune("a"), term.Coordinates{})
			if tt.all {
				h.replaceAll("aa")
			} else {
				h.replaceNext("aa")
			}
			assert.Equal(t, tt.want, buf.View().String())
		})
	}
}

func TestFindOpenFailureFallsBackAndReplaceCleansUp(t *testing.T) {
	for _, mode := range []searchMode{searchModeFind, searchModeReplace} {
		t.Run(map[searchMode]string{searchModeFind: "find", searchModeReplace: "replace"}[mode],
			func(t *testing.T) {
				wm := &searchTestWindowManager{err: errors.New("boom")}
				buf := cell.NewBuffer()
				buf.WriteString("text")
				root := NewHandler(buf, workspaceapi.URI{}, '\t', 0).(*standardHandler)
				root.Resize(20, 4)
				owner := &searchHandler{Handler: root, controller: root,
					config: SearchConfig{WindowManager: wm}}
				owner.open(mode)
				assert.Nil(t, owner.floating)
				assert.Equal(t, mode == searchModeFind, root.find.active)
			})
	}
}
