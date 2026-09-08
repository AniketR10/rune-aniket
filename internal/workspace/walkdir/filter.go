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

package walkdir

import "context"

// Filter decides whether an entry produced by walkdir should be excluded.
// MatchRelPath returns true when the entry at the given workspace-relative
// path must be skipped. The signature deliberately takes a relative path
// (rather than a workspaceapi.URI) so walkdir can avoid an extra w.URI()
// round trip per directory entry — non-trivial on remote workspaces. The
// interface mirrors vctrl.Matcher.MatchRelPath so a vctrl.Matcher value
// satisfies Filter directly.
type Filter interface {
	MatchRelPath(relpath string, isDir bool) bool
}

type filterContextKey struct{}

// WithContextFilter returns a context configured to exclude entries matching f
// from walkdir traversal results. The filter is consulted for every regular
// file and every directory before recursing into it. Constructing a filter
// (e.g. via vctrl.LoadGitignore) is expensive, so callers should build the
// filter once and reuse the resulting context across walkdir invocations.
// Passing a nil filter is a no-op.
func WithContextFilter(ctx context.Context, f Filter) context.Context {
	if f == nil {
		return ctx
	}
	return context.WithValue(ctx, filterContextKey{}, f)
}

func filterFromContext(ctx context.Context) Filter {
	f, _ := ctx.Value(filterContextKey{}).(Filter)
	return f
}
