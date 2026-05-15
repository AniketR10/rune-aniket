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


package llamacpp_test

// End-to-end suite that exercises the llamacpp.Service through the
// llmapi.Service interface against a real, small GGUF model. The model is
// fetched via our own ociregistry.Client the first time the suite runs
// and cached on disk at ${UserCacheDir}/rune/llamacpp-e2e/ so subsequent
// runs are fast.
//
// Environment overrides:
//
//	RUNE_LLAMACPP_E2E=1       — opt in to the suite. Without this flag
//	                            (and without RUNE_LLAMACPP_E2E_MODEL) the
//	                            suite skips — running it costs a ~1 GiB
//	                            download the first time.
//	RUNE_LLAMACPP_E2E_MODEL   — absolute path to a local .gguf file. When
//	                            set, the download step is skipped entirely
//	                            (useful for CI and offline dev) and the
//	                            suite runs against the given file.
//	RUNE_LLAMACPP_E2E_OFFLINE=1
//	                          — never hit the network. Only use what's
//	                            already cached; skip otherwise.
//
// The default model is a Q2_K quantization of Gemma-3n-E2B-IT (~1 GiB)
// published by Unsloth. It is small enough to load on a modest laptop,
// yet instruction-tuned well enough to exercise tool calling and prefix
// reuse.

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/unstablebuild/rune-go-sdk/iterator"
	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"unstable.build/go-tui/llm/llamacpp"
	"unstable.build/go-tui/llm/llamacpp/ociregistry"
)

const (
	// e2eReference is fetched via ociregistry the first time the suite
	// runs. Pin a specific quant so the download size and inference
	// behaviour stay stable across test runs.
	e2eReference = "unsloth/gemma-3n-E2B-it-GGUF:Q2_K"
	// e2eSeed keeps completions reproducible enough for tests that assert
	// on the observable shape of streaming events.
	e2eSeed = 42
)

var (
	serviceOnce sync.Once
	sharedSvc   *llamacpp.Service
	sharedErr   error
)

// loadSharedService returns a process-wide llamacpp.Service for the suite.
// Subtests share the service (and its loaded weights) via sync.Once to
// keep the overall run time bounded; each CreateCompletion call is
// independent anyway because the service serializes them.
func loadSharedService(t *testing.T) *llamacpp.Service {
	t.Helper()
	if os.Getenv("RUNE_LLAMACPP_E2E") != "1" && os.Getenv("RUNE_LLAMACPP_E2E_MODEL") == "" {
		t.Skip("llamacpp e2e: set RUNE_LLAMACPP_E2E=1 (or RUNE_LLAMACPP_E2E_MODEL=/path/to/model.gguf) to run")
	}
	serviceOnce.Do(func() {
		path, err := cachedE2EModel(t)
		if err != nil {
			sharedErr = err
			return
		}
		sharedSvc, sharedErr = llamacpp.NewService(llamacpp.Config{
			Model:         "gemma-3n-e2b-q2k-e2e",
			ModelPath:     path,
			ContextWindow: 2048,
			NGPULayers:    -1,
			Sampling: llamacpp.SamplerParams{
				Seed:          e2eSeed,
				Temperature:   0.2,
				TopK:          40,
				TopP:          0.95,
				MinP:          0.05,
				RepeatPenalty: 1.1,
				RepeatLastN:   64,
			},
		})
		if sharedErr == nil && sharedSvc != nil {
			// Close the loaded Service before the test binary exits so the
			// ggml-metal singleton device is empty when its static
			// destructor runs.
			llamacpp.RegisterTestCleanup(func() {
				if sharedSvc != nil {
					sharedSvc.Close()
					sharedSvc = nil
				}
			})
		}
	})
	if sharedErr != nil {
		if errors.Is(sharedErr, errSkipE2E) {
			t.Skip(sharedErr.Error())
		}
		t.Fatalf("loadSharedService: %v", sharedErr)
	}
	return sharedSvc
}

// errSkipE2E is returned by cachedE2EModel when the environment indicates
// the suite should be skipped (no network, user opted out, etc).
var errSkipE2E = errors.New("llamacpp e2e: skipped")

// cachedE2EModel returns the absolute path of the GGUF weight file to
// feed into llamacpp.NewService. When RUNE_LLAMACPP_E2E_MODEL is set we
// use that path verbatim; otherwise we pull e2eReference into the
// per-user cache directory using our own ociregistry client.
func cachedE2EModel(t *testing.T) (string, error) {
	t.Helper()
	if p := os.Getenv("RUNE_LLAMACPP_E2E_MODEL"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("RUNE_LLAMACPP_E2E_MODEL=%q: %w", p, err)
		}
		return p, nil
	}

	baseDir, err := os.UserCacheDir()
	if err != nil {
		return "", fmt.Errorf("UserCacheDir: %w", err)
	}
	cacheDir := filepath.Join(baseDir, "rune", "llamacpp-e2e")
	cache, err := ociregistry.OpenCache(cacheDir)
	if err != nil {
		return "", fmt.Errorf("OpenCache(%s): %w", cacheDir, err)
	}

	ref, err := ociregistry.ParseReferenceWithDefault(e2eReference, "huggingface.co")
	if err != nil {
		return "", fmt.Errorf("parse %q: %w", e2eReference, err)
	}

	// Fast path: a previous run already pulled this reference.
	if pr, err := ociregistry.Resolve(cache, ref); err == nil && pr.ModelPath != "" {
		if _, err := os.Stat(pr.ModelPath); err == nil {
			return pr.ModelPath, nil
		}
	}

	if os.Getenv("RUNE_LLAMACPP_E2E_OFFLINE") == "1" {
		return "", fmt.Errorf("%w: offline and no cached model at %s",
			errSkipE2E, cacheDir)
	}

	t.Logf("llamacpp e2e: downloading %s into %s (first-run only)", ref.String(), cacheDir)

	client := ociregistry.NewClient()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	pr, err := client.Pull(ctx, cache, ref, nil)
	if err != nil {
		return "", fmt.Errorf("%w: pull %s: %v", errSkipE2E, ref.String(), err)
	}
	if pr.ModelPath == "" {
		return "", fmt.Errorf("%w: no model layer in manifest for %s",
			errSkipE2E, ref.String())
	}
	return pr.ModelPath, nil
}

func TestE2E_ContextWindow(t *testing.T) {
	svc := loadSharedService(t)
	if svc.ContextWindow() <= 0 {
		t.Fatalf("expected positive context window, got %d", svc.ContextWindow())
	}
}

// TestE2E_RegistryReadsContextWindowFromGGUF exercises the metadata-driven
// path of llamacpp.Registry against a real GGUF pulled from the cache. It
// asserts the registry reports exactly the training context advertised by
// the GGUF metadata — which is the contract of contextWindowForPath when
// no override is set.
func TestE2E_RegistryReadsContextWindowFromGGUF(t *testing.T) {
	// Ground-truth NCtxTrain comes from the already-loaded shared
	// model to avoid loading a second copy. loadSharedService also
	// handles the skip path when the e2e suite is disabled.
	svc := loadSharedService(t)
	want := svc.Model().NCtxTrain()
	if want <= 0 {
		t.Fatalf("expected positive NCtxTrain from shared model, got %d", want)
	}

	// Point the registry at the real cache used to pull e2eReference
	// so the manifest + blob are already in place. The registry
	// reads context-window metadata straight from the GGUF header.
	baseDir, err := os.UserCacheDir()
	if err != nil {
		t.Fatalf("UserCacheDir: %v", err)
	}
	cacheDir := filepath.Join(baseDir, "rune", "llamacpp-e2e")
	r, err := llamacpp.NewRegistry(cacheDir)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	refs, err := r.CachedReferences()
	if err != nil || len(refs) == 0 {
		t.Skipf("no cached e2e references (err=%v, len=%d)", err, len(refs))
	}
	var e llmapi.ModelEntry
	var ok bool
	for _, ref := range refs {
		if e, ok = r.Get(context.Background(), ref.String()); ok {
			break
		}
	}
	if !ok {
		t.Fatalf("registry did not resolve any cached reference: %v", refs)
	}
	if e.ContextWindow != want {
		t.Fatalf("registry.ContextWindow = %d, want %d (NCtxTrain from GGUF "+
			"metadata at %s)", e.ContextWindow, want, e.BaseURL)
	}
}

func TestE2E_CountTokens(t *testing.T) {
	svc := loadSharedService(t)
	n, err := svc.CountTokens(llmapi.ModelEntry{}, []llmapi.Message{
		{Role: llmapi.RoleUser, Content: "hello world"},
	})
	if err != nil {
		t.Fatalf("CountTokens: %v", err)
	}
	if n <= 0 {
		t.Fatalf("expected positive token count, got %d", n)
	}
}

// TestE2E_CreateCompletion_Basic drives a short single-turn request and
// asserts the shape of the event stream: text deltas concatenate into the
// final message, Usage is populated, and a single EventStreamDone closes
// the stream.
func TestE2E_CreateCompletion_Basic(t *testing.T) {
	svc := loadSharedService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	it, err := svc.CreateCompletion(ctx, llmapi.ModelEntry{}, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "Say hi in one word."},
		},
		MaxOutputTokens: 8,
	})
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}
	defer func() { _ = it.Close() }()
	evs, err := iterator.ToSlice(ctx, it)
	if err != nil {
		t.Fatalf("ToSlice: %v", err)
	}

	var (
		text    strings.Builder
		done    *llmapi.DoneData
		numDone int
	)
	for _, e := range evs {
		switch e.Type {
		case llmapi.EventTextDelta:
			text.WriteString(e.Text)
		case llmapi.EventStreamDone:
			numDone++
			done = e.DoneData
		case llmapi.EventStreamError:
			t.Fatalf("stream error: %v", e.Error)
		}
	}
	if numDone != 1 {
		t.Fatalf("expected exactly one EventStreamDone, got %d", numDone)
	}
	if done == nil {
		t.Fatal("EventStreamDone without DoneData")
	}
	if text.Len() == 0 {
		t.Fatal("expected some generated text")
	}
	if done.Message.Content != text.String() {
		t.Fatalf("done.Message.Content %q != accumulated deltas %q",
			done.Message.Content, text.String())
	}
	if done.Usage.TokensSent <= 0 {
		t.Fatalf("expected TokensSent > 0, got %d", done.Usage.TokensSent)
	}
	if done.Usage.TokensReceived <= 0 {
		t.Fatalf("expected TokensReceived > 0, got %d", done.Usage.TokensReceived)
	}
	switch done.FinishReason {
	case llmapi.FinishReasonStop, llmapi.FinishReasonLength:
		// ok
	default:
		t.Fatalf("unexpected finish reason %q", done.FinishReason)
	}
}

// TestE2E_CreateCompletion_Length asserts the FinishReasonLength path by
// capping MaxOutputTokens at a tiny value.
func TestE2E_CreateCompletion_Length(t *testing.T) {
	svc := loadSharedService(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	it, err := svc.CreateCompletion(ctx, llmapi.ModelEntry{}, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "Tell me a long story about a dragon and a knight."},
		},
		MaxOutputTokens: 4,
	})
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}
	defer func() { _ = it.Close() }()
	evs, err := iterator.ToSlice(ctx, it)
	if err != nil {
		t.Fatalf("ToSlice: %v", err)
	}
	for _, e := range evs {
		if e.Type == llmapi.EventStreamDone {
			if e.DoneData.FinishReason != llmapi.FinishReasonLength {
				t.Fatalf("finish reason = %q, want %q",
					e.DoneData.FinishReason, llmapi.FinishReasonLength)
			}
			return
		}
	}
	t.Fatal("no EventStreamDone in stream")
}

// TestE2E_CreateCompletion_Tools exercises the tool-calling path. We use a
// permissive assertion — if the model emits no tool call (weaker quants
// sometimes answer in free text), we skip rather than fail.
func TestE2E_CreateCompletion_Tools(t *testing.T) {
	svc := loadSharedService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	tool := llmapi.Tool{
		Type: llmapi.ToolTypeFunction,
		Function: llmapi.FunctionDefinition{
			Name:        "get_time",
			Description: "Return the current time.",
			Parameters: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			},
		},
	}

	it, err := svc.CreateCompletion(ctx, llmapi.ModelEntry{}, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleSystem, Content: "You are a helpful assistant. Prefer calling tools when available."},
			{Role: llmapi.RoleUser, Content: "What time is it right now? Call the tool."},
		},
		Tools:           []llmapi.Tool{tool},
		MaxOutputTokens: 128,
	})
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}
	defer func() { _ = it.Close() }()
	evs, err := iterator.ToSlice(ctx, it)
	if err != nil {
		t.Fatalf("ToSlice: %v", err)
	}

	var (
		toolCalls []llmapi.ToolCall
		done      *llmapi.DoneData
	)
	for _, e := range evs {
		switch e.Type {
		case llmapi.EventToolCallDone:
			if e.ToolCall == nil {
				t.Fatal("EventToolCallDone without ToolCall")
			}
			toolCalls = append(toolCalls, *e.ToolCall)
		case llmapi.EventStreamDone:
			done = e.DoneData
		}
	}
	if done == nil {
		t.Fatal("missing EventStreamDone")
	}
	if len(toolCalls) == 0 {
		t.Skipf("model did not emit tool call (content=%q) — weak quant, not a bug", done.Message.Content)
	}
	if done.FinishReason != llmapi.FinishReasonToolCall {
		t.Fatalf("finish reason = %q, want %q", done.FinishReason, llmapi.FinishReasonToolCall)
	}
	tc := toolCalls[0]
	if tc.Function.Name != "get_time" {
		t.Fatalf("tool name = %q, want %q", tc.Function.Name, "get_time")
	}
	// Arguments must at minimum parse as JSON. An empty object is fine
	// since the schema has no required fields.
	if tc.Function.Arguments == "" {
		t.Fatal("tool call has empty arguments")
	}
	if !strings.HasPrefix(strings.TrimSpace(tc.Function.Arguments), "{") {
		t.Fatalf("arguments do not look like JSON: %q", tc.Function.Arguments)
	}
}

// TestE2E_PrefixCacheReuse issues two completions that share a long prefix
// and asserts the second turn reports TokensCached > 0.
func TestE2E_PrefixCacheReuse(t *testing.T) {
	svc := loadSharedService(t)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()

	run := func(msgs []llmapi.Message) *llmapi.DoneData {
		t.Helper()
		it, err := svc.CreateCompletion(ctx, llmapi.ModelEntry{}, llmapi.Request{
			Messages:        msgs,
			MaxOutputTokens: 8,
		})
		if err != nil {
			t.Fatalf("CreateCompletion: %v", err)
		}
		defer func() { _ = it.Close() }()
		evs, err := iterator.ToSlice(ctx, it)
		if err != nil {
			t.Fatalf("ToSlice: %v", err)
		}
		for _, e := range evs {
			if e.Type == llmapi.EventStreamDone {
				return e.DoneData
			}
		}
		t.Fatal("no EventStreamDone")
		return nil
	}

	first := []llmapi.Message{
		{Role: llmapi.RoleSystem, Content: "You are a terse assistant that answers in one short sentence."},
		{Role: llmapi.RoleUser, Content: "Give me a fun fact about the ocean."},
	}
	// The shared service may retain cached tokens from earlier subtests in the
	// same process. Compare the two turns relatively: turn 2 must cache more
	// than turn 1 because it adds a full turn on top of turn 1's prompt.
	d1 := run(first)

	second := append([]llmapi.Message{}, first...)
	second = append(second,
		llmapi.Message{Role: llmapi.RoleAssistant, Content: d1.Message.Content},
		llmapi.Message{Role: llmapi.RoleUser, Content: "Another, please."},
	)
	d2 := run(second)
	if d2.Usage.TokensCached <= d1.Usage.TokensCached {
		t.Fatalf("expected turn 2 TokensCached (%d) > turn 1 TokensCached (%d)",
			d2.Usage.TokensCached, d1.Usage.TokensCached)
	}
}

// TestE2E_ContextCancellation starts a completion and cancels the context
// after the first delta — the iterator must drain cleanly without
// wedging.
func TestE2E_ContextCancellation(t *testing.T) {
	svc := loadSharedService(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	it, err := svc.CreateCompletion(ctx, llmapi.ModelEntry{}, llmapi.Request{
		Messages: []llmapi.Message{
			{Role: llmapi.RoleUser, Content: "Count slowly from one to one hundred, one number per line."},
		},
		MaxOutputTokens: 128,
	})
	if err != nil {
		t.Fatalf("CreateCompletion: %v", err)
	}

	var sawDelta bool
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if ev.Type == llmapi.EventTextDelta && !sawDelta {
			sawDelta = true
			cancel()
		}
	}
	if err := it.Close(); err != nil {
		t.Fatalf("iterator close after cancel: %v", err)
	}
	if !sawDelta {
		t.Fatal("never saw a delta before cancelling")
	}
}
