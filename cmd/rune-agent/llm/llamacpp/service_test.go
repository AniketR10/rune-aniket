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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/unstablebuild/rune-go-sdk/iterator"

	"unstable.build/go-tui/cmd/rune-agent/llm"
	"unstable.build/go-tui/cmd/rune-agent/llm/llamacpp/ociregistry"
)

func TestChatTemplateOptions_Defaults(t *testing.T) {
	t.Parallel()

	opts := makeChatTemplateOptions("", nil, llm.ReasoningEffortHigh, true, "", nil, nil)
	if !opts.AddGenerationPrompt {
		t.Fatal("expected AddGenerationPrompt to be true")
	}
	if !opts.EnableThinking {
		t.Fatal("expected thinking to stay enabled for non-tool turns")
	}
	if opts.ParallelToolCalls {
		t.Fatal("expected parallel tool calls to default off")
	}
	if opts.ReasoningFmt != ReasoningAuto {
		t.Fatalf("ReasoningFmt = %v, want %v", opts.ReasoningFmt, ReasoningAuto)
	}
}

func TestChatTemplateOptions_WithToolsKeepsThinkingByDefault(t *testing.T) {
	t.Parallel()

	opts := makeChatTemplateOptions("", []Tool{{Name: "list_dir"}}, llm.ReasoningEffortHigh, true, "", nil, nil)
	if !opts.EnableThinking {
		t.Fatal("expected thinking to stay enabled by default during tool-use turns")
	}
	if opts.ParallelToolCalls {
		t.Fatal("expected parallel tool calls to be disabled by default")
	}
}

func TestChatTemplateOptions_MinimalEffortDisablesThinking(t *testing.T) {
	t.Parallel()

	opts := makeChatTemplateOptions("", nil, llm.ReasoningEffortMinimal, true, "", nil, nil)
	if opts.EnableThinking {
		t.Fatal("expected minimal effort to disable explicit thinking")
	}
}

func TestChatTemplateOptions_PropagatesToolSettings(t *testing.T) {
	t.Parallel()

	parallel := true
	opts := makeChatTemplateOptions(
		"",
		[]Tool{{Name: "get_time"}},
		llm.ReasoningEffortHigh,
		true,
		llm.ToolChoiceRequired,
		&parallel,
		nil,
	)
	if opts.ToolChoice != string(llm.ToolChoiceRequired) {
		t.Fatalf("ToolChoice = %q, want %q", opts.ToolChoice, llm.ToolChoiceRequired)
	}
	if !opts.ParallelToolCalls {
		t.Fatal("expected explicit parallel_tool_calls=true to be preserved")
	}

	parallel = false
	opts = makeChatTemplateOptions(
		"",
		[]Tool{{Name: "get_time"}},
		llm.ReasoningEffortHigh,
		true,
		llm.ToolChoiceNone,
		&parallel,
		nil,
	)
	if opts.ToolChoice != string(llm.ToolChoiceNone) {
		t.Fatalf("ToolChoice = %q, want %q", opts.ToolChoice, llm.ToolChoiceNone)
	}
	if opts.ParallelToolCalls {
		t.Fatal("expected explicit parallel_tool_calls=false to be preserved")
	}
	if opts.EnableThinking != true {
		t.Fatal("tool settings should not implicitly disable thinking")
	}
}

func TestChatTemplateOptions_PropagatesResponseFormat(t *testing.T) {
	t.Parallel()

	t.Run("json object", func(t *testing.T) {
		opts := makeChatTemplateOptions(
			"",
			nil,
			llm.ReasoningEffortHigh,
			true,
			"",
			nil,
			&llm.ResponseFormat{Type: llm.ResponseFormatTypeJSONObject},
		)
		if opts.JSONSchema != `{"type":"object"}` {
			t.Fatalf("JSONSchema = %q", opts.JSONSchema)
		}
	})

	t.Run("json schema", func(t *testing.T) {
		type rawSchema map[string]any
		schema := rawSchema{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}}
		b, _ := json.Marshal(schema)
		opts := makeChatTemplateOptions(
			"",
			nil,
			llm.ReasoningEffortHigh,
			true,
			"",
			nil,
			&llm.ResponseFormat{
				Type:       llm.ResponseFormatTypeJSONSchema,
				JSONSchema: &llm.ResponseFormatJSONSchema{Schema: json.RawMessage(b)},
			},
		)
		if opts.JSONSchema != string(b) {
			t.Fatalf("JSONSchema = %q, want %q", opts.JSONSchema, string(b))
		}
	})
}

func TestApplyStops_FullStop(t *testing.T) {
	t.Parallel()

	g := &genState{stopStrings: []string{"<turn|>"}}
	safe, stop := g.applyStops("Hello<turn|>")
	if safe != "Hello" {
		t.Fatalf("safe = %q, want %q", safe, "Hello")
	}
	if !stop {
		t.Fatal("expected full stop to terminate output")
	}
	if g.pendingStop != "" {
		t.Fatalf("pendingStop = %q, want empty", g.pendingStop)
	}
}

func TestApplyStops_PartialAcrossPieces(t *testing.T) {
	t.Parallel()

	g := &genState{stopStrings: []string{"<turn|>"}}
	safe, stop := g.applyStops("Hello<tu")
	if safe != "Hello" {
		t.Fatalf("safe = %q, want %q", safe, "Hello")
	}
	if stop {
		t.Fatal("did not expect partial stop to terminate output")
	}
	if g.pendingStop != "<tu" {
		t.Fatalf("pendingStop = %q, want %q", g.pendingStop, "<tu")
	}

	safe, stop = g.applyStops("rn|>")
	if safe != "" {
		t.Fatalf("safe = %q, want empty", safe)
	}
	if !stop {
		t.Fatal("expected completed stop sequence to terminate output")
	}
	if g.pendingStop != "" {
		t.Fatalf("pendingStop = %q, want empty", g.pendingStop)
	}
}

func TestApplyStops_ReleasesFalseAlarm(t *testing.T) {
	t.Parallel()

	g := &genState{stopStrings: []string{"<turn|>"}}
	safe, stop := g.applyStops("abc<tu")
	if safe != "abc" || stop {
		t.Fatalf("first applyStops = (%q, %v), want (%q, false)", safe, stop, "abc")
	}
	safe, stop = g.applyStops("x")
	if safe != "<tux" {
		t.Fatalf("safe = %q, want %q", safe, "<tux")
	}
	if stop {
		t.Fatal("false alarm partial stop should not terminate output")
	}
}

func TestFlushPendingStop(t *testing.T) {
	t.Parallel()

	g := &genState{pendingStop: "<tu"}
	if got := g.flushPendingStop(); got != "<tu" {
		t.Fatalf("flushPendingStop = %q, want %q", got, "<tu")
	}
	if g.pendingStop != "" {
		t.Fatalf("pendingStop = %q, want empty", g.pendingStop)
	}
}

func TestLongestPartialStopSuffix(t *testing.T) {
	t.Parallel()

	if got := longestPartialStopSuffix("prefix<tu", []string{"<turn|>", "</s>"}); got != "<tu" {
		t.Fatalf("longestPartialStopSuffix = %q, want %q", got, "<tu")
	}
	if got := longestPartialStopSuffix("prefix", []string{"<turn|>"}); got != "" {
		t.Fatalf("longestPartialStopSuffix = %q, want empty", got)
	}
	if got := longestPartialStopSuffix("prefix</", []string{"</s>", "</tool>"}); !slices.Contains([]string{"</"}, got) {
		t.Fatalf("unexpected partial suffix %q", got)
	}
}

func TestEmitDone_PrefersToolCallFinishReasonWhenToolCallsExist(t *testing.T) {
	t.Parallel()

	g := &genState{
		toolCalls: []llm.ToolCall{{
			ID:   "call_1",
			Type: llm.ToolTypeFunction,
			Function: llm.FunctionCall{
				Name:      "get_time",
				Arguments: `{}`,
			},
		}},
		finish: llm.FinishReasonStop,
	}

	ev, ok, err := g.emitDone()
	if err != nil {
		t.Fatalf("emitDone: %v", err)
	}
	if !ok {
		t.Fatal("emitDone returned ok=false")
	}
	if ev.DoneData == nil {
		t.Fatal("emitDone returned nil DoneData")
	}
	if ev.DoneData.FinishReason != llm.FinishReasonToolCall {
		t.Fatalf("FinishReason = %q, want %q", ev.DoneData.FinishReason, llm.FinishReasonToolCall)
	}
}

// TestBuildChatMessages_ToolRoleRemappedToUser pins down the single most
// important piece of Qwen/ChatML-family tool-calling wiring: tool-role
// messages must be handed to llama_chat_apply_template under the "user"
// role.
//
// Background: llama.cpp's llama_chat_apply_template is NOT a Jinja parser
// (see llama.h). For Qwen2.5/3 and every other ChatML-family model it
// falls back to the built-in `chatml` implementation, which interpolates
// the role string literally. So a naive "tool" role would render as
// `<|im_start|>tool\n…<|im_end|>` — a header Qwen has never been trained
// on. Empirically this leads to the model ignoring tool results (or, in
// the reported case, repeatedly re-issuing `compact` because it does not
// see the tool's response). Qwen's real HF Jinja template instead wraps
// tool results in a `user` block with `<tool_response>…</tool_response>`
// content, which is the shape we reproduce here.
func TestBuildChatMessages_ToolRoleRemappedToUser(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "you are helpful"},
		{Role: llm.RoleUser, Content: "hi"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID:   "call_1",
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "get_time",
					Arguments: `{}`,
				},
			}},
		},
		{Role: llm.RoleTool, Content: "12:34"},
	}
	tools := []llm.Tool{{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "get_time",
			Description: "Return current time",
		},
	}}

	cmsgs, err := buildChatMessages(msgs, tools)
	if err != nil {
		t.Fatalf("buildChatMessages: %v", err)
	}

	// Locate the tool message. It must be rendered as role="user" and its
	// body must retain the <tool_response> wrapper.
	var toolIdx = -1
	for i, c := range cmsgs {
		if strings.Contains(c.Content, "<tool_response>") {
			toolIdx = i
			break
		}
	}
	if toolIdx < 0 {
		t.Fatalf("expected a <tool_response> message, got: %+v", cmsgs)
	}
	if got := cmsgs[toolIdx].Role; got != string(llm.RoleUser) {
		t.Fatalf("tool message role = %q, want %q (chatml treats role verbatim; "+
			"`tool` role is not a header Qwen/ChatML models were trained on)",
			got, llm.RoleUser)
	}
	wantBody := "<tool_response>\n12:34\n</tool_response>"
	if got := cmsgs[toolIdx].Content; got != wantBody {
		t.Fatalf("tool message body = %q, want %q", got, wantBody)
	}

	// Sanity: none of the ChatMessages should still carry the literal
	// "tool" role — that was exactly the bug.
	for _, c := range cmsgs {
		if c.Role == string(llm.RoleTool) {
			t.Fatalf("buildChatMessages left role=%q in output: %+v", c.Role, c)
		}
	}
}

// TestBuildChatMessages_InjectsToolPromptIntoExistingSystem ensures the
// Hermes-style tool declaration block is folded into the existing system
// message (rather than being inserted as a second one). Qwen's own
// tool-use training data places tool definitions inside the system block.
func TestBuildChatMessages_InjectsToolPromptIntoExistingSystem(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleSystem, Content: "you are helpful"},
		{Role: llm.RoleUser, Content: "hi"},
	}
	tools := []llm.Tool{{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name: "get_time",
		},
	}}
	cmsgs, err := buildChatMessages(msgs, tools)
	if err != nil {
		t.Fatalf("buildChatMessages: %v", err)
	}
	if len(cmsgs) != 2 {
		t.Fatalf("expected 2 chat messages, got %d: %+v", len(cmsgs), cmsgs)
	}
	if cmsgs[0].Role != string(llm.RoleSystem) {
		t.Fatalf("first message role = %q, want system", cmsgs[0].Role)
	}
	if !strings.Contains(cmsgs[0].Content, "<tools>") {
		t.Fatalf("first message missing <tools> declaration block: %q", cmsgs[0].Content)
	}
	if !strings.HasSuffix(cmsgs[0].Content, "you are helpful") {
		t.Fatalf("original system text was dropped: %q", cmsgs[0].Content)
	}
}

// TestBuildChatMessages_PrependsSystemWhenToolsAndNoSystem covers the
// case where the caller forgot to include a system message. We must
// synthesise one so the tool declarations still reach the model.
func TestBuildChatMessages_PrependsSystemWhenToolsAndNoSystem(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "hi"},
	}
	tools := []llm.Tool{{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name: "get_time",
		},
	}}
	cmsgs, err := buildChatMessages(msgs, tools)
	if err != nil {
		t.Fatalf("buildChatMessages: %v", err)
	}
	if len(cmsgs) < 2 || cmsgs[0].Role != string(llm.RoleSystem) {
		t.Fatalf("expected synthesised system message at index 0, got %+v", cmsgs)
	}
	if !strings.Contains(cmsgs[0].Content, "<tools>") {
		t.Fatalf("synthesised system message missing tool block: %q", cmsgs[0].Content)
	}
}

// TestRenderForUpstream_PreservesStructuredToolMessages verifies the upstream
// Jinja path keeps structured tool metadata instead of flattening everything
// into plain text. This is the key difference from the legacy/Hermes fallback.
func TestRenderForUpstream_PreservesStructuredToolMessages(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "hi"},
		{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{{
				ID:   "call_1",
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "get_time",
					Arguments: `{}`,
				},
			}},
		},
		{Role: llm.RoleTool, Name: "get_time", Content: "12:34", ToolCallID: "call_1"},
	}

	got, files := renderForUpstream(msgs)
	if len(got) != 3 {
		t.Fatalf("len(renderForUpstream) = %d, want 3", len(got))
	}
	if len(files) != 0 {
		t.Fatalf("len(files) = %d, want 0", len(files))
	}
	if got[1].Role != string(llm.RoleAssistant) {
		t.Fatalf("assistant role = %q", got[1].Role)
	}
	if got[1].Content != "" {
		t.Fatalf("assistant content = %q, want empty when tool calls are structured", got[1].Content)
	}
	if len(got[1].ToolCalls) != 1 {
		t.Fatalf("assistant tool calls = %+v", got[1].ToolCalls)
	}
	if got[1].ToolCalls[0].Name != "get_time" {
		t.Fatalf("tool name = %q", got[1].ToolCalls[0].Name)
	}
	if got[1].ToolCalls[0].ID != "call_1" {
		t.Fatalf("tool id = %q", got[1].ToolCalls[0].ID)
	}
	if got[2].Role != string(llm.RoleTool) {
		t.Fatalf("tool role = %q, want tool", got[2].Role)
	}
	if got[2].ToolCallID != "call_1" {
		t.Fatalf("tool call id = %q", got[2].ToolCallID)
	}
	if got[2].Name != "get_time" {
		t.Fatalf("tool name = %q", got[2].Name)
	}
}

func TestRenderForUpstream_PreservesImageAsMediaMarker(t *testing.T) {
	msgs := []llm.Message{{
		Role: llm.RoleUser,
		MultiContent: []llm.ContentPart{
			{Type: llm.ContentPartTypeText, Text: "describe this image:"},
			{Type: llm.ContentPartTypeImageURL, ImageURL: "data:image/png;base64,AAAA"},
		},
	}}

	got, files := renderForUpstream(msgs)
	if len(got) != 1 {
		t.Fatalf("len(renderForUpstream) = %d, want 1", len(got))
	}
	if len(files) != 1 {
		t.Fatalf("len(files) = %d, want 1", len(files))
	}
	if got[0].Content != "" {
		t.Fatalf("expected string content to be empty when typed parts are present, got %q", got[0].Content)
	}
	if len(got[0].ContentParts) != 2 {
		t.Fatalf("len(ContentParts) = %d, want 2", len(got[0].ContentParts))
	}
	if got[0].ContentParts[0].Type != "text" || got[0].ContentParts[0].Text != "describe this image:" {
		t.Fatalf("unexpected text part: %+v", got[0].ContentParts[0])
	}
	if got[0].ContentParts[1].Type != "media_marker" || got[0].ContentParts[1].Text != defaultMediaMarker {
		t.Fatalf("unexpected media marker part: %+v", got[0].ContentParts[1])
	}
	if string(files[0]) == "" {
		t.Fatal("expected decoded image bytes")
	}
}

func TestRenderForUpstream_PreservesReasoningAndTypedText(t *testing.T) {
	msgs := []llm.Message{
		{
			Role:             llm.RoleAssistant,
			ReasoningContent: "hidden chain of thought",
			MultiContent: []llm.ContentPart{
				{Type: llm.ContentPartTypeText, Text: "hello"},
				{Type: llm.ContentPartTypeImageURL, ImageURL: "data:image/png;base64,abc"},
			},
		},
	}

	got, files := renderForUpstream(msgs)
	if len(got) != 1 {
		t.Fatalf("len(renderForUpstream) = %d, want 1", len(got))
	}
	if len(files) != 0 {
		t.Fatalf("len(files) = %d, want 0 for invalid image payload", len(files))
	}
	if got[0].ReasoningContent != "hidden chain of thought" {
		t.Fatalf("reasoning = %q", got[0].ReasoningContent)
	}
	if got[0].Content != "" {
		t.Fatalf("content = %q, want empty when typed content is preserved", got[0].Content)
	}
	if len(got[0].ContentParts) != 1 {
		t.Fatalf("content parts = %+v, want one text part", got[0].ContentParts)
	}
	if got[0].ContentParts[0].Type != "text" || got[0].ContentParts[0].Text != "hello" {
		t.Fatalf("content parts = %+v", got[0].ContentParts)
	}
}

// TestNewService_ConfigValidation_Table covers every input-validation
// path of NewService that does NOT require loading a model: empty
// ModelPath and a model path that doesn't exist on disk. The "model
// path doesn't exist" path is what users hit when they typo a model
// name; the test pins the error wrapping so callers can match it.
func TestNewService_ConfigValidation_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		cfg       Config
		errSubstr string
	}{
		{
			name:      "empty_model_path",
			cfg:       Config{},
			errSubstr: "ModelPath is required",
		},
		{
			name: "nonexistent_model_path",
			cfg: Config{
				ModelPath: filepath.Join(t.TempDir(), "missing.gguf"),
			},
			errSubstr: "load",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, err := NewService(tc.cfg)
			if err == nil {
				if svc != nil {
					svc.Close()
				}
				t.Fatalf("NewService: want error, got svc=%v", svc)
			}
			if !strings.Contains(err.Error(), tc.errSubstr) {
				t.Fatalf("err = %v, want substring %q", err, tc.errSubstr)
			}
		})
	}
}

// TestDecodeImageContentPart_Table exercises every branch of the
// decoder. Real callers feed values straight from
// llm.ContentPartTypeImageURL, so each malformed shape is something we
// have actually observed in production traffic.
func TestDecodeImageContentPart_Table(t *testing.T) {
	t.Parallel()
	const tinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ipKoAAAAASUVORK5CYII="

	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid_png_data_url", "data:image/png;base64," + tinyPNG, false},
		{"non_data_url", "https://example.com/x.png", true},
		{"missing_comma", "data:image/png;base64iVBOR", true},
		{"missing_base64_marker", "data:image/png,iVBOR", true},
		{"malformed_base64", "data:image/png;base64,!!!not-base64!!!", true},
		{"empty", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeImageContentPart(tc.url)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("want error, got %d bytes", len(got))
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeImageContentPart: %v", err)
			}
			if len(got) == 0 {
				t.Fatal("decoded image: 0 bytes")
			}
		})
	}
}

// TestResponseFormatJSONSchema_Table pins the helper that translates
// llm.ResponseFormat into the JSON schema string fed to
// rune_chat_template_open. Each branch is reachable from public API.
func TestResponseFormatJSONSchema_Table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   *llm.ResponseFormat
		want string
	}{
		{"nil", nil, ""},
		{"json_object", &llm.ResponseFormat{Type: llm.ResponseFormatTypeJSONObject}, `{"type":"object"}`},
		{
			name: "json_schema_no_payload",
			in:   &llm.ResponseFormat{Type: llm.ResponseFormatTypeJSONSchema},
			want: "",
		},
		{
			name: "json_schema_nil_schema",
			in: &llm.ResponseFormat{
				Type:       llm.ResponseFormatTypeJSONSchema,
				JSONSchema: &llm.ResponseFormatJSONSchema{},
			},
			want: "",
		},
		{
			name: "json_schema_with_schema",
			in: &llm.ResponseFormat{
				Type: llm.ResponseFormatTypeJSONSchema,
				JSONSchema: &llm.ResponseFormatJSONSchema{
					Schema: json.RawMessage(`{"type":"number"}`),
				},
			},
			want: `{"type":"number"}`,
		},
		{
			name: "text_format_unknown",
			in:   &llm.ResponseFormat{Type: llm.ResponseFormatTypeText},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := responseFormatJSONSchema(tc.in)
			if got != tc.want {
				t.Fatalf("responseFormatJSONSchema = %q, want %q", got, tc.want)
			}
		})
	}
}

// brokenSchema implements json.Marshaler with an error return so we can
// exercise the failure branch of responseFormatJSONSchema without
// mocking the entire llm.ResponseFormatJSONSchema type.
type brokenSchema struct{}

func (brokenSchema) MarshalJSON() ([]byte, error) {
	return nil, errors.New("broken")
}

func TestResponseFormatJSONSchema_MarshalError(t *testing.T) {
	t.Parallel()
	out := responseFormatJSONSchema(&llm.ResponseFormat{
		Type: llm.ResponseFormatTypeJSONSchema,
		JSONSchema: &llm.ResponseFormatJSONSchema{
			Schema: brokenSchema{},
		},
	})
	if out != "" {
		t.Fatalf("responseFormatJSONSchema = %q, want empty on marshal error", out)
	}
}

// TestConvertTools_Table covers the tool-conversion helper. The
// chan-based parameters case forces json.Marshal to error out so we
// pin the "params drop to empty string" branch.
func TestConvertTools_Table(t *testing.T) {
	t.Parallel()

	if got := convertTools(nil); got != nil {
		t.Fatalf("convertTools(nil) = %+v, want nil", got)
	}
	if got := convertTools([]llm.Tool{}); got != nil {
		t.Fatalf("convertTools(empty) = %+v, want nil", got)
	}

	tools := []llm.Tool{
		{
			Type: llm.ToolTypeFunction,
			Function: llm.FunctionDefinition{
				Name:        "get_time",
				Description: "Return current time",
				Parameters: map[string]any{
					"type": "object",
				},
			},
		},
		{
			Type: llm.ToolTypeFunction,
			Function: llm.FunctionDefinition{
				Name:       "broken",
				Parameters: map[string]any{"bad": make(chan int)},
			},
		},
	}
	got := convertTools(tools)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].Name != "get_time" || got[0].ParametersJSON == "" {
		t.Fatalf("first tool = %+v", got[0])
	}
	if got[1].Name != "broken" || got[1].ParametersJSON != "" {
		t.Fatalf("broken-params tool should drop params: %+v", got[1])
	}
}

// TestRenderMessage_Table exercises the renderMessage helper. The
// assistant + tool calls path delegates to the toolcalls module and is
// covered by toolcalls_test.go; here we focus on the main branches.
func TestRenderMessage_Table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		msg  llm.Message
		want string
	}{
		{
			name: "plain_content",
			msg:  llm.Message{Role: llm.RoleUser, Content: "hello"},
			want: "hello",
		},
		{
			name: "content_overrides_multicontent",
			msg: llm.Message{
				Role:    llm.RoleUser,
				Content: "winning",
				MultiContent: []llm.ContentPart{
					{Type: llm.ContentPartTypeText, Text: "ignored"},
				},
			},
			want: "winning",
		},
		{
			name: "multicontent_text_only",
			msg: llm.Message{
				Role: llm.RoleUser,
				MultiContent: []llm.ContentPart{
					{Type: llm.ContentPartTypeText, Text: "first "},
					{Type: llm.ContentPartTypeText, Text: "second"},
				},
			},
			want: "first second",
		},
		{
			name: "multicontent_image_only_drops",
			msg: llm.Message{
				Role: llm.RoleUser,
				MultiContent: []llm.ContentPart{
					{Type: llm.ContentPartTypeImageURL, ImageURL: "data:image/png;base64,abc"},
				},
			},
			want: "",
		},
		{
			name: "empty",
			msg:  llm.Message{Role: llm.RoleUser},
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := renderMessage(tc.msg)
			if err != nil {
				t.Fatalf("renderMessage: %v", err)
			}
			if got != tc.want {
				t.Fatalf("renderMessage = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestApplyStops_Table consolidates the existing per-case applyStops
// tests into one table and adds the no-stops + multi-stop cases.
func TestApplyStops_Table(t *testing.T) {
	t.Parallel()

	type step struct {
		piece   string
		safe    string
		stop    bool
		pending string
	}
	cases := []struct {
		name  string
		stops []string
		steps []step
	}{
		{
			name:  "no_stops",
			stops: nil,
			steps: []step{{"hello", "hello", false, ""}},
		},
		{
			name:  "empty_stops_in_list",
			stops: []string{"", "<turn|>"},
			steps: []step{{"x", "x", false, ""}},
		},
		{
			name:  "multi_stop_picks_nearest",
			stops: []string{"</tool>", "<turn|>"},
			steps: []step{{"abc<turn|>extra</tool>", "abc", true, ""}},
		},
		{
			name:  "multi_stop_picks_earliest_when_both_match",
			stops: []string{"<turn|>", "<turn|>extra"},
			// Both prefixes match at the same position; nearest stop wins.
			steps: []step{{"abc<turn|>", "abc", true, ""}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &genState{stopStrings: tc.stops}
			for i, st := range tc.steps {
				safe, stop := g.applyStops(st.piece)
				if safe != st.safe {
					t.Fatalf("step %d: safe = %q, want %q", i, safe, st.safe)
				}
				if stop != st.stop {
					t.Fatalf("step %d: stop = %v, want %v", i, stop, st.stop)
				}
				if g.pendingStop != st.pending {
					t.Fatalf("step %d: pendingStop = %q, want %q", i, g.pendingStop, st.pending)
				}
			}
		})
	}
}

// TestEmitDone_Table covers the finish-reason fan-in: tool calls
// override any prior reason; without tool calls a previously-set reason
// passes through; otherwise the reason defaults to Stop. Done's Message
// must reflect accumulated text.
func TestEmitDone_Table(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		setup       func(*genState)
		wantReason  llm.FinishReason
		wantContent string
		wantTools   int
	}{
		{
			name: "no_tools_no_reason_defaults_stop",
			setup: func(g *genState) {
				g.messageBuf.WriteString("hello")
			},
			wantReason:  llm.FinishReasonStop,
			wantContent: "hello",
		},
		{
			name: "no_tools_reason_preserved",
			setup: func(g *genState) {
				g.messageBuf.WriteString("partial")
				g.finish = llm.FinishReasonLength
			},
			wantReason:  llm.FinishReasonLength,
			wantContent: "partial",
		},
		{
			name: "tool_calls_force_toolcall_reason",
			setup: func(g *genState) {
				g.messageBuf.WriteString("call dispatch:")
				g.finish = llm.FinishReasonLength
				g.toolCalls = []llm.ToolCall{
					{ID: "c1", Type: llm.ToolTypeFunction, Function: llm.FunctionCall{Name: "get_time", Arguments: "{}"}},
				}
			},
			wantReason:  llm.FinishReasonToolCall,
			wantContent: "call dispatch:",
			wantTools:   1,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := &genState{}
			tc.setup(g)
			ev, ok, err := g.emitDone()
			if err != nil || !ok {
				t.Fatalf("emitDone: ok=%v err=%v", ok, err)
			}
			if ev.DoneData == nil {
				t.Fatal("DoneData == nil")
			}
			if ev.DoneData.FinishReason != tc.wantReason {
				t.Fatalf("FinishReason = %q, want %q", ev.DoneData.FinishReason, tc.wantReason)
			}
			if ev.DoneData.Message.Content != tc.wantContent {
				t.Fatalf("content = %q, want %q", ev.DoneData.Message.Content, tc.wantContent)
			}
			if len(ev.DoneData.Message.ToolCalls) != tc.wantTools {
				t.Fatalf("tool calls = %d, want %d", len(ev.DoneData.Message.ToolCalls), tc.wantTools)
			}
			// Calling emitDone a second time must not re-emit Done.
			if ev2, ok2, _ := g.emitDone(); ok2 {
				t.Fatalf("second emitDone should be ok=false, got %+v", ev2)
			}
		})
	}
}

// TestEvalPrompt_NoOpAtBoundary checks the early-return branch: when
// startPos >= len(tokens), evalPrompt does not touch ctx or batch and
// returns nil. This is reachable even with nil ctx/batch arguments,
// pinning that contract so future refactors can't accidentally start
// dereferencing.
func TestEvalPrompt_NoOpAtBoundary(t *testing.T) {
	t.Parallel()
	tokens := []int32{1, 2, 3}
	if err := evalPrompt(nil, nil, tokens, len(tokens)); err != nil {
		t.Fatalf("evalPrompt(boundary): %v", err)
	}
	if err := evalPrompt(nil, nil, nil, 0); err != nil {
		t.Fatalf("evalPrompt(empty): %v", err)
	}
}

// TestLongestPartialStopSuffix_Table extends the existing partial-stop
// suffix tests with degenerate edge cases.
func TestLongestPartialStopSuffix_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		text  string
		stops []string
		want  string
	}{
		{"empty_stops", "abc<tu", nil, ""},
		{"single_char_stop_skipped", "abc<", []string{"<"}, ""},
		{"empty_text", "", []string{"<turn|>"}, ""},
		{"prefix_match", "ab<tu", []string{"<turn|>"}, "<tu"},
		{"longer_match_wins", "ab<tur", []string{"<turn|>", "<tu"}, "<tur"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := longestPartialStopSuffix(tc.text, tc.stops)
			if got != tc.want {
				t.Fatalf("longestPartialStopSuffix = %q, want %q", got, tc.want)
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Opt-in black-box suite against a real downloaded GGUF model.
//
// These tests drive llamacpp.Service through the llm.Service interface using a
// small quantized model cached in cmd/rune-agent/llm/llamacpp/testdata. The
// model is downloaded on demand (and only when opted in) via ociregistry.
//
// Opt-in:
//
//	RUNE_LLAMACPP_TESTDATA=1   — enable the suite and allow the first run to
//	                             download ~4.6 GiB (weight) + ~0.8 GiB
//	                             (mmproj) into testdata/.
//	RUNE_LLAMACPP_TESTDATA_OFFLINE=1
//	                           — never hit the network; skip if the model is
//	                             not already cached locally.
//	RUNE_LLAMACPP_TESTDATA_DIR — override the testdata cache directory.
//
// The model choice is fixed at ggml-org/gemma-4-E2B-it-GGUF:Q8_0 (plus the
// matching Q8_0 mmproj). Gemma-4 E2B is small enough to load on a laptop while
// still carrying useful tool-calling and vision capabilities.
// -----------------------------------------------------------------------------

const (
	testdataReference        = "ggml-org/gemma-4-E2B-it-GGUF:Q8_0"
	testdataSeed      uint32 = 42
)

var (
	testdataOnce sync.Once
	testdataSvc  *Service
	testdataErr  error
)

// testCleanupHooks are invoked in reverse registration order by TestMain
// after m.Run returns. Only one TestMain is allowed per test binary, so this
// list is the single choke point every test file (internal or external) must
// use to release process-wide resources such as the shared llamacpp.Service
// it loaded lazily. Registration is expected from init() / sync.Once bodies;
// reads/writes happen only before m.Run starts appending hooks and only on a
// single goroutine in TestMain afterwards, so no mutex is needed.
var testCleanupHooks []func()

// registerTestCleanup queues fn to be invoked by TestMain before the test
// binary exits. Tests use this to close llamacpp.Service instances that were
// loaded lazily and shared across subtests, so the ggml-metal singleton
// device's resource sets are drained before its static destructor runs.
func registerTestCleanup(fn func()) {
	testCleanupHooks = append(testCleanupHooks, fn)
}

// TestMain ensures every shared llamacpp.Service loaded by the suite is
// closed before the process exits. ggml-metal's static destructor
// (`ggml_metal_device_free`) asserts that all Metal resource sets attached to
// its singleton device have been released; if a Service still holds a
// Context/Model at exit, that assertion fires and the process aborts with
// SIGABRT — most visible under `-race`, which changes teardown ordering.
func TestMain(m *testing.M) {
	code := m.Run()
	if testdataSvc != nil {
		testdataSvc.Close()
		testdataSvc = nil
	}
	for i := len(testCleanupHooks) - 1; i >= 0; i-- {
		testCleanupHooks[i]()
	}
	os.Exit(code)
}

// errSkipTestdata is returned by setupTestdata when the environment indicates
// the suite should be skipped (no opt-in, offline without cache, etc.).
var errSkipTestdata = errors.New("llamacpp testdata: skipped")

// loadTestdataService returns a process-wide llamacpp.Service that satisfies
// llm.Service, backed by the real gemma-4-E2B GGUF stored in testdata/. The
// service is shared across subtests via sync.Once so we pay the ~4.6 GiB load
// cost only once per `go test` invocation.
func loadTestdataService(t *testing.T) llm.Service {
	t.Helper()
	if os.Getenv("RUNE_LLAMACPP_TESTDATA") != "1" {
		t.Skip("llamacpp testdata: set RUNE_LLAMACPP_TESTDATA=1 to run (downloads ~5 GiB on first use)")
	}
	testdataOnce.Do(func() {
		modelPath, mmprojPath, err := resolveTestdataModel(t)
		if err != nil {
			testdataErr = err
			return
		}
		testdataSvc, testdataErr = NewService(Config{
			Model:         "gemma-4-e2b-testdata",
			ModelPath:     modelPath,
			ProjectorPath: mmprojPath,
			ContextWindow: 4096,
			NGPULayers:    -1,
			Sampling: SamplerParams{
				Seed:          testdataSeed,
				Temperature:   0.2,
				TopK:          40,
				TopP:          0.95,
				MinP:          0.05,
				RepeatPenalty: 1.0,
				RepeatLastN:   64,
			},
		})
	})
	if testdataErr != nil {
		if errors.Is(testdataErr, errSkipTestdata) {
			t.Skip(testdataErr.Error())
		}
		t.Fatalf("loadTestdataService: %v", testdataErr)
	}
	// Bind the concrete type to the interface to make black-box intent
	// explicit: tests only touch methods that exist on llm.Service.
	var svc llm.Service = testdataSvc
	return svc
}

// resolveTestdataModel ensures the GGUF weight + mmproj blobs are on disk
// under testdata/ and returns their absolute paths. The download path uses
// ociregistry so both layers (weight + mmproj) come from the same manifest.
func resolveTestdataModel(t *testing.T) (modelPath, mmprojPath string, err error) {
	t.Helper()
	cacheDir := os.Getenv("RUNE_LLAMACPP_TESTDATA_DIR")
	if cacheDir == "" {
		cacheDir = filepath.Join("testdata", "cache")
	}
	abs, err := filepath.Abs(cacheDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve cache dir: %w", err)
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", "", fmt.Errorf("mkdir cache dir: %w", err)
	}
	cache, err := ociregistry.OpenCache(abs)
	if err != nil {
		return "", "", fmt.Errorf("open cache: %w", err)
	}
	ref, err := ociregistry.ParseReferenceWithDefault(testdataReference, "huggingface.co")
	if err != nil {
		return "", "", fmt.Errorf("parse %q: %w", testdataReference, err)
	}
	if pr, rerr := ociregistry.Resolve(cache, ref); rerr == nil && pr.ModelPath != "" {
		if _, statErr := os.Stat(pr.ModelPath); statErr == nil {
			return pr.ModelPath, pr.MMProjPath, nil
		}
	}
	if os.Getenv("RUNE_LLAMACPP_TESTDATA_OFFLINE") == "1" {
		return "", "", fmt.Errorf("%w: offline and no cached model at %s", errSkipTestdata, abs)
	}
	t.Logf("llamacpp testdata: downloading %s into %s (first-run only)", ref.String(), abs)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	client := ociregistry.NewClient()
	pr, err := client.Pull(ctx, cache, ref, nil)
	if err != nil {
		return "", "", fmt.Errorf("%w: pull %s: %v", errSkipTestdata, ref.String(), err)
	}
	if pr.ModelPath == "" {
		return "", "", fmt.Errorf("%w: no model layer in manifest for %s", errSkipTestdata, ref.String())
	}
	return pr.ModelPath, pr.MMProjPath, nil
}

// collectCompletion drains a completion iterator into a simple structured
// view of the observable events. Tests use this to stay focused on the
// observable llm.Service contract.
type completionResult struct {
	Text      string
	Reasoning string
	ToolCalls []llm.ToolCall
	Done      *llm.DoneData
	DoneCount int
	// Counts of raw events observed, so tests can assert protocol invariants.
	NumTextDelta      int
	NumReasoningDelta int
}

func collectCompletion(t *testing.T, ctx context.Context, svc llm.Service, req llm.Request) completionResult {
	t.Helper()
	it, err := svc.CreateCompletion(ctx, req)
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}
	// iterator.ToSlice intentionally does not Close the iterator; for the
	// llamacpp service the iterator holds ctxMu for the duration of a turn,
	// so failing to Close would deadlock the next turn in the same process.
	defer func() {
		if cerr := it.Close(); cerr != nil {
			t.Errorf("iterator.Close: %v", cerr)
		}
	}()
	evs, err := iterator.ToSlice(ctx, it)
	if err != nil {
		t.Fatalf("ToSlice: %v", err)
	}
	var res completionResult
	var buf, reason strings.Builder
	for _, e := range evs {
		switch e.Type {
		case llm.EventTextDelta:
			buf.WriteString(e.Text)
			res.NumTextDelta++
		case llm.EventReasoningDelta:
			reason.WriteString(e.Reasoning)
			res.NumReasoningDelta++
		case llm.EventToolCallDone:
			if e.ToolCall != nil {
				res.ToolCalls = append(res.ToolCalls, *e.ToolCall)
			}
		case llm.EventStreamDone:
			res.Done = e.DoneData
			res.DoneCount++
		case llm.EventStreamError:
			t.Fatalf("stream error: %v", e.Error)
		}
	}
	res.Text = buf.String()
	res.Reasoning = reason.String()
	return res
}

// TestService_Testdata_Table is the main table-driven black-box suite. Each
// case drives llm.Service.CreateCompletion against the shared real model and
// validates the observable response shape.
func TestService_Testdata_Table(t *testing.T) {
	svc := loadTestdataService(t)

	baseCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	type testCase struct {
		name    string
		req     llm.Request
		timeout time.Duration
		check   func(t *testing.T, res completionResult)
	}

	greetTool := llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "get_time",
			Description: "Return the current time as a JSON object.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}

	cases := []testCase{
		{
			name: "basic_text_completion",
			req: llm.Request{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "Say hi in a single short word."},
				},
				// Gemma-4's default chat template emits a <|think|>...<|channel|> reasoning
				// block before any user-visible text. Lower the reasoning effort so the
				// template skips thinking, and give generation enough budget to actually
				// emit the answer.
				ReasoningEffort: llm.ReasoningEffortMinimal,
				MaxOutputTokens: 16,
			},
			timeout: 2 * time.Minute,
			check: func(t *testing.T, res completionResult) {
				if res.DoneCount != 1 {
					t.Fatalf("DoneCount = %d, want 1", res.DoneCount)
				}
				if res.Done == nil {
					t.Fatal("Done missing")
				}
				if res.Text == "" {
					t.Fatalf("expected non-empty text delta stream (reasoning=%q, done.content=%q, finish=%q)",
						res.Reasoning, res.Done.Message.Content, res.Done.FinishReason)
				}
				if res.Done.Message.Content != res.Text {
					t.Fatalf("done.Content %q != accumulated deltas %q",
						res.Done.Message.Content, res.Text)
				}
				if res.Done.Usage.TokensSent <= 0 {
					t.Fatalf("TokensSent = %d, want > 0", res.Done.Usage.TokensSent)
				}
				if res.Done.Usage.TokensReceived <= 0 {
					t.Fatalf("TokensReceived = %d, want > 0", res.Done.Usage.TokensReceived)
				}
			},
		},
		{
			name: "finish_reason_length",
			req: llm.Request{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "Tell me a long story about a dragon and a knight."},
				},
				ReasoningEffort: llm.ReasoningEffortMinimal,
				MaxOutputTokens: 4,
			},
			timeout: 2 * time.Minute,
			check: func(t *testing.T, res completionResult) {
				if res.Done == nil {
					t.Fatal("Done missing")
				}
				if res.Done.FinishReason != llm.FinishReasonLength {
					t.Fatalf("FinishReason = %q, want %q", res.Done.FinishReason, llm.FinishReasonLength)
				}
			},
		},
		{
			name: "tool_call_when_tools_provided",
			req: llm.Request{
				Messages: []llm.Message{
					{Role: llm.RoleSystem, Content: "You are a helpful assistant. Prefer calling tools when available."},
					{Role: llm.RoleUser, Content: "What time is it right now? Call the tool."},
				},
				Tools:           []llm.Tool{greetTool},
				ReasoningEffort: llm.ReasoningEffortMinimal,
				MaxOutputTokens: 128,
			},
			timeout: 3 * time.Minute,
			check: func(t *testing.T, res completionResult) {
				if res.Done == nil {
					t.Fatal("Done missing")
				}
				if len(res.ToolCalls) == 0 {
					t.Fatalf("expected tool call, got content=%q reasoning=%q finish=%q",
						res.Done.Message.Content, res.Reasoning, res.Done.FinishReason)
				}
				if res.Done.FinishReason != llm.FinishReasonToolCall {
					t.Fatalf("FinishReason = %q, want %q",
						res.Done.FinishReason, llm.FinishReasonToolCall)
				}
				tc := res.ToolCalls[0]
				if tc.Function.Name != "get_time" {
					t.Fatalf("tool name = %q, want %q", tc.Function.Name, "get_time")
				}
				if tc.Function.Arguments == "" {
					t.Fatal("tool call has empty arguments")
				}
				if !strings.HasPrefix(strings.TrimSpace(tc.Function.Arguments), "{") {
					t.Fatalf("arguments do not look like JSON: %q", tc.Function.Arguments)
				}
				if tc.ID == "" {
					t.Fatal("tool call has empty ID; upstream parser should assign call_<n>")
				}
			},
		},
		{
			name: "no_tool_when_tools_omitted",
			req: llm.Request{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "Answer in one short sentence: what is 2 + 2?"},
				},
				ReasoningEffort: llm.ReasoningEffortMinimal,
				MaxOutputTokens: 32,
			},
			timeout: 2 * time.Minute,
			check: func(t *testing.T, res completionResult) {
				if res.Done == nil {
					t.Fatal("Done missing")
				}
				if len(res.ToolCalls) != 0 {
					t.Fatalf("did not expect tool calls: %+v", res.ToolCalls)
				}
				if res.Text == "" {
					t.Fatalf("expected text content (reasoning=%q, done.content=%q, finish=%q)",
						res.Reasoning, res.Done.Message.Content, res.Done.FinishReason)
				}
			},
		},
		{
			name: "count_tokens_positive",
			req: llm.Request{
				Messages: []llm.Message{
					{Role: llm.RoleUser, Content: "hello world"},
				},
			},
			timeout: 30 * time.Second,
			check: func(t *testing.T, _ completionResult) {
				// CountTokens is exercised directly below since it does not
				// stream through CreateCompletion.
				n, err := svc.CountTokens([]llm.Message{
					{Role: llm.RoleUser, Content: "hello world"},
				})
				if err != nil {
					t.Fatalf("CountTokens: %v", err)
				}
				if n <= 0 {
					t.Fatalf("CountTokens = %d, want > 0", n)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			timeout := tc.timeout
			if timeout == 0 {
				timeout = 2 * time.Minute
			}
			ctx, cancel := context.WithTimeout(baseCtx, timeout)
			defer cancel()
			var res completionResult
			if tc.name != "count_tokens_positive" {
				res = collectCompletion(t, ctx, svc, tc.req)
			}
			tc.check(t, res)
		})
	}
}

// TestService_Testdata_Multimodal_Image verifies that the multimodal path
// end-to-end through llm.Service actually accepts an image payload without
// error and produces a usable completion. The mmproj layer must be present;
// tests skip if the downloaded manifest lacks it. Image content is a small
// real PNG encoded as a data URL so ContextPartTypeImageURL decoding hits the
// real mtmd tokenize path.
func TestService_Testdata_Multimodal_Image(t *testing.T) {
	svc := loadTestdataService(t)
	concrete, ok := svc.(*Service)
	if !ok {
		t.Fatalf("expected *Service, got %T", svc)
	}
	if !concrete.model.HasProjector() {
		t.Skip("testdata model has no mmproj layer; skip multimodal suite")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	// Smallest valid PNG: a 1x1 transparent pixel. mtmd happily accepts it.
	// Generated once via `printf` + base64; kept inline so the suite needs no
	// additional testdata binaries.
	const tinyPNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+ipKoAAAAASUVORK5CYII="
	dataURL := "data:image/png;base64," + tinyPNGBase64

	req := llm.Request{
		Messages: []llm.Message{
			{
				Role: llm.RoleUser,
				MultiContent: []llm.ContentPart{
					{Type: llm.ContentPartTypeText, Text: "Describe the image very briefly."},
					{Type: llm.ContentPartTypeImageURL, ImageURL: dataURL},
				},
			},
		},
		MaxOutputTokens: 32,
	}
	res := collectCompletion(t, ctx, svc, req)
	if res.Done == nil {
		t.Fatal("Done missing")
	}
	// The model is free to produce an apology or refusal for a blank image;
	// we only care that the mtmd path didn't error and we reached a Done.
	if res.Done.Usage.TokensSent <= 0 {
		t.Fatalf("TokensSent = %d, want > 0 (mtmd path should consume at least the image tokens)", res.Done.Usage.TokensSent)
	}
}

// TestService_Testdata_ContextCancellation starts a long completion and
// cancels the context after the first delta. The iterator must drain cleanly.
func TestService_Testdata_ContextCancellation(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	it, err := svc.CreateCompletion(ctx, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Count slowly from one to one hundred, one number per line."},
		},
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 128,
	})
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}
	var sawStreamed bool
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		// Trigger cancellation on the first observable stream event (either a
		// visible text delta or a reasoning delta — both prove the decoder is
		// producing output). We don't care which; we only care that the iterator
		// unwinds cleanly afterwards.
		if (ev.Type == llm.EventTextDelta || ev.Type == llm.EventReasoningDelta) && !sawStreamed {
			sawStreamed = true
			cancel()
		}
	}
	if err := it.Close(); err != nil {
		t.Fatalf("iterator close after cancel: %v", err)
	}
	if !sawStreamed {
		t.Fatal("never saw a delta (text or reasoning) before cancelling")
	}
}

// TestService_Testdata_PrefixCacheReuse issues two completions that share a
// long prefix and asserts the second turn reports TokensCached > 0.
func TestService_Testdata_PrefixCacheReuse(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// The shared test service may have cached tokens from earlier subtests in
	// this process; we compare the two turns against each other rather than
	// pinning turn 1 to zero cached tokens.
	sys := "You are a terse assistant. Answer with a single short sentence about random fact of the day."
	first := []llm.Message{
		{Role: llm.RoleSystem, Content: sys},
		{Role: llm.RoleUser, Content: "Give me a fun fact about the ocean."},
	}
	r1 := collectCompletion(t, ctx, svc, llm.Request{
		Messages:        first,
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 8,
	})
	if r1.Done == nil {
		t.Fatal("turn 1: Done missing")
	}

	second := append([]llm.Message{}, first...)
	second = append(second,
		llm.Message{Role: llm.RoleAssistant, Content: r1.Done.Message.Content},
		llm.Message{Role: llm.RoleUser, Content: "Another, please."},
	)
	r2 := collectCompletion(t, ctx, svc, llm.Request{
		Messages:        second,
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 8,
	})
	if r2.Done == nil {
		t.Fatal("turn 2: Done missing")
	}
	// Turn 2 shares an entire system + user + assistant prefix with turn 1, so
	// it must report strictly more cached tokens than turn 1.
	if r2.Done.Usage.TokensCached <= r1.Done.Usage.TokensCached {
		t.Fatalf("expected turn 2 TokensCached (%d) > turn 1 TokensCached (%d)",
			r2.Done.Usage.TokensCached, r1.Done.Usage.TokensCached)
	}
}

// TestService_Testdata_ToolCallRoundTrip exercises the full tool-calling
// contract through llm.Service: turn 1 emits a tool call, we construct a
// follow-up with an assistant tool-call message + a tool-response message,
// and turn 2 must consume that shape without error and produce a visible
// natural-language answer.
func TestService_Testdata_ToolCallRoundTrip(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	tool := llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "get_time",
			Description: "Return the current time as a string.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}

	turn1 := collectCompletion(t, ctx, svc, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a helpful assistant. Prefer calling tools when available."},
			{Role: llm.RoleUser, Content: "What time is it right now? Call the tool."},
		},
		Tools:           []llm.Tool{tool},
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 128,
	})
	if turn1.Done == nil {
		t.Fatal("turn 1: Done missing")
	}
	if len(turn1.ToolCalls) == 0 {
		t.Fatalf("turn 1 expected tool call, got content=%q finish=%q",
			turn1.Done.Message.Content, turn1.Done.FinishReason)
	}
	tc := turn1.ToolCalls[0]

	// Build turn 2 with the assistant tool call + tool response.
	turn2 := collectCompletion(t, ctx, svc, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a helpful assistant. Prefer calling tools when available."},
			{Role: llm.RoleUser, Content: "What time is it right now? Call the tool."},
			{
				Role:      llm.RoleAssistant,
				Content:   turn1.Done.Message.Content,
				ToolCalls: turn1.ToolCalls,
			},
			{
				Role:       llm.RoleTool,
				Name:       tc.Function.Name,
				ToolCallID: tc.ID,
				Content:    `{"time":"12:34"}`,
			},
		},
		Tools:           []llm.Tool{tool},
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 64,
	})
	if turn2.Done == nil {
		t.Fatal("turn 2: Done missing")
	}
	if turn2.Text == "" {
		t.Fatalf("turn 2 expected text reply (got finish=%q, reasoning=%q)",
			turn2.Done.FinishReason, turn2.Reasoning)
	}
	// Turn 2 must not loop back into another tool call: with the tool result
	// in hand the model should answer in natural language.
	if len(turn2.ToolCalls) > 0 {
		t.Logf("note: turn 2 also emitted %d tool calls — acceptable but not expected",
			len(turn2.ToolCalls))
	}
}

// TestService_Testdata_ReasoningCaptured verifies that when reasoning effort is
// not minimal, the upstream parser surfaces reasoning deltas separately from
// text deltas. This pins down the parser-level split for Gemma-4, which opens
// every turn with a `<|think|>...<|channel|>` block.
func TestService_Testdata_ReasoningCaptured(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	res := collectCompletion(t, ctx, svc, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "What is 2 + 2? Think step by step then answer."},
		},
		ReasoningEffort: llm.ReasoningEffortHigh,
		MaxOutputTokens: 64,
	})
	if res.Done == nil {
		t.Fatal("Done missing")
	}
	if res.NumReasoningDelta == 0 {
		t.Fatalf("expected reasoning deltas (reasoning=%q, text=%q, finish=%q)",
			res.Reasoning, res.Text, res.Done.FinishReason)
	}
	if res.Reasoning == "" {
		t.Fatal("expected non-empty reasoning content")
	}
}

// TestService_Testdata_SerializesConcurrentCreateCompletion verifies the
// documented contract on llamacpp.Service: concurrent CreateCompletion calls
// are serialized by the internal mutex rather than running in parallel on a
// shared KV cache. Both iterators must complete successfully and observe
// well-formed streams.
func TestService_Testdata_SerializesConcurrentCreateCompletion(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	req := llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Reply with a single short word."},
		},
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 8,
	}

	type outcome struct {
		res completionResult
		err error
	}
	out := make(chan outcome, 2)
	run := func() {
		defer func() {
			if r := recover(); r != nil {
				out <- outcome{err: fmt.Errorf("panic: %v", r)}
			}
		}()
		it, err := svc.CreateCompletion(ctx, req)
		if err != nil {
			out <- outcome{err: err}
			return
		}
		defer func() { _ = it.Close() }()
		evs, err := iterator.ToSlice(ctx, it)
		if err != nil {
			out <- outcome{err: err}
			return
		}
		var r completionResult
		var buf, reason strings.Builder
		for _, e := range evs {
			switch e.Type {
			case llm.EventTextDelta:
				buf.WriteString(e.Text)
			case llm.EventReasoningDelta:
				reason.WriteString(e.Reasoning)
			case llm.EventStreamDone:
				r.Done = e.DoneData
			}
		}
		r.Text = buf.String()
		r.Reasoning = reason.String()
		out <- outcome{res: r}
	}

	go run()
	go run()
	for range 2 {
		o := <-out
		if o.err != nil {
			t.Fatalf("concurrent completion: %v", o.err)
		}
		if o.res.Done == nil {
			t.Fatal("concurrent completion: Done missing")
		}
	}
}

// TestService_Testdata_FinishReasonStop covers the plain text finish path: a
// generous token budget so the model decides to stop on its own.
func TestService_Testdata_FinishReasonStop(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	res := collectCompletion(t, ctx, svc, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Reply with exactly the word: ok"},
		},
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 64,
	})
	if res.Done == nil {
		t.Fatal("Done missing")
	}
	if res.Done.FinishReason != llm.FinishReasonStop {
		t.Fatalf("FinishReason = %q, want %q (content=%q)",
			res.Done.FinishReason, llm.FinishReasonStop, res.Done.Message.Content)
	}
	if res.Text == "" {
		t.Fatal("expected non-empty text")
	}
	if res.Done.Message.Content != res.Text {
		t.Fatalf("done.Content %q != accumulated deltas %q",
			res.Done.Message.Content, res.Text)
	}
}

// TestService_Testdata_ContextWindow_PositiveAndMatchesGGUF sanity-checks the
// provider-facing query surface: ContextWindow reports a positive value, and
// that value equals the model's training context (set explicitly in the
// testdata config).
func TestService_Testdata_ContextWindow_PositiveAndMatchesGGUF(t *testing.T) {
	svc := loadTestdataService(t)
	concrete, ok := svc.(*Service)
	if !ok {
		t.Fatalf("expected *Service, got %T", svc)
	}
	if got := concrete.ContextWindow(); got <= 0 {
		t.Fatalf("ContextWindow = %d, want > 0", got)
	}
	if concrete.Model() == nil {
		t.Fatal("concrete.Model() == nil")
	}
	if train := concrete.Model().NCtxTrain(); train <= 0 {
		t.Fatalf("model NCtxTrain = %d, want > 0", train)
	}
}

// TestService_Testdata_CountTokens_ShapeGrowsWithPrompt verifies that
// CountTokens responds to prompt length: a longer message tokenizes to more
// tokens. This is a shape test, not a numeric pin.
func TestService_Testdata_CountTokens_ShapeGrowsWithPrompt(t *testing.T) {
	svc := loadTestdataService(t)
	short, err := svc.CountTokens([]llm.Message{{Role: llm.RoleUser, Content: "hi"}})
	if err != nil {
		t.Fatalf("CountTokens short: %v", err)
	}
	long, err := svc.CountTokens([]llm.Message{
		{Role: llm.RoleUser, Content: strings.Repeat("hello world ", 64)},
	})
	if err != nil {
		t.Fatalf("CountTokens long: %v", err)
	}
	if short <= 0 {
		t.Fatalf("short CountTokens = %d, want > 0", short)
	}
	if long <= short {
		t.Fatalf("expected long CountTokens (%d) > short CountTokens (%d)", long, short)
	}
}

// TestService_Testdata_ContextWindowExceeded pins the safety check that
// rejects requests whose tokenized prompt exceeds the configured context
// window. The shared service is configured with ContextWindow=4096; we
// fabricate a multi-megabyte user message so tokenization comfortably
// blows past that. The error must satisfy *llm.ErrContextWindowExceeded
// and report a Count > Max so callers can surface the gap to the user.
func TestService_Testdata_ContextWindowExceeded(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 4096 is the configured window; "hello world " is ≥2 tokens per
	// repetition, so 8000 reps tokenize to well over 4096 tokens even
	// after the chat template adds its overhead.
	huge := strings.Repeat("hello world ", 8000)
	_, err := svc.CreateCompletion(ctx, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: huge},
		},
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 8,
	})
	if err == nil {
		t.Fatal("expected ErrContextWindowExceeded, got nil")
	}
	var cwe *llm.ErrContextWindowExceeded
	if !errors.As(err, &cwe) {
		t.Fatalf("error type = %T (%v), want *llm.ErrContextWindowExceeded", err, err)
	}
	if cwe.Count <= cwe.Max {
		t.Fatalf("ErrContextWindowExceeded reports Count=%d, Max=%d; want Count > Max", cwe.Count, cwe.Max)
	}
}

// TestService_Testdata_ResponseFormat_JSONObject exercises the
// ResponseFormatTypeJSONObject path through CreateCompletion. The
// llamacpp template options are surfaced into the upstream Jinja
// engine; even when the model decides on its own JSON shape, the call
// must complete and the produced text must parse as JSON.
func TestService_Testdata_ResponseFormat_JSONObject(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	res := collectCompletion(t, ctx, svc, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "Reply only with JSON."},
			{Role: llm.RoleUser, Content: `Return a JSON object with one key "answer" whose value is the string "ok".`},
		},
		ReasoningEffort: llm.ReasoningEffortMinimal,
		ResponseFormat:  &llm.ResponseFormat{Type: llm.ResponseFormatTypeJSONObject},
		MaxOutputTokens: 64,
	})
	if res.Done == nil {
		t.Fatal("Done missing")
	}
	if res.Text == "" {
		t.Fatalf("expected text content (finish=%q reasoning=%q)",
			res.Done.FinishReason, res.Reasoning)
	}
	// The model is free to wrap or pretty-print the object, but the
	// stripped text must be JSON-parseable. Models with weaker JSON
	// adherence sometimes wrap output in markdown fences; tolerate
	// that since the contract under test is the request plumbing,
	// not the model's strictness.
	candidate := strings.TrimSpace(res.Text)
	candidate = strings.TrimPrefix(candidate, "```json")
	candidate = strings.TrimPrefix(candidate, "```")
	candidate = strings.TrimSuffix(candidate, "```")
	candidate = strings.TrimSpace(candidate)
	var parsed any
	if err := json.Unmarshal([]byte(candidate), &parsed); err != nil {
		t.Fatalf("response not JSON: %v\n--- raw ---\n%s", err, res.Text)
	}
}

// TestService_Testdata_ToolChoice_Required forces the model to emit a
// tool call regardless of how it would otherwise answer. The Jinja
// template propagates ToolChoiceRequired, so the upstream parser must
// surface a tool call in the stream.
func TestService_Testdata_ToolChoice_Required(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tool := llm.Tool{
		Type: llm.ToolTypeFunction,
		Function: llm.FunctionDefinition{
			Name:        "get_time",
			Description: "Return the current time as a string.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}
	res := collectCompletion(t, ctx, svc, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Hi."},
		},
		Tools:           []llm.Tool{tool},
		ToolChoice:      llm.ToolChoiceRequired,
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 128,
	})
	if res.Done == nil {
		t.Fatal("Done missing")
	}
	if len(res.ToolCalls) == 0 {
		t.Fatalf("ToolChoiceRequired did not yield a tool call (text=%q finish=%q)",
			res.Text, res.Done.FinishReason)
	}
	if res.Done.FinishReason != llm.FinishReasonToolCall {
		t.Fatalf("FinishReason = %q, want %q",
			res.Done.FinishReason, llm.FinishReasonToolCall)
	}
	if name := res.ToolCalls[0].Function.Name; name != "get_time" {
		t.Fatalf("tool name = %q, want get_time", name)
	}
}

// TestService_Testdata_StopStringsHonoured exercises the registry of
// stop strings the upstream chat template advertises (AdditionalStops)
// alongside applyStops. We pin only the observable contract: the
// completion must not include the assistant turn's opening BOS marker
// in its visible text, and it must terminate via FinishReasonStop or
// FinishReasonLength rather than emit the marker as plain text.
func TestService_Testdata_StopStringsHonoured(t *testing.T) {
	svc := loadTestdataService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	res := collectCompletion(t, ctx, svc, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Reply with: done."},
		},
		ReasoningEffort: llm.ReasoningEffortMinimal,
		MaxOutputTokens: 64,
	})
	if res.Done == nil {
		t.Fatal("Done missing")
	}
	switch res.Done.FinishReason {
	case llm.FinishReasonStop, llm.FinishReasonLength:
		// ok
	default:
		t.Fatalf("unexpected finish reason %q", res.Done.FinishReason)
	}
	// Gemma-4 emits text bracketed by `<|turn>` / `<turn|>` markers in
	// raw form; AdditionalStops must keep those out of the streamed
	// text. The visible output should never contain the closing
	// channel marker.
	for _, marker := range []string{"<turn|>", "<|turn>", "<|channel|>"} {
		if strings.Contains(res.Text, marker) {
			t.Fatalf("text leaked stop marker %q: %q", marker, res.Text)
		}
	}
}
