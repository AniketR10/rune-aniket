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
