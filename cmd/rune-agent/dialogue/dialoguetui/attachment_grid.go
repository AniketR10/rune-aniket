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
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/component"
	"github.com/unstablebuild/rune-go-sdk/term"
	"github.com/unstablebuild/rune-go-sdk/term/graphemecluster"
)

// attachmentChipGap is the blank column separating adjacent chips.
const attachmentChipGap = 1

// attachmentGrid renders the attachments sent with a user message as a
// grid of chips above the message body, so the context that went with a
// turn stays visible in the transcript. Chips wrap onto further rows
// when they do not fit the available width.
type attachmentGrid struct {
	attachments []Attachment
	attr        term.Attributes
	hoverAttr   term.Attributes
	width       int
	rows        int
	slots       []chipSlot
	hovered     int
}

type chipSlot struct {
	x, y, width int
}

// sentAttachmentGrid pairs a rendered chip grid with the list node and
// span that position it, which is what lets a mouse position in the
// messages area be resolved back to a chip.
type sentAttachmentGrid struct {
	node component.ListNode
	span *component.Span
	grid *attachmentGrid
}

func (e sentAttachmentGrid) origin() term.Coordinates {
	return term.CoordinatesSum(e.node.Position(), e.span.ContentOffset())
}

// handleAttachmentChip resolves a mouse event over the messages area
// against the attachment chips rendered above sent messages. Bare
// motion (no button) only arrives from the GUI backend, where it drives
// the hover highlight; terminals that report buttons only still get the
// click behavior.
func (s *dialogueHandler) handleAttachmentChip(
	ev term.Event, dragging bool,
) (exit, handled bool) {
	if s.attachmentOpener == nil || dragging {
		return false, false
	}
	offset := s.comp.MessagesPosition()
	pos := term.Coordinates{X: ev.MouseX - offset.X, Y: ev.MouseY - offset.Y}
	switch ev.Key {
	case 0:
		// Reporting the hover as handled is what schedules the redraw;
		// publishing an interrupt here would do it while holding s.mu.
		return false, s.comp.HoverSentAttachment(pos)
	case term.MouseLeft:
		a, ok := s.comp.SentAttachmentAt(pos)
		if !ok {
			return false, false
		}
		opener := s.attachmentOpener
		s.mu.Unlock()
		opener.OpenAttachment(a)
		s.mu.Lock()
		return false, true
	}
	return false, false
}

var _ component.Responsive = (*attachmentGrid)(nil)

func newAttachmentGrid(atts []Attachment, attr term.Attributes) *attachmentGrid {
	hover := attr
	hover.Bg = term.ColorGray
	return &attachmentGrid{
		attachments: atts,
		attr:        attr,
		hoverAttr:   hover,
		hovered:     -1,
		width:       -1,
	}
}

func chipLabel(a Attachment) string {
	if a.Icon == 0 {
		return " " + a.Name + " "
	}
	return " " + string(a.Icon) + " " + a.Name + " "
}

func (g *attachmentGrid) layout(width int) {
	if width == g.width {
		return
	}
	g.width = width
	g.slots = make([]chipSlot, len(g.attachments))
	x, y := 0, 0
	for i, a := range g.attachments {
		cw := min(graphemecluster.StringWidth(chipLabel(a)), width)
		if x > 0 && x+cw > width {
			x, y = 0, y+1
		}
		g.slots[i] = chipSlot{x: x, y: y, width: cw}
		x += cw + attachmentChipGap
	}
	if len(g.attachments) == 0 {
		g.rows = 0
		return
	}
	g.rows = y + 1
}

// Height satisfies component.Responsive.
func (g *attachmentGrid) Height(width int) int {
	g.layout(width)
	return g.rows
}

// Resize satisfies tui.Component.
func (g *attachmentGrid) Resize(width, height int) {
	g.layout(width)
}

// Draw satisfies tui.Component.
func (g *attachmentGrid) Draw(w term.Writer) {
	for i, slot := range g.slots {
		attr := g.attr
		if i == g.hovered {
			attr = g.hoverAttr
		}
		drawChip(w, slot, chipLabel(g.attachments[i]), attr)
	}
}

// chipAt returns the index of the chip covering pos, which is relative
// to the grid's own origin.
func (g *attachmentGrid) chipAt(pos term.Coordinates) (int, bool) {
	for i, s := range g.slots {
		if pos.Y == s.y && pos.X >= s.x && pos.X < s.x+s.width {
			return i, true
		}
	}
	return -1, false
}

// setHovered marks idx as the chip under the pointer and reports
// whether that changed anything, so callers only redraw when needed.
func (g *attachmentGrid) setHovered(idx int) bool {
	if g.hovered == idx {
		return false
	}
	g.hovered = idx
	return true
}

func drawChip(w term.Writer, slot chipSlot, label string, attr term.Attributes) {
	x, end, state := slot.x, slot.x+slot.width, -1
	for len(label) > 0 && x < end {
		var cluster string
		var width uint8
		cluster, label, width, state = graphemecluster.StepString(label, state)
		r, size := utf8.DecodeRuneInString(cluster)
		cell := term.NewCell(r, width, attr)
		if size < len(cluster) {
			cell.SetCombining([]rune(cluster[size:]))
		}
		w.SetCell(term.Coordinates{X: x, Y: slot.y}, cell)
		x += int(width)
	}
	// Pad so the chip's background reads as one contiguous block even
	// when the label was clipped or is narrower than the slot.
	for ; x < end; x++ {
		w.SetCell(term.Coordinates{X: x, Y: slot.y}, term.NewCell(' ', 1, attr))
	}
}
