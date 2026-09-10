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

package extension

import (
	"fmt"
	"sort"

	"unstable.build/rune/cmd/rune-agent/dialogue/dialoguetui"
)

// finalizedAttachment pairs a submitted attachment with the turn-local ID
// assigned when the message was finalized.
type finalizedAttachment struct {
	id string
	dialoguetui.Attachment
}

// finalizedMessage is the canonical shape a dialoguetui.SubmitMessage is
// reduced to exactly once, before it reaches the agent or gets persisted.
//
// DisplayText is the text the user composed, unchanged. ModelText is the
// same text with every valid inline attachment link rewritten to its
// placeholder ("<attachment-N>"), so the model sees a stable reference
// instead of chat-UI-only label text. Text that was never linked, or whose
// link no longer matches a live attachment, stays literal in both.
type finalizedMessage struct {
	DisplayText string
	ModelText   string
	Attachments []finalizedAttachment
}

// attachmentPlaceholder is the model-visible reference finalizeSubmitMessage
// substitutes for a valid inline attachment link.
func attachmentPlaceholder(id string) string {
	return "<" + id + ">"
}

// finalizeSubmitMessage assigns contiguous turn-local IDs ("attachment-1",
// "attachment-2", ...) to msg's attachments, in the order they appear in
// msg.Attachments, and rewrites msg.Text's valid inline links into
// placeholders referencing those IDs. Every attachment gets an ID and
// canonical content regardless of whether anything links to it.
func finalizeSubmitMessage(msg dialoguetui.SubmitMessage) finalizedMessage {
	ids := make(map[dialoguetui.AttachmentKey]string, len(msg.Attachments))
	labels := make(map[dialoguetui.AttachmentKey]string, len(msg.Attachments))
	attachments := make([]finalizedAttachment, len(msg.Attachments))
	for i, a := range msg.Attachments {
		id := fmt.Sprintf("attachment-%d", i+1)
		ids[a.Key] = id
		labels[a.Key] = a.Label
		attachments[i] = finalizedAttachment{id: id, Attachment: a}
	}

	modelText := msg.Text
	for _, l := range activeLinks(msg.Text, msg.Links, ids, labels) {
		modelText = modelText[:l.Start] + attachmentPlaceholder(ids[l.Key]) + modelText[l.End:]
	}

	return finalizedMessage{
		DisplayText: msg.Text,
		ModelText:   modelText,
		Attachments: attachments,
	}
}

// activeLinks selects the links that validly reference a live attachment:
// their range is in bounds, their key resolves to an attachment, and the
// text they cover still equals that attachment's label. Overlapping links
// are resolved by keeping the earliest-starting one and dropping the rest,
// so the result is a nonoverlapping set. The links are returned sorted by
// Start descending, so callers can rewrite text in place without the
// earlier replacements shifting later offsets.
func activeLinks(
	text string, links []dialoguetui.InlineAttachmentLink,
	ids, labels map[dialoguetui.AttachmentKey]string,
) []dialoguetui.InlineAttachmentLink {
	valid := make([]dialoguetui.InlineAttachmentLink, 0, len(links))
	for _, l := range links {
		if l.Start < 0 || l.End > len(text) || l.Start >= l.End {
			continue
		}
		if _, ok := ids[l.Key]; !ok {
			continue
		}
		if text[l.Start:l.End] != labels[l.Key] {
			continue
		}
		valid = append(valid, l)
	}
	sort.Slice(valid, func(i, j int) bool { return valid[i].Start < valid[j].Start })

	active := make([]dialoguetui.InlineAttachmentLink, 0, len(valid))
	end := -1
	for _, l := range valid {
		if l.Start < end {
			continue
		}
		active = append(active, l)
		end = l.End
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Start > active[j].Start })
	return active
}
