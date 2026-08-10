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

package extension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
)

func TestFinalizeSubmitMessageAssignsContiguousIDs(t *testing.T) {
	msg := dialoguetui.SubmitMessage{
		Text: "look at main.go and helper.go",
		Attachments: []dialoguetui.Attachment{
			attach(1, "main.go"),
			attach(2, "helper.go"),
		},
		Links: []dialoguetui.InlineAttachmentLink{
			{Key: 1, Start: 8, End: 15},  // "main.go"
			{Key: 2, Start: 20, End: 29}, // "helper.go"
		},
	}

	got := finalizeSubmitMessage(msg)

	assert.Equal(t, "look at main.go and helper.go", got.DisplayText)
	assert.Equal(t, "look at <attachment-1> and <attachment-2>", got.ModelText)
	if assert.Len(t, got.Attachments, 2) {
		assert.Equal(t, "attachment-1", got.Attachments[0].id)
		assert.Equal(t, "attachment-2", got.Attachments[1].id)
	}
}

// attach builds a labeled dialoguetui.Attachment. Accepting a '#'
// completion replaces the trigger and typed filter with the label text
// itself (see textHandlerInput.ReplaceBeforeCursor), so a link's range
// covers exactly the label with no leading '#'.
func attach(key dialoguetui.AttachmentKey, label string) dialoguetui.Attachment {
	a := dialoguetui.NewWorkspaceFileAttachment(label)
	a.Key = key
	a.Label = label
	return a
}

func link(key dialoguetui.AttachmentKey, start, end int) dialoguetui.InlineAttachmentLink {
	return dialoguetui.InlineAttachmentLink{Key: key, Start: start, End: end}
}

func TestFinalizeSubmitMessageRewritesEndToStartWithoutShiftingEarlierOffsets(t *testing.T) {
	// Three links of different lengths so a naive left-to-right rewrite
	// (which would need to track shifting offsets) would misplace the
	// later replacements if it were buggy.
	msg := dialoguetui.SubmitMessage{
		Text: "a bb ccc",
		Attachments: []dialoguetui.Attachment{
			attach(1, "a"), attach(2, "bb"), attach(3, "ccc"),
		},
		Links: []dialoguetui.InlineAttachmentLink{
			link(1, 0, 1), link(2, 2, 4), link(3, 5, 8),
		},
	}

	got := finalizeSubmitMessage(msg)

	assert.Equal(t, "<attachment-1> <attachment-2> <attachment-3>", got.ModelText)
}

func TestFinalizeSubmitMessageInvalidLinksStayLiteral(t *testing.T) {
	tests := []struct {
		name string
		msg  dialoguetui.SubmitMessage
		want string
	}{
		{
			name: "out of bounds start",
			msg: dialoguetui.SubmitMessage{
				Text:        "hi a",
				Attachments: []dialoguetui.Attachment{attach(1, "a")},
				Links:       []dialoguetui.InlineAttachmentLink{link(1, -1, 4)},
			},
			want: "hi a",
		},
		{
			name: "out of bounds end",
			msg: dialoguetui.SubmitMessage{
				Text:        "hi a",
				Attachments: []dialoguetui.Attachment{attach(1, "a")},
				Links:       []dialoguetui.InlineAttachmentLink{link(1, 3, 99)},
			},
			want: "hi a",
		},
		{
			name: "start equals end",
			msg: dialoguetui.SubmitMessage{
				Text:        "hi a",
				Attachments: []dialoguetui.Attachment{attach(1, "a")},
				Links:       []dialoguetui.InlineAttachmentLink{link(1, 3, 3)},
			},
			want: "hi a",
		},
		{
			name: "key does not exist",
			msg: dialoguetui.SubmitMessage{
				Text: "hi a",
				Links: []dialoguetui.InlineAttachmentLink{
					link(1, 3, 5),
				},
			},
			want: "hi a",
		},
		{
			name: "key removed from final attachment slice",
			msg: dialoguetui.SubmitMessage{
				Text:        "hi a and b",
				Attachments: []dialoguetui.Attachment{attach(2, "b")},
				Links: []dialoguetui.InlineAttachmentLink{
					link(1, 3, 4), // a's attachment was removed before send
					link(2, 9, 10),
				},
			},
			want: "hi a and <attachment-1>",
		},
		{
			name: "label text does not match covered range",
			msg: dialoguetui.SubmitMessage{
				Text:        "hi a",
				Attachments: []dialoguetui.Attachment{attach(1, "zzz")},
				Links:       []dialoguetui.InlineAttachmentLink{link(1, 3, 5)},
			},
			want: "hi a",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := finalizeSubmitMessage(tc.msg)
			assert.Equal(t, tc.want, got.ModelText)
			assert.Equal(t, tc.msg.Text, got.DisplayText)
		})
	}
}

func TestFinalizeSubmitMessageOverlappingLinksKeepEarliestStart(t *testing.T) {
	msg := dialoguetui.SubmitMessage{
		Text: "abcdef",
		Attachments: []dialoguetui.Attachment{
			attach(1, "abcd"), attach(2, "cdef"),
		},
		Links: []dialoguetui.InlineAttachmentLink{
			link(2, 2, 6), // starts later
			link(1, 0, 4), // starts first, wins
		},
	}

	got := finalizeSubmitMessage(msg)

	assert.Equal(t, "<attachment-1>ef", got.ModelText)
}

func TestFinalizeSubmitMessageRepeatedLinksMapSameID(t *testing.T) {
	msg := dialoguetui.SubmitMessage{
		Text:        "a and a again",
		Attachments: []dialoguetui.Attachment{attach(1, "a")},
		Links: []dialoguetui.InlineAttachmentLink{
			link(1, 0, 1),
			link(1, 6, 7),
		},
	}

	got := finalizeSubmitMessage(msg)

	assert.Equal(t, "<attachment-1> and <attachment-1> again", got.ModelText)
}

func TestFinalizeSubmitMessageUnreferencedAttachmentsStillGetIDs(t *testing.T) {
	msg := dialoguetui.SubmitMessage{
		Text: "no mentions here",
		Attachments: []dialoguetui.Attachment{
			attach(1, "a.go"), attach(2, "b.go"),
		},
	}

	got := finalizeSubmitMessage(msg)

	assert.Equal(t, "no mentions here", got.ModelText)
	if assert.Len(t, got.Attachments, 2) {
		assert.Equal(t, "attachment-1", got.Attachments[0].id)
		assert.Equal(t, "attachment-2", got.Attachments[1].id)
	}
}

func TestFinalizeSubmitMessageUnicodeMultilineByteOffsets(t *testing.T) {
	// "café " is 6 bytes ("é" is 2 bytes), so the link into the second
	// line only lines up correctly when offsets are treated as bytes.
	text := "café \nrésumé.pdf line two"
	label := "résumé.pdf"
	start := len("café \n")
	end := start + len(label)
	msg := dialoguetui.SubmitMessage{
		Text:        text,
		Attachments: []dialoguetui.Attachment{attach(1, label)},
		Links:       []dialoguetui.InlineAttachmentLink{link(1, start, end)},
	}

	got := finalizeSubmitMessage(msg)

	assert.Equal(t, "café \n<attachment-1> line two", got.ModelText)
}
