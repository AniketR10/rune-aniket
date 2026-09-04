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

package vteprobe

import "github.com/unstablebuild/rune-go-sdk/term"

// computeConfidence combines several lightweight signals into a value
// in [0, 1]:
//   - alignment coverage: fraction of rows that matched their file line;
//   - whether the cursor row itself has positive alignment;
//   - whether the cursor cell looks like real content (non-empty rune).
//
// The function is intentionally simple — callers threshold the result
// against the minConfidence value passed to New, so subtle tuning is
// the caller's concern.
func computeConfidence(
	a alignment,
	w wrapInfo,
	rows []extractedRow,
	cur term.Coordinates,
) float64 {
	conf := a.coverage
	if conf > 1 {
		conf = 1
	}
	if cur.Y >= 0 && cur.Y < len(w.rowFileLine) && w.rowFileLine[cur.Y] > 0 {
		conf += 0.1
	}
	if cur.Y >= 0 && cur.Y < len(rows) {
		row := rows[cur.Y]
		idx := row.runeColAt(cur.X)
		if idx >= 0 && idx < len(row.runes) && row.runes[idx] != ' ' && row.runes[idx] != 0 {
			conf += 0.05
		}
	}
	if conf > 1 {
		conf = 1
	}
	return conf
}
