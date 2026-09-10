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
	"path/filepath"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// workspaceBasename returns the sanitised basename of the workspace
// path so it can be embedded in shell arguments without surprises.
// Works for every scheme: commands run via the workspace's executor in
// the workspace's own filesystem, so the URI path is a meaningful
// identifier whether the workspace is local, remote, or in-memory.
func workspaceBasename(uri workspaceapi.URI) string {
	base := filepath.Base(filepath.Clean(uri.Path()))
	return sanitiseBasename(base)
}

// sanitiseBasename replaces characters that are awkward in shell
// arguments / on-disk path segments with '_'. The replacement set is
// deliberately conservative — anything that is not an ASCII letter,
// digit, '.', '-', or '_' is replaced.
func sanitiseBasename(s string) string {
	if s == "" {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z',
			r >= 'A' && r <= 'Z',
			r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	return b.String()
}
