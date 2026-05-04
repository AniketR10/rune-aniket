// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2018-2024 Unstable Build, All Rights Reserved.
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

package openai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/rune-agent/llm"
)

// NOTE: this serves as an example of how to use the llm API backed by openai.
// Only enable when changes to the API or client are made
// or when upgrading the sdk dependency.
// Note that some of the content asserts might fail, as without
// `seed` and `system_fingerprint` funcionality, the output is never deterministic.
func TestCreateCompletion(t *testing.T) {
	t.SkipNow()

	token := os.Getenv("OPENAI_TESTING_KEY")

	t.Run("NewClient with empty model panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewClient(token, Config{}, AvailableModels())
		})
	})

	t.Run("sends a chat completion request, with no context", func(t *testing.T) {
		c := NewClient(token, Config{
			Model:       GPT3Dot5Turbo,
			Temperature: 0.1,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello sir!"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var builder strings.Builder
		var doneData *llm.DoneData
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventTextDelta:
				builder.WriteString(ev.Text)
			case llm.EventStreamDone:
				doneData = ev.DoneData
			}
		}

		require.NoError(t, it.Err())
		require.NotNil(t, doneData)
		assert.Contains(t, builder.String(), "How can I")
		assert.Equal(t, llm.FinishReasonStop, doneData.FinishReason)
	})

	t.Run("sets MaxTokens from configuration", func(t *testing.T) {
		body := captureRequestBody(t, Config{
			MaxTokens: 10000,
			Model:     GPT3Dot5Turbo,
		}, llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello sir!"},
		}})
		assert.Equal(t, float64(10000), body["max_tokens"])
	})

	t.Run("sets PresencePenalty from configuration", func(t *testing.T) {
		body := captureRequestBody(t, Config{
			PresencePenalty: 1.5,
			Model:           GPT3Dot5Turbo,
		}, llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello sir!"},
		}})
		assert.Equal(t, 1.5, body["presence_penalty"])
	})

	t.Run("sets Temperature from configuration", func(t *testing.T) {
		body := captureRequestBody(t, Config{
			Temperature: 0.7,
			Model:       GPT3Dot5Turbo,
		}, llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello sir!"},
		}})
		assert.Equal(t, 0.7, body["temperature"])
	})

	t.Run("sends a chat completion request, with some context, with default configuration", func(t *testing.T) {
		c := NewClient(token, Config{
			Model:       GPT3Dot5Turbo,
			Temperature: 0.1,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: "system", Content: "We are roleplaying and you are an evil AI agent."},
			{Role: llm.RoleUser, Content: "what is your purpouse?"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var builder strings.Builder
		var doneData *llm.DoneData
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventTextDelta:
				builder.WriteString(ev.Text)
			case llm.EventStreamDone:
				doneData = ev.DoneData
			}
		}

		require.NoError(t, it.Err())
		require.NotNil(t, doneData)
		assert.Contains(t, builder.String(), "chaos")
		assert.Equal(t, llm.FinishReasonStop, doneData.FinishReason)
	})

	t.Run("sends a chat completion request with an input image", func(t *testing.T) {
		c := NewClient(token, Config{
			Model:       GPT4Dot1Nano,
			Temperature: 0.1,
		}, AvailableModels())
		ctx := context.Background()
		imgPart, err := llm.NewContentPartFromImage(loadTestImage(t))
		require.NoError(t, err)
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, MultiContent: []llm.ContentPart{
				{Type: llm.ContentPartTypeText, Text: "what's in this image?"},
				imgPart,
			}},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var builder strings.Builder
		var doneData *llm.DoneData
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventTextDelta:
				builder.WriteString(ev.Text)
			case llm.EventStreamDone:
				doneData = ev.DoneData
			}
		}

		require.NoError(t, it.Err())
		require.NotNil(t, doneData)
		assert.Contains(t, builder.String(), "turquoise")
		assert.Equal(t, llm.FinishReasonStop, doneData.FinishReason)
	})

	t.Run("uses tools provided", func(t *testing.T) {
		c := NewClient(token, Config{
			Tools: []llm.Tool{
				{Type: llm.ToolTypeFunction, Function: llm.FunctionDefinition{
					Name:        "getCurrentWeather",
					Description: "Get the weather in location",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{
								"type":        "string",
								"description": "The city and state, e.g. San Francisco, CA",
							},
							"unit": map[string]any{
								"type": "string",
								"enum": []string{"celcius", "fahrenheit"},
							},
						},
						"required": []string{"location"},
					},
				}},
			},
			Model: GPT4,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "what's the weather like in San Francisco right now?"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var toolCalls []llm.ToolCall
		var doneData *llm.DoneData
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventToolCallDone:
				toolCalls = append(toolCalls, *ev.ToolCall)
			case llm.EventStreamDone:
				doneData = ev.DoneData
			}
		}

		require.NoError(t, it.Err())
		require.NotNil(t, doneData)
		assert.Equal(t, llm.FinishReasonToolCall, doneData.FinishReason)

		require.Len(t, toolCalls, 1)
		assert.NotZero(t, toolCalls[0].ID)
		assert.Equal(t, llm.ToolTypeFunction, toolCalls[0].Type)
		assert.Equal(t, "getCurrentWeather", toolCalls[0].Function.Name)
		assert.Equal(t, `{
  "location": "San Francisco, CA"
}`, toolCalls[0].Function.Arguments)
	})

	t.Run("uses JSON schema provided", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"is":{"type":"boolean"}},"required":["is"],"additionalProperties":false}`)

		c := NewClient(token, Config{
			ResponseFormat: &llm.ResponseFormat{
				Type: llm.ResponseFormatTypeJSONSchema,
				JSONSchema: &llm.ResponseFormatJSONSchema{
					Name:        "true-or-false",
					Description: "A single object returning true or false",
					Schema:      schema,
					Strict:      true,
				},
			},
			Model: GPT4Dot1Nano,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "Is 2 greater than 1?"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var builder strings.Builder
		var doneData *llm.DoneData
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventTextDelta:
				builder.WriteString(ev.Text)
			case llm.EventStreamDone:
				doneData = ev.DoneData
			}
		}

		require.NoError(t, it.Err())
		require.NotNil(t, doneData)
		assert.Contains(t, builder.String(), `{"is":true}`)
		assert.Equal(t, llm.FinishReasonStop, doneData.FinishReason)
	})
}

// TestResponsesAPI exercises the /v1/responses streaming path used by
// responses-only models (e.g. gpt-5.3-codex). Enable by removing
// t.SkipNow() and setting OPENAI_TESTING_KEY.
func TestResponsesAPI(t *testing.T) {
	t.SkipNow()

	token := os.Getenv("OPENAI_TESTING_KEY")
	model := GPT5Dot3Codex

	require.True(t, IsResponsesOnlyModel(model), "test model must be responses-only")

	t.Run("streams a simple text response", func(t *testing.T) {
		c := NewClient(token, Config{Model: model}, AvailableModels())
		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You are a helpful assistant. Be very brief."},
			{Role: llm.RoleUser, Content: "What is 2+2? Reply with just the number."},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)
		defer func() { _ = it.Close() }()

		var text strings.Builder
		var doneData *llm.DoneData
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventTextDelta:
				text.WriteString(ev.Text)
			case llm.EventStreamDone:
				doneData = ev.DoneData
			case llm.EventStreamError:
				t.Fatalf("stream error: %v", ev.Error)
			}
		}
		require.NoError(t, it.Err())
		require.NotNil(t, doneData)

		assert.Contains(t, text.String(), "4")
		assert.Equal(t, llm.FinishReasonStop, doneData.FinishReason)
		assert.Equal(t, text.String(), doneData.Message.Content)
		assert.Equal(t, llm.RoleAssistant, doneData.Message.Role)

		// Usage should be populated.
		assert.Greater(t, doneData.Usage.TokensSent, 0)
		assert.Greater(t, doneData.Usage.TokensReceived, 0)
	})

	t.Run("invokes a function tool", func(t *testing.T) {
		tools := []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.FunctionDefinition{
				Name:        "get_weather",
				Description: "Get the current weather for a location",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type":        "string",
							"description": "City name",
						},
					},
					"required":             []string{"location"},
					"additionalProperties": false,
				},
			}},
		}
		c := NewClient(token, Config{
			Model: model,
			Tools: tools,
		}, AvailableModels())
		ctx := context.Background()
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "What's the weather in Tokyo?"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)
		defer func() { _ = it.Close() }()

		var toolCalls []llm.ToolCall
		var doneData *llm.DoneData
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventToolCallDone:
				toolCalls = append(toolCalls, *ev.ToolCall)
			case llm.EventStreamDone:
				doneData = ev.DoneData
			case llm.EventStreamError:
				t.Fatalf("stream error: %v", ev.Error)
			}
		}
		require.NoError(t, it.Err())
		require.NotNil(t, doneData)

		assert.Equal(t, llm.FinishReasonToolCall, doneData.FinishReason)
		require.Len(t, toolCalls, 1)
		assert.Equal(t, "get_weather", toolCalls[0].Function.Name)
		assert.NotEmpty(t, toolCalls[0].ID)
		assert.Contains(t, toolCalls[0].Function.Arguments, "Tokyo")

		// DoneData message should also contain the tool calls.
		require.Len(t, doneData.Message.ToolCalls, 1)
		assert.Equal(t, "get_weather", doneData.Message.ToolCalls[0].Function.Name)
	})

	t.Run("multi-turn with tool result", func(t *testing.T) {
		tools := []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.FunctionDefinition{
				Name:        "get_weather",
				Description: "Get the current weather for a location",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"location": map[string]any{
							"type":        "string",
							"description": "City name",
						},
					},
					"required":             []string{"location"},
					"additionalProperties": false,
				},
			}},
		}
		c := NewClient(token, Config{
			Model: model,
			Tools: tools,
		}, AvailableModels())
		ctx := context.Background()

		// Turn 1: user asks → model calls tool.
		req := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "What's the weather in Paris?"},
		}}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)

		var turn1Done *llm.DoneData
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventStreamDone:
				turn1Done = ev.DoneData
			case llm.EventStreamError:
				t.Fatalf("turn 1 error: %v", ev.Error)
			}
		}
		require.NoError(t, it.Err())
		_ = it.Close()
		require.NotNil(t, turn1Done)
		require.NotEmpty(t, turn1Done.Message.ToolCalls, "model should call get_weather")

		tc := turn1Done.Message.ToolCalls[0]

		// Turn 2: feed tool result → model responds with text.
		req2 := llm.Request{Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "What's the weather in Paris?"},
			turn1Done.Message,
			{Role: llm.RoleTool, ToolCallID: tc.ID, Content: `{"temperature": "18C", "condition": "sunny"}`},
		}}
		it2, err := c.CreateCompletion(ctx, req2)
		require.NoError(t, err)
		defer func() { _ = it2.Close() }()

		var text strings.Builder
		var turn2Done *llm.DoneData
		for {
			ev, ok := it2.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventTextDelta:
				text.WriteString(ev.Text)
			case llm.EventStreamDone:
				turn2Done = ev.DoneData
			case llm.EventStreamError:
				t.Fatalf("turn 2 error: %v", ev.Error)
			}
		}
		require.NoError(t, it2.Err())
		require.NotNil(t, turn2Done)

		assert.Equal(t, llm.FinishReasonStop, turn2Done.FinishReason)
		// The model should mention the weather data we provided.
		response := strings.ToLower(text.String())
		assert.True(t, strings.Contains(response, "18") || strings.Contains(response, "sunny") || strings.Contains(response, "paris"),
			"expected response to reference the weather data, got: %s", text.String())
	})

	t.Run("per-request tools override client tools", func(t *testing.T) {
		// Client has get_weather as a client-level tool, but we pass
		// calculate per-request — the model should only see calculate.
		clientTools := []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.FunctionDefinition{
				Name:        "get_weather",
				Description: "Get current weather for a city",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"city": map[string]any{
							"type":        "string",
							"description": "The city name",
						},
					},
					"required":             []string{"city"},
					"additionalProperties": false,
				},
			}},
		}
		c := NewClient(token, Config{Model: model, Tools: clientTools}, AvailableModels())
		ctx := context.Background()

		perRequestTools := []llm.Tool{
			{Type: llm.ToolTypeFunction, Function: llm.FunctionDefinition{
				Name:        "calculate",
				Description: "Evaluate a math expression. You MUST use this tool for any math.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"expression": map[string]any{
							"type":        "string",
							"description": "The math expression to evaluate",
						},
					},
					"required":             []string{"expression"},
					"additionalProperties": false,
				},
			}},
		}
		req := llm.Request{
			Messages: []llm.Message{
				{Role: llm.RoleSystem, Content: "Always use the calculate tool for math. Never compute math yourself."},
				{Role: llm.RoleUser, Content: "What is 123 * 456? Use the calculate tool."},
			},
			Tools: perRequestTools,
		}
		it, err := c.CreateCompletion(ctx, req)
		require.NoError(t, err)
		defer func() { _ = it.Close() }()

		var toolCalls []llm.ToolCall
		for {
			ev, ok := it.Next(ctx)
			if !ok {
				break
			}
			switch ev.Type {
			case llm.EventToolCallDone:
				toolCalls = append(toolCalls, *ev.ToolCall)
			case llm.EventStreamError:
				t.Fatalf("stream error: %v", ev.Error)
			}
		}
		require.NoError(t, it.Err())
		require.Len(t, toolCalls, 1)
		assert.Equal(t, "calculate", toolCalls[0].Function.Name)
		// Must NOT call get_weather — that's the client-level tool that should be overridden.
		for _, tc := range toolCalls {
			assert.NotEqual(t, "get_weather", tc.Function.Name, "per-request tools should override client tools")
		}
	})

	t.Run("context window exceeded returns ErrContextWindowExceeded", func(t *testing.T) {
		models := AvailableModels()
		models[model] = 50 // artificially small

		c := NewClient(token, Config{Model: model}, models)
		ctx := context.Background()
		msgs := makeMessageTokens(c.(client), 51)
		_, err := c.CreateCompletion(ctx, llm.Request{Messages: msgs})
		require.True(t, errors.Is(err, &llm.ErrContextWindowExceeded{}))
	})
}

func TestContextWindows(t *testing.T) {
	models := AvailableModels()
	models[GPT4] = 50

	t.Run("CreateCompletion errors with ErrContextWindowExceeded", func(t *testing.T) {
		c := NewClient("", Config{
			Model: GPT4,
		}, models).(client)
		ctx := context.Background()
		msgs := makeMessageTokens(c, 51)

		// sut
		req := llm.Request{Messages: msgs}
		_, err := c.CreateCompletion(ctx, req)
		require.True(t, errors.Is(err, &llm.ErrContextWindowExceeded{}), err.Error())
	})
}

func TestCountTokens(t *testing.T) {
	for model := range AvailableModels() {
		t.Run(model, func(t *testing.T) {
			c := NewClient("", Config{
				Model: model,
			}, AvailableModels()).(client)
			text := "!Hola mundo!"
			count, _ := c.CountTokens([]llm.Message{{Content: text}})
			assert.Equal(t, 10, count)
		})
	}

	t.Run("counts_tool_calls_in_messages", func(t *testing.T) {
		c := NewClient("", Config{
			Model: GPT4o,
		}, AvailableModels()).(client)

		// Baseline: message with only text content.
		baseCount, _ := c.CountTokens([]llm.Message{{
			Role:    llm.RoleAssistant,
			Content: "hello",
		}})

		// Same message with a tool call — should count more tokens.
		withTC, _ := c.CountTokens([]llm.Message{{
			Role:    llm.RoleAssistant,
			Content: "hello",
			ToolCalls: []llm.ToolCall{{
				ID:   "call_abc123",
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "read_file",
					Arguments: `{"path":"/foo/bar.go"}`,
				},
			}},
		}})
		assert.Greater(t, withTC, baseCount, "tool calls should add tokens")
	})

	t.Run("counts_tool_call_id_in_tool_messages", func(t *testing.T) {
		c := NewClient("", Config{
			Model: GPT4o,
		}, AvailableModels()).(client)

		// Baseline: tool message with empty ToolCallID.
		baseCount, _ := c.CountTokens([]llm.Message{{
			Role:    llm.RoleTool,
			Content: "file contents here",
		}})

		// Same message with a ToolCallID.
		withID, _ := c.CountTokens([]llm.Message{{
			Role:       llm.RoleTool,
			Content:    "file contents here",
			ToolCallID: "call_abc123",
		}})
		assert.Greater(t, withID, baseCount, "ToolCallID should add tokens")
	})
}

func makeMessageTokens(c client, greaterThan int) []llm.Message {
	var msgs []llm.Message
	for i := 0; ; i++ {
		count, _ := c.CountTokens(msgs)
		if count >= greaterThan {
			break
		}
		msgs = append(msgs, llm.Message{
			Content: strconv.Itoa(i), Role: llm.RoleUser,
		})
	}
	return msgs
}

func loadTestImage(t *testing.T) image.Image {
	f, err := os.Open("./testdata/chatgpt_image.png")
	require.NoError(t, err)
	img, err := png.Decode(f)
	require.NoError(t, err)
	return img
}

func TestResponsesStreamReasoningTextDelta(t *testing.T) {
	// Build a mock Responses API SSE stream that includes
	// response.reasoning_text.delta events.
	sseEvents := []string{
		`{"type":"response.reasoning_text.delta","delta":"Let me ","content_index":0,"item_id":"item_0","output_index":0,"sequence_number":1}`,
		`{"type":"response.reasoning_text.delta","delta":"think...","content_index":0,"item_id":"item_0","output_index":0,"sequence_number":2}`,
		`{"type":"response.output_text.delta","delta":"The answer is 4.","content_index":0,"item_id":"item_1","output_index":1,"sequence_number":3}`,
		`{"type":"response.completed","response":{"id":"resp_1","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":5}}}`,
	}
	srv := responsesSSEServer(t, sseEvents)

	c := NewClient("test-key", Config{
		Model:   GPT5Dot3Codex,
		BaseURL: srv.URL,
	}, AvailableModels())

	ctx := context.Background()
	it, err := c.CreateCompletion(ctx, llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "What is 2+2?"},
	}})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	var reasoning, text strings.Builder
	var doneData *llm.DoneData
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		switch ev.Type {
		case llm.EventReasoningDelta:
			reasoning.WriteString(ev.Reasoning)
		case llm.EventTextDelta:
			text.WriteString(ev.Text)
		case llm.EventStreamDone:
			doneData = ev.DoneData
		case llm.EventStreamError:
			t.Fatalf("stream error: %v", ev.Error)
		}
	}
	require.NoError(t, it.Err())
	require.NotNil(t, doneData)

	assert.Equal(t, "Let me think...", reasoning.String())
	assert.Equal(t, "The answer is 4.", text.String())
	assert.Equal(t, "Let me think...", doneData.Message.ReasoningContent)
}

func TestResponsesStreamReasoningSummaryDelta(t *testing.T) {
	// Verify that response.reasoning_summary_text.delta events
	// are also emitted as EventReasoningDelta.
	sseEvents := []string{
		`{"type":"response.reasoning_summary_text.delta","delta":"Summary: ","summary_index":0,"item_id":"item_0","output_index":0,"sequence_number":1}`,
		`{"type":"response.reasoning_summary_text.delta","delta":"simple math","summary_index":0,"item_id":"item_0","output_index":0,"sequence_number":2}`,
		`{"type":"response.output_text.delta","delta":"4","content_index":0,"item_id":"item_1","output_index":1,"sequence_number":3}`,
		`{"type":"response.completed","response":{"id":"resp_2","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":5}}}`,
	}
	srv := responsesSSEServer(t, sseEvents)

	c := NewClient("test-key", Config{
		Model:   GPT5Dot3Codex,
		BaseURL: srv.URL,
	}, AvailableModels())

	ctx := context.Background()
	it, err := c.CreateCompletion(ctx, llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "What is 2+2?"},
	}})
	require.NoError(t, err)
	defer func() { _ = it.Close() }()

	var reasoning strings.Builder
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if ev.Type == llm.EventReasoningDelta {
			reasoning.WriteString(ev.Reasoning)
		}
		if ev.Type == llm.EventStreamError {
			t.Fatalf("stream error: %v", ev.Error)
		}
	}
	require.NoError(t, it.Err())
	assert.Equal(t, "Summary: simple math", reasoning.String())
}

func TestResponsesReasoningSummaryConfig(t *testing.T) {
	body := captureResponsesRequestBody(t, Config{
		Model:            GPT5Dot3Codex,
		ReasoningSummary: "concise",
	}, llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "hello"},
	}})

	reasoning, ok := body["reasoning"].(map[string]any)
	require.True(t, ok, "request should have a 'reasoning' field")
	assert.Equal(t, "concise", reasoning["summary"])
}

func TestResponsesReasoningSummaryDefaultAuto(t *testing.T) {
	// When no ReasoningSummary is configured and the model supports
	// reasoning, "auto" should be sent as the default.
	body := captureResponsesRequestBody(t, Config{
		Model:             GPT5Dot3Codex,
		ForceResponsesAPI: true,
	}, llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "hello"},
	}})

	reasoning, ok := body["reasoning"].(map[string]any)
	require.True(t, ok, "request should have a 'reasoning' field")
	assert.Equal(t, "auto", reasoning["summary"])
}

func TestResponsesReasoningSummaryFromRequest(t *testing.T) {
	// Per-request ReasoningSummary should take precedence over config.
	body := captureResponsesRequestBody(t, Config{
		Model:            GPT5Dot3Codex,
		ReasoningSummary: "concise",
	}, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "hello"},
		},
		ReasoningSummary: llm.ReasoningSummaryDetailed,
	})

	reasoning, ok := body["reasoning"].(map[string]any)
	require.True(t, ok, "request should have a 'reasoning' field")
	assert.Equal(t, "detailed", reasoning["summary"])
}

// TestResponsesIncludesEncryptedReasoningContent verifies that, for any
// reasoning model on the Responses API, the client sets
// include=[reasoning.encrypted_content] so reasoning items can be threaded
// back across turns. This is required for stateless (store=false) callers and
// harmless otherwise — see https://platform.openai.com/docs/guides/reasoning.
func TestResponsesIncludesEncryptedReasoningContent(t *testing.T) {
	body := captureResponsesRequestBody(t, Config{
		Model:             GPT5Dot3Codex,
		ForceResponsesAPI: true,
	}, llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	require.Equal(t, []any{"reasoning.encrypted_content"}, body["include"])
}

// TestResponsesSessionHeadersFromPromptCacheKey verifies that any caller
// supplying a PromptCacheKey gets correlation headers usable by the Codex
// backend (and ignored by OpenAI hosted).
func TestResponsesSessionHeadersFromPromptCacheKey(t *testing.T) {
	captured := captureResponsesRequest(t, Config{
		Model:             GPT5Dot3Codex,
		ForceResponsesAPI: true,
	}, llm.Request{
		Messages:       []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
		PromptCacheKey: "thread-abc",
	})
	assert.Equal(t, "thread-abc", captured.Header.Get("session_id"))
	assert.Equal(t, "thread-abc", captured.Header.Get("x-client-request-id"))
}

// TestResponsesStorefalseAndDisabledParallel verifies the generic stateless
// knobs, used by the Codex caller (and any other ZDR / store=false consumer).
func TestResponsesStorefalseAndDisabledParallel(t *testing.T) {
	storeFalse := false
	body := captureResponsesRequestBody(t, Config{
		Model:                    GPT5Dot3Codex,
		ForceResponsesAPI:        true,
		Store:                    &storeFalse,
		DisableParallelToolCalls: true,
		ClientMetadata:           map[string]string{"x-codex-installation-id": "install-1"},
	}, llm.Request{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "hi"}},
	})
	assert.Equal(t, false, body["store"])
	assert.Equal(t, false, body["parallel_tool_calls"])
	metadata, ok := body["client_metadata"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "install-1", metadata["x-codex-installation-id"])
}

// TestResponsesReasoningRoundTrip verifies that reasoning items emitted by
// the Responses API stream are captured into Message.ProviderItems with their
// encrypted_content intact, and that they are threaded back verbatim as input
// items on the next request. Per OpenAI's reasoning guide, this is required
// for stateless callers (store=false / ZDR) and recommended for any caller
// that does function calling with a reasoning model and is not relying on
// previous_response_id.
func TestResponsesReasoningRoundTrip(t *testing.T) {
	reasoningItem := `{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"plan"}],"encrypted_content":"ENC_BLOB","status":"completed"}`
	functionCallItem := `{"type":"function_call","id":"fc_item_1","call_id":"call_1","name":"skill","arguments":"{\"name\":\"explore\",\"args\":\"\"}","status":"completed"}`
	sseEvents := []string{
		`{"type":"response.output_item.done","output_index":0,"sequence_number":1,"item":` + reasoningItem + `}`,
		`{"type":"response.output_item.done","output_index":1,"sequence_number":2,"item":` + functionCallItem + `}`,
		`{"type":"response.completed","sequence_number":3,"response":{"id":"resp_1","status":"completed","output":[],"usage":{"input_tokens":10,"output_tokens":5}}}`,
	}
	srv := responsesSSEServer(t, sseEvents)

	c := NewClient("test-key", Config{
		Model:             GPT5Dot3Codex,
		BaseURL:           srv.URL,
		ForceResponsesAPI: true,
	}, AvailableModels())

	ctx := context.Background()
	it, err := c.CreateCompletion(ctx, llm.Request{Messages: []llm.Message{
		{Role: llm.RoleUser, Content: "explore"},
	}})
	require.NoError(t, err)

	var doneData *llm.DoneData
	for {
		ev, ok := it.Next(ctx)
		if !ok {
			break
		}
		if ev.Type == llm.EventStreamDone {
			doneData = ev.DoneData
		}
	}
	require.NoError(t, it.Err())
	require.NoError(t, it.Close())
	require.NotNil(t, doneData)

	// The assistant message must carry both opaque output items in stream
	// order so they can be replayed.
	require.Len(t, doneData.Message.ProviderItems, 2)
	assert.JSONEq(t, reasoningItem, string(doneData.Message.ProviderItems[0]))
	assert.JSONEq(t, functionCallItem, string(doneData.Message.ProviderItems[1]))
	require.Len(t, doneData.Message.ToolCalls, 1)
	assert.Equal(t, "call_1", doneData.Message.ToolCalls[0].ID)

	// Now feed that assistant message + a tool result back into a new
	// request and verify the input array faithfully reproduces the
	// reasoning + function_call + function_call_output sequence.
	captured := captureResponsesRequest(t, Config{
		Model:             GPT5Dot3Codex,
		ForceResponsesAPI: true,
	}, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "explore"},
			doneData.Message,
			{Role: llm.RoleTool, ToolCallID: "call_1", Content: "ok"},
		},
	})

	input, ok := captured.Body["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 4)

	// 0: original user message.
	user, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", user["role"])

	// 1: reasoning item with encrypted_content preserved.
	reasoning, ok := input[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "reasoning", reasoning["type"])
	assert.Equal(t, "rs_1", reasoning["id"])
	assert.Equal(t, "ENC_BLOB", reasoning["encrypted_content"])

	// 2: function_call item with original call_id.
	fnCall, ok := input[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function_call", fnCall["type"])
	assert.Equal(t, "call_1", fnCall["call_id"])
	assert.Equal(t, "skill", fnCall["name"])

	// 3: function_call_output we produced from the tool message.
	fnOut, ok := input[3].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function_call_output", fnOut["type"])
	assert.Equal(t, "call_1", fnOut["call_id"])
	assert.Equal(t, "ok", fnOut["output"])
}

// TestResponsesAutoDiagProviderItemsRoundTrip locks in the contract that
// when auto-diagnostics injects a synthetic `function_call` ProviderItem
// alongside a synthetic ToolCall, the Responses converter emits both the
// original and the synthetic function_call items in order, followed by
// both function_call_output items. Without the synthetic ProviderItem,
// the Responses/Codex backend rejects the request with
// 400 "No tool call found for function call output ...".
func TestResponsesAutoDiagProviderItemsRoundTrip(t *testing.T) {
	originalCallID := "call_orig"
	syntheticCallID := "auto-diag-" + originalCallID
	originalFnCall := `{"type":"function_call","call_id":"` + originalCallID +
		`","name":"apply_patch","arguments":"{\"patch\":\"p\"}"}`
	syntheticFnCall := `{"type":"function_call","call_id":"` + syntheticCallID +
		`","name":"check_file_errors","arguments":"{\"path\":\"/workspace/main.go\"}"}`

	assistantMsg := llm.Message{
		Role: llm.RoleAssistant,
		ProviderItems: []json.RawMessage{
			json.RawMessage(originalFnCall),
			json.RawMessage(syntheticFnCall),
		},
		ToolCalls: []llm.ToolCall{
			{
				ID:   originalCallID,
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "apply_patch",
					Arguments: `{"patch":"p"}`,
				},
			},
			{
				ID:   syntheticCallID,
				Type: llm.ToolTypeFunction,
				Function: llm.FunctionCall{
					Name:      "check_file_errors",
					Arguments: `{"path":"/workspace/main.go"}`,
				},
			},
		},
	}

	captured := captureResponsesRequest(t, Config{
		Model:             GPT5Dot3Codex,
		ForceResponsesAPI: true,
	}, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "edit"},
			assistantMsg,
			{Role: llm.RoleTool, ToolCallID: originalCallID, Content: "applied"},
			{Role: llm.RoleTool, ToolCallID: syntheticCallID, Content: "no errors"},
		},
	})

	input, ok := captured.Body["input"].([]any)
	require.True(t, ok)
	require.Len(t, input, 5)

	// 0: user message.
	user, ok := input[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", user["role"])

	// 1: original function_call from ProviderItems.
	origCall, ok := input[1].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function_call", origCall["type"])
	assert.Equal(t, originalCallID, origCall["call_id"])
	assert.Equal(t, "apply_patch", origCall["name"])

	// 2: synthetic function_call appended by auto-diagnostics.
	syntheticCall, ok := input[2].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function_call", syntheticCall["type"])
	assert.Equal(t, syntheticCallID, syntheticCall["call_id"])
	assert.Equal(t, "check_file_errors", syntheticCall["name"])

	// 3: original function_call_output.
	origOut, ok := input[3].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function_call_output", origOut["type"])
	assert.Equal(t, originalCallID, origOut["call_id"])
	assert.Equal(t, "applied", origOut["output"])

	// 4: synthetic function_call_output. This must follow a matching
	// function_call earlier in the input array — that is the entire
	// point of this test.
	syntheticOut, ok := input[4].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "function_call_output", syntheticOut["type"])
	assert.Equal(t, syntheticCallID, syntheticOut["call_id"])
	assert.Equal(t, "no errors", syntheticOut["output"])
}

// TestMessageProviderItemsJSONRoundTrip ensures opaque provider items survive
// persistence (they are stored alongside the assistant message in the
// dialogue history).
func TestMessageProviderItemsJSONRoundTrip(t *testing.T) {
	original := llm.Message{
		Role:          llm.RoleAssistant,
		Content:       "hi",
		ProviderItems: []json.RawMessage{json.RawMessage(`{"type":"reasoning","encrypted_content":"X"}`)},
	}
	b, err := json.Marshal(original)
	require.NoError(t, err)

	var got llm.Message
	require.NoError(t, json.Unmarshal(b, &got))
	require.Len(t, got.ProviderItems, 1)
	assert.JSONEq(t,
		`{"type":"reasoning","encrypted_content":"X"}`,
		string(got.ProviderItems[0]))
}

// responsesSSEServer creates a test HTTP server that returns the given SSE events
// in the Responses API format. Use for testing the responses stream iterator.
func responsesSSEServer(t *testing.T, events []string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for _, ev := range events {
			_, _ = fmt.Fprintf(w, "data: %s\n\n", ev)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// captureResponsesRequestBody creates a test HTTP server that captures the JSON
// request body for a Responses API call. The server returns a minimal SSE
// response so the client completes without error.
func captureResponsesRequestBody(t *testing.T, cfg Config, req llm.Request) map[string]any {
	return captureResponsesRequest(t, cfg, req).Body
}

type capturedResponsesRequest struct {
	Method string
	Path   string
	Header http.Header
	Body   map[string]any
}

func captureResponsesRequest(t *testing.T, cfg Config, req llm.Request) capturedResponsesRequest {
	t.Helper()
	var captured capturedResponsesRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.Method = r.Method
		captured.Path = r.URL.Path
		captured.Header = r.Header.Clone()
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &captured.Body))
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, "data: %s\n\n",
			`{"type":"response.completed","response":{"id":"resp_test","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1}}}`)
	}))
	t.Cleanup(srv.Close)

	cfg.BaseURL = srv.URL
	c := NewClient("test-key", cfg, AvailableModels())
	ctx := context.Background()
	it, err := c.CreateCompletion(ctx, req)
	require.NoError(t, err)
	for {
		_, ok := it.Next(ctx)
		if !ok {
			break
		}
	}
	_ = it.Close()
	require.NotNil(t, captured.Body, "server should have received a request")
	return captured
}

// captureRequestBody creates a test HTTP server that captures the JSON request
// body, then creates a client pointing at that server and calls CreateCompletion.
// Returns the parsed request body so tests can assert config values are wired through.
func captureRequestBody(t *testing.T, cfg Config, req llm.Request) map[string]any {
	t.Helper()
	var captured map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &captured))
		// Return a minimal SSE response so the client doesn't hang.
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"\"}, \"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(srv.Close)

	cfg.BaseURL = srv.URL
	c := NewClient("test-key", cfg, AvailableModels())
	ctx := context.Background()
	it, err := c.CreateCompletion(ctx, req)
	require.NoError(t, err)
	// Drain the stream.
	for {
		_, ok := it.Next(ctx)
		if !ok {
			break
		}
	}
	_ = it.Close()
	require.NotNil(t, captured, "server should have received a request")
	return captured
}
