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

	"github.com/unstablebuild/rune-go-sdk/api/workspaceapi"
	"unstable.build/go-tui/cmd/rune-agent/hooks"
)

type contextKey int

const (
	parentToolCallIDKey contextKey = iota
	currentModelKey
	hooksRunnerKey
	workspaceURIKey
	dialogueIDKey
	prompterKey
)

// WithParentToolCallID returns a context carrying the parent tool call ID.
func WithParentToolCallID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, parentToolCallIDKey, id)
}

// ParentToolCallID extracts the parent tool call ID from the context.
func ParentToolCallID(ctx context.Context) string {
	v, _ := ctx.Value(parentToolCallIDKey).(string)
	return v
}

// WithCurrentModel returns a context carrying the current model name.
func WithCurrentModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, currentModelKey, model)
}

// CurrentModel extracts the current model name from the context.
func CurrentModel(ctx context.Context) string {
	v, _ := ctx.Value(currentModelKey).(string)
	return v
}

// WithHooks returns a context carrying the agent's hook runner.
func WithHooks(ctx context.Context, r *hooks.Runner) context.Context {
	return context.WithValue(ctx, hooksRunnerKey, r)
}

// HooksFromContext extracts the hook runner from the context.
func HooksFromContext(ctx context.Context) *hooks.Runner {
	v, _ := ctx.Value(hooksRunnerKey).(*hooks.Runner)
	return v
}

// WithWorkspaceURI returns a context carrying the agent's workspace
// URI. The URI is intentionally not collapsed to a string so callers
// see that it may be non-local (e.g. ssh://) and is unsafe to feed
// straight into local filesystem APIs.
func WithWorkspaceURI(ctx context.Context, uri workspaceapi.URI) context.Context {
	return context.WithValue(ctx, workspaceURIKey, uri)
}

// WorkspaceURIFromContext extracts the workspace URI from the
// context. Returns the zero URI when none was attached.
func WorkspaceURIFromContext(ctx context.Context) workspaceapi.URI {
	v, _ := ctx.Value(workspaceURIKey).(workspaceapi.URI)
	return v
}

// WithDialogueID returns a context carrying the parent dialogue ID.
func WithDialogueID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, dialogueIDKey, id)
}

// DialogueIDFromContext extracts the dialogue ID from the context.
func DialogueIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(dialogueIDKey).(string)
	return v
}

// WithPrompter returns a context carrying the agent's prompter so that
// tools (e.g. bash) can block and ask the user a question while the
// agent loop is running. A nil prompter is allowed; tools should treat
// it as "no prompter available".
func WithPrompter(ctx context.Context, p Prompter) context.Context {
	return context.WithValue(ctx, prompterKey, p)
}

// PrompterFromContext extracts the prompter from the context. Returns
// nil when none was attached.
func PrompterFromContext(ctx context.Context) Prompter {
	v, _ := ctx.Value(prompterKey).(Prompter)
	return v
}
