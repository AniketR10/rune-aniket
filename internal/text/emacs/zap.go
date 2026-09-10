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

package emacs

import (
	"github.com/unstablebuild/rune-go-sdk/term"
)

// startZap begins M-z: the next typed character selects the zap target.
// count picks the occurrence to zap through; a negative count zaps
// backward.
func (h *emacsHandler) startZap(count int) bool {
	h.pendingZap = true
	h.zapCount = count
	h.setTransientMode("ZAP")
	// The whole zap is one command: reading the target character must not
	// break a kill-accumulation chain ending at the M-z keystroke.
	h.killedNow = h.lastKill
	h.less.SetMessage("Zap to char: ")
	return true
}

// handleZapKey consumes the character read after M-z. C-g and non-character
// keys abort; anything else kills from point through and including the next
// occurrence of the character.
func (h *emacsHandler) handleZapKey(ev term.Event) {
	h.pendingZap = false
	h.setTransientMode("")
	if ev.Type != term.EventKey || (ev.Mod == term.ModCtrl && ev.Ch == 'g') {
		h.less.SetMessage("")
		return
	}
	ch := ev.Ch
	switch ev.Key {
	case term.KeyEnter:
		ch = '\n'
	case term.KeySpace:
		ch = ' '
	}
	if ch == 0 || ev.Mod&(term.ModCtrl|term.ModAlt) != 0 {
		h.less.SetMessage("")
		return
	}
	h.less.SetMessage("")
	h.zapToChar(ch)
}

// zapToChar kills from point through the zapCount'th occurrence of ch, GNU
// zap-to-char. The forward scan starts at point, so zapping to the
// character under point kills just that character; a negative count scans
// and kills backward.
func (h *emacsHandler) zapToChar(ch rune) {
	count := h.zapCount
	if count == 0 {
		count = 1
	}
	point := h.cursor.CursorAtScroll()
	if count > 0 {
		cur := point
		for {
			r, ok := h.docRuneAt(cur)
			if !ok {
				h.less.SetMessage("Search failed: %q", string(ch))
				return
			}
			if r == ch {
				if count--; count == 0 {
					break
				}
			}
			cur = h.docAdvance(cur)
		}
		if h.cursor.SelectRange(point, h.docAdvance(cur)) {
			h.killSelection(false)
		}
		return
	}
	cur := point
	for {
		prev, ok := h.docRetreat(cur)
		if !ok {
			h.less.SetMessage("Search failed: %q", string(ch))
			return
		}
		cur = prev
		if r, _ := h.docRuneAt(cur); r == ch {
			if count++; count == 0 {
				break
			}
		}
	}
	if h.cursor.SelectRange(point, cur) {
		h.killSelection(true)
	}
}
