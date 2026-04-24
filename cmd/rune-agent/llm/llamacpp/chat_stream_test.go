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
	"unsafe"
)

// feedStream pumps raw text into a ChatStream one byte at a time and
// concatenates the resulting deltas by kind. Byte-at-a-time feeding
// stresses the upstream partial-parse contract — the real model emits
// tokens one at a time and the parser must not surface partial markers
// as content.
func feedStream(t *testing.T, h *ChatTemplateHandle, raw string) (text, reasoning string, tools []ChatStreamEvent) {
	t.Helper()
	stream, err := NewChatStream(h)
	if err != nil {
		t.Fatalf("NewChatStream: %v", err)
	}
	defer stream.Close()

	var txt, rea strings.Builder
	drain := func() {
		for {
			ev, ok := stream.Next()
			if !ok {
				return
			}
			switch ev.Kind {
			case ChatStreamEventTextDelta:
				txt.WriteString(ev.Text)
			case ChatStreamEventReasoningDelta:
				rea.WriteString(ev.Text)
			case ChatStreamEventToolCallDelta:
				tools = append(tools, ev)
			}
		}
	}

	for i := 0; i < len(raw); i++ {
		if err := stream.Feed(raw[i:i+1], true); err != nil {
			t.Fatalf("Feed: %v", err)
		}
		drain()
	}
	if err := stream.Feed("", false); err != nil {
		t.Fatalf("Feed(final): %v", err)
	}
	drain()
	return txt.String(), rea.String(), tools
}

// gemma4Template is the minimal Jinja template needed for
// common_chat_templates_apply to recognise Gemma 4 (upstream detects by
// searching for the literal "'<|tool_call>call:'" in the source;
// common/chat.cpp:2120). We don't need the full tool grammar — a bare
// mention is enough to route to common_chat_params_init_gemma4, which
// installs the reasoning-channel parser we're exercising here.
const gemma4Template = `{%- for message in messages -%}` +
	`{{- '<|turn>' + (message['role'] if message['role'] != 'assistant' else 'model') + '\n' -}}` +
	`{{- message['content'] | trim -}}` +
	`{{- '<turn|>\n' -}}` +
	`{%- endfor -%}` +
	`{%- if add_generation_prompt -%}` +
	`{{- '<|turn>model\n' -}}` +
	`{%- endif -%}` +
	`{#- tool-call marker: '<|tool_call>call:' — presence of this literal` +
	` triggers Gemma 4 detection in common/chat.cpp -#}`

// TestChatStream_Gemma4_Reasoning reproduces the original bug: Gemma 4
// emits '<|channel>thought ...<channel|>' wrappers around its chain of
// thought. The upstream parser peels those off into
// reasoning_content_delta; we surface them as ChatStreamEventReasoningDelta.
// Before this wiring the markers and inner monologue leaked into the
// user-visible message.
func TestChatStream_Gemma4_Reasoning(t *testing.T) {
	t.Parallel()
	var m *Model
	h, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{
			TemplateOverride:    gemma4Template,
			AddGenerationPrompt: true,
			EnableThinking:      true,
			ReasoningFmt:        ReasoningAuto,
		},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer h.Close()

	// Shape observed from the real model output. Gemma 4 emits the
	// thought channel header, the reasoning body, the closing tag, and
	// then the user-visible answer. (The template's add_generation_prompt
	// only emits up to '<|turn>model\n'; the model itself emits the
	// thought channel.)
	raw := "<|channel>thought\nThe user said \"hello\".<channel|>Hello! How can I help?"
	text, reasoning, tools := feedStream(t, h, raw)
	if len(tools) != 0 {
		t.Errorf("unexpected tool events: %+v", tools)
	}
	if reasoning != `The user said "hello".` {
		t.Errorf("reasoning = %q", reasoning)
	}
	if text != "Hello! How can I help?" {
		t.Errorf("text = %q", text)
	}
}

// TestChatStream_NoReasoning_PassesThrough verifies that a model whose
// template has no reasoning channel (plain ChatML) still produces text
// deltas intact.
func TestChatStream_NoReasoning_PassesThrough(t *testing.T) {
	t.Parallel()
	const chatmlTemplate = `{%- for message in messages -%}` +
		`{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}` +
		`{%- endfor -%}` +
		`{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`

	var m *Model
	h, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{
			TemplateOverride:    chatmlTemplate,
			AddGenerationPrompt: true,
		},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer h.Close()

	text, reasoning, tools := feedStream(t, h, "Hello there!")
	if reasoning != "" {
		t.Errorf("unexpected reasoning: %q", reasoning)
	}
	if len(tools) != 0 {
		t.Errorf("unexpected tool events: %+v", tools)
	}
	if text != "Hello there!" {
		t.Errorf("text = %q", text)
	}
}

// TestChatTemplateHandle_Prompt smokes the handle → prompt round-trip
// against ChatML so we can verify prompt rendering without a live
// model.
func TestChatTemplateHandle_Prompt(t *testing.T) {
	t.Parallel()
	const tmpl = `{%- for message in messages -%}` +
		`{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}` +
		`{%- endfor -%}` +
		`{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`

	var m *Model
	h, err := m.OpenChatTemplate(
		[]ChatMessage{
			{Role: "system", Content: "be terse"},
			{Role: "user", Content: "hi"},
		},
		ChatTemplateOptions{TemplateOverride: tmpl, AddGenerationPrompt: true},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer h.Close()

	prompt, err := h.Prompt()
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	want := "<|im_start|>system\nbe terse<|im_end|>\n" +
		"<|im_start|>user\nhi<|im_end|>\n" +
		"<|im_start|>assistant\n"
	if prompt != want {
		t.Fatalf("prompt = %q, want %q", prompt, want)
	}
}

func TestOpenChatTemplate_AssistantToolCalls_NoCgoPointerPanic(t *testing.T) {
	t.Parallel()
	const tmpl = `{%- for message in messages -%}` +
		`{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}` +
		`{%- endfor -%}` +
		`{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("OpenChatTemplate panicked: %v", r)
		}
	}()

	var m *Model
	h, err := m.OpenChatTemplate(
		[]ChatMessage{
			{Role: "user", Content: "hi"},
			{
				Role: "assistant",
				ToolCalls: []ChatToolCall{{
					Name:      "get_time",
					Arguments: `{}`,
					ID:        "call_1",
				}},
			},
		},
		ChatTemplateOptions{TemplateOverride: tmpl, AddGenerationPrompt: true},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer h.Close()
	if _, err := h.Prompt(); err != nil {
		t.Fatalf("Prompt: %v", err)
	}
}

// TestBuildCRichChatMessages_ZerosOptionalFields pins down the fix for
// a crash observed when resuming a long agent dialogue (see segfault
// report). buildCRichChatMessages historically used C.malloc, which
// returns uninitialized memory, and only populated the optional
// content_parts / tool_calls (and their counts) when the corresponding
// Go-side slice was non-empty. Messages without those parts therefore
// inherited whatever bytes malloc returned; when the bytes happened to
// contain a non-zero n_content_parts paired with a garbage
// content_parts pointer, the C-side to_common_chat_msg would walk that
// pointer as an array and segfault.
//
// The function must zero-initialize its allocation so optional fields
// read as (nil, 0) regardless of heap state. This test enforces the
// contract directly: after buildCRichChatMessages returns, every
// message whose Go-side value had no ContentParts or ToolCalls must
// expose nil/0 for both the pointer and the count field. The test is
// independent of the specific allocator (some libc implementations
// happen to zero freed blocks; we must not depend on that).
func TestBuildCRichChatMessages_ZerosOptionalFields(t *testing.T) {
	t.Parallel()
	msgs := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u"},
		{Role: "assistant", Content: "a"},
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "a2"},
	}
	base, cleanup := buildCRichChatMessages(msgs)
	defer cleanup()
	if base == nil {
		t.Fatal("buildCRichChatMessages returned nil base")
	}
	for i := range msgs {
		cpPtr, cpLen, tcPtr, tcLen := cMsgOptionalFieldsSnapshot(
			unsafe.Pointer(base), i, len(msgs))
		if cpPtr != 0 || cpLen != 0 || tcPtr != 0 || tcLen != 0 {
			t.Errorf("msg[%d] optional fields not zeroed: "+
				"content_parts=%#x/%d tool_calls=%#x/%d",
				i, cpPtr, cpLen, tcPtr, tcLen)
		}
	}
}

// TestChatTemplateHandle_CloseIsIdempotent guards the only contract the
// runtime relies on: Close on a nil receiver, on a fresh zero-value, and
// twice in a row must all be no-ops. Every other method assumes a
// constructed, open handle and is allowed to crash if called otherwise.
func TestChatTemplateHandle_CloseIsIdempotent(t *testing.T) {
	t.Parallel()

	var nilHandle *ChatTemplateHandle
	nilHandle.Close() // nil receiver
	(&ChatTemplateHandle{}).Close()

	const tmpl = `{%- for message in messages -%}` +
		`{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}` +
		`{%- endfor -%}` +
		`{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`
	var m *Model
	h, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{TemplateOverride: tmpl, AddGenerationPrompt: true},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	h.Close()
	h.Close() // double-close is a no-op
}

// TestChatStream_CloseIsIdempotent matches TestChatTemplateHandle_CloseIsIdempotent
// for ChatStream: the only out-of-band guarantee is that Close cannot crash
// when called on nil/zero/closed values.
func TestChatStream_CloseIsIdempotent(t *testing.T) {
	t.Parallel()

	var nilStream *ChatStream
	nilStream.Close()
	(&ChatStream{}).Close()

	const tmpl = `{%- for message in messages -%}` +
		`{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}` +
		`{%- endfor -%}` +
		`{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`
	var m *Model
	h, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{TemplateOverride: tmpl, AddGenerationPrompt: true},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer h.Close()
	s, err := NewChatStream(h)
	if err != nil {
		t.Fatalf("NewChatStream: %v", err)
	}
	s.Close()
	s.Close() // double-close is a no-op
}

// TestOpenChatTemplate_NilModelNoOverridePanics pins the new construction
// contract: passing neither a *Model nor a TemplateOverride is a
// programmer error and must panic rather than be tolerated.
func TestOpenChatTemplate_NilModelNoOverridePanics(t *testing.T) {
	t.Parallel()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on (nil model, no template override)")
		}
	}()
	var m *Model
	_, _ = m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{},
	)
}

// TestChatTemplateHandle_PromptBufferGrowth feeds a synthetic message
// whose rendered prompt easily exceeds the initial 1024-byte buffer in
// ChatTemplateHandle.Prompt, exercising the resize loop. Without the
// loop the function would silently truncate.
func TestChatTemplateHandle_PromptBufferGrowth(t *testing.T) {
	t.Parallel()
	const tmpl = `{%- for message in messages -%}` +
		`{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}` +
		`{%- endfor -%}` +
		`{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`

	long := strings.Repeat("xy", 2048) // 4 KiB body, beyond 1024 initial.
	var m *Model
	h, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: long}},
		ChatTemplateOptions{TemplateOverride: tmpl, AddGenerationPrompt: true},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer h.Close()
	got, err := h.Prompt()
	if err != nil {
		t.Fatalf("Prompt: %v", err)
	}
	if !strings.Contains(got, long) {
		t.Fatalf("rendered prompt missing the long body (len=%d)", len(got))
	}
	if len(got) <= 1024 {
		t.Fatalf("prompt unexpectedly short: len=%d", len(got))
	}
}

// TestChatStream_FeedEmpty_Repeated proves Feed("") is a safe no-op
// regardless of repetition. The byte-at-a-time streaming path can pass
// empty strings on cancellation boundaries; we must not crash or leak
// queued state.
func TestChatStream_FeedEmpty_Repeated(t *testing.T) {
	t.Parallel()
	const tmpl = `{%- for message in messages -%}` +
		`{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}` +
		`{%- endfor -%}` +
		`{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`
	var m *Model
	h, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{TemplateOverride: tmpl, AddGenerationPrompt: true},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	defer h.Close()
	stream, err := NewChatStream(h)
	if err != nil {
		t.Fatalf("NewChatStream: %v", err)
	}
	defer stream.Close()
	for range 50 {
		if err := stream.Feed("", true); err != nil {
			t.Fatalf("Feed(\"\"): %v", err)
		}
	}
	if ev, ok := stream.Next(); ok {
		t.Fatalf("Next after empty feeds returned event: %+v", ev)
	}
}

// TestChatStream_DoubleClose verifies idempotent Close on both the
// stream and its template handle.
func TestChatStream_DoubleClose(t *testing.T) {
	t.Parallel()
	const tmpl = `{%- for message in messages -%}` +
		`{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}` +
		`{%- endfor -%}` +
		`{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`
	var m *Model
	h, err := m.OpenChatTemplate(
		[]ChatMessage{{Role: "user", Content: "hi"}},
		ChatTemplateOptions{TemplateOverride: tmpl, AddGenerationPrompt: true},
	)
	if err != nil {
		t.Fatalf("OpenChatTemplate: %v", err)
	}
	stream, err := NewChatStream(h)
	if err != nil {
		h.Close()
		t.Fatalf("NewChatStream: %v", err)
	}
	stream.Close()
	stream.Close()
	h.Close()
	h.Close()
}

// TestOpenChatTemplate_ReasoningFmtMatrix smokes every {ReasoningFormat,
// EnableThinking} combination on a ChatML template so we don't regress
// the option-plumbing (and so the C-side rune_chat_template_open
// accepts every documented enum value).
func TestOpenChatTemplate_ReasoningFmtMatrix(t *testing.T) {
	t.Parallel()
	const tmpl = `{%- for message in messages -%}` +
		`{{- '<|im_start|>' + message['role'] + '\n' + message['content'] + '<|im_end|>\n' -}}` +
		`{%- endfor -%}` +
		`{%- if add_generation_prompt -%}{{- '<|im_start|>assistant\n' -}}{%- endif -%}`

	formats := []struct {
		name string
		fmt  ReasoningFormat
	}{
		{"none", ReasoningNone},
		{"auto", ReasoningAuto},
		{"deepseek", ReasoningDeepseek},
		{"deepseek_legacy", ReasoningDeepseekLegacy},
	}
	for _, f := range formats {
		for _, thinking := range []bool{false, true} {
			name := f.name
			if thinking {
				name += "_thinking"
			}
			t.Run(name, func(t *testing.T) {
				var m *Model
				h, err := m.OpenChatTemplate(
					[]ChatMessage{{Role: "user", Content: "hi"}},
					ChatTemplateOptions{
						TemplateOverride:    tmpl,
						AddGenerationPrompt: true,
						ReasoningFmt:        f.fmt,
						EnableThinking:      thinking,
					},
				)
				if err != nil {
					t.Fatalf("OpenChatTemplate: %v", err)
				}
				defer h.Close()
				if _, err := h.Prompt(); err != nil {
					t.Fatalf("Prompt: %v", err)
				}
			})
		}
	}
}

// TestToolChoiceToC_Table pins the string→C int mapping consumed by
// rune_chat_template_open. The set is small but mistakes here silently
// route every request through the wrong choice.
func TestToolChoiceToC_Table(t *testing.T) {
	t.Parallel()
	if got := toolChoiceToC("required"); got != 1 {
		t.Fatalf("required => %d, want 1", got)
	}
	if got := toolChoiceToC("none"); got != 2 {
		t.Fatalf("none => %d, want 2", got)
	}
	if got := toolChoiceToC("auto"); got != 0 {
		t.Fatalf("auto => %d, want 0", got)
	}
	if got := toolChoiceToC(""); got != 0 {
		t.Fatalf("\"\" => %d, want 0", got)
	}
	if got := toolChoiceToC("nonsense"); got != 0 {
		t.Fatalf("nonsense => %d, want 0", got)
	}
}

// TestBuildCTools_Table covers the empty / nil / multi paths through
// buildCTools. We assert pointers without crossing the cgo boundary
// from the test file by relying solely on the public Go interface.
func TestBuildCTools_Table(t *testing.T) {
	t.Parallel()

	t.Run("nil", func(t *testing.T) {
		ptr, free := buildCTools(nil)
		defer free()
		if ptr != nil {
			t.Fatalf("ptr = %v, want nil for nil tools", ptr)
		}
	})
	t.Run("empty", func(t *testing.T) {
		ptr, free := buildCTools([]Tool{})
		defer free()
		if ptr != nil {
			t.Fatalf("ptr = %v, want nil for empty tools", ptr)
		}
	})
	t.Run("multi", func(t *testing.T) {
		ptr, free := buildCTools([]Tool{
			{Name: "a", Description: "first", ParametersJSON: `{"type":"object"}`},
			{Name: "b", Description: "second", ParametersJSON: `{}`},
		})
		defer free()
		if ptr == nil {
			t.Fatal("ptr = nil for non-empty tools")
		}
	})
	t.Run("empty_strings", func(t *testing.T) {
		// Every CString call must succeed (allocates a NUL byte even for
		// the empty string); the cleanup func must not panic on free.
		ptr, free := buildCTools([]Tool{{}})
		defer free()
		if ptr == nil {
			t.Fatal("ptr = nil for one empty Tool")
		}
	})
}

// Suppress unsafe import being unused on builds where snapshot helpers
// move out of test-visible API.
var _ = unsafe.Pointer(nil)
