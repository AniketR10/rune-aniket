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

package langext

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestFindProjectRoot(t *testing.T) {
	const ws = "/ws"
	markers := []string{"pyproject.toml", ".venv"}

	cases := []struct {
		name    string
		paths   []string // file/dir markers present, relative to ws
		file    string   // opened file, absolute
		wantRel string
		wantOK  bool
	}{
		{
			name:    "nearest root wins over ancestor",
			paths:   []string{"pyproject.toml", "deploy/worker/pyproject.toml"},
			file:    "/ws/deploy/worker/app/main.py",
			wantRel: "deploy/worker",
			wantOK:  true,
		},
		{
			name:    "marker at workspace root",
			paths:   []string{"pyproject.toml"},
			file:    "/ws/main.py",
			wantRel: "",
			wantOK:  true,
		},
		{
			name:    "no marker yields not found",
			paths:   []string{"deploy/worker/app/main.py"},
			file:    "/ws/deploy/worker/app/main.py",
			wantRel: "",
			wantOK:  false,
		},
		{
			name:    "file in same dir as marker",
			paths:   []string{"deploy/worker/pyproject.toml"},
			file:    "/ws/deploy/worker/main.py",
			wantRel: "deploy/worker",
			wantOK:  true,
		},
		{
			name:    "venv directory marker",
			paths:   []string{"svc/.venv/"},
			file:    "/ws/svc/main.py",
			wantRel: "svc",
			wantOK:  true,
		},
		{
			name:    "file outside workspace ignored",
			paths:   []string{"pyproject.toml"},
			file:    "/elsewhere/main.py",
			wantRel: "",
			wantOK:  false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mfs := newMemFS(ws)
			for _, p := range tc.paths {
				if strings.HasSuffix(p, "/") {
					mfs.addDir(filepath.Join(ws, p))
					continue
				}
				mfs.addFile(filepath.Join(ws, p))
			}
			wsURI, err := workspaceapi.ParseURI("file://" + ws)
			require.NoError(t, err)
			fileURI, err := workspaceapi.ParseURI("file://" + tc.file)
			require.NoError(t, err)

			root, ok := FindProjectRoot(mfs, wsURI, fileURI, markers)
			assert.Equal(t, tc.wantOK, ok)
			if !tc.wantOK {
				return
			}
			assert.Equal(t, tc.wantRel, root.RelPath)
			wantDir := filepath.Join(ws, tc.wantRel)
			assert.Equal(t, wantDir, root.Dir)
			assert.Equal(t, "file://"+wantDir, root.URI)
		})
	}
}

func TestInitializerOpenTriggersOneInitRoot(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws)
	mfs.addFile("/ws/svc/pyproject.toml")

	var calls atomic.Int32
	roots := make(chan Root, 4)
	cfg := pyConfig(func(_ context.Context, r Root) error {
		calls.Add(1)
		roots <- r
		return nil
	})

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())
	require.Equal(t, []textapi.EventType{textapi.EventTypeOpen}, ed.subscribedTo)

	ed.fire(t, openEvent("/ws/svc/main.py"))
	got := <-roots
	assert.Equal(t, "svc", got.RelPath)

	// A second open of the same root must not re-run InitRoot.
	ed.fire(t, openEvent("/ws/svc/other.py"))
	assertNoMoreRoots(t, roots)
	assert.Equal(t, int32(1), calls.Load())
}

// TestInitializerAsyncSurvivesEventContextCancel reproduces the bug where
// the background bring-up was tied to the editor's per-event dispatch
// context: that context is canceled as soon as Handle returns, which
// killed the in-flight uv/LSP bring-up with "context canceled". The async
// work must instead run under the long-lived workspace context, so the
// context InitRoot observes stays live after Handle returns.
func TestInitializerAsyncSurvivesEventContextCancel(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws)
	mfs.addFile("/ws/svc/pyproject.toml")

	proceed := make(chan struct{})
	errs := make(chan error, 1)
	cfg := pyConfig(func(ctx context.Context, _ Root) error {
		<-proceed // ensure the event ctx is canceled before we inspect ours
		errs <- ctx.Err()
		return nil
	})

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())

	// Deliver the open with a context that is canceled the instant Handle
	// returns, exactly as the editor's event dispatch does.
	evCtx, cancel := context.WithCancel(context.Background())
	ed.handler.Handle(evCtx, openEvent("/ws/svc/main.py"))
	cancel()
	close(proceed)

	select {
	case err := <-errs:
		assert.NoError(t, err, "bring-up must not observe a canceled context")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for InitRoot")
	}
}

func TestInitializerConcurrentOpensDedupe(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws)
	mfs.addFile("/ws/svc/pyproject.toml")

	var calls atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{}, 16)
	cfg := pyConfig(func(_ context.Context, _ Root) error {
		calls.Add(1)
		started <- struct{}{}
		<-release // hold the first claim open so racing opens observe it
		return nil
	})

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())

	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			ed.fire(t, openEvent("/ws/svc/main.py"))
		})
	}
	<-started // exactly one bring-up should start
	close(release)
	wg.Wait()

	assertNoExtraStart(t, started)
	assert.Equal(t, int32(1), calls.Load())
}

func TestInitializerIgnoresNonMatchingFiles(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws)
	mfs.addFile("/ws/svc/pyproject.toml")

	var calls atomic.Int32
	cfg := pyConfig(func(_ context.Context, _ Root) error {
		calls.Add(1)
		return nil
	})

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())

	ed.fire(t, openEvent("/ws/svc/README.md"))
	assert.Equal(t, int32(0), calls.Load())
}

func TestInitializerNoMarkerDoesNotInitialize(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws) // no markers anywhere

	var calls atomic.Int32
	cfg := pyConfig(func(_ context.Context, _ Root) error {
		calls.Add(1)
		return nil
	})

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())

	ed.fire(t, openEvent("/ws/svc/stray.py"))
	assert.Equal(t, int32(0), calls.Load())
}

func TestInitializeAtDedupedAgainstLaterEvents(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws)
	mfs.addFile("/ws/pyproject.toml")

	var calls atomic.Int32
	roots := make(chan Root, 4)
	cfg := pyConfig(func(_ context.Context, r Root) error {
		calls.Add(1)
		roots <- r
		return nil
	})

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())

	root := rootForDir(mfs, ws, ws)
	require.NoError(t, i.InitializeAt(context.Background(), root))
	got := <-roots
	assert.Equal(t, "", got.RelPath)

	// An open that resolves to the already-initialized root is a no-op.
	ed.fire(t, openEvent("/ws/main.py"))
	assertNoMoreRoots(t, roots)
	assert.Equal(t, int32(1), calls.Load())
}

func TestInitializeAtRetriesAfterFailure(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws)
	mfs.addFile("/ws/pyproject.toml")

	var calls atomic.Int32
	cfg := pyConfig(func(_ context.Context, _ Root) error {
		if calls.Add(1) == 1 {
			return assert.AnError
		}
		return nil
	})

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())

	root := rootForDir(mfs, ws, ws)
	require.Error(t, i.InitializeAt(context.Background(), root))
	require.NoError(t, i.InitializeAt(context.Background(), root))
	assert.Equal(t, int32(2), calls.Load(), "failed bring-up must be retryable")
}

func TestReinitializeRerunsAllKnownRoots(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws)
	mfs.addFile("/ws/a/Cargo.toml")
	mfs.addFile("/ws/b/Cargo.toml")

	var calls atomic.Int32
	cfg := ProjectConfig{
		LanguageID: "rust",
		Markers:    []string{"Cargo.toml"},
		FileMatch: func(uri workspaceapi.URI) bool {
			return strings.HasSuffix(uri.Path(), ".rs")
		},
		InitRoot: func(_ context.Context, _ Root) error {
			calls.Add(1)
			return nil
		},
	}

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())

	require.NoError(t, i.InitializeAt(context.Background(), rootForDir(mfs, ws, "/ws/a")))
	require.NoError(t, i.InitializeAt(context.Background(), rootForDir(mfs, ws, "/ws/b")))
	require.Equal(t, int32(2), calls.Load())

	require.NoError(t, i.Reinitialize(context.Background()))
	assert.Equal(t, int32(4), calls.Load(), "every known root must be re-initialized")

	// Roots stay known after reinit, so a later reinit re-runs them again.
	require.NoError(t, i.Reinitialize(context.Background()))
	assert.Equal(t, int32(6), calls.Load())
}

func TestReinitializeNoOpWhenNothingInitialized(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws)

	var calls atomic.Int32
	cfg := pyConfig(func(_ context.Context, _ Root) error {
		calls.Add(1)
		return nil
	})

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())

	require.NoError(t, i.Reinitialize(context.Background()))
	assert.Equal(t, int32(0), calls.Load())
}

func TestReinitializeSkipsRootStillInitializing(t *testing.T) {
	const ws = "/ws"
	mfs := newMemFS(ws)
	mfs.addFile("/ws/svc/pyproject.toml")

	var calls atomic.Int32
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	cfg := pyConfig(func(_ context.Context, _ Root) error {
		calls.Add(1)
		started <- struct{}{}
		<-release // hold the in-flight claim open across Reinitialize
		return nil
	})

	ed := &fakeEditor{}
	i := NewInitializer(context.Background(), mfs, ed, cfg)
	require.NoError(t, i.Start())

	// Drive an open that starts a bring-up and blocks it in InitRoot.
	go ed.fire(t, openEvent("/ws/svc/main.py"))
	<-started

	// The only root is still initializing, so Reinitialize must skip it.
	require.NoError(t, i.Reinitialize(context.Background()))
	assert.Equal(t, int32(1), calls.Load(), "an initializing root must not be re-run")

	close(release)
}

// pyConfig builds a python-shaped ProjectConfig with the given InitRoot.
func pyConfig(initRoot func(context.Context, Root) error) ProjectConfig {
	return ProjectConfig{
		LanguageID: "python",
		Markers:    []string{"pyproject.toml", ".venv"},
		FileMatch: func(uri workspaceapi.URI) bool {
			return strings.HasSuffix(uri.Path(), ".py")
		},
		InitRoot: initRoot,
	}
}

func openEvent(path string) textapi.Event {
	uri, _ := workspaceapi.ParseURI("file://" + path)
	return textapi.Event{Type: textapi.EventTypeOpen, URI: uri}
}

func assertNoMoreRoots(t *testing.T, roots <-chan Root) {
	t.Helper()
	select {
	case r := <-roots:
		t.Fatalf("unexpected extra InitRoot for %q", r.Dir)
	default:
	}
}

func assertNoExtraStart(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
		t.Fatal("a second bring-up started; dedupe failed")
	default:
	}
}

// fakeEditor is a textapi.Editor that only records the open-event
// subscription so tests can drive Handle directly. Every other method is
// an unused stub.
type fakeEditor struct {
	mu           sync.Mutex
	subscribedTo []textapi.EventType
	handler      textapi.EventHandler
}

var _ textapi.Editor = (*fakeEditor)(nil)

func (e *fakeEditor) SubscribeEvents(types []textapi.EventType, h textapi.EventHandler) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subscribedTo = types
	e.handler = h
	return nil
}

// fire delivers ev to the subscribed handler, failing if Start was not
// called first.
func (e *fakeEditor) fire(t *testing.T, ev textapi.Event) {
	t.Helper()
	e.mu.Lock()
	h := e.handler
	e.mu.Unlock()
	require.NotNil(t, h, "no event handler subscribed")
	h.Handle(context.Background(), ev)
}

func (e *fakeEditor) Editor(workspaceapi.URI) (textapi.Handler, error) { return nil, nil }
func (e *fakeEditor) SetLocationList(
	textapi.Handler, textapi.LocationPriority, string, textapi.LocationList,
) error {
	return nil
}
func (e *fakeEditor) MoveToNextLocation(textapi.Handler, string) error { return nil }
func (e *fakeEditor) MoveToPrevLocation(textapi.Handler, string) error { return nil }
func (e *fakeEditor) Cursor(textapi.Handler) (term.Coordinates, error) {
	return term.Coordinates{}, nil
}
func (e *fakeEditor) SetCursor(textapi.Handler, term.Coordinates) error { return nil }
func (e *fakeEditor) CellView(textapi.Handler) textapi.CellView         { return nil }
func (e *fakeEditor) CellEditor(textapi.Handler) textapi.CellEditor     { return nil }
func (e *fakeEditor) SetDefaultAttributes(textapi.Handler, term.Attributes) error {
	return nil
}

// memFS is an in-memory workspaceapi.FileSystem with real path-join
// semantics so the upward marker walk can be exercised without touching
// disk.
type memFS struct {
	root  string
	files map[string]bool
	dirs  map[string]bool
}

func newMemFS(root string) *memFS {
	return &memFS{root: root, files: map[string]bool{}, dirs: map[string]bool{}}
}

func (m *memFS) addFile(p string) { m.files[filepath.Clean(p)] = true }
func (m *memFS) addDir(p string)  { m.dirs[filepath.Clean(p)] = true }

func (m *memFS) resolve(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(m.root, p)
}

func (m *memFS) URI(p string) (workspaceapi.URI, error) {
	return workspaceapi.ParseURI("file://" + m.resolve(p))
}

func (m *memFS) Stat(p string) (os.FileInfo, error) {
	name := m.resolve(p)
	if m.files[name] {
		return memFileInfo{name: filepath.Base(name)}, nil
	}
	if m.dirs[name] {
		return memFileInfo{name: filepath.Base(name), dir: true}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: p, Err: os.ErrNotExist}
}

func (m *memFS) ReadDir(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist }
func (m *memFS) OpenFile(string, int, os.FileMode) (workspaceapi.File, error) {
	return nil, os.ErrInvalid
}
func (m *memFS) Remove(string) error                { return os.ErrInvalid }
func (m *memFS) MkdirAll(string, os.FileMode) error { return os.ErrInvalid }

type memFileInfo struct {
	name string
	dir  bool
}

func (i memFileInfo) Name() string       { return i.name }
func (i memFileInfo) Size() int64        { return 0 }
func (i memFileInfo) Mode() os.FileMode  { return 0 }
func (i memFileInfo) ModTime() time.Time { return time.Time{} }
func (i memFileInfo) IsDir() bool        { return i.dir }
func (i memFileInfo) Sys() any           { return nil }
