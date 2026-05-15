// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.

package idehistory

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
	tcomponent "unstable.build/go-tui/component"
)

func mustURI(t *testing.T, s string) workspaceapi.URI {
	t.Helper()
	u, err := workspaceapi.ParseURI(s)
	require.NoError(t, err)
	return u
}

// countingStorage wraps a storageapi.Service and counts each call.
type countingStorage struct {
	storageapi.Service

	mu      sync.Mutex
	getN    int
	setN    int
	deleteN int
	setErr  error
}

func newCountingStorage() *countingStorage {
	return &countingStorage{Service: storagestub.NewInMemoryService()}
}

func (s *countingStorage) Get(ctx context.Context, id string, doc any) error {
	s.mu.Lock()
	s.getN++
	s.mu.Unlock()
	return s.Service.Get(ctx, id, doc)
}

func (s *countingStorage) Set(ctx context.Context, id string, doc any) error {
	s.mu.Lock()
	s.setN++
	err := s.setErr
	s.mu.Unlock()
	if err != nil {
		return err
	}
	return s.Service.Set(ctx, id, doc)
}

func (s *countingStorage) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	s.deleteN++
	s.mu.Unlock()
	return s.Service.Delete(ctx, id)
}

func (s *countingStorage) counts() (get, set, del int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.getN, s.setN, s.deleteN
}

// failingCloseStorage tracks whether Close was called.
type failingCloseStorage struct {
	storageapi.Service
	closed atomic.Bool
}

func (s *failingCloseStorage) Close() error {
	s.closed.Store(true)
	return nil
}

func sampleState(uri workspaceapi.URI) State {
	return State{
		Files: []File{{
			URI:    uri,
			OpenAt: time.Unix(123, 0).UTC(),
			Cursor: term.Coordinates{X: 1, Y: 2},
		}},
		HasLayout: true,
		Layout:    tcomponent.TileLayout{},
	}
}

func TestStoreLoadRoundTrip(t *testing.T) {
	store := New(newCountingStorage())
	uri := mustURI(t, "memory:///roundtrip")
	want := sampleState(uri)

	require.NoError(t, store.StoreWorkspaceState(
		context.Background(), uri, want))

	got, err := store.LoadWorkspaceState(context.Background(), uri)
	require.NoError(t, err)
	require.Len(t, got.Files, 1)
	assert.Equal(t, want.Files[0].URI, got.Files[0].URI)
	assert.Equal(t, want.Files[0].Cursor, got.Files[0].Cursor)
	assert.True(t, got.HasLayout)
}

func TestStoreNestedLayoutRoundTrip(t *testing.T) {
	store := New(newCountingStorage())
	uri := mustURI(t, "memory:///nested-layout")
	layout := tcomponent.TileLayout{
		Split: tcomponent.SplitOrientationVertical,
		Children: []tcomponent.TileLayout{
			{WindowID: 1, Children: []tcomponent.TileLayout{}},
			{
				Split: tcomponent.SplitOrientationHorizontal,
				Children: []tcomponent.TileLayout{
					{WindowID: 2, Children: []tcomponent.TileLayout{}},
					{WindowID: 3, Children: []tcomponent.TileLayout{}},
				},
			},
		},
	}
	require.NoError(t, store.StoreWorkspaceState(
		context.Background(), uri,
		State{HasLayout: true, Layout: layout}))

	got, err := store.LoadWorkspaceState(context.Background(), uri)
	require.NoError(t, err)
	require.True(t, got.HasLayout)
	assert.Equal(t, layout, got.Layout)
}

func TestStoreWorkspaceURIsAreIsolated(t *testing.T) {
	store := New(newCountingStorage())
	uri1 := mustURI(t, "memory:///isolated/one")
	uri2 := mustURI(t, "memory:///isolated/two")

	require.NoError(t, store.StoreWorkspaceState(
		context.Background(), uri1, State{
			Files: []File{{URI: uri1, OpenAt: time.Unix(1, 0)}},
		}))
	require.NoError(t, store.StoreWorkspaceState(
		context.Background(), uri2, State{
			Files: []File{{URI: uri2, OpenAt: time.Unix(2, 0)}},
		}))

	got1, err := store.LoadWorkspaceState(context.Background(), uri1)
	require.NoError(t, err)
	require.Len(t, got1.Files, 1)
	assert.Equal(t, uri1, got1.Files[0].URI)

	got2, err := store.LoadWorkspaceState(context.Background(), uri2)
	require.NoError(t, err)
	require.Len(t, got2.Files, 1)
	assert.Equal(t, uri2, got2.Files[0].URI)
}

func TestClearWorkspaceStateRemoves(t *testing.T) {
	store := New(newCountingStorage())
	uri := mustURI(t, "memory:///clear")

	require.NoError(t, store.StoreWorkspaceState(
		context.Background(), uri, sampleState(uri)))
	require.NoError(t, store.ClearWorkspaceState(context.Background(), uri))

	got, err := store.LoadWorkspaceState(context.Background(), uri)
	require.NoError(t, err)
	assert.True(t, got.IsEmpty())
}

func TestClearMissingDocIsNotAnError(t *testing.T) {
	store := New(newCountingStorage())
	uri := mustURI(t, "memory:///clear-missing")
	require.NoError(t, store.ClearWorkspaceState(context.Background(), uri))
}

func TestLoadMissingDocReturnsZeroState(t *testing.T) {
	store := New(newCountingStorage())
	uri := mustURI(t, "memory:///missing")

	got, err := store.LoadWorkspaceState(context.Background(), uri)
	require.NoError(t, err)
	assert.True(t, got.IsEmpty())
}

func TestStoreReturnsBackendError(t *testing.T) {
	cs := newCountingStorage()
	cs.setErr = errors.New("boom")
	store := New(cs)
	uri := mustURI(t, "memory:///err")

	err := store.StoreWorkspaceState(
		context.Background(), uri, sampleState(uri))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestStoreNotClosedByCloseStore(t *testing.T) {
	backing := &failingCloseStorage{Service: storagestub.NewInMemoryService()}
	_ = New(backing) // never call Close on the backing service
	assert.False(t, backing.closed.Load(),
		"idehistory.Store must not close the underlying storage")
}

func TestStoreWorkspaceStateForClose(t *testing.T) {
	store := New(newCountingStorage())
	uri := mustURI(t, "memory:///for-close")

	snap := stubSnapshotter{
		layout:    tcomponent.TileLayout{WindowID: 42},
		hasLayout: true,
		terminals: []TerminalSession{{Name: "t1"}},
		tasks:     []TaskSession{{Name: "task1", Cmd: "echo"}},
	}
	require.NoError(t, store.StoreWorkspaceStateForClose(
		context.Background(), uri, snap))

	got, err := store.LoadWorkspaceState(context.Background(), uri)
	require.NoError(t, err)
	assert.True(t, got.HasLayout)
	require.Len(t, got.Terminals, 1)
	assert.Equal(t, "t1", got.Terminals[0].Name)
	require.Len(t, got.Tasks, 1)
	assert.Equal(t, "task1", got.Tasks[0].Name)
}

// stubSnapshotter is a minimal Snapshotter for tracker tests.
type stubSnapshotter struct {
	layout    tcomponent.TileLayout
	hasLayout bool
	terminals []TerminalSession
	tasks     []TaskSession
	winIDs    map[string]uint64
}

func (s stubSnapshotter) Terminals() []TerminalSession { return s.terminals }
func (s stubSnapshotter) Tasks() []TaskSession         { return s.tasks }
func (s stubSnapshotter) Layout() (tcomponent.TileLayout, bool) {
	return s.layout, s.hasLayout
}
func (s stubSnapshotter) FileWindowIDs() map[string]uint64 { return s.winIDs }

func TestTrackPersistsOnEditorEvents(t *testing.T) {
	cs := newCountingStorage()
	store := New(cs)
	uri := mustURI(t, "memory:///track")
	fileURI := mustURI(t, "memory:///track/a.go")

	tr := &tracker{
		store: store,
		uri:   uri,
		snap:  stubSnapshotter{},
		ctx:   context.Background(),
		files: make(map[string]File),
		skip:  map[string]struct{}{},
	}

	// Open: persists.
	tr.Handle(context.Background(), textapi.Event{
		Type: textapi.EventTypeOpen,
		URI:  fileURI,
	})
	state, err := store.LoadWorkspaceState(context.Background(), uri)
	require.NoError(t, err)
	require.Len(t, state.Files, 1)

	// Cursor: does NOT persist immediately.
	_, before, _ := cs.counts()
	tr.Handle(context.Background(), textapi.Event{
		Type: textapi.EventTypeCursor,
		URI:  fileURI,
		From: term.Coordinates{X: 5, Y: 7},
	})
	_, after, _ := cs.counts()
	assert.Equal(t, before, after, "Cursor event must not write")

	// Edit: marks dirty but does NOT persist.
	tr.Handle(context.Background(), textapi.Event{
		Type: textapi.EventTypeEdit,
		URI:  fileURI,
	})
	_, after2, _ := cs.counts()
	assert.Equal(t, before, after2, "Edit event must not write")

	// Flush event: persists with Dirty cleared.
	tr.Handle(context.Background(), textapi.Event{
		Type: textapi.EventTypeFlush,
		URI:  fileURI,
	})
	state, err = store.LoadWorkspaceState(context.Background(), uri)
	require.NoError(t, err)
	require.Len(t, state.Files, 1)
	assert.False(t, state.Files[0].Dirty)

	// Close: persists with file removed.
	tr.Handle(context.Background(), textapi.Event{
		Type: textapi.EventTypeClose,
		URI:  fileURI,
	})
	state, err = store.LoadWorkspaceState(context.Background(), uri)
	require.NoError(t, err)
	assert.Empty(t, state.Files)
}

func TestTrackDropsSkippedURIEvents(t *testing.T) {
	store := New(newCountingStorage())
	uri := mustURI(t, "memory:///explorer")
	skippedURI := mustURI(t, "memory:///skip-me")

	tr := &tracker{
		store: store,
		uri:   uri,
		snap:  stubSnapshotter{},
		ctx:   context.Background(),
		files: make(map[string]File),
		skip:  map[string]struct{}{skippedURI.String(): {}},
	}
	tr.Handle(context.Background(), textapi.Event{
		Type: textapi.EventTypeOpen,
		URI:  skippedURI,
	})
	state, err := store.LoadWorkspaceState(context.Background(), uri)
	require.NoError(t, err)
	assert.Empty(t, state.Files,
		"skip-listed URI must not enter persisted state")
}
