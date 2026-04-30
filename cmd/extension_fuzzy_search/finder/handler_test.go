// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package finder

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/config"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/handler/handlertest"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/tui"
	"unstable.build/go-tui/handler/search"
	"unstable.build/go-tui/ide/vctrl"
	"unstable.build/go-tui/workspace"
	"unstable.build/go-tui/workspace/walkdir"
)

// TestNativeBackend_GitignoreFiltersResults exercises the native fuzzy-search
// backend end-to-end. It builds the handler with a real workspace.FileScheme
// rooted at testdata/, where .gitignore declares "*.TEST". The test asserts
// that:
//
//   - sample.text shows up in the rendered list
//   - sample.TEST does not (it is filtered by the gitignore matcher plumbed
//     through Clients.IgnoreMatcher → walkdir.WithContextFilter)
func TestNativeBackend_GitignoreFiltersResults(t *testing.T) {
	testdata, err := filepath.Abs("testdata")
	require.NoError(t, err)

	cwdURI, err := workspaceapi.CurrentUserHostURI(testdata)
	require.NoError(t, err)

	scheme, err := workspace.NewFileScheme(context.Background(), config.NopConfig(), cwdURI)
	require.NoError(t, err)
	t.Cleanup(func() { _ = scheme.Close() })

	matcher, err := vctrl.LoadGitignore(scheme)
	require.NoError(t, err)

	clients := Clients{
		ResourceOpener: stubResourceOpener{},
		WindowManager:  stubWindowManager{},
		Interrupter:    term.NopInterrupter(),
		Notifications:  stubNotifications{},
		Editor:         nil,
		FileSystem:     scheme,
		Executor:       nil,
		IgnoreMatcher:  matcher,
	}

	listFiles := func(fs workspaceapi.FileSystem, ctx context.Context) (
		iterator.Iterator[string], error,
	) {
		return walkdir.ListFiles(ctx, fs, ".")
	}
	getResource := func(fs workspaceapi.FileSystem, line string) (
		workspaceapi.URI, term.Coordinates, bool,
	) {
		uri, err := fs.URI(line)
		return uri, term.Coordinates{}, err == nil
	}

	listCfg := search.ListConfig{
		Algo:        search.FuzzyMatch,
		Interrupter: term.NopInterrupter(),
		SyncSearch:  true,
	}
	rh, err := newV2WithListConfig(
		context.Background(), clients, stubWindow(0),
		term.KeyComb{}, "", "", 0, listCfg,
		listFiles, getResource,
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rh.Close() })

	h := rh.(*fuzzyFinderHandler)

	// Wait until the workspace scan has fully drained into the list before
	// asserting on rendered output.
	waitForScanWithTimeout(t, h, 5*time.Second)

	// Drive the handler through RunHandlerSequence as a black-box check.
	// Two assertions in one sequence:
	//
	//  1. With no query typed, the list shows the unfiltered scan output:
	//     ".gitignore" and "sample.text" — but NOT "sample.TEST", which is
	//     excluded by the testdata/.gitignore rule "*.TEST" applied through
	//     Clients.IgnoreMatcher.
	//  2. Typing "text" narrows the list to sample.text only and the count
	//     drops to "1/2".
	runner := &resizableHandler{h: h}
	initial := strings.Join([]string{
		"▐                                    2/2",
		".gitignore                              ",
		"sample.text                             ",
		"                                        ",
		"                                        ",
		"                                        ",
	}, "\n")
	filtered := strings.Join([]string{
		"text▐                                1/2",
		"sample.text                             ",
		"                                        ",
		"                                        ",
		"                                        ",
		"                                        ",
	}, "\n")
	handlertest.RunHandlerSequence(t, runner, 40, 6, []handlertest.SequenceTestCase{
		{InputSequence: "", Expected: initial},
		{InputSequence: "text", Expected: filtered},
	})
}

// waitForScanWithTimeout invokes the test-only scan barrier with a deadline so
// failures surface as a clear test error rather than a hang.
func waitForScanWithTimeout(t *testing.T, h *fuzzyFinderHandler, d time.Duration) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		waitForScanForTest(h)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatalf("scan did not complete within %s", d)
	}
}

func waitForScanForTest(h *fuzzyFinderHandler) {
	<-h.scanDone
	// Re-invoking Push cancels the prior push and waits for the prior
	// consumer goroutine to exit, guaranteeing the deferred pushData()
	// and sortMatchesList() in consumeAsyncElements have drained the
	// data channel. Close the new channel immediately so the freshly
	// started consumer also returns and the list returns to idle.
	ch := h.list.Push(context.Background())
	close(ch)
}

// resizableHandler is a thin tui.Handler shim around fuzzyFinderHandler so
// handlertest.RunHandlerSequence can drive it directly.
type resizableHandler struct {
	h *fuzzyFinderHandler
}

func (r *resizableHandler) Handle(ev term.Event) (exit, handled bool) {
	return r.h.Handle(ev)
}
func (r *resizableHandler) Resize(w, h int)    { r.h.Resize(w, h) }
func (r *resizableHandler) Draw(w term.Writer) { r.h.Draw(w) }
func (r *resizableHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return r.h.Cursor()
}
func (r *resizableHandler) Selection() (string, bool) { return r.h.Selection() }

// --- stubs ----------------------------------------------------------------

type stubWindow uint64

func (w stubWindow) WindowID() uint64 { return uint64(w) }

type stubResourceOpener struct{}

func (stubResourceOpener) Open(workspaceapi.URI) (browserapi.Handler, error) {
	return stubBrowserHandler{}, nil
}

type stubBrowserHandler struct{}

func (stubBrowserHandler) Handle(term.Event) (bool, bool) { return false, false }
func (stubBrowserHandler) Resize(int, int)                {}
func (stubBrowserHandler) Draw(term.Writer)               {}
func (stubBrowserHandler) Cursor() (term.Coordinates, term.CursorStyle, bool) {
	return term.Coordinates{}, 0, false
}
func (stubBrowserHandler) Selection() (string, bool) { return "", false }
func (stubBrowserHandler) Close() error              { return nil }

type stubWindowManager struct{}

func (stubWindowManager) Focus() (browserapi.Window, error) { return stubWindow(0), nil }
func (stubWindowManager) Split(browserapi.Orientation, browserapi.Window, browserapi.Handler) (browserapi.Window, error) {
	return stubWindow(0), nil
}
func (stubWindowManager) Floating(browserapi.Floating, browserapi.FloatingConfig) (browserapi.Window, error) {
	return stubWindow(0), nil
}
func (stubWindowManager) Bar(browserapi.BarConfig, tui.Handler) error { return nil }
func (stubWindowManager) Tab(workspaceapi.URI, rune, string, browserapi.Handler) (browserapi.Handler, error) {
	return stubBrowserHandler{}, nil
}
func (stubWindowManager) SetWindowContent(browserapi.Window, browserapi.Handler) error { return nil }
func (stubWindowManager) CloseWindow(browserapi.Window) error                          { return nil }

type stubNotifications struct{}

func (stubNotifications) Notify(browserapi.NotificationLevel, string, ...any) (string, error) {
	return "", nil
}
func (stubNotifications) NotifyOnce(browserapi.NotificationLevel, string, ...any) (string, error) {
	return "", nil
}
func (stubNotifications) UpdateNotificationProgress(string, string, int64, int64) error {
	return nil
}
