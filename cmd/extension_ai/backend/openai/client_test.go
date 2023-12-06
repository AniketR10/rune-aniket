package openai

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sashabaranov/go-openai/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"unstable.build/go-tui/cmd/extension_ai/backend"
)

// NOTE: this serves as an example of how to use the backend API backed by openai.
// Only enable when changes to the API or client are made
// or when upgrading the sdk dependency.
// Note that some of the content asserts might fail, as without
// `seed` and `system_fingerprint` funcionality, the output is never deterministic.
func TestCreateChatCompletion(t *testing.T) {
	t.SkipNow()

	token := os.Getenv("OPENAI_TESTING_KEY")

	t.Run("NewClient with empty model panics", func(t *testing.T) {
		assert.Panics(t, func() {
			NewClient(token, Config{})
		})
	})

	t.Run("sends a chat completion request, with no context", func(t *testing.T) {
		client := NewClient(token, Config{
			Model:       GPT3Dot5Turbo,
			Temperature: 0.1,
		})
		ctx := context.Background()
		req := backend.ChatCompletionRequest{Messages: []backend.ChatCompletionMessage{
			{Role: backend.RoleUser, Content: "hello sir!"},
		}}
		it, err := client.CreateChatCompletion(ctx, req)
		require.NoError(t, err)

		var finishReason backend.FinishReason
		var builder strings.Builder
		for i := 0; ; i++ {
			resp, ok := it.Next()
			if !ok {
				break
			}
			assert.NotZero(t, resp.ID)
			assert.WithinDuration(t, time.Now(), resp.Created, 1*time.Minute)
			assert.Equal(t, backend.RoleAssistant, resp.Message.Role)
			assert.Len(t, resp.Message.Metadata.(Metadata).ToolCalls, 0)
			builder.WriteString(resp.Message.Content)
			finishReason = resp.FinishReason
		}

		require.NoError(t, it.Err())
		assert.Contains(t, builder.String(), "How can I")
		assert.Equal(t, backend.FinishReasonStop, finishReason)
	})

	t.Run("sets MaxTokens from configuration", func(t *testing.T) {
		client := NewClient(token, Config{
			MaxTokens: 10000, // force error, so we know that it is set
			Model:     GPT3Dot5Turbo,
		})
		ctx := context.Background()
		req := backend.ChatCompletionRequest{Messages: []backend.ChatCompletionMessage{
			{Role: backend.RoleUser, Content: "hello sir!"},
		}}
		_, err := client.CreateChatCompletion(ctx, req)
		require.Error(t, err)
	})

	t.Run("sets PresencePenalty from configuration", func(t *testing.T) {
		client := NewClient(token, Config{
			PresencePenalty: -100, // force error, so we know that it is set
			Model:           GPT3Dot5Turbo,
		})
		ctx := context.Background()
		req := backend.ChatCompletionRequest{Messages: []backend.ChatCompletionMessage{
			{Role: backend.RoleUser, Content: "hello sir!"},
		}}
		_, err := client.CreateChatCompletion(ctx, req)
		require.Error(t, err)
	})

	t.Run("sets Temperature from configuration", func(t *testing.T) {
		client := NewClient(token, Config{
			Temperature: -100, // force error, so we know that it is set
			Model:       GPT3Dot5Turbo,
		})
		ctx := context.Background()
		req := backend.ChatCompletionRequest{Messages: []backend.ChatCompletionMessage{
			{Role: backend.RoleUser, Content: "hello sir!"},
		}}
		_, err := client.CreateChatCompletion(ctx, req)
		require.Error(t, err)
	})

	t.Run("sends a chat completion request, with some context, with default configuration", func(t *testing.T) {
		client := NewClient(token, Config{
			Model:       GPT3Dot5Turbo,
			Temperature: 0.1,
		})
		ctx := context.Background()
		req := backend.ChatCompletionRequest{Messages: []backend.ChatCompletionMessage{
			{Role: "system", Content: "We are roleplaying and you are an evil AI agent."},
			{Role: backend.RoleUser, Content: "what is your purpouse?"},
		}}
		it, err := client.CreateChatCompletion(ctx, req)
		require.NoError(t, err)

		var finishReason backend.FinishReason
		var builder strings.Builder
		for i := 0; ; i++ {
			resp, ok := it.Next()
			if !ok {
				break
			}
			assert.NotZero(t, resp.ID)
			assert.WithinDuration(t, time.Now(), resp.Created, 1*time.Minute)
			assert.Equal(t, backend.RoleAssistant, resp.Message.Role)
			assert.Len(t, resp.Message.Metadata.(Metadata).ToolCalls, 0)
			builder.WriteString(resp.Message.Content)
			finishReason = resp.FinishReason
		}

		require.NoError(t, it.Err())
		assert.Contains(t, builder.String(), "chaos")
		assert.Equal(t, backend.FinishReasonStop, finishReason)
	})

	t.Run("uses tools provided", func(t *testing.T) {
		client := NewClient(token, Config{
			Tools: []Tool{
				{Type: ToolTypeFunction, Function: FunctionDefinition{
					Name:        "getCurrentWeather",
					Description: "Get the weather in location",
					Parameters: jsonschema.Definition{
						Type: jsonschema.Object,
						Properties: map[string]jsonschema.Definition{
							"location": {
								Type:        jsonschema.String,
								Description: "The city and state, e.g. San Francisco, CA",
							},
							"unit": {
								Type: jsonschema.String,
								Enum: []string{"celcius", "fahrenheit"},
							},
						},
						Required: []string{"location"},
					},
				}},
			},
			Model: GPT3Dot5Turbo,
		})
		ctx := context.Background()
		req := backend.ChatCompletionRequest{Messages: []backend.ChatCompletionMessage{
			{Role: backend.RoleUser, Content: "what's the weather like in San Francisco right now?"},
		}}
		it, err := client.CreateChatCompletion(ctx, req)
		require.NoError(t, err)

		var builder strings.Builder
		for i := 0; ; i++ {
			resp, ok := it.Next()
			if !ok {
				break
			}
			assert.NotZero(t, resp.ID)
			assert.WithinDuration(t, time.Now(), resp.Created, 1*time.Minute)
			assert.Equal(t, backend.RoleAssistant, resp.Message.Role)

			// first message
			if i == 0 {
				require.Len(t, resp.Message.Metadata.(Metadata).ToolCalls, 1, "%+v", resp)
				calls := resp.Message.Metadata.(Metadata).ToolCalls
				assert.NotZero(t, calls[0].ID)
				assert.Equal(t, ToolTypeFunction, calls[0].Type)
				assert.Equal(t, "getCurrentWeather", calls[0].Function.Name)
				continue
			}

			// last message
			if resp.Message.Metadata == nil || len(resp.Message.Metadata.(Metadata).ToolCalls) == 0 {
				assert.Equal(t, backend.FinishReasonToolCall, resp.FinishReason)
				continue
			}

			// rest of messages
			require.Len(t, resp.Message.Metadata.(Metadata).ToolCalls, 1, i)
			builder.WriteString(resp.Message.Metadata.(Metadata).ToolCalls[0].Function.Arguments)
		}

		require.NoError(t, it.Err())
		assert.Equal(t, `{
  "location": "San Francisco, CA"
}`, builder.String())
	})
}

func TestContextWindows(t *testing.T) {
	t.Run("CreateCompletionRequest errors with ErrContextWindowExceeded", func(t *testing.T) {
		client := NewClient("", Config{
			Model: GPT4,
		}).(client)
		ctx := context.Background()
		msgs := makeMessageTokens(client, 8193)

		// sut
		req := backend.ChatCompletionRequest{Messages: msgs}
		_, err := client.CreateChatCompletion(ctx, req)
		require.Equal(t, backend.ErrContextWindowExceeded, err)
	})

	t.Run("ExceedsContextWindow", func(t *testing.T) {
		client := NewClient("", Config{
			Model: GPT4,
		}).(client)
		msgs := makeMessageTokens(client, 8193)

		// sut
		ok, err := client.ExceedsContextWindow(msgs)
		require.NoError(t, err)
		require.True(t, ok)

		ok, err = client.ExceedsContextWindow(msgs[1:])
		require.NoError(t, err)
		require.False(t, ok)
	})
}

func TestCountTokens(t *testing.T) {
	for model := range modelContextWindow {
		t.Run(model, func(t *testing.T) {
			client := NewClient("", Config{
				Model: model,
			}).(client)
			text := "¡Hola mundo!"
			assert.Equal(t, 10, client.countTokens([]backend.ChatCompletionMessage{{Content: text}}))
		})
	}
}

func makeMessageTokens(c client, greaterThan int) []backend.ChatCompletionMessage {
	var msgs []backend.ChatCompletionMessage
	for i := 0; c.countTokens(msgs) < greaterThan; i++ {
		msgs = append(msgs, backend.ChatCompletionMessage{
			Content: strconv.Itoa(i), Role: backend.RoleUser,
		})
	}
	return msgs
}
