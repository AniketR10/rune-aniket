// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.
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

package agentools

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	_ "golang.org/x/image/webp"
	"unstable.build/go-tui/cmd/rune-agent/agent"
	"unstable.build/go-tui/cmd/rune-agent/agent/utf8validate"
)

// maxImageBytes is the maximum raw file size for image reads. The image
// is base64-encoded into the LLM request, which inflates payload size
// by ~33%, and the extension host's gRPC server enforces the default
// 4 MiB inbound message cap. Capping raw bytes at 3 MiB keeps the
// encoded request comfortably under that ceiling once the rest of the
// message envelope is accounted for.
const maxImageBytes = 3 * 1024 * 1024

// maxImageEdge is Anthropic's per-image dimension cap for many-image
// requests. An image exceeding it on either axis causes the API to
// reject the entire request with a 400, poisoning the conversation.
const maxImageEdge = 2000

type readFileTool struct {
	fs           workspaceapi.FileSystem
	cwd          workspaceapi.URI
	tracker      *FileTracker
	maxLineBytes int
}

type readFileArgs struct {
	Path   string `json:"path"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

func newReadFile(fs workspaceapi.FileSystem, cwd workspaceapi.URI, tracker *FileTracker, maxLineBytes int) agent.Tool {
	if maxLineBytes <= 0 {
		maxLineBytes = agent.DefaultMaxLineBytes
	}
	return &readFileTool{fs: fs, cwd: cwd, tracker: tracker, maxLineBytes: maxLineBytes}
}

func (t *readFileTool) NeedsDeterministicOrder() bool { return false }

func (t *readFileTool) Definition() llmapi.Tool {
	return llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name: "read_file",
			Description: `Read the contents of a file. Returns each line prefixed with its 1-based
line number (e.g. "L1: hello world").

Supports image files (.png, .jpg, .jpeg, .gif, .webp) — the image is
returned as visual content that you can see directly. Offset and limit
are ignored for images.

Use offset (1-based) and limit to read a slice of a large text file —
for example, offset=10 limit=20 returns lines 10 through 29. Omit both
to read the entire file.

When you only need to know what symbols a file contains, prefer the
outline_file tool instead — it returns the structure without the full
source. When you need a symbol's type or documentation, prefer the
describe_symbol tool.

The path can be relative to the workspace root or absolute. Always read
a file before modifying it so you understand its current contents.`,
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path": map[string]any{
						"type":        "string",
						"description": "The file path to read (relative to workspace root or absolute).",
					},
					"offset": map[string]any{
						"type":        []string{"integer", "null"},
						"description": "Optional 1-based line number to start reading from.",
					},
					"limit": map[string]any{
						"type":        []string{"integer", "null"},
						"description": "Optional maximum number of lines to read.",
					},
				},
				"required":             []string{"path", "offset", "limit"},
				"additionalProperties": false,
			},
		},
	}
}

func (t *readFileTool) Summary(arguments string) string {
	var args readFileArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return ""
	}
	s := summaryPath(t.cwd, args.Path)
	if args.Offset > 0 {
		end := args.Offset + args.Limit - 1
		if args.Limit > 0 {
			s += fmt.Sprintf(":%d-%d", args.Offset, end)
		} else {
			s += fmt.Sprintf(":%d-", args.Offset)
		}
	}
	return s
}

func (t *readFileTool) Execute(ctx context.Context, arguments string) agent.ToolResult {
	var args readFileArgs
	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: invalid arguments: %v", err), IsError: true}
	}
	path := resolvePath(t.cwd, args.Path)

	data, err := readFile(t.fs, path)
	if err != nil {
		return agent.ToolResult{Content: fmt.Sprintf("error: %v", err), IsError: true}
	}

	if mime, ok := imageMediaType(path); ok {
		return t.executeImage(ctx, path, data, mime)
	}

	t.tracker.Record(path, data)

	variant := readFileVariant(args.Offset, args.Limit)

	// Binary files: skip in-band content and return a metadata stub
	// so the model doesn't try to reason about raw bytes (and so the
	// proto-go marshaller doesn't reject the request). Still track
	// the read so re-reads detect mutation as usual.
	if utf8validate.IsBinary(data) {
		staleIDs := t.tracker.TrackRead(ctx, "read_file", path, variant)
		staleIDs = append(staleIDs, t.tracker.ConsumeDiscoveries(path)...)
		return agent.ToolResult{
			Content:           utf8validate.BinaryStub(filepath.Base(path), len(data), sha256.Sum256(data)),
			DropToolResultIDs: staleIDs,
		}
	}

	lines := strings.Split(string(data), "\n")

	// Apply offset (1-based)
	start := 0
	if args.Offset > 0 {
		start = args.Offset - 1
	}
	if start > len(lines) {
		start = len(lines)
	}

	end := len(lines)
	if args.Limit > 0 && start+args.Limit < end {
		end = start + args.Limit
	}

	var sb strings.Builder
	for i := start; i < end; i++ {
		fmt.Fprintf(&sb, "L%d: %s\n", i+1, agent.TruncateLine(lines[i], t.maxLineBytes))
	}

	content := sb.String()
	if n := utf8validate.CountInvalidBytes(content); n > 0 {
		content = utf8validate.Sanitize(content) + "\n" + utf8validate.InvalidBytesMarker(n) + "\n"
	}

	staleIDs := t.tracker.TrackRead(ctx, "read_file", path, variant)
	staleIDs = append(staleIDs, t.tracker.ConsumeDiscoveries(path)...)
	return agent.ToolResult{
		Content:           content,
		DropToolResultIDs: staleIDs,
	}
}

// readFileVariant returns a string that distinguishes ranged reads
// from full-file reads so the FileTracker can track them independently.
func readFileVariant(offset, limit int) string {
	if offset <= 0 && limit <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", offset, limit)
}

func resolvePath(cwd workspaceapi.URI, path string) string {
	expanded, err := workspaceapi.ExpandPathWithURI(path, cwd)
	if err != nil {
		return path
	}
	return expanded
}

// imageMediaType returns the MIME type for recognised image extensions.
// SVG is excluded because it's text XML and should be read normally.
func imageMediaType(path string) (string, bool) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
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

func (t *readFileTool) executeImage(ctx context.Context, path string, data []byte, mime string) agent.ToolResult {
	if len(data) > maxImageBytes {
		return agent.ToolResult{
			Content: fmt.Sprintf(
				"error: image file %s is too large to send to the model "+
					"(%d bytes, max %d bytes / %d MiB). Reduce the image "+
					"size (resize or recompress) and try again.",
				filepath.Base(path), len(data), maxImageBytes,
				maxImageBytes/(1024*1024)),
			IsError: true,
		}
	}

	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		if cfg.Width > maxImageEdge || cfg.Height > maxImageEdge {
			return agent.ToolResult{
				Content: fmt.Sprintf(
					"error: image file %s dimensions are too large to send "+
						"to the model (%dx%d px, max %dpx per dimension). "+
						"Resize or crop the image so neither dimension "+
						"exceeds %dpx and try again.",
					filepath.Base(path), cfg.Width, cfg.Height,
					maxImageEdge, maxImageEdge),
				IsError: true,
			}
		}
	}

	t.tracker.Record(path, data)

	dataURI := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	summary := fmt.Sprintf("Read image file: %s (%d bytes, %s)", filepath.Base(path), len(data), mime)

	staleIDs := t.tracker.TrackRead(ctx, "read_file", path, "")
	staleIDs = append(staleIDs, t.tracker.ConsumeDiscoveries(path)...)

	return agent.ToolResult{
		Content: summary,
		MultiContent: []llmapi.ContentPart{
			{Type: llmapi.ContentPartTypeText, Text: summary},
			{Type: llmapi.ContentPartTypeImageURL, ImageURL: dataURI},
		},
		DropToolResultIDs: staleIDs,
	}
}

// summaryPath returns a workspace-relative form of the given path for display.
// Paths outside the workspace collapse "../" chains into ".../".
func summaryPath(cwd workspaceapi.URI, path string) string {
	if path == "" {
		return ""
	}
	abs := resolvePath(cwd, path)
	rel, err := filepath.Rel(cwd.Path(), abs)
	if err != nil {
		return path
	}
	if strings.HasPrefix(rel, "..") {
		for strings.HasPrefix(rel, "../") {
			rel = rel[len("../"):]
		}
		if rel == ".." {
			return "..."
		}
		return ".../" + rel
	}
	return rel
}
