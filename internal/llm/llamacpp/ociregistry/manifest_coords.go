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

package ociregistry

import (
	"path/filepath"
	"strings"
)

// manifestCoords parses the directory layout written by Cache.PutManifest
// (manifests/<host>/<repo path...>/<target>) back into its (host, repo,
// target) triple. Returns ok=false for paths that do not match the layout.
//
// Kept as an unexported helper so internal callers (Cache, tests) share a
// single parser; the CacheRegistry that previously owned this function
// has been folded into the llamacpp package which re-implements the
// mapping on top of the public Cache API.
func manifestCoords(root, path string) (host, repo, target string, ok bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", "", "", false
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 3 {
		return "", "", "", false
	}
	host = parts[0]
	target = parts[len(parts)-1]
	repo = strings.Join(parts[1:len(parts)-1], "/")
	// Cache.ManifestPath rewrites "sha256:..." to "sha256-..." so the file
	// name is safe on Windows. Invert that here so digests round-trip to
	// their canonical form for anyone feeding the name back into Pull/Get.
	if alg, rest, cut := strings.Cut(target, "-"); cut && (alg == "sha256" || alg == "sha512") {
		target = alg + ":" + rest
	}
	return host, repo, target, true
}
