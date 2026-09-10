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
	"path/filepath"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// AnyMatcher returns a Matcher that matches an entry when any of the
// given matchers match it. Nil matchers are ignored.
func AnyMatcher(matchers ...Matcher) Matcher {
	return anyMatcher(matchers)
}

type anyMatcher []Matcher

func (a anyMatcher) Match(file workspaceapi.URI, isDir bool) bool {
	for _, m := range a {
		if m != nil && m.Match(file, isDir) {
			return true
		}
	}
	return false
}

func (a anyMatcher) MatchRelPath(relpath string, isDir bool) bool {
	for _, m := range a {
		if m != nil && m.MatchRelPath(relpath, isDir) {
			return true
		}
	}
	return false
}

// ProtectedDirMatcher returns a Matcher that excludes OS-protected
// directories whose contents must not be read without an explicit user
// grant. On macOS, reading another app's data container under the user's
// ~/Library triggers the system "access data from other apps" (TCC App
// Data) prompt, so traversals must skip it by default. The match is
// home-aware: only the current user's real ~/Library is excluded, not
// workspace folders that happen to be named "Library". Returns
// NopMatcher(false) when no protected roots apply.
func ProtectedDirMatcher(cwd FileReader) Matcher {
	cwduri, err := cwd.URI(".")
	if err != nil {
		return NopMatcher(false)
	}
	return protectedDirMatcherForBase(filepath.Clean(cwduri.Path()))
}

// protectedDirMatcherForBase lets callers that already resolved the
// workspace path avoid a second URI(".") round trip.
func protectedDirMatcherForBase(base string) Matcher {
	roots := protectedRoots()
	if len(roots) == 0 {
		return NopMatcher(false)
	}
	return protectedMatcher{base: base, roots: roots}
}

type protectedMatcher struct {
	base  string
	roots []string
}

func (m protectedMatcher) Match(uri workspaceapi.URI, _ bool) bool {
	return m.matchAbs(filepath.Clean(uri.Path()))
}

func (m protectedMatcher) MatchRelPath(relpath string, _ bool) bool {
	abs := relpath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(m.base, relpath)
	}
	return m.matchAbs(filepath.Clean(abs))
}

func (m protectedMatcher) matchAbs(abs string) bool {
	for _, root := range m.roots {
		if abs == root || strings.HasPrefix(abs, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
