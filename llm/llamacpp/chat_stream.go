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

/*
#include <stdlib.h>
#include "common_chat_wrap.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"
)

// Tool is the Go view of a tool definition passed to OpenChatTemplate.
// ParametersJSON must be a JSON schema object serialised as a string.
type Tool struct {
	Name           string
	Description    string
	ParametersJSON string
}

// ReasoningFormat selects how llama.cpp's common_chat_parse surfaces
// reasoning content (<think>…</think>, Gemma 4's <|channel>thought…
// <channel|>, etc.). ReasoningAuto mirrors llama-server's default:
// extract reasoning into a separate channel so the bytes never leak into
// the assistant-visible message.
type ReasoningFormat int

// Supported ReasoningFormat values. These mirror llama.cpp's
// common_reasoning_format enum: None disables reasoning extraction,
// Auto lets the template decide, and the Deepseek variants select the
// explicit Deepseek parsers.
const (
	ReasoningNone           ReasoningFormat = C.RUNE_REASONING_NONE
	ReasoningAuto           ReasoningFormat = C.RUNE_REASONING_AUTO
	ReasoningDeepseekLegacy ReasoningFormat = C.RUNE_REASONING_DEEPSEEK_LEGACY
	ReasoningDeepseek       ReasoningFormat = C.RUNE_REASONING_DEEPSEEK
)

// ChatTemplateOptions bundles the optional inputs to OpenChatTemplate.
type ChatTemplateOptions struct {
	// TemplateOverride, when non-empty, replaces the model's embedded
	// Jinja template. Callers rarely need this; leave blank to use the
	// template shipped inside the GGUF.
	TemplateOverride string

	// Tools declared to the model. Upstream's Jinja templates consume
	// the JSON schema and emit the appropriate tool-call preamble.
	Tools []Tool

	// ParallelToolCalls enables multi-call outputs in templates that
	// support them.
	ParallelToolCalls bool

	// ToolChoice selects whether tool use is automatic, required, or disabled.
	// Empty means the upstream default (auto).
	ToolChoice string

	// ReasoningFmt controls reasoning-token extraction. Defaults to
	// ReasoningAuto when zero-valued, matching llama-server.
	ReasoningFmt ReasoningFormat

	// EnableThinking asks templates that gate thinking behind a flag
	// (e.g. Gemma 4) to keep the thinking channel active.
	EnableThinking bool

	// AddGenerationPrompt appends the assistant-turn opener. Nearly
	// always true during completion.
	AddGenerationPrompt bool

	// JSONSchema, when non-empty, requests schema-constrained output through
	// llama.cpp's common_chat path. The schema must be a JSON object string.
	JSONSchema string
}

// ChatTemplateHandle owns the C-side common_chat_templates + the
// resolved common_chat_params for a single prompt render. It produces
// the rendered Prompt and is the anchor for NewChatStream, which reuses
// the template's parser arena while streaming model output.
type ChatTemplateHandle struct {
	c *C.rune_chat_template
}

// OpenChatTemplate renders msgs into a prompt using the model's Jinja
// template and opens a handle that the caller uses both to retrieve the
// prompt and to construct a streaming parser for the model's output.
// Close the handle when done; leaking it leaks the upstream Jinja
// program + parser arena.
func (m *Model) OpenChatTemplate(msgs []ChatMessage, opts ChatTemplateOptions) (*ChatTemplateHandle, error) {
	var cTmpl *C.char
	if opts.TemplateOverride != "" {
		cTmpl = C.CString(opts.TemplateOverride)
		defer C.free(unsafe.Pointer(cTmpl))
	}
	// OpenChatTemplate accepts either a *Model (whose embedded template is
	// used) or a TemplateOverride source string — at least one must be
	// provided. A nil receiver paired with no override is a programmer
	// error.
	if m == nil && cTmpl == nil {
		panic("llamacpp: OpenChatTemplate requires a model or a TemplateOverride")
	}

	cmsgsPtr, freeMsgs := buildCRichChatMessages(msgs)
	defer freeMsgs()

	cTools, freeTools := buildCTools(opts.Tools)
	defer freeTools()

	var cResp C.struct_rune_response_format
	var cSchema *C.char
	if opts.JSONSchema != "" {
		cSchema = C.CString(opts.JSONSchema)
		defer C.free(unsafe.Pointer(cSchema))
		cResp.json_schema = cSchema
	}

	rf := opts.ReasoningFmt
	if rf == ReasoningNone && opts.EnableThinking {
		// Default to Auto when thinking is requested so the extractor is
		// actually enabled. Explicit ReasoningNone overrides only when
		// EnableThinking is false.
		rf = ReasoningAuto
	}
	if rf == 0 {
		rf = ReasoningAuto
	}

	var errOut *C.char
	var modelPtr *C.struct_llama_model
	if m != nil {
		modelPtr = m.c
	}
	h := C.rune_chat_template_open(
		modelPtr,
		cTmpl,
		cmsgsPtr,
		C.size_t(len(msgs)),
		cTools,
		C.size_t(len(opts.Tools)),
		&cResp,
		toolChoiceToC(opts.ToolChoice),
		boolToCInt(opts.ParallelToolCalls),
		C.int(rf),
		boolToCInt(opts.EnableThinking),
		boolToCInt(opts.AddGenerationPrompt),
		&errOut,
	)
	if h == nil {
		msg := "rune_chat_template_open returned null"
		if errOut != nil {
			msg = C.GoString(errOut)
			C.free(unsafe.Pointer(errOut))
		}
		return nil, fmt.Errorf("llamacpp: open chat template: %s", msg)
	}
	handle := &ChatTemplateHandle{c: h}
	runtime.SetFinalizer(handle, func(x *ChatTemplateHandle) { x.Close() })
	return handle, nil
}

// Prompt returns the rendered prompt string.
func (h *ChatTemplateHandle) Prompt() (string, error) {
	bufSize := 1024
	for {
		buf := make([]byte, bufSize)
		n := C.rune_chat_template_prompt(h.c, (*C.char)(unsafe.Pointer(&buf[0])), C.int(bufSize))
		if n < 0 {
			return "", fmt.Errorf("llamacpp: rune_chat_template_prompt failed (%d)", int(n))
		}
		if int(n) <= bufSize {
			return string(buf[:int(n)]), nil
		}
		bufSize = int(n)
	}
}

// HasParser reports whether the template resolved to a PEG grammar that
// common_chat_parse can use. Returns false for plain content-only
// templates (ChatML, vanilla Llama). Callers use this to decide
// between the upstream streaming parser and their own fallback.
func (h *ChatTemplateHandle) HasParser() bool {
	return C.rune_chat_template_has_parser(h.c) != 0
}

// AdditionalStops returns any stop strings the resolved template derived for
// this request.
func (h *ChatTemplateHandle) AdditionalStops() []string {
	n := int(C.rune_chat_template_additional_stop_count(h.c))
	if n == 0 {
		return nil
	}
	stops := make([]string, 0, n)
	for i := range n {
		bufSize := 64
		for {
			buf := make([]byte, bufSize)
			need := C.rune_chat_template_additional_stop(h.c, C.size_t(i), (*C.char)(unsafe.Pointer(&buf[0])), C.int(bufSize))
			if need < 0 {
				break
			}
			if int(need) <= bufSize {
				stops = append(stops, string(buf[:int(need)]))
				break
			}
			bufSize = int(need)
		}
	}
	return stops
}

// Close frees the C-side template. Safe to call more than once.
func (h *ChatTemplateHandle) Close() {
	if h == nil || h.c == nil {
		return
	}
	C.rune_chat_template_free(h.c)
	h.c = nil
	runtime.SetFinalizer(h, nil)
}

// ChatStreamEventKind classifies events emitted by ChatStream.Next.
type ChatStreamEventKind int

// Supported ChatStreamEventKind values. None is the zero value and is
// never emitted by Next; the remaining kinds classify the delta's
// payload (reasoning text, assistant text, or a tool-call fragment).
const (
	ChatStreamEventNone           ChatStreamEventKind = C.RUNE_CHAT_EVENT_NONE
	ChatStreamEventReasoningDelta ChatStreamEventKind = C.RUNE_CHAT_EVENT_REASONING_DELTA
	ChatStreamEventTextDelta      ChatStreamEventKind = C.RUNE_CHAT_EVENT_TEXT_DELTA
	ChatStreamEventToolCallDelta  ChatStreamEventKind = C.RUNE_CHAT_EVENT_TOOL_CALL_DELTA
)

// ChatStreamEvent is a single delta produced by the upstream parser.
// Zero/empty fields are omitted at the C layer; the struct is produced
// by ChatStream.Next and owns its strings (copied from the C buffers).
type ChatStreamEvent struct {
	Kind          ChatStreamEventKind
	Text          string
	ToolIndex     int
	ToolName      string
	ToolArguments string
	ToolID        string
}

// ChatStream is a stateful accumulator+parser tied to a
// ChatTemplateHandle. Feed model-generated text into it as it arrives;
// Next() drains the resulting deltas.
type ChatStream struct {
	c *C.rune_chat_stream
}

// NewChatStream opens a streaming parser tied to the given template
// handle. The stream must be closed to release C-side buffers.
func NewChatStream(h *ChatTemplateHandle) (*ChatStream, error) {
	s := C.rune_chat_stream_new(h.c)
	if s == nil {
		return nil, fmt.Errorf("llamacpp: rune_chat_stream_new returned null")
	}
	out := &ChatStream{c: s}
	runtime.SetFinalizer(out, func(x *ChatStream) { x.Close() })
	return out, nil
}

// Feed appends a chunk of raw model output to the stream and runs
// common_chat_parse on the accumulated text. isPartial should be true
// for every token mid-generation and false for the final flush at
// end-of-stream (so the parser tolerates unterminated structural tokens
// vs reporting them as errors).
func (s *ChatStream) Feed(text string, isPartial bool) error {
	var cText *C.char
	if text != "" {
		cText = C.CString(text)
		defer C.free(unsafe.Pointer(cText))
	}
	var errOut *C.char
	rc := C.rune_chat_stream_feed(s.c, cText, C.int(len(text)), boolToCInt(isPartial), &errOut)
	if rc != 0 {
		msg := "rune_chat_stream_feed failed"
		if errOut != nil {
			msg = C.GoString(errOut)
			C.free(unsafe.Pointer(errOut))
		}
		return fmt.Errorf("llamacpp: %s", msg)
	}
	return nil
}

// Next returns the next queued delta and true, or a zero event and
// false when the queue is empty. Callers drain the queue after each
// Feed call.
func (s *ChatStream) Next() (ChatStreamEvent, bool) {
	var ev C.struct_rune_chat_stream_event
	rc := C.rune_chat_stream_next(s.c, &ev)
	if rc != 0 || ev.kind == C.RUNE_CHAT_EVENT_NONE {
		return ChatStreamEvent{}, false
	}
	out := ChatStreamEvent{
		Kind:      ChatStreamEventKind(ev.kind),
		ToolIndex: int(ev.tool_index),
	}
	if ev.text != nil && ev.text_len > 0 {
		out.Text = C.GoStringN(ev.text, ev.text_len)
	}
	if ev.tool_name != nil {
		out.ToolName = C.GoString(ev.tool_name)
	}
	if ev.tool_arguments != nil {
		out.ToolArguments = C.GoString(ev.tool_arguments)
	}
	if ev.tool_id != nil {
		out.ToolID = C.GoString(ev.tool_id)
	}
	return out, true
}

// Close releases the stream. Safe to call more than once.
func (s *ChatStream) Close() {
	if s == nil || s.c == nil {
		return
	}
	C.rune_chat_stream_free(s.c)
	s.c = nil
	runtime.SetFinalizer(s, nil)
}

// buildCTools mirrors buildCChatMessages for tool definitions.
func buildCTools(tools []Tool) (*C.struct_rune_tool, func()) {
	n := len(tools)
	if n == 0 {
		return nil, func() {}
	}
	ctools := make([]C.struct_rune_tool, n)
	keep := make([]unsafe.Pointer, 0, 3*n)
	for i, t := range tools {
		np := unsafe.Pointer(C.CString(t.Name))
		dp := unsafe.Pointer(C.CString(t.Description))
		pp := unsafe.Pointer(C.CString(t.ParametersJSON))
		keep = append(keep, np, dp, pp)
		ctools[i].name = (*C.char)(np)
		ctools[i].description = (*C.char)(dp)
		ctools[i].parameters_json = (*C.char)(pp)
	}
	cleanup := func() {
		for _, p := range keep {
			C.free(p)
		}
	}
	return &ctools[0], cleanup
}

func toolChoiceToC(choice string) C.int {
	switch choice {
	case "required":
		return 1
	case "none":
		return 2
	default:
		return 0
	}
}
