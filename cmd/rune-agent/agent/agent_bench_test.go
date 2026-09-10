// Copyright (C) 2017-2026 The Rune Authors
// SPDX-License-Identifier: GPL-3.0-or-later
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or (at
// your option) any later version.
//
// This program is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
// General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package agent

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/unstablebuild/rune-go-sdk/api/llmapi"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// benchFixtures returns the shared test data used by both benchmarks:
// the mock LLM response sequence and the tool set. Keeping fixtures in one
// place ensures the two benchmarks measure the same workload so their
// numbers are directly comparable.
//
// The scenario mirrors a typical prod session:
//  1. LLM reads two files in parallel (two tool calls in one response).
//  2. LLM performs a search (one tool call).
//  3. LLM edits a file and reads another in parallel (two tool calls).
//  4. LLM delivers a long, multi-chunk final response (stop).
func benchFixtures() (responses []mockResponse, tools []Tool) {
	const mediumOutput = "I have read the files and analysed the code. " +
		"The implementation looks correct but there are a few areas that could be improved. " +
		"Let me make the necessary edits now."

	// longOutputChunks simulates a large final assistant response — a detailed
	// analysis report that a real coding assistant would produce after reading,
	// searching, and editing files across multiple tool-call turns. Each chunk
	// is delivered as a separate EventTextDelta, matching how streaming
	// providers emit tokens, so the agent's strings.Builder accumulates many
	// small writes before producing the final persisted message.
	longOutputChunks := []string{
		"# Analysis Report\n\n",
		"## Summary\n\n",
		"I have completed a thorough review of the `agent` package and applied the requested improvements. ",
		"Below is a detailed account of every finding, the rationale behind each change, and the remaining work ",
		"that you may wish to address in a follow-up session.\n\n",
		"## Files Examined\n\n",
		"### `agent/agent.go`\n\n",
		"This is the central orchestration file. The `run` method implements the agentic loop: it repeatedly ",
		"calls the LLM, consumes the streaming response, executes any requested tool calls in parallel via a ",
		"fan-out/fan-in pattern, appends results to the conversation, and iterates until the model emits a ",
		"`stop` finish reason or the maximum-iteration guard triggers.\n\n",
		"Key observations:\n",
		"- The `emit` helper wraps every channel send in a `select` with `ctx.Done()`, which prevents the ",
		"  goroutine from leaking when the caller cancels the context mid-stream. This is correct.\n",
		"- `persistMessages` is called on every exit path (stop, error, max-iterations), ensuring the ",
		"  conversation is always durable even after partial failures.\n",
		"- The `errorTracker` correctly supersedes stale error tool-results when a subsequent call with the ",
		"  same name and arguments succeeds. This avoids polluting the context window with redundant failures.\n",
		"- Auto-compaction at 85 % and the hint injection at 75 % create a two-stage degradation: the model ",
		"  is first nudged to compact voluntarily, and if it does not, the runtime intervenes automatically. ",
		"  This is a sensible heuristic for long-running sessions.\n\n",
		"### `agent/tool.go`\n\n",
		"The `Registry` type is straightforward. Provider overrides work via a two-level map lookup: base tools ",
		"first, then overlay. Exclusions are represented as `nil` entries, which is idiomatic Go but requires ",
		"callers to distinguish `not found` from `excluded` — both return `false` from `Get`. ",
		"This is fine for the current use case but worth documenting explicitly.\n\n",
		"### `llm/service.go`\n\n",
		"The `Service` interface is minimal and provider-agnostic. Streaming is exposed through ",
		"`iterator.Iterator[llm.Event]`, which cleanly separates backpressure management from the agent loop. ",
		"The `DoneData` event carries the canonical assistant message and usage statistics, making it the ",
		"authoritative source of truth for what the model produced — the intermediate `strings.Builder` ",
		"accumulation is only used as a fallback when `DoneData` is absent.\n\n",
		"## Changes Applied\n\n",
		"### 1. Patch to `agent/agent.go` (line 240)\n\n",
		"```diff\n",
		"-old line\n",
		"+new line\n",
		"```\n\n",
		"**Rationale:** The original line contained a subtle off-by-one error in the slice capacity hint ",
		"passed to `make` when building `reqMessages`. Because `resourceMsgs` can be non-empty, the capacity ",
		"calculation must account for it. The corrected expression is:\n\n",
		"```go\n",
		"messages := make([]llmapi.Message, 0,\n",
		"    len(dialogue.Messages)+len(resourceMsgs)+1)\n",
		"```\n\n",
		"This avoids a reallocation on the first append when context resources are present, which is the ",
		"common case in production (users typically have at least one file open).\n\n",
		"## Remaining Work\n\n",
		"The following items are outside the scope of this session but are worth addressing:\n\n",
		"1. **Token counting accuracy** — `estimateToolDefTokens` divides the JSON-serialised byte length by 4. ",
		"   This is a rough approximation; for Anthropic models the ratio is closer to 3.5 for structured JSON. ",
		"   Consider using the provider's native token counter (already available via `Service.CountTokens`) ",
		"   for tool definitions as well.\n\n",
		"2. **Parallel tool result ordering** — tool results are collected via a channel and assigned to ",
		"   `toolMsgs` by index, which correctly preserves original order for the next LLM call. However, ",
		"   `EventToolResult` events are emitted in completion order (fastest tool first). This is intentional ",
		"   for responsive UI, but should be documented so future contributors do not \"fix\" it.\n\n",
		"3. **Memory recall latency** — `a.memory.Recall` is called synchronously before the loop begins. ",
		"   If the memory backend is slow (e.g. a vector store), this adds latency to every `Run` invocation. ",
		"   Consider running it concurrently with the first `CountTokens` call.\n\n",
		"4. **Skill registry snapshot** — `skillsPromptSection(a.skillRegistry.List())` is called on every ",
		"   loop iteration even when no skills have changed. Caching the serialised section and invalidating ",
		"   it on registry mutation would remove an allocation per turn.\n\n",
		"## Conclusion\n\n",
		"The `agent` package is well-structured and the core loop is correct. The changes applied in this ",
		"session address the most impactful issue (slice capacity hint). The remaining items are optimisations ",
		"and documentation improvements that can be tackled incrementally without risk to correctness.",
	}

	const largeFileContent = `// Package foo implements the core business logic.
// It coordinates between the storage layer and the HTTP handlers,
// applying validation, rate limiting, and structured logging at each step.

package foo

import (
	"context"
	"errors"
	"fmt"
)

// Service handles all business-logic operations.
type Service struct {
	store  Store
	logger Logger
}

func (s *Service) Process(ctx context.Context, id string) error {
	item, err := s.store.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("process: get: %w", err)
	}
	if err := s.validate(item); err != nil {
		return fmt.Errorf("process: validate: %w", err)
	}
	return nil
}

func (s *Service) validate(item Item) error {
	if item.ID == "" {
		return errors.New("item ID must not be empty")
	}
	return nil
}
`
	const searchResult = `agent/agent.go:240: func (a *Agent) run(...) {
agent/agent.go:365: for i := range a.config.MaxIterations {
agent/agent.go:463: emit(ctx, ch, Event{Type: EventInferenceStart})
agent/tool.go:42: type Tool interface {`

	// Realistic usage: providers report TokensSent in every response.
	// After the first completion, the agent uses this instead of calling
	// CountTokens + estimateToolDefTokens. Setting it here matches
	// production behavior — without it, every turn falls into the
	// expensive fallback path, which doesn't reflect real usage.
	reportedUsage := llmapi.Usage{TokensSent: 2000, TokensReceived: 150}

	responses = []mockResponse{
		// Step 1: read two files in parallel.
		{
			chunks:       []string{""},
			finishReason: llmapi.FinishReasonToolCall,
			usage:        reportedUsage,
			toolCalls: []llmapi.ToolCall{
				{
					ID:   "c1",
					Type: llmapi.ToolTypeFunction,
					Function: llmapi.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"agent/agent.go"}`,
					},
				},
				{
					ID:   "c2",
					Type: llmapi.ToolTypeFunction,
					Function: llmapi.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"agent/tool.go"}`,
					},
				},
			},
		},
		// Step 2: search for a symbol.
		{
			chunks:       []string{""},
			finishReason: llmapi.FinishReasonToolCall,
			usage:        reportedUsage,
			toolCalls: []llmapi.ToolCall{
				{
					ID:   "c3",
					Type: llmapi.ToolTypeFunction,
					Function: llmapi.FunctionCall{
						Name:      "search_content",
						Arguments: `{"query":"Agent.run","path":"agent"}`,
					},
				},
			},
		},
		// Step 3: edit a file and read another in parallel.
		{
			chunks:       []string{mediumOutput},
			finishReason: llmapi.FinishReasonToolCall,
			usage:        reportedUsage,
			toolCalls: []llmapi.ToolCall{
				{
					ID:   "c4",
					Type: llmapi.ToolTypeFunction,
					Function: llmapi.FunctionCall{
						Name:      "apply_patch",
						Arguments: `{"path":"agent/agent.go","patch":"--- a/agent/agent.go\n+++ b/agent/agent.go\n@@ -1 +1 @@\n-old line\n+new line"}`,
					},
				},
				{
					ID:   "c5",
					Type: llmapi.ToolTypeFunction,
					Function: llmapi.FunctionCall{
						Name:      "read_file",
						Arguments: `{"path":"llm/service.go"}`,
					},
				},
			},
		},
		// Step 4: final stop response — a long, multi-chunk streaming answer
		// that exercises the strings.Builder accumulation path and produces
		// a large assistant message for persistence.
		{
			chunks:       longOutputChunks,
			finishReason: llmapi.FinishReasonStop,
			usage:        reportedUsage,
		},
	}

	tools = []Tool{
		&mockTool{name: "read_file", result: ToolResult{Content: largeFileContent}},
		&mockTool{name: "search_content", result: ToolResult{Content: searchResult}},
		&mockTool{name: "apply_patch", result: ToolResult{Content: "patch applied successfully"}},
	}
	return responses, tools
}

// BenchmarkAgentRun measures the end-to-end throughput of Agent.Run, including
// the goroutine launch, buffered channel creation, and concurrent event delivery.
// The event iterator is drained on the calling goroutine as fast as possible so
// that back-pressure on the channel never stalls the agent goroutine.
//
// Compare against BenchmarkAgentRunDirect to isolate the cost of the async
// event-delivery layer.
func BenchmarkAgentRun(b *testing.B) {
	responses, tools := benchFixtures()

	// Hoist immutable, setup-only objects out of the timed loop.
	// noSkills() parses built-in YAML on every call; NewRegistry allocates
	// maps. Neither mutates during a run, so sharing them across iterations
	// keeps the benchmark focused on the agent logic rather than setup cost.
	skillRegistry := noSkills()
	registry := NewRegistry(tools...)

	// GOMAXPROCS=1 eliminates two sources of profile noise that would
	// otherwise swamp the agent logic:
	//
	//  1. Idle OS threads: with GOMAXPROCS=10 and only 1-2 active
	//     goroutines, 8 threads sit in pthread_cond_wait and get sampled
	//     constantly, inflating "runtime" to ~97% of the profile.
	//
	//  2. Goroutine fan-out overhead: the agent spawns one goroutine per
	//     tool call. Mock tools complete in nanoseconds, so goroutine
	//     creation and OS-level wakeups (pthread_cond_signal) dominate
	//     their execution time. With GOMAXPROCS=1 scheduling is
	//     cooperative; the goroutines still run but without real
	//     OS-thread preemption.
	//
	// This matches the bottleneck that actually matters: the agent's
	// message-building, storage, and event-emission logic.
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		// Fresh store and service for every iteration so state does not leak
		// across runs and the benchmark always exercises the new-dialogue path.
		store := newMockStore()
		svc := &mockService{responses: responses}
		ag := NewAgent(svc, registry, skillRegistry, store, NoMemory(),
			Config{SystemPrompt: "You are a helpful coding assistant."})

		ctx, cancel := context.WithCancel(context.Background())
		it := ag.Run(ctx, fmt.Sprintf("bench-%d", i), "Analyse and improve the agent package.")

		// Drain the event iterator as fast as possible so the buffered
		// channel never applies back-pressure to the agent goroutine.
		drainIterator(it)
		cancel()
	}
}

// BenchmarkAgentRunDirect benchmarks the internal run method directly,
// bypassing the goroutine and channel machinery added by Run. The channel
// is pre-allocated with a buffer large enough (~256) that run never blocks
// on a send, eliminating goroutine scheduling and channel contention from
// the measurement entirely.
//
// Compare against BenchmarkAgentRun to isolate the cost of the async
// event-delivery layer.
func BenchmarkAgentRunDirect(b *testing.B) {
	responses, tools := benchFixtures()
	skillRegistry := noSkills()
	registry := NewRegistry(tools...)

	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		store := newMockStore()
		svc := &mockService{responses: responses}
		ag := NewAgent(svc, registry, skillRegistry, store, NoMemory(),
			Config{SystemPrompt: "You are a helpful coding assistant."})

		ctx, cancel := context.WithCancel(context.Background())

		// Buffer sized above the maximum number of events emitted by our
		// 4-turn scenario (~73) so run never blocks on a channel send.
		ch := make(chan Event, 128)
		ag.run(ctx, ch,
			fmt.Sprintf("bench-%d", i),
			"Analyse and improve the agent package.",
			runOptions{})
		cancel()

		// Drain remaining events after run returns (outside the hot path).
		// run does not close the channel, so use len-based draining.
		for len(ch) > 0 {
			<-ch
		}
	}
}

// drainIterator consumes all events from the iterator without
// blocking on anything other than the iterator itself.
func drainIterator(it iterator.Iterator[Event]) {
	ctx := context.Background()
	for {
		_, ok := it.Next(ctx)
		if !ok {
			break
		}
	}
}
