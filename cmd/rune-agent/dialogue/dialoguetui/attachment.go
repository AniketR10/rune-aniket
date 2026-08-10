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

package dialoguetui

import (
	"os"
	"path/filepath"
	"strings"
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
	// IsImage reports whether the file is sent to the model as visual
	// content rather than as text.
	IsImage bool
	// Icon is the glyph rendered before Name.
	Icon rune
	// ID identifies a virtual attachment that is not backed by a file.
	// Attachments sharing a non-empty ID replace one another instead of
	// stacking up in the strip. It is empty for file attachments.
	ID string
	// Content is the inline text a virtual attachment contributes to the
	// next user message. It is empty for file attachments.
	Content string
}

const (
	imageAttachmentIcon = '🎆'
	fileAttachmentIcon  = '📎'
	// maxAttachmentNameLen caps the attachment label so a handful of
	// attachments still fit in the strip without shrinking each other
	// into single graphemes.
	maxAttachmentNameLen = 16
)

// NewAttachment builds an Attachment for the file at path.
func NewAttachment(path string) Attachment {
	name := filepath.Base(path)
	if r := []rune(name); len(r) > maxAttachmentNameLen {
		name = string(r[:maxAttachmentNameLen-1]) + "…"
	}
	_, isImage := imageMediaType(path)
	icon := fileAttachmentIcon
	if isImage {
		icon = imageAttachmentIcon
	}
	return Attachment{Path: path, Name: name, IsImage: isImage, Icon: icon}
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
