// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package ide

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/rune-go-sdk/api/schemeapi"
	"github.com/unstablebuild/rune-go-sdk/api/textapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/rune/internal/debug"
	"unstable.build/rune/internal/ide/vctrl"
)

var outOfRootTabEvents = []textapi.EventType{
	textapi.EventTypeOpen,
	textapi.EventTypeClose,
}

type watchScheme interface {
	Watch(path string, c chan<- schemeapi.EventInfo, events ...schemeapi.Event) (int, error)
	StopWatch(id int) error
}

// outOfRootWatcher watches the files of tabs that live outside the workspace
// root. The workspace watcher is recursive from the root, so without this
// those tabs never observe external changes and silently go stale.
//
// It watches parent directories rather than files: saving by renaming a swap
// file over the original replaces the inode a file watch is attached to.
// Tabs in the same directory share a single watch.
type outOfRootWatcher struct {
	ex     *ex
	scheme watchScheme
	root   workspaceapi.URI
	// uiMu is the lock filesystem events are dispatched under.
	uiMu sync.Locker

	mu     sync.Mutex
	dirs   map[string]*outOfRootDir
	closed bool

	watches sync.WaitGroup
}

type outOfRootDir struct {
	cancel context.CancelFunc
	// files maps base names to the URI of their open tab. Events are
	// dispatched with the tab URI rather than the one the scheme reports,
	// which may spell the directory differently (e.g. with symlinks
	// resolved) and would then not match the tab.
	files map[string]workspaceapi.URI
}

// watchOutOfRootTabs starts watching tabs opened outside the workspace root
// through scheme, dispatching their filesystem events under uiMu.
func (e *ex) watchOutOfRootTabs(scheme watchScheme, uiMu sync.Locker) error {
	w := &outOfRootWatcher{
		ex:     e,
		scheme: scheme,
		root:   e.workspaceURI,
		uiMu:   uiMu,
		dirs:   make(map[string]*outOfRootDir),
	}
	if err := e.comp.SubscribeEvents(outOfRootTabEvents, w); err != nil {
		return err
	}
	e.outOfRootTabs = w
	return nil
}

// Handle implements textapi.EventHandler.
func (w *outOfRootWatcher) Handle(_ context.Context, ev textapi.Event) bool {
	switch ev.Type {
	case textapi.EventTypeOpen:
		if w.isOutOfRoot(ev.URI) {
			w.add(ev.URI)
		}
	case textapi.EventTypeClose:
		w.remove(ev.URI)
	}
	return false
}

func (w *outOfRootWatcher) isOutOfRoot(uri workspaceapi.URI) bool {
	// A URI on another scheme or host cannot be watched through this
	// workspace, and internal tabs (memory://, terminals) have no file.
	sameHost, err := workspaceapi.WithPath(uri, w.root.Path())
	if err != nil || !sameHost.Equal(w.root) || !filepath.IsAbs(uri.Path()) {
		return false
	}
	rel, err := filepath.Rel(w.root.Path(), uri.Path())
	if err != nil {
		return true
	}
	return rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (w *outOfRootWatcher) add(uri workspaceapi.URI) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return
	}
	dir, name := filepath.Split(uri.Path())
	dir = filepath.Clean(dir)
	d, ok := w.dirs[dir]
	if !ok {
		ctx, cancel := context.WithCancel(context.Background())
		d = &outOfRootDir{cancel: cancel, files: make(map[string]workspaceapi.URI)}
		w.dirs[dir] = d
		w.watches.Add(1)
		go debug.CapturePanicReport(func() {
			defer w.watches.Done()
			w.watch(ctx, dir)
		})
	}
	d.files[name] = uri
}

func (w *outOfRootWatcher) remove(uri workspaceapi.URI) {
	w.mu.Lock()
	defer w.mu.Unlock()
	dir, name := filepath.Split(uri.Path())
	dir = filepath.Clean(dir)
	d, ok := w.dirs[dir]
	if !ok {
		return
	}
	if tab, ok := d.files[name]; !ok || !tab.Equal(uri) {
		return
	}
	delete(d.files, name)
	if len(d.files) == 0 {
		d.cancel()
		delete(w.dirs, dir)
	}
}

// watch arms the directory watch off the UI goroutine, since on remote
// workspaces every Watch is an RPC round trip. It owns StopWatch, so a watch
// canceled while still arming is released once Watch returns.
func (w *outOfRootWatcher) watch(ctx context.Context, dir string) {
	ch := make(chan schemeapi.EventInfo, 64)
	id, err := w.scheme.Watch(dir, ch,
		schemeapi.Create, schemeapi.Write,
		schemeapi.Remove, schemeapi.Rename)
	if err != nil {
		w.ex.log(log.WarnLevel, "watch out-of-root tabs in %s: %v", dir, err)
		return
	}
	defer w.scheme.StopWatch(id) //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return
		case fsev, ok := <-ch:
			if !ok || ctx.Err() != nil {
				return
			}
			tab, ok := w.tab(dir, filepath.Base(fsev.URI().Path()))
			if !ok {
				continue
			}
			dispatchFilesystemEvent(w.ex, w.uiMu, vctrl.NopMatcher(false),
				tabEventInfo{EventInfo: fsev, uri: tab})
		}
	}
}

func (w *outOfRootWatcher) tab(dir, name string) (workspaceapi.URI, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	d, ok := w.dirs[dir]
	if !ok {
		return workspaceapi.URI{}, false
	}
	uri, ok := d.files[name]
	return uri, ok
}

func (w *outOfRootWatcher) watchedDirs() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	dirs := make([]string, 0, len(w.dirs))
	for dir := range w.dirs {
		dirs = append(dirs, dir)
	}
	slices.Sort(dirs)
	return dirs
}

// Close stops every watch. Closing tabs normally releases them, but a
// workspace can be torn down without closing its tabs first. It does not wait
// for the watch goroutines, which may be blocked on the UI lock Close is
// called under.
func (w *outOfRootWatcher) Close() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	for dir, d := range w.dirs {
		d.cancel()
		delete(w.dirs, dir)
	}
}

type tabEventInfo struct {
	schemeapi.EventInfo
	uri workspaceapi.URI
}

func (e tabEventInfo) URI() workspaceapi.URI {
	return e.uri
}

// wait blocks until every watch goroutine has exited. It should be used for
// testing only.
func (w *outOfRootWatcher) wait() {
	w.watches.Wait()
}
