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

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/unstablebuild/rune-go-sdk/term"
)

// makeStyledRow builds an extractedRow whose every non-space rune carries
// the given attributes (used to simulate reverse-video status rows).
func makeStyledRow(s string, a term.Attributes) extractedRow {
	runes := []rune(s)
	r := extractedRow{
		runes:      runes,
		attrs:      make([]term.Attributes, len(runes)),
		runeColMap: make([]int, len(runes)),
	}
	for i, ru := range runes {
		r.runeColMap[i] = i
		if ru != ' ' {
			r.attrs[i] = a
		}
	}
	return r
}

func TestDetectChrome(t *testing.T) {
	t.Parallel()

	reverse := term.Attributes{Attrs: term.AttrReverse}

	t.Run("status row at bottom is peeled", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow("line 1"),
			makeRow("line 2"),
			makeStyledRow("-- INSERT --", reverse),
		}
		top, bot := detectChrome(rows, nil)
		assert.Equal(t, 0, top)
		assert.Equal(t, 1, bot)
	})

	t.Run("status row at top is peeled", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeStyledRow("file.go +", reverse),
			makeRow("line 1"),
			makeRow("line 2"),
		}
		top, bot := detectChrome(rows, nil)
		assert.Equal(t, 1, top)
		assert.Equal(t, 2, bot)
	})

	t.Run("plain rows: no chrome peeled", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow("a"),
			makeRow("b"),
			makeRow("c"),
		}
		top, bot := detectChrome(rows, cellLines([]string{"a", "b", "c"}))
		assert.Equal(t, 0, top)
		assert.Equal(t, 2, bot)
	})

	t.Run("two-row status line at bottom is peeled", func(t *testing.T) {
		t.Parallel()
		rows := []extractedRow{
			makeRow("content"),
			makeStyledRow("file.go [+]", reverse),
			makeStyledRow(":w", reverse),
		}
		top, bot := detectChrome(rows, nil)
		assert.Equal(t, 0, top)
		assert.Equal(t, 0, bot)
	})

	t.Run("leading blank file line is not peeled as chrome", func(t *testing.T) {
		t.Parallel()
		// A viewport scrolled so its first visible row is a real blank
		// file line. With no chrome above it, the blank must stay content
		// so the inferred scroll position is not shifted down by one.
		rows := []extractedRow{
			makeRow(""),
			makeRow("func main() {"),
			makeRow("\treturn"),
		}
		top, bot := detectChrome(rows,
			cellLines([]string{"", "func main() {", "\treturn"}))
		assert.Equal(t, 0, top)
		assert.Equal(t, 2, bot)
	})

	t.Run("blank separator under top chrome is peeled", func(t *testing.T) {
		t.Parallel()
		// nano draws a title bar then a blank separator before content.
		// The blank trails peeled chrome, so it is padding and peeled.
		rows := []extractedRow{
			makeStyledRow("File: main.go", reverse),
			makeRow(""),
			makeRow("package main"),
		}
		top, bot := detectChrome(rows,
			cellLines([]string{"package main"}))
		assert.Equal(t, 2, top)
		assert.Equal(t, 2, bot)
	})

	t.Run("blank file line under top chrome is kept", func(t *testing.T) {
		t.Parallel()
		// A viewport scrolled so its first file line is a real blank
		// line (e.g. the blank above a doc comment), drawn under nano's
		// title bar. The blank mirrors the file line preceding the
		// content below it, so it must stay content rather than be
		// peeled as a separator.
		rows := []extractedRow{
			makeStyledRow("File: main.go", reverse),
			makeRow(""),
			makeRow("// Doc comment."),
			makeRow("func main() {"),
		}
		top, bot := detectChrome(rows, cellLines([]string{
			"package main",
			"",
			"// Doc comment.",
			"func main() {",
		}))
		assert.Equal(t, 1, top)
		assert.Equal(t, 3, bot)
	})
}
