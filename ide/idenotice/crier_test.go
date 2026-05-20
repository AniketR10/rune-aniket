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

package idenotice

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/syntaxapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/tui"
)

type stubWM struct {
	calls int
	last  browserapi.Floating
}

func (stubWM) Focus() (browserapi.Window, error) { return stubWindow(1), nil }
func (stubWM) Split(browserapi.Orientation, browserapi.Window, browserapi.Handler) (browserapi.Window, error) {
	return nil, nil
}
func (s *stubWM) Floating(h browserapi.Floating, _ browserapi.FloatingConfig) (browserapi.Window, error) {
	s.calls++
	s.last = h
	return stubWindow(uint64(s.calls + 1)), nil
}
func (stubWM) Bar(browserapi.BarConfig, tui.Handler) error { return nil }
func (stubWM) Tab(workspaceapi.URI, rune, string, browserapi.Handler) (browserapi.Handler, error) {
	return nil, nil
}
func (stubWM) SetWindowContent(browserapi.Window, browserapi.Handler) error { return nil }
func (stubWM) CloseWindow(browserapi.Window) error                          { return nil }

type stubWindow uint64

func (s stubWindow) WindowID() uint64 { return uint64(s) }

// nopParser satisfies syntaxapi.Parser; the notice doesn't rely on
// highlighting being applied during construction (it's scheduled via
// the tick callback).
type nopParser struct{}

func (nopParser) Search(string, []string, ...string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}
func (nopParser) SearchNode(syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}
func (nopParser) Query(workspaceapi.URI, string, []string) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}
func (nopParser) QueryNode(workspaceapi.URI, syntaxapi.NodeCaptureName) (iterator.Iterator[syntaxapi.Result], error) {
	return iterator.Empty[syntaxapi.Result](), nil
}
func (nopParser) Highlight(workspaceapi.URI, string) (iterator.Iterator[textapi.Location], error) {
	return iterator.Empty[textapi.Location](), nil
}

func syncTick(fn func()) bool { fn(); return true }

func nopLinkClick(*url.URL) bool { return false }

func mustURI(t *testing.T, s string) workspaceapi.URI {
	t.Helper()
	uri, err := workspaceapi.ParseURI(s)
	require.NoError(t, err)
	return uri
}

type mapFS struct {
	files map[string]string
	err   error
}

func (m mapFS) OpenFile(name string, _ int, _ os.FileMode) (workspaceapi.File, error) {
	if m.err != nil {
		return nil, m.err
	}
	content, ok := m.files[name]
	if !ok {
		return nil, os.ErrNotExist
	}
	return &stubFile{r: strings.NewReader(content)}, nil
}

type stubFile struct {
	r *strings.Reader
}

func (f *stubFile) Read(p []byte) (int, error)              { return f.r.Read(p) }
func (f *stubFile) Close() error                            { return nil }
func (f *stubFile) Fd() uintptr                             { return 0 }
func (f *stubFile) Name() string                            { return "" }
func (f *stubFile) ReadAt(p []byte, off int64) (int, error) { return f.r.ReadAt(p, off) }
func (f *stubFile) Seek(o int64, w int) (int64, error)      { return f.r.Seek(o, w) }
func (f *stubFile) Stat() (os.FileInfo, error)              { return nil, nil }
func (f *stubFile) Sync() error                             { return nil }
func (f *stubFile) Truncate(int64) error                    { return nil }
func (f *stubFile) Write([]byte) (int, error)               { return 0, io.ErrUnexpectedEOF }

func TestCrierShowEmptyConfigIsNoOp(t *testing.T) {
	uri := mustURI(t, "file:///workspace")
	wm := &stubWM{}
	c := New(mapFS{}, wm, nopParser{}, syncTick, nopLinkClick, Config{
		Storage:      storagestub.NewInMemoryService(),
		WorkspaceURI: uri,
	})
	require.NoError(t, c.Show(context.Background()))
	assert.Equal(t, 0, wm.calls)
}

func TestCrierShowLiteralOpensFloating(t *testing.T) {
	uri := mustURI(t, "file:///workspace")
	wm := &stubWM{}
	c := New(mapFS{}, wm, nopParser{}, syncTick, nopLinkClick, Config{
		Literal:      "# Hello",
		Show:         ShowAlways,
		Storage:      storagestub.NewInMemoryService(),
		WorkspaceURI: uri,
	})
	require.NoError(t, c.Show(context.Background()))
	assert.Equal(t, 1, wm.calls)
	require.NotNil(t, wm.last)
}

func TestCrierShowLiteralWinsOverPath(t *testing.T) {
	uri := mustURI(t, "file:///workspace")
	wm := &stubWM{}
	fs := mapFS{files: map[string]string{"notice.md": "FROM-FILE"}}
	c := New(fs, wm, nopParser{}, syncTick, nopLinkClick, Config{
		Path:         "notice.md",
		Literal:      "FROM-LITERAL",
		Show:         ShowAlways,
		Storage:      storagestub.NewInMemoryService(),
		WorkspaceURI: uri,
	})
	require.NoError(t, c.Show(context.Background()))
	assert.Equal(t, 1, wm.calls)
}

func TestCrierShowPathMarkdownVsPlain(t *testing.T) {
	uri := mustURI(t, "file:///workspace")
	cases := []struct {
		name      string
		path      string
		content   string
		wantShown bool
	}{
		{name: "markdown_extension", path: "notice.md", content: "# Hi", wantShown: true},
		{name: "plain_text", path: "notice.txt", content: "Hello world", wantShown: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wm := &stubWM{}
			fs := mapFS{files: map[string]string{tc.path: tc.content}}
			c := New(fs, wm, nopParser{}, syncTick, nopLinkClick, Config{
				Path:         tc.path,
				Show:         ShowAlways,
				Storage:      storagestub.NewInMemoryService(),
				WorkspaceURI: uri,
			})
			require.NoError(t, c.Show(context.Background()))
			if tc.wantShown {
				assert.Equal(t, 1, wm.calls)
			} else {
				assert.Equal(t, 0, wm.calls)
			}
		})
	}
}

func TestCrierShowOnceSkipsSecondCall(t *testing.T) {
	uri := mustURI(t, "file:///workspace")
	wm := &stubWM{}
	c := New(mapFS{}, wm, nopParser{}, syncTick, nopLinkClick, Config{
		Literal:      "# Once",
		Show:         ShowOnce,
		Storage:      storagestub.NewInMemoryService(),
		WorkspaceURI: uri,
	})
	require.NoError(t, c.Show(context.Background()))
	require.NoError(t, c.Show(context.Background()))
	assert.Equal(t, 1, wm.calls)
}

func TestCrierShowAlwaysRepeats(t *testing.T) {
	uri := mustURI(t, "file:///workspace")
	wm := &stubWM{}
	c := New(mapFS{}, wm, nopParser{}, syncTick, nopLinkClick, Config{
		Literal:      "# Always",
		Show:         ShowAlways,
		Storage:      storagestub.NewInMemoryService(),
		WorkspaceURI: uri,
	})
	require.NoError(t, c.Show(context.Background()))
	require.NoError(t, c.Show(context.Background()))
	assert.Equal(t, 2, wm.calls)
}

func TestCrierShowOnceFingerprintChangeRetriggers(t *testing.T) {
	uri := mustURI(t, "file:///workspace")
	wm := &stubWM{}
	storage := storagestub.NewInMemoryService()
	first := New(mapFS{}, wm, nopParser{}, syncTick, nopLinkClick, Config{
		Literal:      "# v1",
		Show:         ShowOnce,
		Storage:      storage,
		WorkspaceURI: uri,
	})
	require.NoError(t, first.Show(context.Background()))
	assert.Equal(t, 1, wm.calls)

	second := New(mapFS{}, wm, nopParser{}, syncTick, nopLinkClick, Config{
		Literal:      "# v2 changed",
		Show:         ShowOnce,
		Storage:      storage,
		WorkspaceURI: uri,
	})
	require.NoError(t, second.Show(context.Background()))
	assert.Equal(t, 2, wm.calls)

	require.NoError(t, second.Show(context.Background()))
	assert.Equal(t, 2, wm.calls)
}

func TestCrierShowOnceDefaultsForEmptyShow(t *testing.T) {
	uri := mustURI(t, "file:///workspace")
	wm := &stubWM{}
	c := New(mapFS{}, wm, nopParser{}, syncTick, nopLinkClick, Config{
		Literal:      "# default",
		Storage:      storagestub.NewInMemoryService(),
		WorkspaceURI: uri,
	})
	require.NoError(t, c.Show(context.Background()))
	require.NoError(t, c.Show(context.Background()))
	assert.Equal(t, 1, wm.calls)
}

func TestCrierShowPathReadErrorPropagates(t *testing.T) {
	uri := mustURI(t, "file:///workspace")
	wm := &stubWM{}
	c := New(mapFS{err: errors.New("boom")}, wm, nopParser{}, syncTick, nopLinkClick, Config{
		Path:         "notice.md",
		Show:         ShowAlways,
		Storage:      storagestub.NewInMemoryService(),
		WorkspaceURI: uri,
	})
	err := c.Show(context.Background())
	require.Error(t, err)
	assert.Equal(t, 0, wm.calls)
}
