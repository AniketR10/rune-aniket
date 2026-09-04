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

// Package ideupgrade implements the in-product self-upgrade flow for
// the Rune client. It periodically polls a static manifest object
// on the public downloads CDN (no server-side proxy), prompts the
// user when a newer version is available, and (on confirmation)
// downloads, verifies and replaces the running app bundle with
// rollback support.
package ideupgrade

// Manifest describes a release artifact for one "<os>-<arch>" target.
// The shape matches the JSON written by `cmd/rune/dist.sh` and
// uploaded to `<downloads-host>/<os>-<arch>/manifest.json`.
type Manifest struct {
	Version             string `json:"version"`
	Commit              string `json:"commit,omitempty"`
	OS                  string `json:"os"`
	Arch                string `json:"arch"`
	Filename            string `json:"filename"`
	URL                 string `json:"url"`
	SHA256              string `json:"sha256"`
	Size                int64  `json:"size"`
	PublishedAt         string `json:"published_at,omitempty"`
	MinSupportedVersion string `json:"min_supported_version,omitempty"`
	Changelog           string `json:"changelog,omitempty"`
}
