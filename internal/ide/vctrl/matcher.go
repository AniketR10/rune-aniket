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

package vctrl

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6/plumbing/format/gitignore"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// Matcher abstracts the ability to match files against glob patterns.
type Matcher interface {
	Match(file workspaceapi.URI, isDir bool) bool
	// MatchRelPath is a fast path equivalent to Match for callers that
	// already have the path relative to the matcher's cwd. It avoids the
	// workspaceapi.RelPath round trip — useful in hot loops like walkdir
	// traversal where every entry would otherwise require a w.URI(path)
	// call (and a remote round trip on non-local workspaces).
	MatchRelPath(relpath string, isDir bool) bool
}

// MatcherFromPatterns returns a matcher that matches files with the given patterns.
func MatcherFromPatterns(
	cwd FileReader, patterns ...gitignore.Pattern,
) (Matcher, error) {
	cwduri, err := cwd.URI(".")
	if err != nil {
		return nil, fmt.Errorf("get workspace uri: %w", err)
	}
	return matcherFromPatternsURI(cwduri, patterns...), nil
}

// matcherFromPatternsURI composes the gitignore matcher with the
// protected-dir exclusion so every gitignore-derived traversal also skips
// OS-protected app-data directories (e.g. ~/Library on macOS) that would
// otherwise trip the system "access data from other apps" prompt.
func matcherFromPatternsURI(
	cwduri workspaceapi.URI, patterns ...gitignore.Pattern,
) Matcher {
	return AnyMatcher(
		uriMatcher{cwduri: cwduri, matcher: gitignore.NewMatcher(patterns)},
		protectedDirMatcherForBase(filepath.Clean(cwduri.Path())),
	)
}

// NopMatcher returns a Matcher that either always or never matches.
func NopMatcher(match bool) Matcher {
	return nopMatcher{match: match}
}

// HiddenBaseMatcher returns a Matcher that matches any entry whose
// final path component begins with a dot (e.g. .git, .DS_Store,
// .vscode). It is used by callers that want to skip hidden entries
// without configuring a full gitignore matcher — for example
// interactive directory completion or the file explorer.
//
// The returned matcher only inspects the path's basename, so it
// never produces false positives based on intermediate components
// (a/b.c/d is not considered hidden).
func HiddenBaseMatcher() Matcher {
	return hiddenBaseMatcher{}
}

type uriMatcher struct {
	cwduri  workspaceapi.URI
	matcher gitignore.Matcher
}

func (u uriMatcher) Match(uri workspaceapi.URI, isDir bool) (match bool) {
	relpath := workspaceapi.RelPath(u.cwduri, uri)
	return u.MatchRelPath(relpath, isDir)
}

func (u uriMatcher) MatchRelPath(relpath string, isDir bool) bool {
	pathcomps := strings.Split(relpath, string(filepath.Separator))
	return u.matcher.Match(pathcomps, isDir)
}

type nopMatcher struct {
	match bool
}

func (n nopMatcher) Match(uri workspaceapi.URI, isDir bool) bool {
	return n.match
}

func (n nopMatcher) MatchRelPath(string, bool) bool {
	return n.match
}

type hiddenBaseMatcher struct{}

func (hiddenBaseMatcher) Match(uri workspaceapi.URI, _ bool) bool {
	return strings.HasPrefix(filepath.Base(uri.Path()), ".")
}

func (hiddenBaseMatcher) MatchRelPath(relpath string, _ bool) bool {
	return strings.HasPrefix(filepath.Base(relpath), ".")
}
