// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2017-2026 Unstable Build, All Rights Reserved.
//
// NOTICE: All information contained herein is, and remains the property of COMPANY.
// The intellectual and technical concepts contained herein are proprietary to
// COMPANY and may be covered by U.S. and Foreign Patents, patents in process,
// and are protected by trade secret or copyright law. Dissemination of this information
// or reproduction of this material is strictly forbidden unless prior written permission
// is obtained from COMPANY. Access to the source code contained herein is hereby
// forbidden to anyone except current COMPANY employees, managers or contractors who
// have executed Confidentiality and Non-disclosure agreements explicitly covering such access.
//
// The copyright notice above does not evidence any actual or intended publication or
// disclosure of this source code, which includes information that is confidential and/or
// proprietary, and is a trade secret, of COMPANY. ANY REPRODUCTION, MODIFICATION,
// DISTRIBUTION, PUBLIC  PERFORMANCE, OR PUBLIC DISPLAY OF OR THROUGH USE OF THIS SOURCE CODE
// WITHOUT  THE EXPRESS WRITTEN CONSENT OF COMPANY IS STRICTLY PROHIBITED, AND IN
// VIOLATION OF APPLICABLE LAWS AND INTERNATIONAL TREATIES. THE RECEIPT OR POSSESSION OF
// THIS SOURCE CODE AND/OR RELATED INFORMATION DOES NOT CONVEY OR IMPLY ANY RIGHTS TO
// REPRODUCE, DISCLOSE OR DISTRIBUTE ITS CONTENTS, OR TO MANUFACTURE, USE, OR SELL
// ANYTHING THAT IT MAY DESCRIBE, IN WHOLE OR IN PART.

package text

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/go-tui/cell"
	"unstable.build/go-tui/component"
)

func TestExternalEditorPreservesLogicalCursor(t *testing.T) {
	tests := []struct {
		name       string
		content    string
		cursor     term.Coordinates
		start      term.Coordinates
		end        term.Coordinates
		insert     string
		want       string
		wantCursor term.Coordinates
	}{
		{
			name: "same-line insert before cursor", content: "abcdef",
			cursor: term.Coordinates{X: 5}, start: term.Coordinates{X: 1},
			end: term.Coordinates{X: 1}, insert: "Z", want: "aZbcdef",
			wantCursor: term.Coordinates{X: 6},
		},
		{
			name: "same-line insert at cursor", content: "abcdef",
			cursor: term.Coordinates{X: 5}, start: term.Coordinates{X: 5},
			end: term.Coordinates{X: 5}, insert: "Z", want: "abcdeZf",
			wantCursor: term.Coordinates{X: 6},
		},
		{
			name: "same-line insert after cursor", content: "abcdef",
			cursor: term.Coordinates{X: 3}, start: term.Coordinates{X: 5},
			end: term.Coordinates{X: 5}, insert: "Z", want: "abcdeZf",
			wantCursor: term.Coordinates{X: 3},
		},
		{
			name: "same-line delete before cursor", content: "abcdef",
			cursor: term.Coordinates{X: 5}, start: term.Coordinates{X: 1},
			end: term.Coordinates{X: 3}, want: "adef",
			wantCursor: term.Coordinates{X: 3},
		},
		{
			name: "same-line delete ending at cursor", content: "abcdef",
			cursor: term.Coordinates{X: 5}, start: term.Coordinates{X: 3},
			end: term.Coordinates{X: 5}, want: "abcf",
			wantCursor: term.Coordinates{X: 3},
		},
		{
			name: "same-line delete through cursor", content: "abcdef",
			cursor: term.Coordinates{X: 4}, start: term.Coordinates{X: 2},
			end: term.Coordinates{X: 6}, want: "ab",
			wantCursor: term.Coordinates{X: 2},
		},
		{
			name: "same-line delete after cursor", content: "abcdef",
			cursor: term.Coordinates{X: 2}, start: term.Coordinates{X: 4},
			end: term.Coordinates{X: 6}, want: "abcd",
			wantCursor: term.Coordinates{X: 2},
		},
		{
			name: "same-line shorter replacement before cursor", content: "abcdef",
			cursor: term.Coordinates{X: 5}, start: term.Coordinates{X: 1},
			end: term.Coordinates{X: 4}, insert: "Z", want: "aZef",
			wantCursor: term.Coordinates{X: 3},
		},
		{
			name: "same-line longer replacement before cursor", content: "abcdef",
			cursor: term.Coordinates{X: 5}, start: term.Coordinates{X: 1},
			end: term.Coordinates{X: 2}, insert: "WXYZ", want: "aWXYZcdef",
			wantCursor: term.Coordinates{X: 8},
		},
		{
			name: "same-line replacement through cursor", content: "abcdef",
			cursor: term.Coordinates{X: 4}, start: term.Coordinates{X: 2},
			end: term.Coordinates{X: 5}, insert: "Q", want: "abQf",
			wantCursor: term.Coordinates{X: 3},
		},
		{
			name: "multiline insert above cursor", content: "aa\nbb\ncc\ndd",
			cursor: term.Coordinates{Y: 2, X: 1}, start: term.Coordinates{},
			end: term.Coordinates{}, insert: "x\ny", want: "x\nyaa\nbb\ncc\ndd",
			wantCursor: term.Coordinates{Y: 3, X: 1},
		},
		{
			name: "multiline insert below cursor", content: "aa\nbb\ncc\ndd",
			cursor: term.Coordinates{Y: 1, X: 1}, start: term.Coordinates{Y: 3, X: 2},
			end: term.Coordinates{Y: 3, X: 2}, insert: "\nee", want: "aa\nbb\ncc\ndd\nee",
			wantCursor: term.Coordinates{Y: 1, X: 1},
		},
		{
			name: "multiline insert before cursor on same row", content: "abcdef",
			cursor: term.Coordinates{X: 5}, start: term.Coordinates{X: 1},
			end: term.Coordinates{X: 1}, insert: "X\nYZ", want: "aX\nYZbcdef",
			wantCursor: term.Coordinates{Y: 1, X: 2},
		},
		{
			name: "multiline delete above cursor", content: "aa\nbb\ncc\ndd",
			cursor: term.Coordinates{Y: 3, X: 1}, start: term.Coordinates{},
			end: term.Coordinates{Y: 2}, want: "cc\ndd",
			wantCursor: term.Coordinates{Y: 1, X: 1},
		},
		{
			name: "multiline delete below cursor", content: "aa\nbb\ncc\ndd",
			cursor: term.Coordinates{Y: 1, X: 1}, start: term.Coordinates{Y: 2},
			end: term.Coordinates{Y: 3}, want: "aa\nbb\ndd",
			wantCursor: term.Coordinates{Y: 1, X: 1},
		},
		{
			name: "multiline delete ending before cursor on same row", content: "aa\nbb\ncc\ndd",
			cursor: term.Coordinates{Y: 2, X: 2}, start: term.Coordinates{X: 1},
			end: term.Coordinates{Y: 2, X: 1}, want: "ac\ndd",
			wantCursor: term.Coordinates{X: 2},
		},
		{
			name: "multiline delete through cursor", content: "aa\nbb\ncc\ndd",
			cursor: term.Coordinates{Y: 1, X: 1}, start: term.Coordinates{X: 1},
			end: term.Coordinates{Y: 2, X: 2}, want: "a\ndd",
			wantCursor: term.Coordinates{X: 1},
		},
		{
			name: "multiline replacement above cursor", content: "aa\nbb\ncc\ndd",
			cursor: term.Coordinates{Y: 3, X: 1}, start: term.Coordinates{},
			end: term.Coordinates{Y: 2}, insert: "X\nY", want: "X\nYcc\ndd",
			wantCursor: term.Coordinates{Y: 2, X: 1},
		},
		{
			name: "reversed delete through cursor", content: "aa\nbb\ncc",
			cursor: term.Coordinates{Y: 1, X: 1}, start: term.Coordinates{Y: 2, X: 1},
			end: term.Coordinates{X: 1}, want: "ac",
			wantCursor: term.Coordinates{X: 1},
		},
		{
			name: "delete whole buffer", content: "aa\nbb\ncc",
			cursor: term.Coordinates{Y: 1, X: 1}, start: term.Coordinates{},
			end: term.Coordinates{Y: 2, X: 2}, want: "",
			wantCursor: term.Coordinates{},
		},
		{
			name: "insert into empty buffer", content: "",
			cursor: term.Coordinates{}, start: term.Coordinates{},
			end: term.Coordinates{}, insert: "x", want: "x",
			wantCursor: term.Coordinates{X: 1},
		},
		{
			name: "rejected negative edit", content: "abc",
			cursor: term.Coordinates{X: 2}, start: term.Coordinates{X: -1},
			end: term.Coordinates{X: 2}, insert: "x", want: "abc",
			wantCursor: term.Coordinates{X: 2},
		},
		{
			name: "no-op edit", content: "abc",
			cursor: term.Coordinates{X: 2}, start: term.Coordinates{X: 1},
			end: term.Coordinates{X: 1}, want: "abc",
			wantCursor: term.Coordinates{X: 2},
		},
		{
			name: "same-line delete clamps past end", content: "abcdef",
			cursor: term.Coordinates{X: 5}, start: term.Coordinates{X: 2},
			end: term.Coordinates{X: 100}, want: "ab",
			wantCursor: term.Coordinates{X: 2},
		},
		{
			name: "multiline delete clamps past end", content: "aa\nbb\ncc",
			cursor: term.Coordinates{Y: 2, X: 1}, start: term.Coordinates{X: 1},
			end: term.Coordinates{Y: 100, X: 100}, want: "a",
			wantCursor: term.Coordinates{X: 1},
		},
		{
			name: "insert before stale column clamps result", content: "abc",
			cursor: term.Coordinates{X: 20}, start: term.Coordinates{},
			end: term.Coordinates{}, insert: "x", want: "xabc",
			wantCursor: term.Coordinates{X: 4},
		},
		{
			name: "insert before stale row clamps result", content: "aa\nbb",
			cursor: term.Coordinates{Y: 20, X: 20}, start: term.Coordinates{},
			end: term.Coordinates{}, insert: "x", want: "xaa\nbb",
			wantCursor: term.Coordinates{Y: 1, X: 2},
		},
		{
			name: "unicode insert before cursor", content: "a🔥界z",
			cursor: term.Coordinates{X: 3}, start: term.Coordinates{X: 1},
			end: term.Coordinates{X: 1}, insert: "é", want: "aé🔥界z",
			wantCursor: term.Coordinates{X: 4},
		},
		{
			name: "unicode delete before cursor", content: "a🔥界z",
			cursor: term.Coordinates{X: 3}, start: term.Coordinates{X: 1},
			end: term.Coordinates{X: 2}, want: "a界z",
			wantCursor: term.Coordinates{X: 2},
		},
		{
			name: "combining grapheme delete before cursor", content: "aéz",
			cursor: term.Coordinates{X: 2}, start: term.Coordinates{X: 1},
			end: term.Coordinates{X: 2}, want: "az",
			wantCursor: term.Coordinates{X: 1},
		},
		{
			name: "tab insert before cursor", content: "a\tb",
			cursor: term.Coordinates{X: 2}, start: term.Coordinates{},
			end: term.Coordinates{}, insert: "x", want: "xa\tb",
			wantCursor: term.Coordinates{X: 3},
		},
		{
			name: "delete blank rows above cursor", content: "aa\n\n\nbb",
			cursor: term.Coordinates{Y: 3, X: 1}, start: term.Coordinates{Y: 1},
			end: term.Coordinates{Y: 3}, want: "aa\nbb",
			wantCursor: term.Coordinates{Y: 1, X: 1},
		},
	}

	viewports := []struct {
		name           string
		width, height  int
		inverted, wrap bool
	}{
		{name: "ordinary", width: 10, height: 2},
		{name: "ordinary-unsized"},
		{name: "ordinary-tiny", width: 1, height: 1},
		{name: "ordinary-wrapped", width: 2, height: 2, wrap: true},
		{name: "bottom-anchored", width: 10, height: 2, inverted: true},
		{name: "bottom-anchored-unsized", inverted: true},
		{name: "bottom-anchored-tiny", width: 1, height: 1, inverted: true},
	}

	for _, test := range tests {
		for _, viewport := range viewports {
			t.Run(test.name+"/"+viewport.name, func(t *testing.T) {
				cursor, buf := newExternalEditorTestCursor(t, test.content, test.cursor, viewport)
				editor := ExternalEditor(cursor, buf.Editor())
				baseline := cell.NewBuffer()
				_, _, _ = baseline.Edit(context.Background(), term.Coordinates{}, term.Coordinates{}, test.content)
				wantFrom, wantTo, wantOld := baseline.Editor().Edit(
					context.Background(), test.start, test.end, test.insert)

				var from, to term.Coordinates
				var old string
				assert.NotPanics(t, func() {
					from, to, old = editor.Edit(context.Background(), test.start, test.end, test.insert)
				})
				assert.Equal(t, wantFrom, from)
				assert.Equal(t, wantTo, to)
				assert.Equal(t, wantOld, old)
				assert.Equal(t, baseline.String(), buf.String())
				assert.Equal(t, test.want, buf.String())
				assert.Equal(t, test.wantCursor, cursor.CursorAtScroll())
				assertCursorInBufferBounds(t, cursor.CursorAtScroll(), buf)
			})
		}
	}
}

func TestExternalEditorPreservesCursorAcrossNullCells(t *testing.T) {
	buf := new(cell.Buffer)
	buf.InitPerformance(1, 8, 0)
	_, ok := buf.ExtendRowToWidth(0, 6)
	require.True(t, ok)

	scroll := new(component.Scroll)
	scroll.InitPerformance(buf)
	scroll.InvertOffset = true
	scroll.Resize(3, 1)
	cursor := new(Cursor)
	cursor.InitPerformance(scroll)
	cursor.SetCursorAtScroll(term.Coordinates{X: 5})

	from, to, old := ExternalEditor(cursor, buf.Editor()).Edit(
		context.Background(), term.Coordinates{X: 1}, term.Coordinates{X: 3}, "")

	assert.Equal(t, term.Coordinates{X: 1}, from)
	assert.Equal(t, term.Coordinates{X: 1}, to)
	assert.Equal(t, "\x00\x00", old)
	assert.Equal(t, 4, buf.Columns(0))
	assert.Equal(t, term.Coordinates{X: 3}, cursor.CursorAtScroll())
	assertCursorInBufferBounds(t, cursor.CursorAtScroll(), buf)
}

func newExternalEditorTestCursor(
	t *testing.T, content string, at term.Coordinates,
	viewport struct {
		name           string
		width, height  int
		inverted, wrap bool
	},
) (*Cursor, *cell.Buffer) {
	t.Helper()
	buf := new(cell.Buffer)
	if viewport.inverted {
		buf.InitPerformance(1, 1, 0)
	} else {
		buf.Init()
	}
	_, _, _ = buf.Edit(context.Background(), term.Coordinates{}, term.Coordinates{}, content)

	scroll := new(component.Scroll)
	if viewport.inverted {
		scroll.InitPerformance(buf)
	} else {
		scroll.Init(buf)
	}
	scroll.InvertOffset = viewport.inverted
	scroll.Wrap = viewport.wrap
	scroll.SetTabspaces(1)
	scroll.Resize(viewport.width, viewport.height)
	if viewport.wrap && viewport.width > 0 {
		scroll.RecalculateWraps()
	}

	cursor := new(Cursor)
	if viewport.inverted {
		cursor.InitPerformance(scroll)
	} else {
		cursor.Init(scroll, func(fn func()) bool { fn(); return true })
	}
	cursor.SetCursorAtScroll(at)
	require.Equal(t, at, cursor.CursorAtScroll())
	return cursor, buf
}

func assertCursorInBufferBounds(t *testing.T, at term.Coordinates, buf *cell.Buffer) {
	t.Helper()
	require.Greater(t, buf.Rows(), 0)
	assert.GreaterOrEqual(t, at.Y, 0)
	assert.Less(t, at.Y, buf.Rows())
	assert.GreaterOrEqual(t, at.X, 0)
	if at.Y >= 0 && at.Y < buf.Rows() {
		assert.LessOrEqual(t, at.X, buf.Columns(at.Y))
	}
}
