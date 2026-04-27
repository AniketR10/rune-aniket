// Unstable Build LLC ("COMPANY") CONFIDENTIAL
//
// Unpublished Copyright (c) 2024-2026 Unstable Build, All Rights Reserved.

package hooks

import (
	"encoding/json"

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
)

// Payload is the typed event payload sent on stdin to command hooks
// and as the JSON body to HTTP hooks. The runtime fills the common
// envelope fields (SessionID, TranscriptPath, Cwd, HookEventName)
// before dispatch; callers populate event-specific fields.
//
// Only fields applicable to a given event are serialized — others are
// omitted via `omitempty`. The on-the-wire field names mirror Claude
// Code's hook protocol.
type Payload struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	// Cwd is the workspace URI the hook is firing for. It is
	// serialized to JSON via Payload.MarshalJSON as a plain string
	// (the URI's String() form) to keep the wire format compatible
	// with Claude Code's `cwd` field.
	Cwd           workspaceapi.URI `json:"-"`
	HookEventName Event            `json:"hook_event_name"`

	// SessionStart
	Source string `json:"source,omitempty"`
	Model  string `json:"model,omitempty"`

	// SessionEnd
	Reason string `json:"reason,omitempty"`

	// UserPromptSubmit
	Prompt string `json:"prompt,omitempty"`

	// PostToolUse
	ToolName     string          `json:"tool_name,omitempty"`
	ToolInput    json.RawMessage `json:"tool_input,omitempty"`
	ToolResponse json.RawMessage `json:"tool_response,omitempty"`
	ToolUseID    string          `json:"tool_use_id,omitempty"`

	// Stop / SubagentStop
	StopHookActive bool `json:"stop_hook_active,omitempty"`

	// PreCompact
	Trigger string `json:"trigger,omitempty"`

	// Notification
	Level   string `json:"level,omitempty"`
	Message string `json:"message,omitempty"`
}

// MarshalJSON renders the payload with Cwd serialized as the URI's
// String form. workspaceapi.URI has unexported fields and would
// otherwise marshal to "{}", which breaks the hook wire protocol.
func (p Payload) MarshalJSON() ([]byte, error) {
	type alias Payload
	return json.Marshal(&struct {
		Cwd string `json:"cwd"`
		*alias
	}{
		Cwd:   p.Cwd.String(),
		alias: (*alias)(&p),
	})
}

// matcherDiscriminator returns the value used as the matcher key for
// the payload's event. Events that don't filter by matcher return the
// empty string (the matcher then matches everything by default).
func (p Payload) matcherDiscriminator() string {
	switch p.HookEventName {
	case EventSessionStart:
		return p.Source
	case EventSessionEnd:
		return p.Reason
	case EventPreCompact:
		return p.Trigger
	case EventPostToolUse:
		return p.ToolName
	case EventNotification:
		return p.Level
	}
	return ""
}

// HookOutput is the JSON shape a hook can write to stdout / return
// in an HTTP response body. All fields are optional.
type HookOutput struct {
	Continue           *bool               `json:"continue,omitempty"`
	StopReason         string              `json:"stopReason,omitempty"`
	SuppressOutput     bool                `json:"suppressOutput,omitempty"`
	SystemMessage      string              `json:"systemMessage,omitempty"`
	Decision           string              `json:"decision,omitempty"`
	Reason             string              `json:"reason,omitempty"`
	HookSpecificOutput *HookSpecificOutput `json:"hookSpecificOutput,omitempty"`
}

// HookSpecificOutput carries event-specific output fields. Today only
// AdditionalContext is read by the runner.
type HookSpecificOutput struct {
	HookEventName     string `json:"hookEventName,omitempty"`
	AdditionalContext string `json:"additionalContext,omitempty"`
}
