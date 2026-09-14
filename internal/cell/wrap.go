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

package cell

import (
	"strings"

	"github.com/unstablebuild/rune-go-sdk/term"
)

// WrapMarker is stored in the Bytes field of a row's last cell to record
// that the row overflowed the right margin and continues on the next
// row. Bytes otherwise holds the cell's UTF-8 byte count, which never
// reaches this value for a real grapheme cluster.
const WrapMarker uint8 = 1 << 7

// IsRowWrapped reports whether row is the head of a logical line that
// continues on the following row.
func IsRowWrapped(row []term.Cell) bool {
	return len(row) > 0 && row[len(row)-1].Bytes == WrapMarker
}

// RowsToString renders rows as text, separating them with a newline
// except after a row marked with WrapMarker, whose continuation is
// joined back into the single logical line it was written as.
func RowsToString(rows [][]term.Cell) string {
	var builder strings.Builder
	for i := range rows {
		// An empty row is never a wrap continuation: a row only wraps
		// once it is full.
		if i != 0 && (len(rows[i]) == 0 || !IsRowWrapped(rows[i-1])) {
			builder.WriteByte('\n')
		}
		term.CellsToStringBuilder(&builder, rows[i:i+1])
	}
	return builder.String()
}
