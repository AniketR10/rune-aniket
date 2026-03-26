// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2023-2026 Unstable Build, All Rights Reserved.
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

package dialoguetui

import "time"

// MessageEventType describes the kind of structured event sent to the TUI.
type MessageEventType int

const (
	// MessageEventText is a text chunk from the assistant.
	MessageEventText MessageEventType = iota
	// MessageEventBreak signals the end of the current message.
	MessageEventBreak
	// MessageEventToolCall signals a tool invocation has started.
	MessageEventToolCall
	// MessageEventToolResult signals a tool invocation has completed.
	MessageEventToolResult
	// MessageEventReasoning is a reasoning chunk from reasoning models.
	MessageEventReasoning
	// MessageEventError is an error message to display inline.
	MessageEventError
	// MessageEventPrompt shows a selection UI for user input.
	MessageEventPrompt
	// MessageEventPromptDismiss removes an active prompt.
	MessageEventPromptDismiss
	// MessageEventWarning is a non-fatal warning message to display inline.
	MessageEventWarning
	// MessageEventToolsDropped signals that tool results were dropped from history.
	MessageEventToolsDropped
	// MessageEventMemoryRecall signals a memory was recalled for this conversation.
	MessageEventMemoryRecall
	// MessageEventTaskProgress signals a task progress update.
	MessageEventTaskProgress
	// MessageEventBusy signals the start (Busy=true) or end (Busy=false) of
	// an active agent completion. The dialogue handler uses this to decide
	// whether to queue follow-up messages or send them directly.
	MessageEventBusy
)

// MessageEvent is a structured event sent through the display channel.
type MessageEvent struct {
	Type             MessageEventType
	Text             string // MessageEventText: text chunk
	ToolCallID       string // MessageEventToolCall / MessageEventToolResult: unique call identifier
	ToolName         string // MessageEventToolCall / MessageEventToolResult
	ToolArgs         string // MessageEventToolCall: JSON arguments
	ToolSummary      string // MessageEventToolCall / MessageEventToolResult: short human-readable args summary
	ToolOutput       string // MessageEventToolResult: execution output
	IsError          bool          // MessageEventToolResult: whether the tool errored
	ToolStartTime    time.Time     // MessageEventToolCall: when the tool call was announced
	ToolDuration     time.Duration // MessageEventToolResult: how long the tool took
	ParentToolCallID   string        // if set, this is a child event nested under this parent tool call
	DroppedToolCallIDs []string      // MessageEventToolsDropped: tool call IDs removed from history
	Memories           []MemoryRecallEntry // MessageEventMemoryRecall: recalled memories
	MemoryDuration     time.Duration      // MessageEventMemoryRecall: how long recall took
	TaskProgress       ProgressTaskEntry  // MessageEventTaskProgress: single task update

	// Prompt fields (MessageEventPrompt)
	PromptTitle       string
	PromptHeader      string
	PromptBody        string // optional markdown content shown before options
	PromptOptions     []PromptEventOption
	PromptMultiSelect bool
	PromptResult      chan<- []string // side-channel for response

	// Busy field (MessageEventBusy): true when the agent starts processing,
	// false when it finishes.
	Busy bool
}

// MemoryRecallEntry represents a single memory in a recall event.
type MemoryRecallEntry struct {
	ID      string
	Content string
}

// PromptEventOption describes an option in a prompt event.
type PromptEventOption struct {
	Label       string
	Description string
	// RequiresInput indicates that selecting this option should
	// open a text input box for the user to type a response.
	RequiresInput bool
}
