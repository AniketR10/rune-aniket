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
	"sync"
)

type contextKey int

const (
	parentToolCallIDKey contextKey = iota
	activatedSkillsKey
	currentModelKey
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

// activatedSkills tracks which skills have been loaded in a single Run.
type activatedSkills struct {
	mu sync.Mutex
	m  map[string]bool
}

// WithActivatedSkills returns a context carrying a fresh activated-skills set.
// Call once per Agent.Run to scope deduplication to a single conversation turn.
func WithActivatedSkills(ctx context.Context) context.Context {
	return context.WithValue(ctx, activatedSkillsKey, &activatedSkills{m: make(map[string]bool)})
}

// MarkSkillActivated records that a skill has been loaded in this run.
func MarkSkillActivated(ctx context.Context, name string) {
	s, _ := ctx.Value(activatedSkillsKey).(*activatedSkills)
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m[name] = true
	s.mu.Unlock()
}

// IsSkillActivated reports whether a skill has already been loaded in this run.
func IsSkillActivated(ctx context.Context, name string) bool {
	s, _ := ctx.Value(activatedSkillsKey).(*activatedSkills)
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[name]
}
