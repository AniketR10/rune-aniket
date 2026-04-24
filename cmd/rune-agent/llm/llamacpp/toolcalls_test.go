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

import (
	"strings"
	"testing"

	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// feedAll pushes the chunks through a fresh parser, concatenates text deltas,
// and collects tool calls. Used by table-driven tests.
func feedAll(t *testing.T, chunks []string) (string, []llm.ToolCall, error) {
	t.Helper()
	var p toolCallParser
	var text strings.Builder
	var calls []llm.ToolCall
	for _, c := range chunks {
		txt, cs, err := p.Feed(c)
		if err != nil {
			return text.String(), calls, err
		}
		text.WriteString(txt)
		calls = append(calls, cs...)
	}
	txt, cs, err := p.Flush()
	if err != nil {
		return text.String(), calls, err
	}
	text.WriteString(txt)
	calls = append(calls, cs...)
	return text.String(), calls, nil
}

func TestToolCallParser_PlainText(t *testing.T) {
	text, calls, err := feedAll(t, []string{"Hello, ", "world!"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Hello, world!" {
		t.Fatalf("text = %q", text)
	}
	if len(calls) != 0 {
		t.Fatalf("expected no calls, got %d", len(calls))
	}
}

func TestToolCallParser_SingleCallInOneChunk(t *testing.T) {
	body := `<tool_call>{"name": "read_file", "arguments": {"path": "/tmp/x"}}</tool_call>`
	text, calls, err := feedAll(t, []string{body})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "" {
		t.Fatalf("expected empty text, got %q", text)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "read_file" {
		t.Fatalf("name = %q", calls[0].Function.Name)
	}
	if calls[0].Function.Arguments != `{"path":"/tmp/x"}` {
		t.Fatalf("args = %q", calls[0].Function.Arguments)
	}
	if calls[0].Type != llm.ToolTypeFunction {
		t.Fatalf("type = %q", calls[0].Type)
	}
	if calls[0].ID == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestToolCallParser_SplitAcrossChunks(t *testing.T) {
	// Exercises splits at every interesting boundary.
	chunks := []string{
		"Before text ",
		"<tool_ca", "ll>",
		`{"name":"f","arguments":{}}`,
		"</tool_", "call>",
		" after",
	}
	text, calls, err := feedAll(t, chunks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "Before text  after" {
		t.Fatalf("text = %q", text)
	}
	if len(calls) != 1 || calls[0].Function.Name != "f" {
		t.Fatalf("calls = %+v", calls)
	}
	if calls[0].Function.Arguments != "{}" {
		t.Fatalf("args = %q", calls[0].Function.Arguments)
	}
}

func TestToolCallParser_MultipleCalls(t *testing.T) {
	s := `pre <tool_call>{"name":"a","arguments":{"x":1}}</tool_call>` +
		`mid<tool_call>{"name":"b","arguments":{"y":"z"}}</tool_call> post`
	text, calls, err := feedAll(t, []string{s})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "pre mid post" {
		t.Fatalf("text = %q", text)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", len(calls))
	}
	if calls[0].Function.Name != "a" || calls[1].Function.Name != "b" {
		t.Fatalf("names = %q, %q", calls[0].Function.Name, calls[1].Function.Name)
	}
	if calls[0].Function.Arguments != `{"x":1}` {
		t.Fatalf("args[0] = %q", calls[0].Function.Arguments)
	}
	if calls[1].Function.Arguments != `{"y":"z"}` {
		t.Fatalf("args[1] = %q", calls[1].Function.Arguments)
	}
}

func TestToolCallParser_HoldsPotentialMarkerPrefix(t *testing.T) {
	// A suffix of the open marker must be held rather than flushed as text —
	// otherwise a call split as "<tool_" + "call>..." would leak "<tool_"
	// into the user-visible stream.
	var p toolCallParser
	text, calls, err := p.Feed("hello <tool_")
	if err != nil {
		t.Fatalf("feed 1: %v", err)
	}
	if text != "hello " {
		t.Fatalf("stage1 text = %q", text)
	}
	if len(calls) != 0 {
		t.Fatalf("expected no calls, got %d", len(calls))
	}
	text, calls, err = p.Feed(`call>{"name":"n","arguments":{}}</tool_call>`)
	if err != nil {
		t.Fatalf("feed 2: %v", err)
	}
	if text != "" {
		t.Fatalf("stage2 text = %q", text)
	}
	if len(calls) != 1 || calls[0].Function.Name != "n" {
		t.Fatalf("calls = %+v", calls)
	}
}

func TestToolCallParser_FalseAlarmPrefix(t *testing.T) {
	// A partial-marker tail that never completes must be flushed as plain
	// text on Flush.
	text, calls, err := feedAll(t, []string{"look: <tool_ca"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if text != "look: <tool_ca" {
		t.Fatalf("text = %q", text)
	}
	if len(calls) != 0 {
		t.Fatalf("expected no calls, got %d", len(calls))
	}
}

func TestToolCallParser_UnterminatedCallReturnsError(t *testing.T) {
	_, _, err := feedAll(t, []string{`<tool_call>{"name":"x"`})
	if err == nil {
		t.Fatal("expected error on unterminated tool call")
	}
}

func TestToolCallParser_InvalidJSONReturnsError(t *testing.T) {
	_, _, err := feedAll(t, []string{`<tool_call>not json</tool_call>`})
	if err == nil {
		t.Fatal("expected error on malformed tool_call body")
	}
}

func TestToolCallParser_StringArguments(t *testing.T) {
	// Some models emit arguments as a JSON string literal rather than an
	// object. We preserve the string so callers can decode it themselves.
	body := `<tool_call>{"name":"f","arguments":"{\"k\":\"v\"}"}</tool_call>`
	_, calls, err := feedAll(t, []string{body})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Arguments != `{"k":"v"}` {
		t.Fatalf("args = %q", calls[0].Function.Arguments)
	}
}

func TestToolCallParser_NullOrMissingArguments(t *testing.T) {
	for _, body := range []string{
		`<tool_call>{"name":"f","arguments":null}</tool_call>`,
		`<tool_call>{"name":"f"}</tool_call>`,
	} {
		_, calls, err := feedAll(t, []string{body})
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", body, err)
		}
		if len(calls) != 1 || calls[0].Function.Arguments != "{}" {
			t.Fatalf("%s: args = %+v", body, calls)
		}
	}
}

func TestBuildToolSystemPrompt_EmptyTools(t *testing.T) {
	s, err := buildToolSystemPrompt(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s != "" {
		t.Fatalf("expected empty prompt, got %q", s)
	}
}

func TestBuildToolSystemPrompt_IncludesFunctionNames(t *testing.T) {
	tools := []llm.Tool{
		{
			Type: llm.ToolTypeFunction,
			Function: llm.FunctionDefinition{
				Name:        "grep",
				Description: "search contents",
				Parameters:  map[string]any{"type": "object"},
			},
		},
	}
	s, err := buildToolSystemPrompt(tools)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(s, "<tools>") || !strings.Contains(s, "</tools>") {
		t.Fatalf("missing tools tags: %q", s)
	}
	if !strings.Contains(s, `"name":"grep"`) {
		t.Fatalf("missing function name: %q", s)
	}
	if !strings.Contains(s, `<tool_call>`) {
		t.Fatalf("missing <tool_call> instruction: %q", s)
	}
}

func TestRenderAssistantWithToolCalls(t *testing.T) {
	msg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: "Sure, let me look.",
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call_1",
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path":"/tmp/x"}`,
				},
			},
		},
	}
	out, err := renderAssistantWithToolCalls(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "Sure, let me look.") {
		t.Fatalf("missing content: %q", out)
	}
	if !strings.Contains(out, `<tool_call>`) || !strings.Contains(out, `</tool_call>`) {
		t.Fatalf("missing tool_call tags: %q", out)
	}
	if !strings.Contains(out, `"name":"read_file"`) {
		t.Fatalf("missing name: %q", out)
	}
	if !strings.Contains(out, `"arguments":{"path":"/tmp/x"}`) {
		t.Fatalf("missing arguments: %q", out)
	}
}

func TestRenderAssistantWithToolCalls_NoCalls(t *testing.T) {
	msg := llm.Message{Role: llm.RoleAssistant, Content: "just text"}
	out, err := renderAssistantWithToolCalls(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "just text" {
		t.Fatalf("out = %q", out)
	}
}

func TestLongestPrefixOverlap(t *testing.T) {
	tests := []struct {
		s, marker string
		want      int
	}{
		{"", "<tool_call>", 0},
		{"hello", "<tool_call>", 0},
		{"foo<", "<tool_call>", 1},
		{"<tool_", "<tool_call>", 6},
		{"<tool_call", "<tool_call>", 10},
		{"<tool_call>", "<tool_call>", 0}, // full match is not a "prefix overlap"
	}
	for _, tc := range tests {
		got := longestPrefixOverlap(tc.s, tc.marker)
		if got != tc.want {
			t.Errorf("longestPrefixOverlap(%q, %q) = %d, want %d",
				tc.s, tc.marker, got, tc.want)
		}
	}
}
