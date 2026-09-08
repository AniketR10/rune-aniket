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

package idecursor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"github.com/unstablebuild/rune-go-sdk/term"
)

func TestShouldRecord(t *testing.T) {
	base := location{URI: "file:///a.go", Cursor: term.Coordinates{X: 1, Y: 10}}
	assert.False(t, shouldRecord(base, base))
	assert.False(t, shouldRecord(base, location{URI: "file:///a.go", Cursor: term.Coordinates{X: 20, Y: 15}}))
	assert.True(t, shouldRecord(base, location{URI: "file:///a.go", Cursor: term.Coordinates{X: 1, Y: 25}}))
	assert.True(t, shouldRecord(base, location{URI: "file:///b.go", Cursor: term.Coordinates{X: 1, Y: 11}}))
}

func TestHistoryDocumentRecordPrevNextAndBranch(t *testing.T) {
	ws, err := workspaceapi.ParseURI("file:///workspace")
	require.NoError(t, err)
	doc := newHistoryDocument(ws)
	a := location{URI: "file:///a.go", Cursor: term.Coordinates{Y: 1}}
	b := location{URI: "file:///b.go", Cursor: term.Coordinates{Y: 2}}
	c := location{URI: "file:///c.go", Cursor: term.Coordinates{Y: 3}}
	d := location{URI: "file:///d.go", Cursor: term.Coordinates{Y: 4}}

	assert.True(t, doc.record(a))
	assert.True(t, doc.record(b))
	assert.True(t, doc.record(c))
	require.Len(t, doc.Entries, 3)
	assert.Equal(t, 2, doc.Index)

	prev, err := doc.prev()
	require.NoError(t, err)
	assert.Equal(t, b, prev)
	assert.Equal(t, 1, doc.Index)

	next, err := doc.next()
	require.NoError(t, err)
	assert.Equal(t, c, next)

	_, err = doc.prev()
	require.NoError(t, err)
	assert.True(t, doc.record(d))
	assert.Equal(t, []location{a, b, d}, doc.Entries)
	assert.Equal(t, 2, doc.Index)
}
