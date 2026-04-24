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


package llamacpp

// Tool-calling support using the Hermes/Qwen2.5 `<tool_call>…</tool_call>`
// convention. This is the de-facto standard emitted by modern instruction-tuned
// GGUF models (Qwen2.5, Hermes-3, Llama-3.1-tool-use, etc.) and is also what
// llama.cpp's own server expects by default.
//
// The wire format is:
//
//	<tool_call>
//	{"name": "<fn>", "arguments": {...}}
//	</tool_call>
//
// Tool results come back to the model wrapped in a `<tool_response>` block in
// a `tool`-role message (or folded into a `user`-role message when the chat
// template does not recognise a tool role).

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"unstable.build/go-tui/cmd/rune-agent/llm"
)

const (
	toolCallOpen  = "<tool_call>"
	toolCallClose = "</tool_call>"

	toolRespOpen  = "<tool_response>"
	toolRespClose = "</tool_response>"
)

// toolCallJSON is the on-the-wire shape of a single tool call.
type toolCallJSON struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// buildToolSystemPrompt renders the Hermes-style tools declaration that is
// prepended to the system message when the request includes tools.
func buildToolSystemPrompt(tools []llm.Tool) (string, error) {
	if len(tools) == 0 {
		return "", nil
	}

	type fn struct {
		Name        string `json:"name"`
		Description string `json:"description,omitempty"`
		Parameters  any    `json:"parameters,omitempty"`
	}
	type entry struct {
		Type     string `json:"type"`
		Function fn     `json:"function"`
	}

	decls := make([]entry, 0, len(tools))
	for _, t := range tools {
		e := entry{
			Type: string(t.Type),
			Function: fn{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  t.Function.Parameters,
			},
		}
		if e.Type == "" {
			e.Type = string(llm.ToolTypeFunction)
		}
		decls = append(decls, e)
	}

	buf, err := json.Marshal(decls)
	if err != nil {
		return "", fmt.Errorf("llamacpp: marshal tools: %w", err)
	}

	var b strings.Builder
	b.WriteString("You may call one or more functions to help answer the user's ")
	b.WriteString("request.\n\nYou are provided with function signatures inside ")
	b.WriteString("<tools></tools> XML tags:\n<tools>\n")
	b.Write(buf)
	b.WriteString("\n</tools>\n\n")
	b.WriteString("When a tool would help, emit a <tool_call> block instead of ")
	b.WriteString("describing the action in prose. Do not say that you will use a ")
	b.WriteString("tool — call it.\n\n")
	b.WriteString("For each function call, return a JSON object with the function ")
	b.WriteString("name and arguments inside <tool_call></tool_call> XML tags:\n")
	b.WriteString("<tool_call>\n")
	b.WriteString(`{"name": "<function-name>", "arguments": <args-json-object>}`)
	b.WriteString("\n</tool_call>")
	return b.String(), nil
}

// renderAssistantWithToolCalls turns a persisted assistant message into the
// textual form expected by the chat template — inlining each tool call as a
// <tool_call> block so the model sees its own prior decisions verbatim.
func renderAssistantWithToolCalls(msg llm.Message) (string, error) {
	if len(msg.ToolCalls) == 0 {
		return msg.Content, nil
	}

	var b strings.Builder
	b.WriteString(strings.TrimRight(msg.Content, "\n"))
	for _, tc := range msg.ToolCalls {
		var args any
		if tc.Function.Arguments != "" {
			// Preserve the argument payload as raw JSON when possible so the
			// model sees the exact formatting it produced. Fall back to a
			// JSON string literal for non-JSON payloads.
			var v any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &v); err == nil {
				args = v
			} else {
				args = tc.Function.Arguments
			}
		} else {
			args = map[string]any{}
		}
		payload, err := json.Marshal(map[string]any{
			"name":      tc.Function.Name,
			"arguments": args,
		})
		if err != nil {
			return "", fmt.Errorf("llamacpp: marshal tool call: %w", err)
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(toolCallOpen)
		b.WriteString("\n")
		b.Write(payload)
		b.WriteString("\n")
		b.WriteString(toolCallClose)
	}
	return b.String(), nil
}

// newToolCallID generates a short, unique identifier for tool calls that the
// model did not assign one to. The concrete format is not meaningful to the
// model; downstream persistence just needs a stable string.
func newToolCallID() string {
	var buf [8]byte
	_, _ = rand.Read(buf[:])
	return "call_" + hex.EncodeToString(buf[:])
}

// toolCallParser is a streaming state machine that consumes incremental text
// deltas from the model and splits them into plain-text deltas and completed
// tool calls. It is resilient to tag tokens being split across pieces.
//
// Output semantics:
//
//	Feed(chunk) → (text, calls, err)
//	  text  — any text that can be flushed immediately as a delta
//	  calls — zero or more completed tool calls
//
//	Flush() → (text, calls, err)
//	  Returns any buffered content at stream end. Unterminated tool calls are
//	  dropped (the caller logs a warning through the error channel).
type toolCallParser struct {
	// buf holds pending text that could be (or is part of) a marker.
	buf strings.Builder
	// inCall is true while we are accumulating the body of a <tool_call>…
	inCall bool
}

// Feed processes a chunk of model output. It returns any text that should be
// streamed to the caller verbatim and any tool calls that became complete
// during the chunk.
func (p *toolCallParser) Feed(chunk string) (string, []llm.ToolCall, error) {
	if chunk == "" {
		return "", nil, nil
	}
	p.buf.WriteString(chunk)

	var (
		outText strings.Builder
		calls   []llm.ToolCall
	)

	for {
		s := p.buf.String()
		if !p.inCall {
			// Look for the start of a tool call.
			idx := strings.Index(s, toolCallOpen)
			if idx < 0 {
				// No full open marker. Flush everything except a tail that
				// could be a partial marker prefix.
				tail := longestPrefixOverlap(s, toolCallOpen)
				if tail > 0 {
					outText.WriteString(s[:len(s)-tail])
					p.buf.Reset()
					p.buf.WriteString(s[len(s)-tail:])
				} else {
					outText.WriteString(s)
					p.buf.Reset()
				}
				break
			}
			// Flush pre-marker text.
			if idx > 0 {
				outText.WriteString(s[:idx])
			}
			// Discard the open marker and enter call mode.
			rest := s[idx+len(toolCallOpen):]
			p.buf.Reset()
			p.buf.WriteString(rest)
			p.inCall = true
			continue
		}

		// inCall: look for the close marker.
		idx := strings.Index(s, toolCallClose)
		if idx < 0 {
			// Hold the body until we see the closer. Don't flush anything.
			break
		}
		body := strings.TrimSpace(s[:idx])
		rest := s[idx+len(toolCallClose):]
		p.buf.Reset()
		p.buf.WriteString(rest)
		p.inCall = false

		tc, err := parseToolCall(body)
		if err != nil {
			return outText.String(), calls, err
		}
		calls = append(calls, tc)
	}

	return outText.String(), calls, nil
}

// Flush finalises the stream. Any unterminated tool call is treated as an
// error (returned alongside the already-parsed calls); plain text that was
// being held behind a potential marker prefix is flushed.
func (p *toolCallParser) Flush() (string, []llm.ToolCall, error) {
	s := p.buf.String()
	p.buf.Reset()
	if p.inCall {
		return "", nil, fmt.Errorf("llamacpp: unterminated %s block (body=%q)", toolCallOpen, s)
	}
	return s, nil, nil
}

// parseToolCall decodes a single <tool_call> body into an llm.ToolCall.
func parseToolCall(body string) (llm.ToolCall, error) {
	var raw toolCallJSON
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return llm.ToolCall{}, fmt.Errorf("llamacpp: tool_call json: %w (body=%q)", err, body)
	}
	if raw.Name == "" {
		return llm.ToolCall{}, fmt.Errorf("llamacpp: tool_call missing name (body=%q)", body)
	}

	// Normalise arguments to a compact JSON string. Accept either an object
	// or a JSON string literal (some models emit the latter).
	args := string(raw.Arguments)
	if args == "" || args == "null" {
		args = "{}"
	} else {
		var s string
		if err := json.Unmarshal(raw.Arguments, &s); err == nil {
			// The model emitted a string literal — use it as-is so callers
			// can json.Unmarshal it themselves if they wish.
			args = s
		} else {
			// Re-encode to strip insignificant whitespace.
			var v any
			if err := json.Unmarshal(raw.Arguments, &v); err == nil {
				if buf, err := json.Marshal(v); err == nil {
					args = string(buf)
				}
			}
		}
	}

	return llm.ToolCall{
		ID:   newToolCallID(),
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionCall{
			Name:      raw.Name,
			Arguments: args,
		},
	}, nil
}

// longestPrefixOverlap returns the length n such that s[len(s)-n:] is a
// prefix of marker and 0 < n < len(marker). 0 means no overlap. This is used
// to decide how much of the tail buffer we must hold in case it forms part of
// a future marker.
func longestPrefixOverlap(s, marker string) int {
	maxN := len(s)
	if maxN >= len(marker) {
		maxN = len(marker) - 1
	}
	for n := maxN; n > 0; n-- {
		if strings.HasPrefix(marker, s[len(s)-n:]) {
			return n
		}
	}
	return 0
}
