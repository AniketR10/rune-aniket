// Copyright (C) 2017-2026 Unstable Build, LLC
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

package main

import (
	"context"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/rune/internal/handler/search"
)

// recentWorkspacesDocumentID is the storage key for the projects opened
// through the app menu / open panel. Prompt-driven opens live in the
// IDE's own command history; the two are merged for display.
const recentWorkspacesDocumentID = "recent-workspaces:appmenu"

// recentWorkspacesMax caps the persisted menu-open history.
const recentWorkspacesMax = 20

// recentMenuLimit caps how many entries the Open Recent menu shows
// after merging the menu-open and command-prompt histories.
const recentMenuLimit = 10

// recentWorkspaces records the projects opened via the app menu so they
// can be offered under Open Recent alongside the command-prompt history.
type recentWorkspaces struct {
	history *search.History
}

func newRecentWorkspaces(storage storageapi.Service) *recentWorkspaces {
	h := search.NewHistory(storage, recentWorkspacesDocumentID, recentWorkspacesMax)
	// Load materializes the backing document (creating it when absent),
	// which Add requires; without it the first record would fail.
	if err := h.Load(); err != nil {
		log.Errorf("load recent workspaces: %v", err)
	}
	return &recentWorkspaces{history: h}
}

// record prepends path to the menu-open history, most-recent-first and
// de-duplicated. A blank path is ignored.
func (r *recentWorkspaces) record(path string) {
	if r == nil || strings.TrimSpace(path) == "" {
		return
	}
	if err := r.history.Add(path); err != nil {
		log.Errorf("record recent workspace %q: %v", path, err)
	}
}

// paths returns the menu-open history newest-first, or nil when empty.
func (r *recentWorkspaces) paths() []string {
	if r == nil {
		return nil
	}
	it, ok := r.history.HistoryIterator(context.Background(), nil)
	if !ok || it == nil {
		return nil
	}
	paths, err := iterator.ToSlice(context.Background(), it)
	if err != nil {
		log.Errorf("read recent workspaces: %v", err)
		return nil
	}
	return paths
}

// recentEntry is one Open Recent menu row: a disambiguated display
// label and the path to dispatch when selected.
type recentEntry struct {
	label string
	path  string
}

// recentLabels turns recent workspace paths into display entries. Order
// is preserved (callers pass newest-first). Exact-duplicate paths are
// collapsed to the first occurrence. Labels are the trailing directory
// name; when several paths share a name, each grows by one more parent
// segment until they are distinct, so "web" and "web" become
// "app/web" and "api/web".
func recentLabels(paths []string) []recentEntry {
	normalized := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, p := range paths {
		norm := normalizeWorkspacePath(p)
		if norm == "" || seen[norm] {
			continue
		}
		seen[norm] = true
		normalized = append(normalized, norm)
	}

	entries := make([]recentEntry, len(normalized))
	for i, p := range normalized {
		entries[i] = recentEntry{label: labelFor(p, normalized), path: p}
	}
	return entries
}

// labelFor picks the display label for one path. Remote URIs (with a
// scheme) are shown verbatim since their basename alone is misleading;
// local paths are disambiguated by trailing segments.
func labelFor(target string, all []string) string {
	if strings.Contains(target, "://") {
		return target
	}
	return disambiguateLabel(target, all)
}

// disambiguateLabel returns the shortest trailing path suffix of target
// that no other path shares. It starts at the basename and adds one
// leading segment at a time; if every suffix collides (identical paths,
// already deduped) it falls back to the full path.
func disambiguateLabel(target string, all []string) string {
	segs := pathSegments(target)
	for take := 1; take <= len(segs); take++ {
		suffix := strings.Join(segs[len(segs)-take:], "/")
		unique := true
		for _, other := range all {
			if other == target {
				continue
			}
			if pathHasSuffix(pathSegments(other), segs[len(segs)-take:]) {
				unique = false
				break
			}
		}
		if unique {
			return suffix
		}
	}
	return target
}

// pathHasSuffix reports whether segs ends with suffix.
func pathHasSuffix(segs, suffix []string) bool {
	if len(suffix) > len(segs) {
		return false
	}
	tail := segs[len(segs)-len(suffix):]
	for i := range suffix {
		if tail[i] != suffix[i] {
			return false
		}
	}
	return true
}

// normalizeWorkspacePath reduces a workspace path or file:// URI to a
// clean absolute-ish path for display and comparison, dropping a
// trailing separator. Non-file URIs (e.g. ssh://) are left intact so
// remote workspaces still round-trip to workspaceopen.
func normalizeWorkspacePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(p, "file://"); ok {
		p = rest
	} else if strings.Contains(p, "://") {
		return p
	}
	return filepath.Clean(p)
}

// pathSegments splits a cleaned path into its non-empty components. The
// leading "/" of an absolute path yields no empty segment.
func pathSegments(p string) []string {
	trimmed := strings.Trim(p, "/")
	if trimmed == "" {
		return []string{p}
	}
	return strings.Split(trimmed, "/")
}
