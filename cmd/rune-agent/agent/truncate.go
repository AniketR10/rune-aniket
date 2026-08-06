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

package agent

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

const (
	// DefaultMaxToolOutputBytes is the default maximum size for tool output
	// before middle-out truncation is applied (~10k tokens).
	DefaultMaxToolOutputBytes = 40_000

	// DefaultMaxLineBytes is the default maximum size for a single line
	// in tool output (e.g. read_file). Lines exceeding this are truncated
	// at a UTF-8 character boundary with a "…" suffix.
	DefaultMaxLineBytes = 500
)

// MaxToolResultBytes is the per-result ceiling enforced before a tool result
// is appended to the conversation. It sits well under the 16 MiB host gRPC
// cap so a single oversized result can never wedge the agent loop. Capping at
// creation time is cache-safe: it never rewrites already-sent history.
const MaxToolResultBytes = 8 * 1024 * 1024

// capToolResult bounds the size of a tool result in place. Textual content is
// middle-out truncated to maxOutput regardless of success or error, and any
// oversized image part is replaced with a short text placeholder.
func capToolResult(r *ToolResult, maxOutput int) {
	r.Content = TruncateMiddle(r.Content, maxOutput)
	for i := range r.MultiContent {
		p := &r.MultiContent[i]
		if len(p.ImageURL) > MaxToolResultBytes {
			n := len(p.ImageURL)
			*p = llmapi.ContentPart{
				Type: llmapi.ContentPartTypeText,
				Text: fmt.Sprintf("[image omitted: %d bytes exceeds limit]", n),
			}
		}
	}
}

// TruncateMiddle truncates s using a middle-out strategy if len(s) > maxBytes.
// It preserves the first and last portions of the string, replacing the middle
// with a marker indicating how many bytes were removed. The cuts are snapped to
// UTF-8 character boundaries. When truncation occurs, a "Total output lines: N"
// header is prepended.
func TruncateMiddle(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}

	totalLines := strings.Count(s, "\n") + 1

	// Reserve space for the marker. We don't know the exact truncated byte
	// count yet, but a generous estimate keeps us under the budget.
	markerMax := fmt.Sprintf("\n…[%d bytes truncated]…\n", len(s))
	headerMax := fmt.Sprintf("Total output lines: %d\n\n", totalLines)
	overhead := len(markerMax) + len(headerMax)

	budget := maxBytes - overhead
	if budget < 2 {
		// Degenerate case: maxBytes too small to hold even the marker.
		// Just return the marker with the header.
		return fmt.Sprintf("Total output lines: %d\n\n…[%d bytes truncated]…", totalLines, len(s))
	}

	half := budget / 2
	prefixEnd := half
	suffixStart := len(s) - (budget - half)

	// Snap prefixEnd forward to a UTF-8 boundary (don't split a rune).
	for prefixEnd > 0 && !utf8.RuneStart(s[prefixEnd]) {
		prefixEnd--
	}

	// Snap suffixStart forward to a UTF-8 boundary.
	for suffixStart < len(s) && !utf8.RuneStart(s[suffixStart]) {
		suffixStart++
	}

	truncatedBytes := suffixStart - prefixEnd
	marker := fmt.Sprintf("\n…[%d bytes truncated]…\n", truncatedBytes)
	header := fmt.Sprintf("Total output lines: %d\n\n", totalLines)

	return header + s[:prefixEnd] + marker + s[suffixStart:]
}

// truncatedLineMarker is appended to lines that exceed the byte limit.
const truncatedLineMarker = " [truncated line]"

// TruncateLine truncates a single line to maxBytes at a UTF-8 character
// boundary, appending a marker when content is removed.
func TruncateLine(line string, maxBytes int) string {
	if len(line) <= maxBytes {
		return line
	}
	cut := maxBytes - len(truncatedLineMarker)
	if cut < 0 {
		cut = 0
	}
	// Snap backward to a UTF-8 character boundary.
	for cut > 0 && !utf8.RuneStart(line[cut]) {
		cut--
	}
	return line[:cut] + truncatedLineMarker
}
