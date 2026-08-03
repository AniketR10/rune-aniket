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

package main

import (
	"context"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/unstablebuild/blue/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi"
	"unstable.build/go-tui/handler/search"
)

// recentWorkspacesDocumentID is the storage key for the projects opened
// through the app menu / open panel. Prompt-driven opens live in the
// IDE's own command history; the two are merged for display.
const recentWorkspacesDocumentID = "recent-workspaces:appmenu"

// recentWorkspacesMax caps the persisted menu-open history.
const recentWorkspacesMax = 20

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
