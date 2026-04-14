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
	"context"

	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// Spawner abstracts the ability to run sub-agents.
type Spawner interface {
	Run(ctx context.Context, req RunRequest) (RunHandle, error)
}

// RunRequest is a request to run a sub-agent.
type RunRequest struct {
	Message        string
	Label          string
	AgentID        string
	Model          string
	TimeoutSeconds int      // 0 = no timeout
	AllowedTools   []string // if non-empty, sub-agent receives only these tools
	SystemPrompt   string   // if non-empty, overrides the agent definition's prompt
	Cleanup        string   // "delete" or "keep"
}

// RunHandle is the result of a Run call.
type RunHandle struct {
	SessionKey string
	DialogueID string
	Label      string
	Events     iterator.Iterator[Event]
}

// AgentSummary describes an agent for listing purposes.
//
//nolint:revive // Preserved imported API name for compatibility and clarity.
type AgentSummary struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ChildEvent wraps an agent Event with the parent tool call ID
// that spawned the sub-agent, so the TUI can nest it.
type ChildEvent struct {
	ParentToolCallID string
	Event            Event
}
