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
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/browserapi"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/component"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/agentools"
	"unstable.build/go-tui/cmd/rune-agent/agent/utf8validate"
	"unstable.build/go-tui/cmd/rune-agent/dialogue/dialoguetui"
	"unstable.build/go-tui/component/markdown"
	"unstable.build/go-tui/debug"
	mdhandler "unstable.build/go-tui/handler/markdown"
)

// Legacy attachment content parts double as the durable record of what
// was attached to a turn, so the chip grid can be rebuilt on replay.
// These prefixes are the contract for messages persisted before the v1
// canonical format below; replay still falls back to them.
const (
	attachedFilePrefix   = "Attached file "
	attachedImagePrefix  = "Attached image "
	attachedSymbolPrefix = "Attached symbol "
)

// attachmentPartKind identifies what a canonical v1 attachment content
// part carries.
type attachmentPartKind string

const (
	attachmentKindFile   attachmentPartKind = "file"
	attachmentKindSymbol attachmentPartKind = "symbol"
	attachmentKindImage  attachmentPartKind = "image"
)

const (
	attachmentStatusOK    = "ok"
	attachmentStatusError = "error"
)

// attachmentPartV1Prefix marks a content part as the canonical v1
// attachment record: a single header line, `attachmentPartV1Prefix` plus
// JSON metadata, optionally followed by a newline and the attachment's
// body.
const attachmentPartV1Prefix = "Rune attachment v1: "

// attachmentPartMeta is the JSON header of a canonical v1 attachment
// content part. It is the durable record of what was attached to a turn,
// so replay reconstructs the chip grid from it instead of a parallel copy
// that could drift from what the model actually received.
type attachmentPartMeta struct {
	ID     string             `json:"id"`
	Kind   attachmentPartKind `json:"kind"`
	Name   string             `json:"name"`
	Path   string             `json:"path,omitempty"`
	Symbol string             `json:"symbol,omitempty"`
	Status string             `json:"status"`
	Error  string             `json:"error,omitempty"`
}

// encodeAttachmentPart renders meta and body as a canonical v1 content
// part. body is omitted entirely when empty, which is how an image's
// header line stays immediately followed by its image content part.
func encodeAttachmentPart(meta attachmentPartMeta, body string) string {
	header, err := json.Marshal(meta)
	if err != nil {
		// meta is built entirely from fields already known to marshal
		// cleanly; degrade rather than lose the attachment record.
		header = []byte("{}")
	}
	if body == "" {
		return attachmentPartV1Prefix + string(header)
	}
	return attachmentPartV1Prefix + string(header) + "\n" + body
}

// parseAttachmentPartV1 parses a canonical v1 content part. It reports
// false for any text that is not v1, including a v1-prefixed header with
// malformed JSON.
func parseAttachmentPartV1(text string) (attachmentPartMeta, string, bool) {
	rest, ok := strings.CutPrefix(text, attachmentPartV1Prefix)
	if !ok {
		return attachmentPartMeta{}, "", false
	}
	head, body, _ := strings.Cut(rest, "\n")
	var meta attachmentPartMeta
	if err := json.Unmarshal([]byte(head), &meta); err != nil {
		return attachmentPartMeta{}, "", false
	}
	return meta, body, true
}

// attachmentFromV1 rebuilds the chip dialoguetui.Attachment a v1 content
// part was rendered from. Status is not consulted: even a failed
// attachment still gets a chip, matching the legacy prefix behavior.
func attachmentFromV1(meta attachmentPartMeta, body string) dialoguetui.Attachment {
	if meta.Kind == attachmentKindSymbol {
		name := meta.Symbol
		if name == "" {
			name = meta.Name
		}
		return dialoguetui.NewResolvedSymbolAttachment(name, body)
	}
	path := meta.Path
	if path == "" {
		path = meta.Name
	}
	if filepath.IsAbs(path) {
		return dialoguetui.NewAttachment(path)
	}
	return dialoguetui.NewWorkspaceFileAttachment(path)
}

// attachmentContentParts reads the files the user attached to the chat
// and renders them as canonical v1 content parts for the next user
// message. A file that cannot be read, or that exceeds the model's image
// caps, becomes a status:error part so the rest of the message still
// goes through; the failure is also surfaced as a notification.
func (h *aiEditorHandler) attachmentContentParts(
	ctx context.Context,
	attachments []finalizedAttachment,
) []llmapi.ContentPart {
	if len(attachments) == 0 {
		return nil
	}
	maxOutput := h.maxToolOutputBytes
	if maxOutput <= 0 {
		maxOutput = agent.DefaultMaxToolOutputBytes
	}
	textPart := func(s string) llmapi.ContentPart {
		return llmapi.ContentPart{Type: llmapi.ContentPartTypeText, Text: s}
	}
	parts := make([]llmapi.ContentPart, 0, len(attachments)+1)
	for _, a := range attachments {
		meta := attachmentPartMeta{ID: a.id, Name: a.Label}
		if a.ID != "" {
			// Virtual attachments (e.g. the /chatreviewchanges comments)
			// carry inline content directly and are never replayed as a
			// chip from persisted content, so they skip the v1 envelope.
			parts = append(parts, textPart(fmt.Sprintf("%s\n%s",
				chatReviewHeading, a.Content)))
			continue
		}
		if a.Kind == dialoguetui.AttachmentSymbol {
			meta.Kind = attachmentKindSymbol
			meta.Symbol = a.Symbol
			meta.Status = attachmentStatusOK
			content := a.Content
			if content == "" {
				content = h.symbolContext(ctx, a.Symbol)
			}
			parts = append(parts, textPart(encodeAttachmentPart(meta, content)))
			continue
		}
		meta.Path = a.Path
		data, err := readWorkspaceFile(h.fs, a.Path)
		if err != nil {
			_, _ = h.n.Notify(browserapi.LevelError,
				"attachment %s: %v", a.Name, err)
			meta.Kind = attachmentKindFile
			meta.Status = attachmentStatusError
			meta.Error = err.Error()
			parts = append(parts, textPart(encodeAttachmentPart(meta,
				fmt.Sprintf("could not be read: %v", err))))
			continue
		}
		if mime, isImage := agentools.ImageMediaType(a.Path); isImage {
			meta.Kind = attachmentKindImage
			uri, encErr := agentools.EncodeImageDataURI(a.Path, data, mime)
			if encErr != nil {
				_, _ = h.n.Notify(browserapi.LevelError,
					"attachment %s: %v", a.Name, encErr)
				meta.Status = attachmentStatusError
				meta.Error = encErr.Error()
				parts = append(parts, textPart(encodeAttachmentPart(meta,
					fmt.Sprintf("could not be sent: %v", encErr))))
				continue
			}
			meta.Status = attachmentStatusOK
			parts = append(parts,
				textPart(encodeAttachmentPart(meta, "")),
				llmapi.ContentPart{
					Type:     llmapi.ContentPartTypeImageURL,
					ImageURL: uri,
				})
			continue
		}
		meta.Kind = attachmentKindFile
		meta.Status = attachmentStatusOK
		if utf8validate.IsBinary(data) {
			parts = append(parts, textPart(encodeAttachmentPart(meta,
				utf8validate.BinaryStub(a.Name, len(data), sha256.Sum256(data)))))
			continue
		}
		parts = append(parts, textPart(encodeAttachmentPart(meta,
			agent.TruncateMiddle(utf8validate.Sanitize(string(data)), maxOutput))))
	}
	return parts
}

// attachmentPreviewer opens attachment previews for one chat.
type attachmentPreviewer struct {
	h *aiEditorHandler
}

func (p attachmentPreviewer) OpenAttachment(a dialoguetui.Attachment) {
	// Resolving a symbol runs three LSP tools, so keep it off the event
	// loop that delivered the click.
	go debug.CapturePanicReport(func() { p.h.openAttachmentPreview(a) })
}

func (h *aiEditorHandler) openAttachmentPreview(a dialoguetui.Attachment) {
	var err error
	if a.Kind == dialoguetui.AttachmentSymbol {
		content := a.Content
		if content == "" {
			content = h.symbolContext(h.ctx, a.Symbol)
		}
		err = h.openSymbolPreview(content)
	} else {
		err = h.openFileAttachment(a.Path)
	}
	if err != nil {
		_, _ = h.n.Notify(browserapi.LevelError,
			"preview %s: %v", a.Name, err)
	}
}

// openFileAttachment opens the attached file as a normal editor tab and
// focuses it, so the user lands on the real buffer rather than a
// throwaway copy.
func (h *aiEditorHandler) openFileAttachment(path string) error {
	uri, err := h.fs.URI(path)
	if err != nil {
		return err
	}
	bh, err := h.o.Open(uri)
	if err != nil {
		return err
	}
	win, err := h.wm.Focus()
	if err != nil {
		return err
	}
	if err := h.wm.SetWindowContent(win, bh); err != nil &&
		!errors.Is(err, browserapi.ErrTabNotFree) {
		return err
	}
	return nil
}

// openSymbolPreview floats the symbol's definition, references and
// documentation. The context is markdown, so it is rendered by the
// markdown handler to keep links, folds and code highlighting.
func (h *aiEditorHandler) openSymbolPreview(content string) error {
	cfg := defaultMarkdownConfig()
	cfg.Parser = h.parser
	md, err := markdown.NewWithConfig(content, *cfg)
	if err != nil {
		return err
	}
	mdh := mdhandler.New(md)
	var win browserapi.Window
	bhandler := browserapi.FuncHandler(mdh, func() error {
		return h.wm.CloseWindow(win)
	})
	floating := browserapi.FuncFloating(bhandler, func() (int, int) {
		return floatingWidth, min(mdh.Height(floatingWidth), floatingHeight)
	})
	win, err = h.wm.Floating(floating, browserapi.FloatingConfig{
		Alignment: component.AlignmentCentered,
	})
	return err
}

// replayedAttachments rebuilds the chip grid for a stored user message.
// The attachment content parts are the durable record, so replay reads
// those instead of a parallel copy that could drift from what the model
// actually received. The first part is the message text itself.
func replayedAttachments(msg llmapi.Message) []dialoguetui.Attachment {
	if len(msg.MultiContent) < 2 {
		return nil
	}
	var out []dialoguetui.Attachment
	for _, p := range msg.MultiContent[1:] {
		if p.Type != llmapi.ContentPartTypeText {
			continue
		}
		if a, ok := parseAttachmentPart(p.Text); ok {
			out = append(out, a)
		}
	}
	return out
}

func parseAttachmentPart(text string) (dialoguetui.Attachment, bool) {
	if meta, body, ok := parseAttachmentPartV1(text); ok {
		return attachmentFromV1(meta, body), true
	}
	return parseLegacyAttachmentPart(text)
}

// parseLegacyAttachmentPart parses the "Attached file/image/symbol "
// prefixed content parts persisted before the v1 canonical format.
func parseLegacyAttachmentPart(text string) (dialoguetui.Attachment, bool) {
	for _, c := range []struct {
		prefix string
		symbol bool
	}{
		{attachedSymbolPrefix, true},
		{attachedFilePrefix, false},
		{attachedImagePrefix, false},
	} {
		rest, ok := strings.CutPrefix(text, c.prefix)
		if !ok {
			continue
		}
		head, _, _ := strings.Cut(rest, "\n")
		name := strings.TrimSuffix(head, ":")
		if i := strings.Index(name, " could not "); i >= 0 {
			name = name[:i]
		}
		if name == "" {
			return dialoguetui.Attachment{}, false
		}
		switch {
		case c.symbol:
			_, body, _ := strings.Cut(text, "\n")
			return dialoguetui.NewResolvedSymbolAttachment(name, body), true
		case filepath.IsAbs(name):
			// Dragged and pasted files were resolved to absolute paths
			// and keep their own icon.
			return dialoguetui.NewAttachment(name), true
		default:
			return dialoguetui.NewWorkspaceFileAttachment(name), true
		}
	}
	return dialoguetui.Attachment{}, false
}
