package backend

import (
	"context"
	"fmt"
	"time"

	"github.com/ernestrc/blue/iterator"
)

// ErrContextWindowExceeded is returned when the number of tokens in a request
// exceeds the context window of a model. Users are encouraged to retry
// with a reduced number of messages.
type ErrContextWindowExceeded struct {
	Count, Max int
}

func (e *ErrContextWindowExceeded) Error() string {
	return fmt.Sprintf("model context window exceeded (%d, max is %d)", e.Count, e.Max)
}

func (e *ErrContextWindowExceeded) Unwrap() error {
	return nil
}

func (e *ErrContextWindowExceeded) Is(target error) bool {
	_, ok := target.(*ErrContextWindowExceeded)
	if !ok {
		return false
	}
	return true
}

// Service encapsulates communications with an AI-capabilities provider.
type Service interface {
	// CreateChatCompletion attempts to complete the given chat completion request
	// using the pre-configured model. This method should return ErrContextWindowExceeded
	// if a request exceeds the context window. Clients can reduce the number of
	// messages until ExceedsContextWindow returns false.
	CreateChatCompletion(
		ctx context.Context,
		request ChatCompletionRequest,
	) (iterator.Iterator[ChatCompletionResponse], error)

	// ExceedsContextWindow returns true if the given message slice exceeds
	// the pre-configured model's context window.
	ExceedsContextWindow([]ChatCompletionMessage) (bool, error)
}

// ChatCompletionRequest represents a request structure for chat completion API.
type ChatCompletionRequest struct {
	// A list of messages comprising the conversation so far.
	Messages []ChatCompletionMessage
}

// ChatCompletionMessage is a message in a chat with an assistant backend.
type ChatCompletionMessage struct {
	// The role of the author of this message.
	Role Role
	// The contents of the message.
	Content string
	// Metadata contains service-specific data.
	// Check the documentation of a service implementation
	// to know what type this Metadata will be.
	//
	// Only the last Metadata field in a ChatCompletionMessage stream
	// should be used. Implementations must ensure that the last
	// ChatCompletionMessage's metadata is complete.
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

// Role is the role of the message author in a message stream.
type Role string

const (
	RoleAssistant Role = "assistant"
	RoleUser           = "user"
	RoleSystem         = "system"
	RoleTool           = "tool"
)

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
