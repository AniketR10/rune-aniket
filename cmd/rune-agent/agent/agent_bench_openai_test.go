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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"testing"

	"unstable.build/go-tui/cmd/rune-agent/llm/openai"
)

// openAIMockTransport is an http.RoundTripper that returns pre-built SSE
// response bodies in sequence for /v1/chat/completions requests. Not
// goroutine-safe; allocate one instance per benchmark iteration.
type openAIMockTransport struct {
	bodies [][]byte
	idx    int
}

func (t *openAIMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body := t.bodies[t.idx%len(t.bodies)]
	t.idx++
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, nil
}

// openAISSEChunk writes a single OpenAI SSE data line to b.
func openAISSEChunk(b *strings.Builder, data string) {
	b.WriteString("data: ")
	b.WriteString(data)
	b.WriteString("\n\n")
}

// buildOpenAIToolCallSSE constructs a complete OpenAI SSE response body that
// emits an optional text prefix, then streams the given tool calls via the
// ChatCompletion chunks format, and ends with finish_reason=tool_calls plus a
// usage chunk.
//
// Tool calls are streamed index-by-index. The SDK's ChatCompletionAccumulator
// detects a call as "finished" when the next call starts (higher index) or
// when the finish chunk arrives. The agent reads tool calls from doneData
// (EventStreamDone), not from EventToolCallDone, so the accumulator must
// complete all calls before the stream ends.
func buildOpenAIToolCallSSE(
	textPrefix string,
	calls []struct{ id, name, args string },
) []byte {
	var b strings.Builder

	if textPrefix != "" {
		openAISSEChunk(&b, fmt.Sprintf(
			`{"choices":[{"index":0,"delta":{"content":%q},"finish_reason":null}]}`,
			textPrefix))
	}

	for i, c := range calls {
		// Start the tool call (includes id, name, empty arguments).
		openAISSEChunk(&b, fmt.Sprintf(
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":%d,"id":%q,"type":"function","function":{"name":%q,"arguments":""}}]},"finish_reason":null}]}`,
			i, c.id, c.name))
		// Stream the full arguments in one chunk.
		openAISSEChunk(&b, fmt.Sprintf(
			`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":%d,"function":{"arguments":%q}}]},"finish_reason":null}]}`,
			i, c.args))
	}

	// Finish chunk — triggers JustFinishedToolCall for the last call in the accumulator.
	openAISSEChunk(&b, `{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`)
	// Usage chunk (stream_options.include_usage=true). PromptTokens=2000 matches
	// reportedUsage in benchFixtures so the agent uses it as lastAPITokensSent
	// and skips CountTokens on subsequent iterations.
	openAISSEChunk(&b, `{"choices":[],"usage":{"prompt_tokens":2000,"completion_tokens":25,"total_tokens":2025}}`)
	b.WriteString("data: [DONE]\n\n")
	return []byte(b.String())
}

// buildOpenAITextSSE constructs a complete OpenAI SSE response body that
// streams text in multiple chunks and ends with finish_reason=stop.
func buildOpenAITextSSE(chunks []string) []byte {
	var b strings.Builder
	for _, chunk := range chunks {
		openAISSEChunk(&b, fmt.Sprintf(
			`{"choices":[{"index":0,"delta":{"content":%q},"finish_reason":null}]}`,
			chunk))
	}
	openAISSEChunk(&b, `{"choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`)
	openAISSEChunk(&b, `{"choices":[],"usage":{"prompt_tokens":2000,"completion_tokens":150,"total_tokens":2150}}`)
	b.WriteString("data: [DONE]\n\n")
	return []byte(b.String())
}

// benchOpenAISSEBodies pre-builds the four SSE response bodies for the
// scenario defined in benchFixtures so no construction cost appears inside the
// timing loop.
func benchOpenAISSEBodies() [][]byte {
	turn1 := buildOpenAIToolCallSSE("", []struct{ id, name, args string }{
		{"call_c1", "read_file", `{"path":"agent/agent.go"}`},
		{"call_c2", "read_file", `{"path":"agent/tool.go"}`},
	})

	turn2 := buildOpenAIToolCallSSE("", []struct{ id, name, args string }{
		{"call_c3", "search_content", `{"query":"Agent.run","path":"agent"}`},
	})

	const mediumOutput = "I have read the files and analysed the code. " +
		"The implementation looks correct but there are a few areas that could be improved. " +
		"Let me make the necessary edits now."

	turn3 := buildOpenAIToolCallSSE(mediumOutput, []struct{ id, name, args string }{
		{"call_c4", "apply_patch", `{"path":"agent/agent.go","patch":"--- a/agent/agent.go\n+++ b/agent/agent.go\n@@ -1 +1 @@\n-old line\n+new line"}`},
		{"call_c5", "read_file", `{"path":"llm/service.go"}`},
	})

	// Same chunks as benchFixtures and benchAnthropicSSEBodies.
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
		"messages := make([]llm.Message, 0,\n",
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

	turn4 := buildOpenAITextSSE(longOutputChunks)
	return [][]byte{turn1, turn2, turn3, turn4}
}

// BenchmarkAgentRunOpenAI measures the combined cost of the real OpenAI client
// (JSON request serialisation, ChatCompletionAccumulator, SSE parsing) and the
// agent loop. The HTTP transport is replaced with an in-process mock so no TCP
// or TLS overhead appears in the profile.
//
// Compare against BenchmarkAgentRunAnthropic and BenchmarkAgentRunDirect to
// isolate how much each client layer contributes to total allocation cost.
func BenchmarkAgentRunOpenAI(b *testing.B) {
	sseBodies := benchOpenAISSEBodies()
	_, tools := benchFixtures()
	skillRegistry := noSkills()
	registry := NewRegistry(tools...)

	// Hoist client construction outside the timing loop. tiktoken loads a BPE
	// table from the offline loader on first use — that's one-time startup cost,
	// not per-request cost, and should not appear in the profile.
	// GPT4Turbo uses the cl100k_base encoding (available in the offline loader)
	// and has a 128 k context window so the safeMax check never trips on our
	// 4-turn scenario.
	transport := &openAIMockTransport{bodies: sseBodies}
	svc := openai.NewClientWithHTTP(
		"test-key",
		openai.Config{Model: openai.GPT4Turbo},
		openai.AvailableModels(),
		&http.Client{Transport: transport},
	)

	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	b.ResetTimer()
	b.ReportAllocs()

	for i := range b.N {
		// Reset transport so each iteration replays the same 4-turn sequence.
		transport.idx = 0

		store := newMockStore()
		ag := NewAgent(svc, registry, skillRegistry, store, NoMemory(),
			Config{SystemPrompt: "You are a helpful coding assistant."})

		ctx, cancel := context.WithCancel(context.Background())

		ch := make(chan Event, 256)
		ag.run(ctx, ch,
			fmt.Sprintf("bench-%d", i),
			"Analyse and improve the agent package.",
			runOptions{})
		cancel()

		for len(ch) > 0 {
			<-ch
		}
	}
}
