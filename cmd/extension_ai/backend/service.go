package backend

import (
	"context"
	"time"

	"github.com/ernestrc/blue/iterator"
)

// Service encapsulates communications with an AI-capabilities provider.
type Service interface {
	CreateChatCompletion(
		ctx context.Context,
		request ChatCompletionRequest,
	) (iterator.Iterator[ChatCompletionResponse], error)
}

// ChatCompletionRequest represents a request structure for chat completion API.
type ChatCompletionRequest struct {
	// A list of messages comprising the conversation so far.
	Messages []ChatCompletionMessage
}

// ChatCompletionMessage is
type ChatCompletionMessage struct {
	// The role of the author of this message.
	Role string
	// The contents of the message.
	Content string
	// Metadata contains service-specific data.
	// Check the documentation of a service implementation
	// to know what type this Metadata will be.
	Metadata any

	// An optional name for the participant.
	// Provides the model information to differentiate between participants of the same role.
	Name string
}

// ChatCompletionResponse represents a response structure for chat completion API.
type ChatCompletionResponse struct {
	// A unique identifier for the chat completion.
	ID string
	// Time when the chat completion was created.
	Created time.Time
	Message ChatCompletionMessage
	// The reason the model stopped generating tokens. This will be stop if the model
	// hit a natural stop point or a provided stop sequence, length if the maximum number
	// of tokens specified in the request was reached, content_filter if content was
	// omitted due to a flag from our content filters, tool_calls if the model
	// called a tool, or function_call (deprecated) if the model called a function.
	FinishReason FinishReason
}

// FinishReason is the reason why the message choice was returned.
type FinishReason string

const (
	// FinishReasonStop API returned complete message,
	// or a message terminated by one of the stop sequences provided via the stop parameter
	FinishReasonStop FinishReason = "stop"
	// FinishReasonLength Incomplete model output due to max_tokens parameter or token limit
	FinishReasonLength FinishReason = "length"
	// FinishReasonToolCall The model decided to use one of the tools provided.
	FinishReasonToolCall FinishReason = "tool_calls"
	// FinishReasonContentFilter Omitted content due to a flag from our content filters
	FinishReasonContentFilter FinishReason = "content_filter"
	// FinishReasonNull API response still in progress or incomplete
	FinishReasonNull FinishReason = "null"
)
