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
	"os"
	"path/filepath"
	"strings"
)

// AttachmentKind distinguishes what an Attachment points at.
type AttachmentKind int

// AttachmentKey identifies a pending attachment within the current
// draft. Inline references in the compose text point at a key rather
// than at a strip index, so removing an earlier chip does not silently
// re-target them. The zero value means "not allocated yet".
type AttachmentKey int64

const (
	// AttachmentFile is a file read from disk or the workspace.
	AttachmentFile AttachmentKind = iota
	// AttachmentSymbol is a workspace symbol expanded into definition,
	// reference and documentation context when the message is sent.
	AttachmentSymbol
)

// Attachment is a file queued to be sent along with the next user
// message. Attachments are created by dropping or pasting file paths
// into the chat and are rendered as a strip of tabs above the compose
// input until they are removed or sent.
type Attachment struct {
	// Path is the file path as resolved when the attachment was created.
	Path string
	// Name is the label rendered in the attachment strip.
	Name string
	// Label is the untruncated name written into the compose text when
	// the attachment is accepted from the '#' completion list. Name is
	// truncated to keep the strip readable, which would make the inline
	// text ambiguous.
	Label string
	// IsImage reports whether the file is sent to the model as visual
	// content rather than as text.
	IsImage bool
	// Icon is the glyph rendered before Name.
	Icon rune
	// ID identifies a virtual attachment that is not backed by a file.
	// Attachments sharing a non-empty ID replace one another instead of
	// stacking up in the strip. It is empty for file attachments.
	ID string
	// Kind selects how the attachment is expanded into content parts.
	Kind AttachmentKind
	// Symbol is the workspace symbol name when Kind is AttachmentSymbol.
	Symbol string
	// Content is the inline text a non-file attachment contributes to the
	// next user message: a virtual attachment's body when ID is set, or
	// an already-resolved symbol snapshot when Kind is AttachmentSymbol.
	// It is empty for file attachments and for name-based '#' symbol
	// attachments, which are resolved when sent.
	Content string
	// Key is this attachment's draft-local identity, allocated by
	// Component. It is UI state: it is neither shown to the user nor
	// persisted.
	Key AttachmentKey
}

const (
	imageAttachmentIcon = '🎆'
	fileAttachmentIcon  = '📎'
	// WorkspaceFileIcon marks a file picked from the '#' completion
	// list, keeping it visually distinct from a dragged or pasted file.
	WorkspaceFileIcon = '\uf15b'
	// SymbolIcon marks a workspace symbol picked from the '#'
	// completion list.
	SymbolIcon = '\U000F0295'
	// removeAttachmentIcon is the affordance for dropping a pending
	// attachment, rendered at the right edge of its tab.
	removeAttachmentIcon = 'ˣ'
	// maxAttachmentNameLen caps the attachment label so a handful of
	// attachments still fit in the strip without shrinking each other
	// into single graphemes.
	maxAttachmentNameLen = 16
)

// NewAttachment builds an Attachment for the file at path.
func NewAttachment(path string) Attachment {
	label := filepath.Base(path)
	_, isImage := imageMediaType(path)
	icon := fileAttachmentIcon
	if isImage {
		icon = imageAttachmentIcon
	}
	return Attachment{
		Path:    path,
		Name:    truncateAttachmentName(label),
		Label:   label,
		IsImage: isImage,
		Icon:    icon,
	}
}

// NewWorkspaceFileAttachment builds an Attachment for a workspace-relative
// path picked from the '#' completion list.
func NewWorkspaceFileAttachment(relpath string) Attachment {
	label := filepath.Base(relpath)
	_, isImage := imageMediaType(relpath)
	icon := WorkspaceFileIcon
	if isImage {
		icon = imageAttachmentIcon
	}
	return Attachment{
		Path:    relpath,
		Name:    truncateAttachmentName(label),
		Label:   label,
		IsImage: isImage,
		Icon:    icon,
	}
}

// NewSymbolAttachment builds an Attachment for a workspace symbol picked
// from the '#' completion list.
func NewSymbolAttachment(name string) Attachment {
	return Attachment{
		Name:   truncateAttachmentName(name),
		Label:  name,
		Icon:   SymbolIcon,
		Kind:   AttachmentSymbol,
		Symbol: name,
	}
}

// NewResolvedSymbolAttachment builds a symbol attachment whose semantic
// context was resolved from an exact document position.
func NewResolvedSymbolAttachment(name, content string) Attachment {
	a := NewSymbolAttachment(name)
	a.Content = content
	return a
}

func truncateAttachmentName(name string) string {
	if r := []rune(name); len(r) > maxAttachmentNameLen {
		return string(r[:maxAttachmentNameLen-1]) + "…"
	}
	return name
}

// imageMediaType reports whether path names an image the model can see.
// It mirrors the extension set accepted by the read_file tool.
func imageMediaType(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png", true
	case ".jpg", ".jpeg":
		return "image/jpeg", true
	case ".gif":
		return "image/gif", true
	case ".webp":
		return "image/webp", true
	default:
		return "", false
	}
}

// pastedFilePaths interprets pasted text as a list of file paths. It
// reports false unless every candidate resolves to an existing regular
// file, in which case the paste is ordinary text and must be inserted
// into the compose input verbatim.
func pastedFilePaths(text string) ([]string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, false
	}
	var out []string
	for line := range strings.SplitSeq(trimmed, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" {
			continue
		}
		if p, ok := statFilePath(line); ok {
			out = append(out, p)
			continue
		}
		fields, ok := splitEscapedPaths(line)
		if !ok || len(fields) < 2 {
			return nil, false
		}
		for _, f := range fields {
			p, ok := statFilePath(f)
			if !ok {
				return nil, false
			}
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// statFilePath resolves candidate to an existing regular file. A bare
// name without any path separator is rejected so pasting a word that
// happens to match a file in the working directory stays text.
func statFilePath(candidate string) (string, bool) {
	if !strings.ContainsRune(candidate, filepath.Separator) &&
		!strings.HasPrefix(candidate, "~") {
		return "", false
	}
	path := expandHome(candidate)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return "", false
	}
	return path, true
}

func expandHome(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

// splitEscapedPaths splits a line of shell-style paths on unescaped
// spaces, honouring backslash escapes and quotes the way a terminal
// writes them when files are dropped onto it. It reports false when the
// line has unbalanced quotes or a dangling escape.
func splitEscapedPaths(line string) ([]string, bool) {
	var (
		out   []string
		cur   strings.Builder
		quote rune
		any   bool
	)
	flush := func() {
		if any {
			out = append(out, cur.String())
			cur.Reset()
			any = false
		}
	}
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		switch {
		case c == '\\' && quote != '\'':
			if i+1 >= len(runes) {
				return nil, false
			}
			i++
			cur.WriteRune(runes[i])
			any = true
		case quote != 0:
			if c == quote {
				quote = 0
				break
			}
			cur.WriteRune(c)
		case c == '\'' || c == '"':
			quote = c
			any = true
		case c == ' ' || c == '\t':
			flush()
		default:
			cur.WriteRune(c)
			any = true
		}
	}
	if quote != 0 {
		return nil, false
	}
	flush()
	return out, true
}
