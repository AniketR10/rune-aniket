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

package dialoguetui

import (
	"context"
	"slices"

	"github.com/unstablebuild/rune-go-sdk/term"
	"unstable.build/rune/cell"
)

// InlineAttachmentLink binds a range of the compose text to a pending
// attachment, so an accepted '#' completion stays readable inline while
// still being resolvable to the content that will be sent.
//
// Start and End are byte offsets into the draft text and form a
// half-open range. Offsets are used rather than buffer coordinates
// because the draft travels through the queue, the recall history and
// the submitted message as plain text.
type InlineAttachmentLink struct {
	Key        AttachmentKey
	Start, End int
}

// Draft is the complete compose state: what the user sees, the pending
// attachments, and the links between them.
type Draft struct {
	Text        string
	Attachments []Attachment
	Links       []InlineAttachmentLink
}

// Empty reports whether there is nothing to submit. An attachment-only
// draft is not empty.
func (d Draft) Empty() bool {
	return d.Text == "" && len(d.Attachments) == 0
}

// Clone deep-copies d so the copy can outlive further compose edits.
func (d Draft) Clone() Draft {
	return Draft{
		Text:        d.Text,
		Attachments: slices.Clone(d.Attachments),
		Links:       slices.Clone(d.Links),
	}
}

// Equal reports whether d and other describe the same draft.
func (d Draft) Equal(other Draft) bool {
	return d.Text == other.Text &&
		slices.Equal(d.Attachments, other.Attachments) &&
		slices.Equal(d.Links, other.Links)
}

// cellBytes returns how many bytes c contributes to the buffer's string
// form. A grapheme cluster is one cell but several runes.
func cellBytes(c term.Cell) int {
	n := len(string(c.Ch))
	for _, r := range c.CombiningRunes() {
		n += len(string(r))
	}
	return n
}

// textOffset converts a buffer coordinate into a byte offset into the
// buffer's string form. Coordinates past the end of a row or of the
// buffer clamp to the end of that row or buffer.
func textOffset(v cell.View, pos term.Coordinates) int {
	rows := v.RawCells()
	off := 0
	for y, row := range rows {
		if y == pos.Y {
			for x := 0; x < pos.X && x < len(row); x++ {
				off += cellBytes(row[x])
			}
			return off
		}
		for _, c := range row {
			off += cellBytes(c)
		}
		off++ // row separator
	}
	return off
}

// textCoordinates is the inverse of textOffset.
func textCoordinates(v cell.View, off int) term.Coordinates {
	rows := v.RawCells()
	at := 0
	for y, row := range rows {
		for x, c := range row {
			if at >= off {
				return term.Coordinates{Y: y, X: x}
			}
			at += cellBytes(c)
		}
		if at >= off {
			return term.Coordinates{Y: y, X: len(row)}
		}
		at++ // row separator
	}
	if n := len(rows); n > 0 {
		return term.Coordinates{Y: n - 1, X: len(rows[n-1])}
	}
	return term.Coordinates{}
}

// trackedLink is a live link: the label is retained so a range can be
// validated against the text it is supposed to cover before it is
// exposed or styled.
type trackedLink struct {
	key        AttachmentKey
	start, end int
	label      string
}

// linkTracker follows inline link ranges across every edit published by
// the compose buffer, including undo and redo.
//
// It is a cell.Subscriber and therefore must never write to the buffer
// or the editor: it only records geometry. Styling is refreshed by the
// input after the edit has returned.
type linkTracker struct {
	// view is set to the *cell.Buffer rather than to the view it holds
	// right now, because an editor may install a different view on the
	// buffer after the input was built.
	view  cell.View
	links []trackedLink
	// del is the range about to be replaced, in pre-edit offsets.
	delStart, delEnd int
}

func (t *linkTracker) OnWillEdit(
	_ context.Context, start, end term.Coordinates, _ string,
) {
	t.delStart = textOffset(t.view, start)
	t.delEnd = textOffset(t.view, end)
}

// OnDidEdit shifts links the edit did not touch and drops the ones it
// overlapped. An edit that ends at a link's start counts as before it
// and shifts it; one that starts at a link's end counts as after it and
// leaves it alone. Neither grows the link.
func (t *linkTracker) OnDidEdit(
	_ context.Context, from, to term.Coordinates, _ string,
) {
	if len(t.links) == 0 {
		return
	}
	inserted := textOffset(t.view, to) - textOffset(t.view, from)
	delta := inserted - (t.delEnd - t.delStart)
	kept := t.links[:0]
	for _, l := range t.links {
		switch {
		case l.end <= t.delStart: // link ends before the edit: unmoved
		case l.start >= t.delEnd:
			l.start += delta
			l.end += delta
		default:
			continue
		}
		kept = append(kept, l)
	}
	t.links = kept
}

// add records a new link. Callers pass post-edit offsets.
func (t *linkTracker) add(key AttachmentKey, label string, start, end int) {
	if key == 0 || start >= end {
		return
	}
	t.links = append(t.links, trackedLink{
		key: key, start: start, end: end, label: label,
	})
	t.sort()
}

func (t *linkTracker) unlink(key AttachmentKey) {
	t.links = slices.DeleteFunc(t.links, func(l trackedLink) bool {
		return l.key == key
	})
}

func (t *linkTracker) reset() {
	t.links = t.links[:0]
}

func (t *linkTracker) sort() {
	slices.SortFunc(t.links, func(a, b trackedLink) int {
		return a.start - b.start
	})
}

// prune drops links that no longer cover their label. Edits reach the
// tracker through the buffer publisher, but a buffer can also be reset
// without publishing, so a range is only trusted while the text it was
// created from is still there.
func (t *linkTracker) prune(text string) {
	t.links = slices.DeleteFunc(t.links, func(l trackedLink) bool {
		return l.start < 0 || l.end > len(text) || l.start >= l.end ||
			text[l.start:l.end] != l.label
	})
}

func (t *linkTracker) snapshot() []InlineAttachmentLink {
	if len(t.links) == 0 {
		return nil
	}
	out := make([]InlineAttachmentLink, 0, len(t.links))
	for _, l := range t.links {
		out = append(out, InlineAttachmentLink{
			Key: l.key, Start: l.start, End: l.end,
		})
	}
	return out
}
