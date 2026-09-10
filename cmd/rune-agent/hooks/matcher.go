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

package hooks

import (
	"log/slog"
	"regexp"
	"strings"
	"sync"
)

// matcher represents a compiled matcher for a Group.
type matcher struct {
	all   bool
	exact map[string]struct{}
	re    *regexp.Regexp
}

var (
	exactMatcherRE   = regexp.MustCompile(`^[A-Za-z0-9_|]+$`)
	matcherCache     sync.Map // matcher pattern string → *matcher
	matcherErrLogged sync.Map // matcher pattern string → struct{}
)

// matchValue reports whether the given value matches the group's
// matcher. A nil/empty/"*" matcher matches everything.
func matchValue(pattern, value string) bool {
	m := compileMatcher(pattern)
	if m == nil {
		return false
	}
	if m.all {
		return true
	}
	if m.exact != nil {
		_, ok := m.exact[value]
		return ok
	}
	if m.re != nil {
		return m.re.MatchString(value)
	}
	return false
}

// compileMatcher resolves the three-tier matcher rule:
//  1. ""/"*" → match all
//  2. [A-Za-z0-9_|]+ → exact or |-separated exact list
//  3. otherwise → RE2 regex (compile errors are logged once and the
//     hook is skipped)
//
// Returns nil only on unrecoverable regex compile errors; callers
// must treat nil as "no match".
func compileMatcher(pattern string) *matcher {
	if pattern == "" || pattern == "*" {
		return &matcher{all: true}
	}
	if v, ok := matcherCache.Load(pattern); ok {
		return v.(*matcher)
	}
	var m *matcher
	if exactMatcherRE.MatchString(pattern) {
		parts := strings.Split(pattern, "|")
		exact := make(map[string]struct{}, len(parts))
		for _, p := range parts {
			if p != "" {
				exact[p] = struct{}{}
			}
		}
		m = &matcher{exact: exact}
	} else {
		re, err := regexp.Compile(pattern)
		if err != nil {
			if _, loaded := matcherErrLogged.LoadOrStore(pattern, struct{}{}); !loaded {
				slog.Warn("hooks: invalid matcher regex; hook will be skipped",
					"pattern", pattern, "error", err)
			}
			// Cache a never-match sentinel so we don't recompile.
			m = &matcher{}
			matcherCache.Store(pattern, m)
			return nil
		}
		m = &matcher{re: re}
	}
	matcherCache.Store(pattern, m)
	return m
}
