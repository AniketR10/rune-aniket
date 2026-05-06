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

package ide

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/handler"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/browser"
	"unstable.build/go-tui/cell"
	fileexplorercomp "unstable.build/go-tui/component/fileexplorer"
	"unstable.build/go-tui/handler/handlertest"
	"unstable.build/go-tui/text"
	"unstable.build/go-tui/text/vi"
)

func TestFileExplorerHandlerRenderAndInteraction(t *testing.T) {
	tests := []struct {
		name     string
		dirs     map[string][]explorerMockEntry
		width    int
		height   int
		sequence []handlertest.SequenceTestCase
		assert   func(t *testing.T, h *fileExplorerHandler, host *testFileExplorerHost)
	}{
		{
			name: "initial render does not leak hidden row id",
			dirs: map[string][]explorerMockEntry{
				"/project": {
					{name: ".claude", isDir: true},
					{name: ".gitignore", isDir: false},
				},
			},
			width:  20,
			height: 6,
			sequence: []handlertest.SequenceTestCase{{
				InputSequence: "",
				Expected:      " ▐  .claude/        \n o .gitignore       \n                    \n                    \n                    \n                    ",
			}},
		},
		{
			name: "enter toggles directory expansion",
			dirs: map[string][]explorerMockEntry{
				"/project": {
					{name: "src", isDir: true},
				},
				"/project/src": {
					{name: "main.go", isDir: false},
				},
			},
			width:  20,
			height: 6,
			sequence: []handlertest.SequenceTestCase{
				{InputSequence: "", Expected: " ▐  src/            \n                    \n                    \n                    \n                    \n                    "},
				{InputSequence: "<enter>", Expected: " ▐  src/            \n │   o main.go      \n                    \n                    \n                    \n                    "},
				{InputSequence: "<enter>", Expected: " ▐  src/            \n                    \n                    \n                    \n                    \n                    "},
			},
		},
		{
			name: "enter opens file at cursor",
			dirs: map[string][]explorerMockEntry{
				"/project": {
					{name: "file.go", isDir: false},
				},
			},
			width:  20,
			height: 6,
			sequence: []handlertest.SequenceTestCase{{
				InputSequence: "<enter>",
				Expected:      " ▐ file.go          \n                    \n                    \n                    \n                    \n                    ",
			}},
			assert: func(t *testing.T, _ *fileExplorerHandler, host *testFileExplorerHost) {
				require.Len(t, host.opened, 1)
				require.Equal(t, "file:///project/file.go", host.opened[0].String())
			},
		},
		{
			name: "dimensions include longest rendered row",
			dirs: map[string][]explorerMockEntry{
				"/project": {
					{name: "very-long-file-name.go", isDir: false},
				},
			},
			width:  40,
			height: 4,
			sequence: []handlertest.SequenceTestCase{{
				InputSequence: "",
				Expected:      " ▐ very-long-file-name.go               \n                                        \n                                        \n                                        ",
			}},
			assert: func(t *testing.T, h *fileExplorerHandler, _ *testFileExplorerHost) {
				w, hgt := h.Dimensions()
				require.GreaterOrEqual(t, w, len("  very-long-file-name.go"))
				require.Equal(t, 1, hgt)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, host := newTestFileExplorerHandler(t, tt.dirs)
			handlertest.RunHandlerSequence(t, h, tt.width, tt.height, tt.sequence)
			if tt.assert != nil {
				tt.assert(t, h, host)
			}
		})
	}
}

func TestFileExplorerHandlerEnterDelegatesWhileSearching(t *testing.T) {
	h, host := newTestFileExplorerHandler(t, map[string][]explorerMockEntry{
		"/project": {
			{name: "src", isDir: true},
			{name: "findme.txt", isDir: false},
		},
		"/project/src": {
			{name: "main.go", isDir: false},
		},
	})
	h.Resize(20, 6)
	beforeRows := h.ed.CellView().Rows()

	keys, err := term.ParseKeys("/findme<enter>")
	require.NoError(t, err)
	for _, key := range keys {
		h.Handle(term.Event{Ch: key.Ch, Mod: key.Mod, Key: key.Key, Type: term.EventKey})
	}

	require.False(t, h.ed.IsSearchMode())
	require.Empty(t, host.opened)
	require.Equal(t, beforeRows, h.ed.CellView().Rows())
	require.Equal(t, 1, h.ed.CursorAtScroll().Y)
}

func TestFileExplorerHandlerRuntimeLikeDimensionsAndRender(t *testing.T) {
	buf := cell.NewBuffer()
	comp, err := fileexplorercomp.New(buf, &explorerMockFS{dirs: map[string][]explorerMockEntry{
		"/project": {
			{name: ".claude", isDir: true},
			{name: "very-long-file-name.go", isDir: false},
		},
	}}, explorerRootURI(), fileexplorercomp.Config{Icons: text.IconSet{Directory: '', Default: 'o'}})
	require.NoError(t, err)

	ed := vi.Editor(
		vi.WithStatusBarConfig(false, text.StatusBarConfig{}),
		vi.WithAuxiliaryBar(true, text.AuxBarConfig{
			LinesEnabled:     true,
			AbsoluteLines:    false,
			FoldsEnabled:     false,
			GitEnabled:       false,
			ScheduleNextTick: func(fn func()) bool { fn(); return true },
		}),
		vi.WithGitBar(false, text.IconsBarConfig{}),
	)
	host := &testFileExplorerHost{focus: &testExplorerWindow{id: 1}, frame: true}
	uri, err := workspaceapi.ParseURI("memory:///fexplorer-test-1")
	require.NoError(t, err)
	edh, err := ed.Edit(context.Background(), uri, buf, false, false)
	require.NoError(t, err)
	h, err := newFileExplorerHandler(host, comp, buf, edh, uri, host.focus)
	require.NoError(t, err)
	h.SetWindow(&testExplorerWindow{id: 2})
	h.Resize(32, 6)

	handlertest.RunHandlerSequence(t, h, 32, 6, []handlertest.SequenceTestCase{{
		InputSequence: "",
		// text.Editor only installs auxiliary chrome (line numbers,
		// status bar, …) when called via text.Component. The file
		// explorer goes through the bare editor path and so renders
		// the buffer view directly, with no leading "1 " line-number
		// column.
		Expected: " ▐  .claude/                    \n o very-long-file-name.go       \n                                \n                                \n                                \n                                ",
	}})

	// With no bars, the handler's Dimensions reflect just the
	// buffer view. comp.Dimensions returns the same content width
	// the bare editor renders, so the handler width matches the
	// component width and stays large enough to fit the longest row.
	w, _ := h.Dimensions()
	cw, ch := comp.Dimensions()
	require.Equal(t, ch, 2)
	require.GreaterOrEqual(t, w, cw,
		"handler width must cover the buffer content width")
	require.GreaterOrEqual(t, w, len("o very-long-file-name.go"))
	h.syncWidth()
	require.Equal(t, w+2, host.lastWidth)
}

// TestFileExplorerHandlerDimensionsForPrecommitConfig reproduces the
// truncation bug reported against the file explorer where rendering
// `.pre-commit-config.yaml` clipped the trailing `l`. The handler's
// Dimensions() must report a width large enough to cover the icon,
// the trailing space, the full filename, and the aux line-number bar
// — otherwise the parent window sizes itself one cell too narrow and
// the last filename character is dropped.
//
// The root cause was that leaf text handlers used cell.View.Columns()
// for width — a count of cells, not the visual width — which
// under-reports any row containing a wide-glyph (Nerd Font icons,
// CJK). The file icon configured in production is a Nerd Font glyph
// listed by graphemecluster as width 2.
func TestFileExplorerHandlerDimensionsForPrecommitConfig(t *testing.T) {
	const fileName = ".pre-commit-config.yaml"
	// Use a width-2 Nerd Font glyph as the default file icon. The
	// graphemecluster package hardcodes U+EAEE (ok-icon) as a
	// two-cell glyph, which is the case that triggers the truncation
	// bug: cell.View.Columns(y) returns the cell count (=len(cells)),
	// NOT the visual width, so a row ending in a width-2 glyph is
	// reported one cell too narrow.
	const wideFileIcon rune = 0xEAEE // == "" in the graphemecluster width table
	buf := cell.NewBuffer()
	comp, err := fileexplorercomp.New(
		buf, &explorerMockFS{dirs: map[string][]explorerMockEntry{
			"/project": {
				{name: fileName, isDir: false},
			},
		}}, explorerRootURI(),
		fileexplorercomp.Config{
			Icons: text.IconSet{
				Extensions: map[string]rune{},
				Directory:  wideFileIcon,
				Default:    wideFileIcon,
			},
			// Match production: indent width follows editor.tabspaces
			// (default 4), so each depth level uses 4 cells.
			IndentWidth: text.DefaultConfig().Tabspaces,
		},
	)
	require.NoError(t, err)

	ed := vi.Editor(
		vi.WithStatusBarConfig(false, text.StatusBarConfig{}),
		vi.WithAuxiliaryBar(true, text.AuxBarConfig{
			LinesEnabled:     true,
			AbsoluteLines:    false,
			FoldsEnabled:     false,
			GitEnabled:       false,
			ScheduleNextTick: func(fn func()) bool { fn(); return true },
		}),
		// Note: text.Editor only installs auxiliary chrome (line
		// numbers, icons bar, …) when called via text.Component.
		// The file explorer takes the bare-editor path, so the bars
		// configured here have no effect on Dimensions and the
		// handler width covers just the buffer content.
		vi.WithIconsBar(true, text.IconsBarConfig{
			ScheduleNextTick: func(fn func()) bool { fn(); return true },
		}),
	)
	host := &testFileExplorerHost{focus: &testExplorerWindow{id: 1}, frame: true}
	uri, err := workspaceapi.ParseURI("memory:///fexplorer-precommit-test")
	require.NoError(t, err)
	edh, err := ed.Edit(context.Background(), uri, buf, false, false)
	require.NoError(t, err)
	h, err := newFileExplorerHandler(host, comp, buf, edh, uri, host.focus)
	require.NoError(t, err)
	h.SetWindow(&testExplorerWindow{id: 2})
	// Resize to a generous width so the editor renders the full row.
	h.Resize(64, 6)

	// Required visible width for the rendered row is:
	//   icon (2, because Nerd Font) + space (1) + len(fileName)
	wantContent := 2 + 1 + len(fileName)
	wantHandler := wantContent

	w, hgt := h.Dimensions()
	require.Equal(t, 1, hgt, "single visible row")
	require.GreaterOrEqual(t, w, wantHandler,
		"handler.Dimensions().width must cover '%s' "+
			"(icon+space+name=%d); got %d",
		fileName, wantContent, w,
	)

	// Also render into a writer at the reported width and verify the
	// final character of the filename is actually visible (i.e. the
	// reported width matches what the editor renders).
	h.Resize(w, 6)
	writer := term.NewStringWriter(w, 6)
	h.Draw(writer)
	require.NoError(t, writer.Flush())
	rendered := writer.String()
	require.Contains(t, rendered, fileName,
		"rendered output must contain full filename %q (rendered=%q, w=%d)",
		fileName, rendered, w,
	)

	// syncWidth is what the file explorer calls in production after
	// every event to size the hosting window. Verify that the width
	// pushed to the host is sufficient to render the row when the
	// frame surrounds the window.
	h.syncWidth()
	hostW := host.lastWidth
	// frame=true adds 2 cells around the window; the inner cells
	// available for rendering are hostW-2.
	require.GreaterOrEqual(t, hostW-2, wantHandler,
		"host width minus frame must cover the handler's full content; "+
			"hostW=%d wantHandler=%d", hostW, wantHandler,
	)
}

// TestFileExplorerConfirmMessageFormat verifies the prompt message
// groups operations by type, uses markdown bold labels, and shows
// paths relative to the workspace root. RENAME is used when the
// source and destination share a parent directory; MOVE is used
// when they have different parents (even if the basename also
// changes).
func TestFileExplorerConfirmMessageFormat(t *testing.T) {
	h, _ := newTestFileExplorerHandler(t, map[string][]explorerMockEntry{
		"/project": {{name: "a", isDir: true}, {name: "b", isDir: true}},
	})
	rootURI, err := workspaceapi.ParseURI("file:///project")
	require.NoError(t, err)
	uri := func(p string) workspaceapi.URI {
		u, err := workspaceapi.ParseURI("file://" + p)
		require.NoError(t, err)
		return u
	}
	ops := []fileexplorercomp.Operation{
		{Type: fileexplorercomp.OpCreate, NewURI: uri("/project/new.go")},
		{Type: fileexplorercomp.OpMkdir, NewURI: uri("/project/dir")},
		// Same parent, different name → RENAME.
		{
			Type:   fileexplorercomp.OpRename,
			URI:    uri("/project/old.go"),
			NewURI: uri("/project/new2.go"),
		},
		// Different parent, same name → MOVE.
		{
			Type:   fileexplorercomp.OpMove,
			URI:    uri("/project/a/x.go"),
			NewURI: uri("/project/b/x.go"),
		},
		// Different parent AND different name → MOVE (not RENAME).
		{
			Type:   fileexplorercomp.OpMove,
			URI:    uri("/project/a/y.go"),
			NewURI: uri("/project/b/z.go"),
		},
		{Type: fileexplorercomp.OpDelete, URI: uri("/project/gone")},
	}

	msg := h.confirmMessageForTest(rootURI, ops)

	require.Contains(t, msg, "Apply the following changes?")
	require.Contains(t, msg, "- **CREATE**\tnew.go")
	require.Contains(t, msg, "- **MKDIR**\tdir")
	require.Contains(t, msg, "- **RENAME**\told.go -> new2.go")
	require.Contains(t, msg, "- **MOVE**\ta/x.go -> b/x.go")
	require.Contains(t, msg, "- **MOVE**\ta/y.go -> b/z.go")
	require.Contains(t, msg, "- **DELETE**\tgone")
	// The rename op whose parents differ should NOT be shown as RENAME.
	require.NotContains(t, msg, "- **RENAME**\ta/y.go")
	// Every rendered operation must be its own list item.
	for _, line := range strings.Split(msg, "\n")[1:] {
		require.True(t, strings.HasPrefix(line, "- **"),
			"expected markdown list item, got %q", line)
	}
}

func newTestFileExplorerHandler(t *testing.T, dirs map[string][]explorerMockEntry) (*fileExplorerHandler, *testFileExplorerHost) {
	t.Helper()
	buf := cell.NewBuffer()
	comp, err := fileexplorercomp.New(buf, &explorerMockFS{dirs: dirs}, explorerRootURI(), fileexplorercomp.Config{
		Icons: text.IconSet{Directory: '', Default: 'o'},
	})
	require.NoError(t, err)
	ed := vi.Editor(
		vi.WithStatusBarConfig(false, text.StatusBarConfig{}),
		vi.WithAuxiliaryBar(false, text.AuxBarConfig{}),
		vi.WithGitBar(false, text.IconsBarConfig{}),
	)
	host := &testFileExplorerHost{focus: &testExplorerWindow{id: 1}}
	uri, err := workspaceapi.ParseURI("memory:///fexplorer-test-2")
	require.NoError(t, err)
	edh, err := ed.Edit(context.Background(), uri, buf, false, false)
	require.NoError(t, err)
	h, err := newFileExplorerHandler(host, comp, buf, edh, uri, host.focus)
	require.NoError(t, err)
	win := &testExplorerWindow{id: 2}
	h.SetWindow(win)
	h.SetTargetWindow(host.focus)
	h.Resize(40, 10)
	return h, host
}

type testFileExplorerHost struct {
	focus       browser.Window
	opened      []workspaceapi.URI
	lastWidth   int
	errs        []error
	promptCalls int
	frame       bool
}

func (h *testFileExplorerHost) Prompt(message string, options []string, bindings []term.KeyComb, promptHandler handler.PromptHandler) browser.Window {
	h.promptCalls++
	return &testExplorerWindow{id: 99}
}

func (h *testFileExplorerHost) SetWindowWidth(win browser.Window, width int) bool {
	h.lastWidth = width
	return true
}

func (h *testFileExplorerHost) SetFocus(win browser.Window) (browser.Window, error) {
	prev := h.focus
	h.focus = win
	return prev, nil
}

func (h *testFileExplorerHost) Focus() (browser.Window, error) {
	return h.focus, nil
}

func (h *testFileExplorerHost) OpenFile(uri workspaceapi.URI, win browser.Window) error {
	h.opened = append(h.opened, uri)
	return nil
}

func (h *testFileExplorerHost) SetError(err error) {
	h.errs = append(h.errs, err)
}

func (h *testFileExplorerHost) FrameEnabled() bool {
	return h.frame
}

type testExplorerWindow struct {
	id      uint64
	closed  bool
	content browserapi.Handler
}

func (w *testExplorerWindow) SetContent(h browserapi.Handler) error {
	w.content = h
	return nil
}

func (w *testExplorerWindow) Focus() (bool, error) {
	return true, nil
}

func (w *testExplorerWindow) Close() error {
	w.closed = true
	return nil
}

func (w *testExplorerWindow) Content() (browserapi.Handler, error) {
	return w.content, nil
}

func (w *testExplorerWindow) WindowID() uint64 {
	return w.id
}

func (w *testExplorerWindow) Closed() bool {
	return w.closed
}

func (w *testExplorerWindow) IsFloating() bool {
	return false
}

func (w *testExplorerWindow) IsMinimized() (component.Alignment, bool) {
	return 0, false
}

func (w *testExplorerWindow) MinimizeUp(padding int) bool {
	return false
}

func (w *testExplorerWindow) MinimizeDown(padding int) bool {
	return false
}

func (w *testExplorerWindow) MinimizeLeft(padding int) bool {
	return false
}

func (w *testExplorerWindow) MinimizeRight(padding int) bool {
	return false
}

func (w *testExplorerWindow) Unminimize() bool {
	return false
}

func (w *testExplorerWindow) SetFrameAttr(attr term.Attributes) (term.Attributes, bool) {
	return term.Attributes{}, false
}

type explorerMockEntry struct {
	name  string
	isDir bool
}

func (e explorerMockEntry) Name() string      { return e.name }
func (e explorerMockEntry) IsDir() bool       { return e.isDir }
func (e explorerMockEntry) Type() fs.FileMode { return 0 }
func (e explorerMockEntry) Info() (fs.FileInfo, error) {
	return explorerMockInfo{e}, nil
}

type explorerMockInfo struct{ e explorerMockEntry }

func (m explorerMockInfo) Name() string       { return m.e.name }
func (m explorerMockInfo) Size() int64        { return 0 }
func (m explorerMockInfo) Mode() os.FileMode  { return 0 }
func (m explorerMockInfo) ModTime() time.Time { return time.Time{} }
func (m explorerMockInfo) IsDir() bool        { return m.e.isDir }
func (m explorerMockInfo) Sys() any           { return nil }

type explorerMockFS struct {
	dirs map[string][]explorerMockEntry
}

func (m *explorerMockFS) URI(path string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + path)
}

func (m *explorerMockFS) ReadDir(name string) ([]os.DirEntry, error) {
	entries, ok := m.dirs[name]
	if !ok {
		return nil, fmt.Errorf("not found: %s", name)
	}
	ret := make([]os.DirEntry, len(entries))
	for i, e := range entries {
		ret[i] = e
	}
	return ret, nil
}

func (m *explorerMockFS) OpenFile(string, int, os.FileMode) (workspaceapi.File, error) {
	panic("not implemented")
}

func (m *explorerMockFS) Remove(string) error {
	panic("not implemented")
}

func (m *explorerMockFS) Stat(string) (os.FileInfo, error) {
	panic("not implemented")
}

func (m *explorerMockFS) MkdirAll(string, os.FileMode) error {
	panic("not implemented")
}

func explorerRootURI() workspaceapi.URI {
	u, err := workspaceapi.ParseURI("file:///project")
	if err != nil {
		panic(err)
	}
	return u
}
