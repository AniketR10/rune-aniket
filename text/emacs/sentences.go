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

package emacs

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

// Sentence motion implements the stock GNU rules: a sentence ends at one of
// sentencePunct followed by any number of sentenceClosers, followed by
// end-of-line, a tab, a space at end-of-line or two spaces
// (sentence-end-double-space defaults to on). Motion is bounded by
// paragraphs: blank lines separate them.

func sentencePunct(r rune) bool {
	switch r {
	case '.', '?', '!', '…', '‽':
		return true
	}
	return false
}

func sentenceCloser(r rune) bool {
	switch r {
	case '"', '\'', ')', ']', '}', '”', '’', '»', '›':
		return true
	}
	return false
}

func sentenceSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == 0
}

// sentenceEndAt reports whether a sentence-end pattern begins at pos. On a
// match it returns the sentence end (just after the closers, before the
// whitespace) and the position after the match's trailing whitespace, which
// is where the next sentence starts.
func (h *emacsHandler) sentenceEndAt(pos term.Coordinates) (end, next term.Coordinates, ok bool) {
	r, found := h.docRuneAt(pos)
	if !found || !sentencePunct(r) {
		return end, next, false
	}
	cur := h.docAdvance(pos)
	for {
		r, found = h.docRuneAt(cur)
		if !found || !sentenceCloser(r) {
			break
		}
		cur = h.docAdvance(cur)
	}
	end = cur
	// The punctuation must be followed by end-of-line (or end of buffer),
	// a tab, or spaces reaching a second space or end-of-line.
	r, found = h.docRuneAt(cur)
	switch {
	case !found || r == '\n' || r == '\t':
	case r == ' ':
		r2, found2 := h.docRuneAt(h.docAdvance(cur))
		if found2 && r2 != ' ' && r2 != '\n' {
			return end, next, false
		}
	default:
		return end, next, false
	}
	for {
		r, found = h.docRuneAt(cur)
		if !found || !sentenceSpace(r) {
			break
		}
		cur = h.docAdvance(cur)
	}
	return end, cur, true
}

// paragraphTextStart returns the position of the first non-whitespace
// character of the paragraph containing line y (which must not be blank).
func (h *emacsHandler) paragraphTextStart(y int) term.Coordinates {
	for y > 0 && !h.lineIsBlank(y-1) {
		y--
	}
	pos := term.Coordinates{Y: y}
	for {
		r, ok := h.docRuneAt(pos)
		if !ok || !sentenceSpace(r) || r == '\n' {
			return pos
		}
		pos = h.docAdvance(pos)
	}
}

// forwardSentenceFrom returns the GNU forward-sentence target from pos: the
// end of the current (or next) sentence, stopping at a paragraph end when
// the paragraph runs out of sentence punctuation. found is false when there
// is no position past pos to move to.
func (h *emacsHandler) forwardSentenceFrom(pos term.Coordinates) (term.Coordinates, bool) {
	rows := h.cursor.View().Rows()
	cur := pos
	for {
		r, ok := h.docRuneAt(cur)
		if !ok {
			// End of buffer ends the last sentence.
			return cur, cur != pos
		}
		if r == '\n' && (cur.Y+1 >= rows || h.lineIsBlank(cur.Y+1)) && cur != pos {
			// Paragraph end without sentence punctuation.
			return cur, true
		}
		if end, _, ok := h.sentenceEndAt(cur); ok && end != pos && coordLess(pos, end) {
			return end, true
		}
		cur = h.docAdvance(cur)
	}
}

// backwardSentenceFrom returns the GNU backward-sentence target from pos:
// the start of the sentence point is in (or, from a sentence start, the
// previous one), never crossing more than one paragraph boundary. found is
// false at the beginning of the buffer.
func (h *emacsHandler) backwardSentenceFrom(pos term.Coordinates) (term.Coordinates, bool) {
	prev, ok := h.docRetreat(pos)
	if !ok {
		return pos, false
	}
	// Anchor on the last real character behind point so a position inside
	// (or right after) inter-sentence whitespace searches the paragraph
	// holding the preceding text.
	for {
		r, _ := h.docRuneAt(prev)
		if !sentenceSpace(r) {
			break
		}
		if prev, ok = h.docRetreat(prev); !ok {
			return term.Coordinates{}, pos != (term.Coordinates{})
		}
	}
	parStart := h.paragraphTextStart(prev.Y)
	// Walk the paragraph forward collecting the latest sentence start
	// strictly before pos; sentences start where the previous sentence's
	// trailing whitespace ends.
	target := parStart
	cur := parStart
	for coordLess(cur, pos) {
		if _, next, ok := h.sentenceEndAt(cur); ok && coordLess(next, pos) {
			target = next
		}
		cur = h.docAdvance(cur)
	}
	return target, target != pos
}

// forwardSentence implements M-e.
func (h *emacsHandler) forwardSentence() bool {
	target, ok := h.forwardSentenceFrom(h.cursor.CursorAtScroll())
	if !ok {
		return false
	}
	_, moved := h.cursor.MoveToScroll(target)
	return moved
}

// backwardSentence implements M-a.
func (h *emacsHandler) backwardSentence() bool {
	target, ok := h.backwardSentenceFrom(h.cursor.CursorAtScroll())
	if !ok {
		return false
	}
	_, moved := h.cursor.MoveToScroll(target)
	return moved
}

// killSentence implements M-k: kill from point to the end of the sentence.
func (h *emacsHandler) killSentence() bool {
	point := h.cursor.CursorAtScroll()
	target, ok := h.forwardSentenceFrom(point)
	if !ok || !h.cursor.SelectRange(point, target) {
		return false
	}
	return h.killSelection(false)
}

// killSentencesBackward implements M-k with a negative argument: kill from
// point back to the start of the nth previous sentence.
func (h *emacsHandler) killSentencesBackward(n int) bool {
	point := h.cursor.CursorAtScroll()
	start := point
	for range n {
		target, ok := h.backwardSentenceFrom(start)
		if !ok {
			break
		}
		start = target
	}
	if start == point || !h.cursor.SelectRange(point, start) {
		return false
	}
	return h.killSelection(true)
}
