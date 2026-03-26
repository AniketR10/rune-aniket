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

	"unstable.build/go-tui/cmd/rune-agent/llm/anthropic"
)

// anthropicMockTransport is an http.RoundTripper that returns pre-built SSE
// response bodies in sequence.
//
// Completion requests (POST /v1/messages) cycle through bodies in order.
// CountTokens requests (POST /v1/messages/count_tokens) always return a
// canned JSON payload so the client does not log a WARN or fall through to
// the character-based estimator.
//
// Not goroutine-safe; allocate one instance per benchmark iteration.
type anthropicMockTransport struct {
	bodies [][]byte
	idx    int
}

func (t *anthropicMockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, "/count_tokens") {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(`{"input_tokens":2000}`)),
		}, nil
	}
	body := t.bodies[t.idx%len(t.bodies)]
	t.idx++
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": {"text/event-stream"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}, nil
}

// sseEvent appends a single SSE event (event + data lines) to b.
func sseEvent(b *strings.Builder, eventType, data string) {
	b.WriteString("event: ")
	b.WriteString(eventType)
	b.WriteString("\ndata: ")
	b.WriteString(data)
	b.WriteString("\n\n")
}

// sseMessageStart returns the JSON payload for a message_start event. Input
// token count matches reportedUsage in benchFixtures so the agent loop uses
// it as lastAPITokensSent on subsequent iterations, skipping CountTokens.
func sseMessageStart(id string) string {
	return fmt.Sprintf(
		`{"type":"message_start","message":{"id":%q,"type":"message","role":"assistant","content":[],"model":"claude-test","stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":2000,"output_tokens":1,"cache_creation_input_tokens":0,"cache_read_input_tokens":0}}}`,
		id,
	)
}

// buildToolCallSSE constructs a complete SSE response body that emits an
// optional text prefix block, then one content_block per tool call, and
// finishes with stop_reason=tool_use.
func buildToolCallSSE(
	msgID, textPrefix string,
	calls []struct{ id, name, args string },
) []byte {
	var b strings.Builder
	sseEvent(&b, "message_start", sseMessageStart(msgID))
	idx := 0

	if textPrefix != "" {
		sseEvent(&b, "content_block_start",
			fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"text","text":""}}`, idx))
		sseEvent(&b, "content_block_delta",
			fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"text_delta","text":%q}}`, idx, textPrefix))
		sseEvent(&b, "content_block_stop",
			fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, idx))
		idx++
	}

	for _, c := range calls {
		sseEvent(&b, "content_block_start",
			fmt.Sprintf(`{"type":"content_block_start","index":%d,"content_block":{"type":"tool_use","id":%q,"name":%q,"input":{}}}`,
				idx, c.id, c.name))
		sseEvent(&b, "content_block_delta",
			fmt.Sprintf(`{"type":"content_block_delta","index":%d,"delta":{"type":"input_json_delta","partial_json":%q}}`,
				idx, c.args))
		sseEvent(&b, "content_block_stop",
			fmt.Sprintf(`{"type":"content_block_stop","index":%d}`, idx))
		idx++
	}

	sseEvent(&b, "message_delta",
		`{"type":"message_delta","delta":{"stop_reason":"tool_use","stop_sequence":null},"usage":{"output_tokens":25}}`)
	sseEvent(&b, "message_stop", `{"type":"message_stop"}`)
	return []byte(b.String())
}

// buildTextSSE constructs a complete SSE response body that streams text in
// multiple chunks and finishes with stop_reason=end_turn.
func buildTextSSE(msgID string, chunks []string) []byte {
	var b strings.Builder
	sseEvent(&b, "message_start", sseMessageStart(msgID))
	sseEvent(&b, "content_block_start",
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`)
	for _, chunk := range chunks {
		sseEvent(&b, "content_block_delta",
			fmt.Sprintf(`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":%q}}`, chunk))
	}
	sseEvent(&b, "content_block_stop", `{"type":"content_block_stop","index":0}`)
	sseEvent(&b, "message_delta",
		`{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":150}}`)
	sseEvent(&b, "message_stop", `{"type":"message_stop"}`)
	return []byte(b.String())
}

// benchAnthropicSSEBodies pre-builds the four SSE response bodies for the
// scenario defined in benchFixtures, so no construction cost appears inside
// the timing loop.
//
//  1. Two parallel reads (read_file × 2).
//  2. One search (search_content).
//  3. Medium-text block + two tool calls (apply_patch, read_file).
//  4. Long multi-chunk final answer (end_turn).
func benchAnthropicSSEBodies() [][]byte {
	turn1 := buildToolCallSSE("msg_1", "", []struct{ id, name, args string }{
		{"toolu_c1", "read_file", `{"path":"agent/agent.go"}`},
		{"toolu_c2", "read_file", `{"path":"agent/tool.go"}`},
	})

	turn2 := buildToolCallSSE("msg_2", "", []struct{ id, name, args string }{
		{"toolu_c3", "search_content", `{"query":"Agent.run","path":"agent"}`},
	})

	const mediumOutput = "I have read the files and analysed the code. " +
		"The implementation looks correct but there are a few areas that could be improved. " +
		"Let me make the necessary edits now."

	turn3 := buildToolCallSSE("msg_3", mediumOutput, []struct{ id, name, args string }{
		{"toolu_c4", "apply_patch", `{"path":"agent/agent.go","patch":"--- a/agent/agent.go\n+++ b/agent/agent.go\n@@ -1 +1 @@\n-old line\n+new line"}`},
		{"toolu_c5", "read_file", `{"path":"llm/service.go"}`},
	})

	// Identical chunks to benchFixtures longOutputChunks so the streaming
	// volume — and therefore the strings.Builder accumulation cost — matches
	// BenchmarkAgentRunDirect exactly.
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

	turn4 := buildTextSSE("msg_4", longOutputChunks)
	return [][]byte{turn1, turn2, turn3, turn4}
}

// BenchmarkAgentRunAnthropic measures the combined cost of the real Anthropic
// client (JSON request serialisation, cache-breakpoint injection, SSE parsing)
// and the agent loop. The HTTP transport is replaced with an in-process mock so
// no TCP or TLS overhead appears in the profile.
//
// Compare against BenchmarkAgentRunDirect (mockService) to isolate the
// allocation contribution of the Anthropic client layer.
func BenchmarkAgentRunAnthropic(b *testing.B) {
	sseBodies := benchAnthropicSSEBodies()
	_, tools := benchFixtures()
	skillRegistry := noSkills()
	registry := NewRegistry(tools...)

	// Hoist client construction outside the timing loop. ant.NewClient wires
	// HTTP options and middleware — one-time setup that should not appear in
	// the per-request profile.
	transport := &anthropicMockTransport{bodies: sseBodies}
	svc := anthropic.NewClientWithHTTP(
		"test-key",
		anthropic.Config{Model: "claude-sonnet-4-5", MaxTokens: 8192},
		nil,
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

		// Buffer sized above the maximum number of events emitted by our
		// 4-turn scenario so run never blocks on a channel send.
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
