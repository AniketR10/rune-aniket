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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// LLMProvider identifies the llama.cpp provider in the model registry.
const LLMProvider = "llamacpp"

// Config configures a Service.
type Config struct {
	// Model is a human-readable model name (the GGUF file's basename is a
	// common choice). Used in registries and logs; does not affect loading.
	Model string

	// ModelPath is the path to a .gguf file on disk.
	ModelPath string

	// ProjectorPath is the path to an optional multimodal projector GGUF
	// (mmproj) paired with ModelPath.
	ProjectorPath string

	// ContextWindow is the n_ctx requested from llama.cpp. 0 means "use the
	// model's training context window".
	ContextWindow uint32

	// BatchSize is the logical batch size (n_batch). 0 picks a sensible default.
	BatchSize uint32

	// NGPULayers is the number of layers to offload to the GPU. Negative
	// offloads every layer the backend supports.
	NGPULayers int

	// Threads is the generation thread count. 0 lets llama.cpp decide.
	Threads int

	// FlashAttention enables flash-attention when supported.
	FlashAttention bool

	// Sampling parameters.
	Sampling SamplerParams

	// MaxOutputTokens caps the number of tokens generated per completion.
	// 0 means "until end-of-generation or context exhausted".
	MaxOutputTokens int

	// ChatTemplate overrides the chat template embedded in the GGUF.
	// Leave empty to use the model's own template.
	ChatTemplate string

	// NCacheReuse is the minimum size (in tokens) of an interior chunk
	// that the KV-cache reuse path will recover by shifting cached
	// positions to align with the new prompt. Mirrors llama-server's
	// `--cache-reuse` flag (see server-context.cpp:2342). Zero disables
	// shift-reuse and falls back to LCP-only matching. The upstream
	// default is 256.
	NCacheReuse int

	// LoadProgress, when non-nil, receives model-load progress updates
	// while the GGUF is opened. This is primarily used to surface UI
	// feedback when opening large local models. The callback must not
	// block for long.
	LoadProgress func(progress, total int64, units string)
}

// Service implements llmapi.Service on top of a locally-loaded GGUF model.
//
// A Service owns a single llama.cpp Context that is serialized across
// concurrent CreateCompletion calls — the underlying KV cache is not safe
// for concurrent mutation. Callers that need parallel inference should
// create multiple Services (or contexts) and pool them.
type Service struct {
	cfg     Config
	model   *Model
	ctxMu   sync.Mutex
	context *Context

	// cachedTokens is the exact token sequence that the KV cache holds from
	// the previous successful CreateCompletion call (prompt + generated
	// reply). It is used to diff against the next turn's prompt and skip
	// re-evaluation of the shared prefix.
	//
	// On any error, cancellation, or malformed-stream path that leaves the
	// KV in an unknown state, cachedTokens must be set back to nil so the
	// next turn performs a full ClearKV() + re-eval.
	cachedTokens []int32

	// cacheSeqID is the sequence id associated with cachedTokens. We pin
	// everything to sequence 0 for now — kept as a field so future
	// multi-session support is a localised change.
	cacheSeqID int
}

// NewService loads the GGUF model at cfg.ModelPath and returns a ready Service.
// The returned Service must be closed with Close when no longer needed.
func NewService(cfg Config) (*Service, error) {
	if cfg.ModelPath == "" {
		return nil, errors.New("llamacpp: ModelPath is required")
	}
	if cfg.Model == "" {
		cfg.Model = cfg.ModelPath
	}
	if cfg.Sampling == (SamplerParams{}) {
		cfg.Sampling = DefaultSamplerParams()
	}

	mp := DefaultModelParams()
	mp.NGPULayers = cfg.NGPULayers
	mp.Progress = cfg.LoadProgress
	m, err := LoadModel(cfg.ModelPath, cfg.ProjectorPath, mp)
	if err != nil {
		return nil, fmt.Errorf("llamacpp: load %q: %w", cfg.ModelPath, err)
	}

	cp := DefaultContextParams()
	cp.NCtx = cfg.ContextWindow
	if cfg.BatchSize > 0 {
		cp.NBatch = cfg.BatchSize
	}
	cp.NThreads = cfg.Threads
	cp.NThreadsBatch = cfg.Threads
	cp.FlashAttention = cfg.FlashAttention

	c, err := NewContext(m, cp)
	if err != nil {
		m.Close()
		return nil, fmt.Errorf("llamacpp: create context: %w", err)
	}

	return &Service{
		cfg:     cfg,
		model:   m,
		context: c,
	}, nil
}

// Close releases all backing resources.
func (s *Service) Close() {
	s.ctxMu.Lock()
	defer s.ctxMu.Unlock()
	if s.context != nil {
		s.context.Close()
		s.context = nil
	}
	if s.model != nil {
		s.model.Close()
		s.model = nil
	}
}

// Model returns the loaded model, or nil after Close.
func (s *Service) Model() *Model { return s.model }

// ContextWindow returns the effective context window size.
func (s *Service) ContextWindow() int {
	s.ctxMu.Lock()
	defer s.ctxMu.Unlock()
	if s.context == nil {
		return 0
	}
	return s.context.NCtx()
}

// Models advertises this service's single loaded model so the Service can
// be used as a stand-alone llmapi.Service. The router typically aggregates
// this with the dynamic Registry over the cache directory.
func (s *Service) Models() iterator.Iterator[llmapi.ModelEntry] {
	if s.cfg.Model == "" {
		return iterator.FromSlice[llmapi.ModelEntry](nil)
	}
	return iterator.FromSlice([]llmapi.ModelEntry{{
		Name:          s.cfg.Model,
		Provider:      LLMProvider,
		ContextWindow: s.ContextWindow(),
	}})
}

// GetModel returns the model entry for the loaded GGUF if the given name
// matches; otherwise reports not-found.
func (s *Service) GetModel(_ context.Context, model llmapi.ModelEntry) (llmapi.ModelEntry, bool) {
	if model.Name == "" || model.Name != s.cfg.Model {
		return llmapi.ModelEntry{}, false
	}
	return llmapi.ModelEntry{
		Name:          s.cfg.Model,
		Provider:      LLMProvider,
		ContextWindow: s.ContextWindow(),
	}, true
}

// CountTokens returns an exact token count for the given messages by applying
// the model's chat template and tokenizing the result.
func (s *Service) CountTokens(_ llmapi.ModelEntry, msgs []llmapi.Message) (int, error) {
	handle, prompt, _, err := s.formatMessages(msgs, nil, "", "", nil, nil, false)
	if err != nil {
		return 0, err
	}
	if handle != nil {
		handle.Close()
	}
	tokens, err := s.model.Tokenize(prompt, true, true)
	if err != nil {
		return 0, err
	}
	return len(tokens), nil
}

// formatMessages renders a prompt string using the model's chat template.
// When addAssistant is true, the trailing token(s) that open an assistant
// turn are appended so sampling begins in the right state.
//
// The implementation always goes through the upstream Jinja engine via
// Model.OpenChatTemplate so reasoning / tool-call handling stays
// consistent with how the model was trained. The returned handle owns
// the resolved common_chat_params — callers that also need to stream the
// model's output should reuse the same handle when constructing a
// ChatStream so the PEG parser arena is shared.
//
// For backwards compatibility with templates that don't understand a
// "tool" role (ChatML fallback used by older Qwen GGUFs), we also run
// buildChatMessages to wrap tool responses in <tool_response> blocks
// when the template has no PEG parser of its own.
func (s *Service) formatMessages(
	msgs []llmapi.Message,
	tools []llmapi.Tool,
	effort llmapi.ReasoningEffort,
	toolChoice llmapi.ToolChoice,
	parallelToolCalls *bool,
	responseFormat *llmapi.ResponseFormat,
	addAssistant bool,
) (*ChatTemplateHandle, string, [][]byte, error) {
	cTools := convertTools(tools)
	opts := makeChatTemplateOptions(s.cfg.ChatTemplate, cTools, effort, addAssistant, toolChoice, parallelToolCalls, responseFormat)
	// First try the template straight, letting upstream handle tool
	// rendering via its Jinja template.
	upstreamMsgs, files := renderForUpstream(msgs)
	handle, err := s.model.OpenChatTemplate(upstreamMsgs, opts)
	if err == nil {
		// Templates without an upstream PEG parser still rely on the older
		// Hermes/Qwen-style <tools>/<tool_call>/<tool_response> framing for
		// robust tool use. renderForUpstream intentionally drops that extra
		// wrapping because templates *with* a parser can consume structured
		// tool/message state directly. When tools are enabled but the chosen
		// template produced no parser, fall back to buildChatMessages so the
		// model continues to see the legacy framing it was trained on.
		if len(tools) > 0 && !handle.HasParser() {
			handle.Close()
			handle = nil
		} else {
			prompt, err := handle.Prompt()
			if err != nil {
				handle.Close()
				return nil, "", nil, err
			}
			return handle, prompt, files, nil
		}
	}
	// Fallback: legacy template detector. Used only if the upstream
	// Jinja engine refused the template outright. buildChatMessages
	// injects Hermes-style <tool_call>/<tool_response> framing so
	// ChatML-family fallbacks can still carry tool traffic.
	cmsgs, ferr := buildChatMessages(msgs, tools)
	if ferr != nil {
		return nil, "", nil, ferr
	}
	prompt, ferr := s.model.ApplyChatTemplate(s.cfg.ChatTemplate, cmsgs, addAssistant)
	if ferr != nil {
		return nil, "", nil, fmt.Errorf("llamacpp: format messages: %w (upstream: %v)", ferr, err)
	}
	return nil, prompt, files, nil
}

func makeChatTemplateOptions(
	templateOverride string,
	tools []Tool,
	effort llmapi.ReasoningEffort,
	addAssistant bool,
	toolChoice llmapi.ToolChoice,
	parallelToolCalls *bool,
	responseFormat *llmapi.ResponseFormat,
) ChatTemplateOptions {
	enableThinking := true
	switch effort {
	case llmapi.ReasoningEffortNone, llmapi.ReasoningEffortMinimal:
		enableThinking = false
	}
	parallel := false
	if parallelToolCalls != nil {
		parallel = *parallelToolCalls
	}
	// Match llama-server's default more closely: the presence of tools does
	// not implicitly disable thinking. Models like Gemma 4 default to
	// enable_thinking=true upstream unless the caller explicitly lowers the
	// reasoning effort (or passes chat_template_kwargs to override it).
	jsonSchema := responseFormatJSONSchema(responseFormat)
	return ChatTemplateOptions{
		TemplateOverride:    templateOverride,
		Tools:               tools,
		ParallelToolCalls:   parallel,
		ToolChoice:          string(toolChoice),
		ReasoningFmt:        ReasoningAuto,
		EnableThinking:      enableThinking,
		AddGenerationPrompt: addAssistant,
		JSONSchema:          jsonSchema,
	}
}

func responseFormatJSONSchema(format *llmapi.ResponseFormat) string {
	if format == nil {
		return ""
	}
	switch format.Type {
	case llmapi.ResponseFormatTypeJSONObject:
		return `{"type":"object"}`
	case llmapi.ResponseFormatTypeJSONSchema:
		if format.JSONSchema == nil || format.JSONSchema.Schema == nil {
			return ""
		}
		b, err := format.JSONSchema.Schema.MarshalJSON()
		if err != nil {
			return ""
		}
		return string(b)
	default:
		return ""
	}
}

const defaultMediaMarker = "<__media__>"

// renderForUpstream adapts []llmapi.Message into the C-side []ChatMessage
// shape. Unlike buildChatMessages, no Hermes-style tool wrapping is
// applied because upstream's Jinja templates emit the right framing
// themselves. Structured fields (tool_calls, tool_call_id, name,
// reasoning_content) are preserved so the upstream common_chat path sees
// the same message shape llama-server passes in. Image parts are converted
// to media markers and returned as decoded file payloads for mtmd-backed
// prompt evaluation.
func renderForUpstream(msgs []llmapi.Message) ([]ChatMessage, [][]byte) {
	out := make([]ChatMessage, 0, len(msgs))
	var files [][]byte
	for _, m := range msgs {
		cm := ChatMessage{
			Role:             string(m.Role),
			Content:          m.Content,
			ReasoningContent: m.ReasoningContent,
			Name:             m.Name,
			ToolCallID:       m.ToolCallID,
		}
		if len(m.ToolCalls) > 0 {
			cm.ToolCalls = make([]ChatToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				cm.ToolCalls = append(cm.ToolCalls, ChatToolCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
					ID:        tc.ID,
				})
			}
			// Preserve structured tool calls rather than rendering them into
			// textual <tool_call> blocks. Older templates without a parser are
			// routed through buildChatMessages instead.
			cm.Content = m.Content
		}
		if len(m.MultiContent) > 0 {
			for _, p := range m.MultiContent {
				switch p.Type {
				case llmapi.ContentPartTypeText:
					cm.ContentParts = append(cm.ContentParts, ChatContentPart{
						Type: "text",
						Text: p.Text,
					})
				case llmapi.ContentPartTypeImageURL:
					decoded, err := decodeImageContentPart(p.ImageURL)
					if err != nil {
						continue
					}
					files = append(files, decoded)
					cm.ContentParts = append(cm.ContentParts, ChatContentPart{
						Type: "media_marker",
						Text: defaultMediaMarker,
					})
				}
			}
			if len(cm.ContentParts) > 0 {
				// Prefer typed content when present, mirroring upstream's
				// common_chat_msg usage. The plain string field remains empty so
				// the template can decide based on its caps.
				cm.Content = ""
			}
		}
		out = append(out, cm)
	}
	return out, files
}

func decodeImageContentPart(url string) ([]byte, error) {
	const prefix = "data:image/"
	if !strings.HasPrefix(url, prefix) {
		return nil, fmt.Errorf("unsupported image url: %q", url)
	}
	parts := strings.SplitN(url, ",", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid data url")
	}
	if !strings.HasSuffix(parts[0], ";base64") {
		return nil, fmt.Errorf("image url must be base64 data url")
	}
	data, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode image data url: %w", err)
	}
	return data, nil
}

// convertTools translates llmapi.Tool definitions into the
// llamacpp.Tool shape that common_chat_templates_apply consumes.
func convertTools(tools []llmapi.Tool) []Tool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]Tool, 0, len(tools))
	for _, t := range tools {
		params := ""
		if t.Function.Parameters != nil {
			if b, err := json.Marshal(t.Function.Parameters); err == nil {
				params = string(b)
			}
		}
		out = append(out, Tool{
			Name:           t.Function.Name,
			Description:    t.Function.Description,
			ParametersJSON: params,
		})
	}
	return out
}

// buildChatMessages converts a slice of llmapi.Messages into the ChatMessage
// shape consumed by llama_chat_apply_template. It is the pure-Go half of
// formatMessages, split out so it can be exercised by unit tests without
// loading a GGUF.
//
// Tool declarations (when tools is non-empty) are injected into the first
// system message or prepended as a fresh system message when none exists.
// Tool-role messages are wrapped in <tool_response> blocks AND remapped to
// the "user" role so chat templates without a native "tool" role (which
// includes llama.cpp's built-in chatml fallback used by Qwen-family
// models) render the header the model was actually trained on.
func buildChatMessages(msgs []llmapi.Message, tools []llmapi.Tool) ([]ChatMessage, error) {
	toolPrompt, err := buildToolSystemPrompt(tools)
	if err != nil {
		return nil, err
	}

	cmsgs := make([]ChatMessage, 0, len(msgs)+1)
	injectedTools := false
	for _, m := range msgs {
		content, err := renderMessage(m)
		if err != nil {
			return nil, err
		}

		role := string(m.Role)
		switch m.Role {
		case llmapi.RoleSystem:
			if toolPrompt != "" && !injectedTools {
				content = toolPrompt + "\n\n" + content
				injectedTools = true
			}
		case llmapi.RoleTool:
			content = toolRespOpen + "\n" + content + "\n" + toolRespClose
			role = string(llmapi.RoleUser)
		}

		cmsgs = append(cmsgs, ChatMessage{Role: role, Content: content})
	}

	if toolPrompt != "" && !injectedTools {
		cmsgs = append([]ChatMessage{{
			Role:    string(llmapi.RoleSystem),
			Content: toolPrompt,
		}}, cmsgs...)
	}
	return cmsgs, nil
}

// renderMessage produces the textual content passed to the chat template for a
// single llmapi.Message. Multi-content parts are folded to their text components
// (images are dropped), and assistant tool calls are inlined as <tool_call>
// blocks so the model sees its prior decisions.
func renderMessage(m llmapi.Message) (string, error) {
	if m.Role == llmapi.RoleAssistant && len(m.ToolCalls) > 0 {
		return renderAssistantWithToolCalls(m)
	}
	if m.Content != "" || len(m.MultiContent) == 0 {
		return m.Content, nil
	}
	var b strings.Builder
	for _, p := range m.MultiContent {
		if p.Type == llmapi.ContentPartTypeText {
			b.WriteString(p.Text)
		}
	}
	return b.String(), nil
}

// CreateCompletion streams a completion for the given request.
//
// Tool calling uses the Hermes/Qwen2.5 `<tool_call>` convention. Tool defs in
// req.Tools are serialized into a system-prompt preamble so the model knows
// the available functions. The streaming parser watches every text delta for
// <tool_call>…</tool_call> blocks: text outside those blocks is surfaced as
// EventTextDelta; each closed block becomes an EventToolCallDone.
func (s *Service) CreateCompletion(
	ctx context.Context,
	_ llmapi.ModelEntry,
	req llmapi.Request,
) (iterator.Iterator[llmapi.Event], error) {
	handle, prompt, files, err := s.formatMessages(
		req.Messages,
		req.Tools,
		req.ReasoningEffort,
		req.ToolChoice,
		req.ParallelToolCalls,
		req.ResponseFormat,
		true,
	)
	if err != nil {
		return nil, err
	}
	// handle may be nil when we fell back to the legacy prompt path; the
	// streaming parser is only wired up when upstream produced a parser
	// arena. Leaks are prevented by closing the handle in every early
	// return below.
	var tokens []int32
	hasMedia := len(files) > 0 && s.model.HasProjector()
	if !hasMedia {
		tokens, err = s.model.Tokenize(prompt, true, true)
		if err != nil {
			if handle != nil {
				handle.Close()
			}
			return nil, err
		}
	}

	s.ctxMu.Lock()
	if s.context == nil {
		s.ctxMu.Unlock()
		return nil, errors.New("llamacpp: service closed")
	}
	if !hasMedia && len(tokens) > s.context.NCtx() {
		s.ctxMu.Unlock()
		if handle != nil {
			handle.Close()
		}
		return nil, &llmapi.ErrContextWindowExceeded{Count: len(tokens), Max: s.context.NCtx()}
	}
	sampl, err := NewSampler(s.model, s.cfg.Sampling, handle)
	if err != nil {
		s.ctxMu.Unlock()
		if handle != nil {
			handle.Close()
		}
		return nil, err
	}

	// We hold ctxMu for the entire streaming iteration. The iterator's Close
	// releases it. This serializes concurrent CreateCompletion calls.

	// Reuse the shared prefix with the KV cache from the previous turn:
	// only the tail of the prompt that differs (or the back-off token when
	// the prompt is identical) needs to be decoded. When partial removal is
	// not supported by the backing cache, fall back to a full reset.
	var nPast int
	if hasMedia {
		// Multimodal prompts don't share our token-level cache: clear the KV
		// and let the mtmd helper repopulate it from scratch.
		s.context.ClearKV()
	} else {
		nPast = resolvePrefixReuse(s.cachedTokens, tokens)
		// n_cache_reuse: try to recover interior chunks past the LCP
		// divergence by shifting their cached positions to align with
		// the new prompt. Mirrors server-context.cpp:2342-2410.
		if s.cfg.NCacheReuse > 0 && s.context.CanShiftKV() {
			nPast = applyCacheReuse(
				s.context, s.cacheSeqID,
				s.cachedTokens, tokens,
				nPast, s.cfg.NCacheReuse,
			)
		}
		if nPast > 0 {
			if !s.context.RemoveKVRange(s.cacheSeqID, nPast, -1) {
				s.context.ClearKV()
				nPast = 0
			}
		} else {
			s.context.ClearKV()
		}
	}
	sampl.Reset()
	for _, tok := range tokens {
		sampl.AcceptPrompt(tok)
	}
	// Any partial prior state is invalidated until this turn finishes
	// cleanly. On error/cancellation we leave cachedTokens nil so the next
	// turn starts from scratch; on success we update it in close().
	s.cachedTokens = nil

	// Evaluate the prompt suffix. Split into batches of NBatch to respect n_batch.
	nBatch := s.context.NBatch()
	batch := NewBatch(nBatch)

	started := time.Now()
	usage := llmapi.Usage{TokensSent: len(tokens)}
	tokensCached := nPast
	usage.TokensCached = tokensCached

	if hasMedia {
		newPast, nTok, mmErr := s.model.EvalMultimodalPrompt(
			s.context, prompt, files, nBatch, s.cacheSeqID, 0, true,
		)
		if mmErr != nil {
			batch.Close()
			sampl.Close()
			s.context.ClearKV()
			s.ctxMu.Unlock()
			if handle != nil {
				handle.Close()
			}
			return nil, mmErr
		}
		nPast = newPast
		usage.TokensSent = nTok
	} else {
		if err := evalPrompt(s.context, batch, tokens, nPast); err != nil {
			batch.Close()
			sampl.Close()
			s.context.ClearKV()
			s.ctxMu.Unlock()
			if handle != nil {
				handle.Close()
			}
			return nil, err
		}
		// After evalPrompt, nPast has advanced to len(tokens).
		nPast = len(tokens)
	}

	maxOut := s.cfg.MaxOutputTokens
	if req.MaxOutputTokens > 0 {
		maxOut = req.MaxOutputTokens
	}
	remaining := s.context.NCtx() - nPast
	if maxOut <= 0 || maxOut > remaining {
		maxOut = remaining
	}

	state := &genState{
		svc:        s,
		sampler:    sampl,
		batch:      batch,
		prompt:     tokens,
		nPast:      nPast,
		produced:   0,
		maxOutput:  maxOut,
		started:    started,
		usage:      usage,
		messageBuf: strings.Builder{},
		hasTools:   len(req.Tools) > 0,
		hasMedia:   hasMedia,
	}
	if handle != nil {
		state.stopStrings = handle.AdditionalStops()
	}
	// Wire the upstream streaming parser when the template produced a
	// PEG grammar. Models without one (vanilla ChatML, etc.) continue to
	// use the Hermes fallback below.
	if handle != nil && handle.HasParser() {
		stream, serr := NewChatStream(handle)
		if serr != nil {
			batch.Close()
			sampl.Close()
			s.context.ClearKV()
			s.ctxMu.Unlock()
			handle.Close()
			return nil, fmt.Errorf("llamacpp: open chat stream: %w", serr)
		}
		state.upstreamHandle = handle
		state.upstreamStream = stream
	} else if handle != nil {
		handle.Close()
	}

	return iterator.FromFunc(state.next, state.close), nil
}

// evalPrompt decodes tokens[startPos:] in nBatch-sized chunks, placing them
// into KV positions [startPos, len(tokens)). Only the very last token
// requests logits so the sampler can draw the next token from them.
//
// When startPos == len(tokens) there is nothing to evaluate — this happens
// when the previous turn's cache already contains every token of the new
// prompt except the final one, which is retained in-cache to supply logits.
// In that case the function is a no-op and the generation loop will sample
// from the logits that were produced on the prior decode.
func evalPrompt(ctx *Context, batch *Batch, tokens []int32, startPos int) error {
	if startPos >= len(tokens) {
		return nil
	}
	nBatch := batch.Capacity()
	for i := startPos; i < len(tokens); i += nBatch {
		end := i + nBatch
		if end > len(tokens) {
			end = len(tokens)
		}
		batch.Clear()
		for j := i; j < end; j++ {
			wantLogits := end == len(tokens) && j == end-1
			batch.Add(tokens[j], j, wantLogits)
		}
		if err := ctx.Decode(batch); err != nil {
			return fmt.Errorf("llamacpp: prompt decode: %w", err)
		}
	}
	return nil
}

// genState carries per-request streaming state. Its methods satisfy the
// iterator.FromFunc signature.
type genState struct {
	svc     *Service
	sampler *Sampler
	batch   *Batch
	// prompt is the full tokenized prompt (including the parts already in
	// the KV cache from previous turns). It is concatenated with the
	// generated tokens to repopulate Service.cachedTokens on a clean exit.
	prompt    []int32
	generated []int32
	nPast     int
	produced  int
	maxOutput int
	started   time.Time

	usage      llmapi.Usage
	messageBuf strings.Builder // accumulates user-visible text only
	closed     bool
	done       bool
	finish     llmapi.FinishReason

	// Tool-calling state. Populated only when hasTools is true.
	hasTools bool
	// hasMedia indicates this turn's prompt went through the mtmd helper
	// path. When true, the token-level KV cache shadow must not be reused
	// for the next turn because the chunk structure is not preserved in the
	// plain token slice.
	hasMedia  bool
	parser    toolCallParser
	toolCalls []llmapi.ToolCall
	// Upstream streaming parser. Populated when the chat template's
	// common_chat_params has a non-empty PEG arena (Gemma 4, DeepSeek,
	// etc.). When set, ingestPiece feeds every token into upstreamStream
	// and emits reasoning/text/tool events the C parser produces,
	// bypassing the Hermes-style fallback below.
	upstreamHandle *ChatTemplateHandle
	upstreamStream *ChatStream
	// pending is a FIFO of events queued for emission. A single sampled
	// token can expand into (plain text delta, tool call event, …) — we
	// queue the extras and drain them on subsequent next() calls.
	pending []llmapi.Event
	// stopStrings are template-provided additional stop sequences. Unlike EOG,
	// these are string-level boundaries that must be trimmed from emitted text.
	stopStrings []string
	// pendingStop buffers a suffix that might be the prefix of a stop string so
	// partial stop sequences don't leak while we wait for the next token.
	pendingStop string

	// hadError is set when a CGo call fails mid-stream and leaves the KV
	// cache in an indeterminate state. It prevents close() from caching
	// the in-flight prompt as a reusable prefix for the next turn.
	hadError bool
}

func (g *genState) next(ctx context.Context) (llmapi.Event, bool, error) {
	// Drain any events queued from a previous token before sampling more.
	if len(g.pending) > 0 {
		ev := g.pending[0]
		g.pending = g.pending[1:]
		return ev, true, nil
	}

	if g.closed || g.done {
		// Emit the terminal EventStreamDone once, after the last text delta.
		return g.emitDone()
	}

	// Honor cancellation before running the (blocking) CGO call.
	if err := ctx.Err(); err != nil {
		g.done = true
		g.finish = llmapi.FinishReasonStop
		return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true, nil
	}

	if g.produced >= g.maxOutput {
		g.finish = llmapi.FinishReasonLength
		g.done = true
		return g.finalize()
	}

	token := g.sampler.Sample(g.svc.svcContext(), -1)
	g.sampler.Accept(token)

	if g.svc.model.IsEOG(token) {
		g.finish = llmapi.FinishReasonStop
		g.done = true
		return g.finalize()
	}

	piece := g.svc.model.TokenToPiece(token, false)
	g.usage.TokensReceived++
	g.produced++
	g.generated = append(g.generated, token)

	// Feed the token back into the context for the next step.
	g.batch.Clear()
	g.batch.Add(token, g.nPast, true)
	g.nPast++
	if err := g.svc.svcContext().Decode(g.batch); err != nil {
		// ErrKVCacheFull here means the model produced a token we had room
		// to sample but no room to store — treat it as a length stop so
		// the partial reply we already assembled is returned cleanly. Any
		// other decode failure is fatal.
		if errors.Is(err, ErrKVCacheFull) {
			g.finish = llmapi.FinishReasonLength
			g.done = true
			return g.finalize()
		}
		g.done = true
		g.finish = llmapi.FinishReasonStop
		g.hadError = true
		return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true, nil
	}

	return g.ingestPiece(ctx, piece)
}

// ingestPiece funnels a newly-generated text piece into whichever
// streaming parser is active — upstream's common_chat_parse when the
// template provides a PEG grammar, or the Hermes-style toolCallParser
// otherwise — queues any resulting events, and yields the next one. If
// the parser is mid-marker and produces nothing, sampling continues
// immediately so the caller never observes an empty delta.
func (g *genState) ingestPiece(ctx context.Context, piece string) (llmapi.Event, bool, error) {
	if piece != "" {
		safe, stop := g.applyStops(piece)
		piece = safe
		if stop {
			g.done = true
			if g.finish == "" {
				g.finish = llmapi.FinishReasonStop
			}
			if piece != "" {
				if g.upstreamStream != nil {
					if _, _, err := g.ingestPieceUpstream(ctx, piece); err != nil {
						return llmapi.Event{}, false, err
					}
				} else if !g.hasTools {
					g.messageBuf.WriteString(piece)
					g.pending = append(g.pending, llmapi.Event{Type: llmapi.EventTextDelta, Text: piece})
				} else {
					text, calls, err := g.parser.Feed(piece)
					if err != nil {
						g.finish = llmapi.FinishReasonStop
						return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true, nil
					}
					if text != "" {
						g.messageBuf.WriteString(text)
						g.pending = append(g.pending, llmapi.Event{Type: llmapi.EventTextDelta, Text: text})
					}
					for i := range calls {
						tc := calls[i]
						g.toolCalls = append(g.toolCalls, tc)
						g.pending = append(g.pending, llmapi.Event{Type: llmapi.EventToolCallDone, ToolCall: &tc})
					}
				}
			}
			if len(g.pending) > 0 {
				ev := g.pending[0]
				g.pending = g.pending[1:]
				return ev, true, nil
			}
			return g.finalize()
		}
		if piece == "" {
			return g.next(ctx)
		}
	}

	if g.upstreamStream != nil {
		return g.ingestPieceUpstream(ctx, piece)
	}

	if !g.hasTools {
		g.messageBuf.WriteString(piece)
		return llmapi.Event{Type: llmapi.EventTextDelta, Text: piece}, true, nil
	}

	text, calls, err := g.parser.Feed(piece)
	if err != nil {
		g.done = true
		g.finish = llmapi.FinishReasonStop
		return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true, nil
	}
	if text != "" {
		g.messageBuf.WriteString(text)
		g.pending = append(g.pending, llmapi.Event{Type: llmapi.EventTextDelta, Text: text})
	}
	for i := range calls {
		tc := calls[i]
		g.toolCalls = append(g.toolCalls, tc)
		g.pending = append(g.pending, llmapi.Event{
			Type:     llmapi.EventToolCallDone,
			ToolCall: &tc,
		})
	}

	if len(g.pending) == 0 {
		// Parser is buffering (partial marker or mid-body). Recurse so the
		// caller sees the next meaningful event rather than a null delta.
		return g.next(ctx)
	}
	ev := g.pending[0]
	g.pending = g.pending[1:]
	return ev, true, nil
}

// ingestPieceUpstream drives common_chat_parse (via ChatStream) with the
// newly-generated piece. Upstream may emit reasoning, text, and
// accumulated-tool-call deltas. Tool call deltas are aggregated into
// g.toolCalls by index and finalised on Flush so the iterator emits
// one EventToolCallDone per complete call, matching the other
// providers' contract.
func (g *genState) ingestPieceUpstream(ctx context.Context, piece string) (llmapi.Event, bool, error) {
	if err := g.upstreamStream.Feed(piece, true); err != nil {
		g.done = true
		g.finish = llmapi.FinishReasonStop
		return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true, nil
	}
	g.drainUpstream()
	if len(g.pending) == 0 {
		return g.next(ctx)
	}
	ev := g.pending[0]
	g.pending = g.pending[1:]
	return ev, true, nil
}

// drainUpstream pulls every queued event off the upstream C parser and
// maps it onto llmapi.Event. Tool-call deltas update g.toolCalls in place;
// complete calls are emitted by finalize() when the stream flushes.
func (g *genState) drainUpstream() {
	for {
		ev, ok := g.upstreamStream.Next()
		if !ok {
			return
		}
		switch ev.Kind {
		case ChatStreamEventTextDelta:
			g.messageBuf.WriteString(ev.Text)
			g.pending = append(g.pending, llmapi.Event{Type: llmapi.EventTextDelta, Text: ev.Text})
		case ChatStreamEventReasoningDelta:
			g.pending = append(g.pending, llmapi.Event{
				Type:      llmapi.EventReasoningDelta,
				Reasoning: ev.Text,
			})
		case ChatStreamEventToolCallDelta:
			g.recordToolCallDelta(ev)
		}
	}
}

// recordToolCallDelta merges a streamed tool-call delta into
// g.toolCalls. Upstream reports deltas by index; we grow the slice
// as needed and append arguments progressively.
func (g *genState) recordToolCallDelta(ev ChatStreamEvent) {
	idx := ev.ToolIndex
	if idx < 0 {
		return
	}
	for len(g.toolCalls) <= idx {
		g.toolCalls = append(g.toolCalls, llmapi.ToolCall{Type: llmapi.ToolTypeFunction})
	}
	tc := &g.toolCalls[idx]
	if ev.ToolID != "" {
		tc.ID = ev.ToolID
	}
	if ev.ToolName != "" {
		tc.Function.Name = ev.ToolName
	}
	if ev.ToolArguments != "" {
		tc.Function.Arguments += ev.ToolArguments
	}
}

// finalize flushes any buffered parser state on stream end, queues the
// resulting events, and returns the next event to yield (either the drained
// pending event or EventStreamDone).
func (g *genState) finalize() (llmapi.Event, bool, error) {
	if flushed := g.flushPendingStop(); flushed != "" {
		if g.upstreamStream != nil {
			if err := g.upstreamStream.Feed(flushed, true); err != nil {
				g.closed = true
				return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true, nil
			}
			g.drainUpstream()
		} else if g.hasTools {
			text, calls, err := g.parser.Feed(flushed)
			if err != nil {
				g.closed = true
				return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true, nil
			}
			if text != "" {
				g.messageBuf.WriteString(text)
				g.pending = append(g.pending, llmapi.Event{Type: llmapi.EventTextDelta, Text: text})
			}
			for i := range calls {
				tc := calls[i]
				g.toolCalls = append(g.toolCalls, tc)
				g.pending = append(g.pending, llmapi.Event{Type: llmapi.EventToolCallDone, ToolCall: &tc})
			}
		} else {
			g.messageBuf.WriteString(flushed)
			g.pending = append(g.pending, llmapi.Event{Type: llmapi.EventTextDelta, Text: flushed})
		}
	}
	if g.upstreamStream != nil {
		// Final feed with is_partial=false so upstream emits any deferred
		// content (e.g. trailing reasoning block).
		if err := g.upstreamStream.Feed("", false); err != nil {
			g.closed = true
			return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true, nil
		}
		g.drainUpstream()
		// Any tool calls accumulated in g.toolCalls are now complete.
		// Emit one EventToolCallDone per call so downstream sees the
		// familiar per-provider shape.
		for i := range g.toolCalls {
			tc := g.toolCalls[i]
			g.pending = append(g.pending, llmapi.Event{
				Type:     llmapi.EventToolCallDone,
				ToolCall: &tc,
			})
		}
		if len(g.pending) > 0 {
			ev := g.pending[0]
			g.pending = g.pending[1:]
			return ev, true, nil
		}
		return g.emitDone()
	}
	if g.hasTools {
		text, _, err := g.parser.Flush()
		if err != nil {
			g.closed = true
			return llmapi.Event{Type: llmapi.EventStreamError, Error: err}, true, nil
		}
		if text != "" {
			g.messageBuf.WriteString(text)
			g.pending = append(g.pending, llmapi.Event{Type: llmapi.EventTextDelta, Text: text})
		}
	}
	if len(g.pending) > 0 {
		ev := g.pending[0]
		g.pending = g.pending[1:]
		return ev, true, nil
	}
	return g.emitDone()
}

func (g *genState) applyStops(piece string) (safe string, stop bool) {
	if len(g.stopStrings) == 0 {
		return piece, false
	}
	combined := g.pendingStop + piece
	g.pendingStop = ""
	if combined == "" {
		return "", false
	}
	stopPos := -1
	for _, stopStr := range g.stopStrings {
		if stopStr == "" {
			continue
		}
		if idx := strings.Index(combined, stopStr); idx >= 0 && (stopPos < 0 || idx < stopPos) {
			stopPos = idx
		}
	}
	if stopPos >= 0 {
		return combined[:stopPos], true
	}
	if partial := longestPartialStopSuffix(combined, g.stopStrings); partial != "" {
		g.pendingStop = partial
		return strings.TrimSuffix(combined, partial), false
	}
	return combined, false
}

func (g *genState) flushPendingStop() string {
	text := g.pendingStop
	g.pendingStop = ""
	return text
}

// longestPartialStopSuffix returns the longest suffix of `text` that is
// also a non-empty proper prefix of any string in `stops`. It defers
// the per-stop scan to llama.cpp's string_find_partial_stop so the
// suffix-matching contract stays in lockstep with llama-server.
func longestPartialStopSuffix(text string, stops []string) string {
	var best string
	for _, stop := range stops {
		if len(stop) <= 1 {
			continue
		}
		off := findPartialStop(text, stop)
		if off < 0 {
			continue
		}
		// Only treat partial-prefix matches as pending: a full stop
		// (suffix length == len(stop)) is the caller's job.
		n := len(text) - off
		if n >= len(stop) {
			continue
		}
		if n > len(best) {
			best = text[off:]
		}
	}
	return best
}

func (g *genState) emitDone() (llmapi.Event, bool, error) {
	if g.closed {
		return llmapi.Event{}, false, nil
	}
	g.closed = true
	_ = time.Since(g.started) // reserved for future timing hooks
	msg := llmapi.Message{
		Role:    llmapi.RoleAssistant,
		Content: g.messageBuf.String(),
	}
	if len(g.toolCalls) > 0 {
		msg.ToolCalls = g.toolCalls
	}
	reason := g.finish
	switch {
	case len(g.toolCalls) > 0:
		reason = llmapi.FinishReasonToolCall
	case reason != "":
		// explicit reason already set
	default:
		reason = llmapi.FinishReasonStop
	}
	return llmapi.Event{
		Type: llmapi.EventStreamDone,
		DoneData: &llmapi.DoneData{
			Message:      msg,
			FinishReason: reason,
			Usage:        g.usage,
		},
	}, true, nil
}

func (g *genState) close() error {
	if g.sampler != nil {
		g.sampler.Close()
		g.sampler = nil
	}
	if g.batch != nil {
		g.batch.Close()
		g.batch = nil
	}
	if g.upstreamStream != nil {
		g.upstreamStream.Close()
		g.upstreamStream = nil
	}
	if g.upstreamHandle != nil {
		g.upstreamHandle.Close()
		g.upstreamHandle = nil
	}
	// Update the service-level KV cache shadow if the stream finished
	// cleanly. A "clean" exit is one where we observed a terminal finish
	// reason — i.e. the iterator drained naturally rather than the caller
	// abandoning it mid-stream. On any other exit we leave cachedTokens
	// nil so the next turn starts from a known-empty cache.
	if g.closed && g.finish != "" && g.finish != llmapi.FinishReasonNull && !g.hasMedia {
		// At this point the KV cache holds prompt + generated, which equals
		// nPast tokens. Stitch them together for the next turn's diff.
		merged := make([]int32, 0, len(g.prompt)+len(g.generated))
		merged = append(merged, g.prompt...)
		merged = append(merged, g.generated...)
		g.svc.cachedTokens = merged
	} else {
		// Caller cancelled or some other unhealthy exit — KV state is
		// indeterminate. Force a full reset on the next turn.
		g.svc.cachedTokens = nil
	}
	g.svc.ctxMu.Unlock()
	return nil
}

// svcContext returns the Context under the caller's lock. Kept as a helper so
// that a future refactor to multi-context pooling has a single choke point.
func (s *Service) svcContext() *Context {
	return s.context
}

// Ensure Service satisfies llmapi.Service at compile time.
var _ llmapi.Service = (*Service)(nil)
