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

package gemini

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
)

// sseServer returns an httptest.Server that replays the given streamGenerateContent
// SSE chunks, plus the gemini.Config pointing the SDK at it.
func sseServer(t *testing.T, chunks []string) (*httptest.Server, Config) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, c := range chunks {
			_, _ = w.Write([]byte("data: " + c + "\n\n"))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, Config{BaseURL: srv.URL}
}

func drain(t *testing.T, it llmapi.Service, model llmapi.ModelEntry, req llmapi.Request) []llmapi.Event {
	t.Helper()
	iter, err := it.CreateCompletion(context.Background(), model, req)
	require.NoError(t, err)
	defer func() { _ = iter.Close() }()
	var events []llmapi.Event
	for {
		ev, ok := iter.Next(context.Background())
		if !ok {
			break
		}
		events = append(events, ev)
	}
	return events
}

func TestCreateCompletionStream(t *testing.T) {
	chunks := []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"thinking...","thought":true}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"Hello"}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"read_file","args":{"path":"a.go"}}}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":5,"thoughtsTokenCount":3}}`,
	}
	srv, cfg := sseServer(t, chunks)
	_ = srv
	svc := NewClient("test-key", cfg)

	model := llmapi.ModelEntry{Name: Gemini_3_Flash_Preview, Provider: LLMProvider}
	events := drain(t, svc, model, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}},
	})

	require.GreaterOrEqual(t, len(events), 4)
	assert.Equal(t, llmapi.EventReasoningDelta, events[0].Type)
	assert.Equal(t, "thinking...", events[0].Reasoning)
	assert.Equal(t, llmapi.EventTextDelta, events[1].Type)
	assert.Equal(t, "Hello", events[1].Text)

	assert.Equal(t, llmapi.EventToolCallDone, events[2].Type)
	require.NotNil(t, events[2].ToolCall)
	assert.Equal(t, "read_file", events[2].ToolCall.Function.Name)
	assert.Equal(t, "call_1", events[2].ToolCall.ID) // synthesized, no ID from Gemini
	assert.JSONEq(t, `{"path":"a.go"}`, events[2].ToolCall.Function.Arguments)

	done := events[len(events)-1]
	require.Equal(t, llmapi.EventStreamDone, done.Type)
	require.NotNil(t, done.DoneData)
	assert.Equal(t, llmapi.FinishReasonToolCall, done.DoneData.FinishReason)
	assert.Equal(t, "Hello", done.DoneData.Message.Content)
	assert.Equal(t, "thinking...", done.DoneData.Message.ReasoningContent)
	require.Len(t, done.DoneData.Message.ToolCalls, 1)
	assert.Equal(t, 12, done.DoneData.Usage.TokensSent)
	assert.Equal(t, 5, done.DoneData.Usage.TokensReceived)
	assert.Equal(t, 3, done.DoneData.Usage.TokensReasoned)
}

func TestCreateCompletionParallelToolCallIDs(t *testing.T) {
	chunks := []string{
		`{"candidates":[{"content":{"role":"model","parts":[` +
			`{"functionCall":{"name":"read_file","args":{"path":"a.go"}}},` +
			`{"functionCall":{"name":"read_file","args":{"path":"b.go"}}}` +
			`]},"finishReason":"STOP"}]}`,
	}
	_, cfg := sseServer(t, chunks)
	svc := NewClient("test-key", cfg)

	model := llmapi.ModelEntry{Name: Gemini_3_Flash_Preview, Provider: LLMProvider}
	events := drain(t, svc, model, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "read both"}},
	})

	var ids []string
	for _, ev := range events {
		if ev.Type == llmapi.EventToolCallDone {
			ids = append(ids, ev.ToolCall.ID)
		}
	}
	require.Len(t, ids, 2)
	// Parallel calls must get distinct IDs so results can be joined back.
	assert.NotEqual(t, ids[0], ids[1])
}

// TestCreateCompletionThoughtSignatureOnSiblingPart reproduces RUNE-AGENT
// Gemini 3+ 400 "Function call is missing a thought_signature": when streaming,
// the model may deliver the signature on an empty-text part that precedes the
// functionCall part rather than on the functionCall part itself. The captured
// tool call must still carry the signature so it can be replayed.
func TestCreateCompletionThoughtSignatureOnSiblingPart(t *testing.T) {
	// "c2ln" base64-decodes to the bytes for "sig".
	chunks := []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"text":"","thoughtSignature":"c2ln"}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"agent","args":{"q":"x"}}}]},"finishReason":"STOP"}]}`,
	}
	_, cfg := sseServer(t, chunks)
	svc := NewClient("test-key", cfg)

	model := llmapi.ModelEntry{Name: Gemini_3_Flash_Preview, Provider: LLMProvider}
	events := drain(t, svc, model, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "go"}},
	})

	var done *llmapi.DoneData
	for i := range events {
		if events[i].Type == llmapi.EventStreamDone {
			done = events[i].DoneData
		}
	}
	require.NotNil(t, done)
	require.Len(t, done.Message.ToolCalls, 1)
	assert.Equal(t, []byte("sig"), thoughtSignature(done.Message.ToolCalls[0]))
}

// TestCreateCompletionParallelToolCallSignature verifies that for parallel
// function calls only the first carries the signature (Gemini only attaches it
// to the first functionCall part), matching the documented replay contract.
func TestCreateCompletionParallelToolCallSignature(t *testing.T) {
	chunks := []string{
		`{"candidates":[{"content":{"role":"model","parts":[` +
			`{"functionCall":{"name":"read_file","args":{"path":"a.go"}},"thoughtSignature":"c2ln"},` +
			`{"functionCall":{"name":"read_file","args":{"path":"b.go"}}}` +
			`]},"finishReason":"STOP"}]}`,
	}
	_, cfg := sseServer(t, chunks)
	svc := NewClient("test-key", cfg)

	model := llmapi.ModelEntry{Name: Gemini_3_Flash_Preview, Provider: LLMProvider}
	events := drain(t, svc, model, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "read both"}},
	})

	var done *llmapi.DoneData
	for i := range events {
		if events[i].Type == llmapi.EventStreamDone {
			done = events[i].DoneData
		}
	}
	require.NotNil(t, done)
	require.Len(t, done.Message.ToolCalls, 2)
	assert.Equal(t, []byte("sig"), thoughtSignature(done.Message.ToolCalls[0]))
	assert.Nil(t, thoughtSignature(done.Message.ToolCalls[1]))
}

// TestCreateCompletionParallelToolCallsAcrossChunks mirrors the real Gemini 3
// streaming shape observed in sub-agent sessions: several parallel function
// calls arrive in separate SSE chunks, with the thought_signature attached only
// to the first call's chunk. The first persisted tool call must carry it.
func TestCreateCompletionParallelToolCallsAcrossChunks(t *testing.T) {
	chunks := []string{
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"search_symbols","args":{"q":"a"}},"thoughtSignature":"c2ln"}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"search_symbols","args":{"q":"b"}}}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"search_symbols","args":{"q":"c"}}}]}}]}`,
		`{"candidates":[{"content":{"role":"model","parts":[{"functionCall":{"name":"search_content","args":{"q":"d"}}}]},"finishReason":"STOP"}]}`,
	}
	_, cfg := sseServer(t, chunks)
	svc := NewClient("test-key", cfg)

	model := llmapi.ModelEntry{Name: Gemini_3_Flash_Preview, Provider: LLMProvider}
	events := drain(t, svc, model, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "explore"}},
	})

	var done *llmapi.DoneData
	for i := range events {
		if events[i].Type == llmapi.EventStreamDone {
			done = events[i].DoneData
		}
	}
	require.NotNil(t, done)
	require.Len(t, done.Message.ToolCalls, 4)
	assert.Equal(t, []byte("sig"), thoughtSignature(done.Message.ToolCalls[0]),
		"first parallel call must carry the signature across chunked streaming")
	for i := 1; i < 4; i++ {
		assert.Nil(t, thoughtSignature(done.Message.ToolCalls[i]))
	}
}

// TestCreateCompletionEmptyCompletionSurfacesError reproduces the silent
// sub-agent termination: Gemini returns a STOP with no text, reasoning, or
// tool calls (an empty completion). This must surface as a stream error so the
// agent loop does not silently end with an empty, non-error result that the
// parent cannot act on.
func TestCreateCompletionEmptyCompletionSurfacesError(t *testing.T) {
	chunks := []string{
		`{"candidates":[{"content":{"role":"model","parts":[]},"finishReason":"STOP"}],` +
			`"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":0}}`,
	}
	_, cfg := sseServer(t, chunks)
	svc := NewClient("test-key", cfg)

	model := llmapi.ModelEntry{Name: Gemini_3_Flash_Preview, Provider: LLMProvider}
	events := drain(t, svc, model, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "continue"}},
	})

	var sawError bool
	for _, ev := range events {
		if ev.Type == llmapi.EventStreamError {
			sawError = true
			assert.Error(t, ev.Error)
		}
		assert.NotEqual(t, llmapi.EventStreamDone, ev.Type,
			"empty completion must not produce a successful done event")
	}
	assert.True(t, sawError, "empty completion must surface a stream error")
}

func TestCreateCompletionAPIErrorSurfaced(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":400,"status":"INVALID_ARGUMENT","message":"function schema is invalid"}}`))
	}))
	t.Cleanup(srv.Close)

	svc := NewClient("test-key", Config{BaseURL: srv.URL})
	model := llmapi.ModelEntry{Name: Gemini_3_Flash_Preview, Provider: LLMProvider}
	events := drain(t, svc, model, llmapi.Request{
		Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}},
	})

	require.NotEmpty(t, events)
	last := events[len(events)-1]
	require.Equal(t, llmapi.EventStreamError, last.Type)
	require.Error(t, last.Error)
	msg := last.Error.Error()
	assert.True(t, strings.Contains(msg, "INVALID_ARGUMENT"), "got %q", msg)
	assert.True(t, strings.Contains(msg, "function schema is invalid"), "got %q", msg)
}

// capturingSSEServer replays the given SSE chunks and records the last request
// body it received, so tests can assert what was sent on the wire.
func capturingSSEServer(t *testing.T, chunks []string) (*string, Config) {
	t.Helper()
	var lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		lastBody = string(b)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, c := range chunks {
			_, _ = w.Write([]byte("data: " + c + "\n\n"))
		}
	}))
	t.Cleanup(srv.Close)
	return &lastBody, Config{BaseURL: srv.URL}
}

// TestCreateCompletionAlwaysSendsThinkingConfigForGemini3 verifies that a
// Gemini 3 request always carries a thinkingConfig even when no reasoning
// effort is requested, so the model returns thought signatures (required for
// function calling). Gemini 2.x omits it when no effort is set.
func TestCreateCompletionAlwaysSendsThinkingConfigForGemini3(t *testing.T) {
	doneChunk := `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}]}`

	t.Run("gemini 3 with no effort still sends thinkingConfig", func(t *testing.T) {
		body, cfg := capturingSSEServer(t, []string{doneChunk})
		svc := NewClient("k", cfg)
		model := llmapi.ModelEntry{Name: Gemini_3_Flash_Preview, Provider: LLMProvider}
		drain(t, svc, model, llmapi.Request{
			Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}},
		})
		assert.Contains(t, *body, "thinkingConfig", "gemini 3 must always send thinkingConfig")
	})

	t.Run("gemini 2.5 with no effort omits thinkingConfig", func(t *testing.T) {
		body, cfg := capturingSSEServer(t, []string{doneChunk})
		svc := NewClient("k", cfg)
		model := llmapi.ModelEntry{Name: Gemini_2_5_Flash, Provider: LLMProvider}
		drain(t, svc, model, llmapi.Request{
			Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}},
		})
		assert.NotContains(t, *body, "thinkingConfig", "gemini 2.x must omit thinkingConfig when no effort")
	})
}

// TestCreateCompletionEffortWarningProvenance pins the rule that only an
// explicit per-request effort produces a warning; the workspace config value
// is a standing preference and is dropped silently on models that cannot
// honor it.
func TestCreateCompletionEffortWarningProvenance(t *testing.T) {
	doneChunk := `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}]}`
	model := llmapi.ModelEntry{Name: Gemini_2_5_Flash, Provider: LLMProvider}

	t.Run("request effort warns", func(t *testing.T) {
		_, cfg := sseServer(t, []string{doneChunk})
		events := drain(t, NewClient("k", cfg), model, llmapi.Request{
			Messages:        []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}},
			ReasoningEffort: llmapi.ReasoningEffort("xhigh"),
		})
		assert.Equal(t, llmapi.EventRateLimitWarning, events[0].Type)
	})

	t.Run("config effort is silent", func(t *testing.T) {
		_, cfg := sseServer(t, []string{doneChunk})
		cfg.ReasoningEffort = "xhigh"
		events := drain(t, NewClient("k", cfg), model, llmapi.Request{
			Messages: []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}},
		})
		for _, ev := range events {
			assert.NotEqual(t, llmapi.EventRateLimitWarning, ev.Type)
		}
	})
}

func TestCreateCompletionMaxOutputTokens(t *testing.T) {
	doneChunk := `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}]}`
	model := llmapi.ModelEntry{Name: Gemini_3_6_Flash, Provider: LLMProvider}

	tests := []struct {
		name       string
		provider   int
		request    int
		wantValue  string
		wantAbsent bool
	}{
		{name: "provider default omitted", wantAbsent: true},
		{name: "request override", request: 4096, wantValue: `"maxOutputTokens":4096`},
		{name: "request override capped", request: 100000, wantValue: `"maxOutputTokens":65536`},
		{name: "config fallback capped", provider: 100000, wantValue: `"maxOutputTokens":65536`},
		{name: "request wins before cap", provider: 2048, request: 4096, wantValue: `"maxOutputTokens":4096`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, cfg := capturingSSEServer(t, []string{doneChunk})
			cfg.MaxOutputTokens = tt.provider
			svc := NewClient("k", cfg)
			drain(t, svc, model, llmapi.Request{
				Messages:        []llmapi.Message{{Role: llmapi.RoleUser, Content: "hi"}},
				MaxOutputTokens: tt.request,
			})
			if tt.wantAbsent {
				assert.NotContains(t, *body, "maxOutputTokens")
				return
			}
			assert.Contains(t, *body, tt.wantValue)
		})
	}
}

func TestEffectiveMaxOutputTokens(t *testing.T) {
	tests := []struct {
		name          string
		model         string
		requestLimit  int
		configLimit   int
		wantEffective int
	}{
		{name: "no override", model: Gemini_3_6_Flash},
		{name: "request override", model: Gemini_3_6_Flash, requestLimit: 4096, configLimit: 2048, wantEffective: 4096},
		{name: "config fallback", model: Gemini_3_6_Flash, configLimit: 2048, wantEffective: 2048},
		{name: "3.6 flash cap", model: Gemini_3_6_Flash, requestLimit: 100000, wantEffective: 65536},
		{name: "3.5 flash-lite cap", model: Gemini_3_5_FlashLite, requestLimit: 100000, wantEffective: 65536},
		{name: "unknown model unchanged", model: "gemini-future", requestLimit: 100000, wantEffective: 100000},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantEffective,
				effectiveMaxOutputTokens(tt.model, tt.requestLimit, tt.configLimit))
		})
	}
}

// listServer serves a single page of the Gemini ListModels response using the
// wire field names the genai SDK expects (supportedGenerationMethods,
// inputTokenLimit). The body is intentionally Gemini-API shaped.
func listServer(t *testing.T, body string) Config {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return Config{BaseURL: srv.URL}
}

const modelsListBody = `{
  "models": [
    {"name":"models/gemini-2.5-pro","inputTokenLimit":1048576,"supportedGenerationMethods":["generateContent","countTokens"]},
    {"name":"models/gemini-2.5-flash","inputTokenLimit":1048576,"supportedGenerationMethods":["generateContent"]},
    {"name":"models/text-embedding-004","inputTokenLimit":2048,"supportedGenerationMethods":["embedContent"]},
    {"name":"models/legacy-no-actions","inputTokenLimit":4096}
  ]
}`

func TestModels(t *testing.T) {
	svc := NewClient("test-key", listServer(t, modelsListBody))
	it := svc.Models()
	defer func() { _ = it.Close() }()

	var got []llmapi.ModelEntry
	for {
		e, ok := it.Next(context.Background())
		if !ok {
			break
		}
		got = append(got, e)
	}
	require.NoError(t, it.Err())

	// Embedding model is filtered out; the rest (incl. the actionless legacy
	// entry) are kept with the "models/" prefix stripped.
	names := make(map[string]int)
	for _, e := range got {
		names[e.Name] = e.ContextWindow
		assert.Equal(t, LLMProvider, e.Provider)
	}
	assert.Contains(t, names, "gemini-2.5-pro")
	assert.Contains(t, names, "gemini-2.5-flash")
	assert.Contains(t, names, "legacy-no-actions")
	assert.NotContains(t, names, "text-embedding-004")
	assert.Equal(t, 1048576, names["gemini-2.5-pro"])
}

// TestModelsFallsBackOnError verifies that when the live listing fails
// (e.g. an invalid key), Models serves the static catalog so callers
// still see models, while Err surfaces the underlying error so the user
// can diagnose and fix the cause.
func TestModelsFallsBackOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403,"status":"PERMISSION_DENIED","message":"invalid key"}}`))
	}))
	t.Cleanup(srv.Close)

	svc := NewClient("bad-key", Config{BaseURL: srv.URL})
	it := svc.Models()
	defer func() { _ = it.Close() }()

	var got []llmapi.ModelEntry
	for {
		e, ok := it.Next(context.Background())
		if !ok {
			break
		}
		got = append(got, e)
	}

	// The static catalog is served as a fallback.
	assert.ElementsMatch(t, ModelEntries(), got)
	// The live error is still surfaced so the user can fix the cause.
	require.Error(t, it.Err())
	assert.Contains(t, it.Err().Error(), "PERMISSION_DENIED")
}

// TestModelsFallsBackOnMissingClient verifies the static catalog is
// served when the client could not be constructed at all (e.g. empty
// key), the bootstrap case where no working key exists yet.
func TestModelsFallsBackOnMissingClient(t *testing.T) {
	svc := NewClient("", Config{})
	it := svc.Models()
	defer func() { _ = it.Close() }()

	var got []llmapi.ModelEntry
	for {
		e, ok := it.Next(context.Background())
		if !ok {
			break
		}
		got = append(got, e)
	}
	assert.ElementsMatch(t, ModelEntries(), got)
	require.Error(t, it.Err())
}

func TestGetModel(t *testing.T) {
	svc := NewClient("test-key", listServer(t, modelsListBody))

	got, err := svc.GetModel(context.Background(), llmapi.ModelEntry{Name: "gemini-2.5-pro"})
	require.NoError(t, err)
	assert.Equal(t, LLMProvider, got.Provider)
	assert.Equal(t, 1048576, got.ContextWindow)

	_, err = svc.GetModel(context.Background(), llmapi.ModelEntry{Name: "nonexistent"})
	assert.ErrorIs(t, err, llmapi.ErrModelNotFound)
}

// TestGetModelFallsBackToStatic verifies GetModel resolves a known model
// from the static catalog when the live query is unavailable (no key).
func TestGetModelFallsBackToStatic(t *testing.T) {
	svc := NewClient("", Config{})

	got, err := svc.GetModel(context.Background(), llmapi.ModelEntry{Name: Gemini_2_5_Pro})
	require.NoError(t, err)
	assert.Equal(t, LLMProvider, got.Provider)
	assert.Positive(t, got.ContextWindow)

	_, err = svc.GetModel(context.Background(), llmapi.ModelEntry{Name: "nonexistent"})
	assert.ErrorIs(t, err, llmapi.ErrModelNotFound)
}
