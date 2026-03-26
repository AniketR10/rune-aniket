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

package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/unstablebuild/rune-go-sdk/api/storageapi/storagestub"
	"github.com/unstablebuild/rune-go-sdk/iterator"
)

// stubAuditService is a minimal Service for testing the audit wrapper.
type stubAuditService struct {
	events      []Event
	createErr   error
	tokenCount  int
	contextWin  int
	lastRequest *Request
}

func (s *stubAuditService) CreateCompletion(
	_ context.Context, req Request,
) (iterator.Iterator[Event], error) {
	s.lastRequest = &req
	if s.createErr != nil {
		return nil, s.createErr
	}
	return iterator.FromSlice(s.events), nil
}

func (s *stubAuditService) CountTokens(_ []Message) (int, error) {
	return s.tokenCount, nil
}

func (s *stubAuditService) ContextWindow() int {
	return s.contextWin
}

func TestAuditService(t *testing.T) {
	tests := []struct {
		name        string
		inner       *stubAuditService
		request     Request
		dialogueID  string
		wantEntries int
		assertEntry func(t *testing.T, e AuditEntry)
		wantErr     bool
	}{
		{
			name: "successful completion records entry with usage",
			inner: &stubAuditService{
				tokenCount: 100,
				contextWin: 4096,
				events: []Event{
					{Type: EventTextDelta, Text: "hello"},
					{Type: EventStreamDone, DoneData: &DoneData{
						Message:      Message{Role: RoleAssistant, Content: "hello"},
						FinishReason: FinishReasonStop,
						Usage:        Usage{TokensSent: 50, TokensReceived: 10, TokensCacheCreated: 5},
					}},
				},
			},
			request: Request{
				Messages: []Message{{Role: RoleUser, Content: "hi"}},
				Tools:    []Tool{{Type: ToolTypeFunction, Function: FunctionDefinition{Name: "do_thing"}}},
			},
			dialogueID:  "conv-1",
			wantEntries: 1,
			assertEntry: func(t *testing.T, e AuditEntry) {
				assert.Equal(t, 100, e.EstimatedTokens)
				assert.Equal(t, 4096, e.ContextWindow)
				assert.Equal(t, 1, e.Tools)
				assert.Equal(t, 50, e.Usage.TokensSent)
				assert.Equal(t, 10, e.Usage.TokensReceived)
				assert.Equal(t, 5, e.Usage.TokensCacheCreated)
				assert.Equal(t, FinishReasonStop, e.FinishReason)
				assert.Empty(t, e.Err)
				require.NotNil(t, e.Response)
				assert.Equal(t, "hello", e.Response.Content)
				require.Len(t, e.Messages, 1)
				assert.Equal(t, "hi", e.Messages[0].Content)
				assert.True(t, e.Duration > 0)
				assert.Equal(t, "test-model", e.Model)
				assert.Equal(t, "test-provider", e.Provider)
				assert.Equal(t, []string{"do_thing"}, e.ToolNames)
			},
		},
		{
			name: "CreateCompletion error records entry",
			inner: &stubAuditService{
				createErr:  errors.New("api down"),
				tokenCount: 42,
				contextWin: 8192,
			},
			request: Request{
				Messages: []Message{{Role: RoleUser, Content: "test"}},
			},
			dialogueID:  "conv-err",
			wantEntries: 1,
			wantErr:     true,
			assertEntry: func(t *testing.T, e AuditEntry) {
				assert.Equal(t, "api down", e.Err)
				assert.Equal(t, 42, e.EstimatedTokens)
				assert.Nil(t, e.Response)
			},
		},
		{
			name: "stream error event records entry",
			inner: &stubAuditService{
				events: []Event{
					{Type: EventTextDelta, Text: "partial"},
					{Type: EventStreamError, Error: errors.New("stream broken")},
				},
			},
			request: Request{
				Messages: []Message{{Role: RoleUser, Content: "test"}},
			},
			dialogueID:  "conv-stream-err",
			wantEntries: 1,
			assertEntry: func(t *testing.T, e AuditEntry) {
				assert.Equal(t, "stream broken", e.Err)
				assert.Nil(t, e.Response)
			},
		},
		{
			name: "all event types are passed through",
			inner: &stubAuditService{
				events: []Event{
					{Type: EventReasoningDelta, Reasoning: "thinking"},
					{Type: EventTextDelta, Text: "answer"},
					{Type: EventToolCallDone, ToolCall: &ToolCall{ID: "t1"}},
					{Type: EventStreamDone, DoneData: &DoneData{
						Message:      Message{Role: RoleAssistant, Content: "answer"},
						FinishReason: FinishReasonToolCall,
						Usage:        Usage{TokensSent: 200, TokensReceived: 50, TokensReasoned: 30},
					}},
				},
			},
			request:     Request{Messages: []Message{{Role: RoleUser, Content: "q"}}},
			dialogueID:  "conv-passthrough",
			wantEntries: 1,
			assertEntry: func(t *testing.T, e AuditEntry) {
				assert.Equal(t, FinishReasonToolCall, e.FinishReason)
				assert.Equal(t, 30, e.Usage.TokensReasoned)
			},
		},
		{
			name: "no dialogue ID skips recording",
			inner: &stubAuditService{
				events: []Event{
					{Type: EventStreamDone, DoneData: &DoneData{
						Message:      Message{Role: RoleAssistant, Content: "ok"},
						FinishReason: FinishReasonStop,
					}},
				},
			},
			request:     Request{Messages: []Message{{Role: RoleUser, Content: "hi"}}},
			dialogueID:  "", // no dialogue ID
			wantEntries: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewAuditStore(storagestub.NewInMemoryService())
			svc := NewAuditService(tt.inner, store, "test-model", "test-provider")

			ctx := context.Background()
			if tt.dialogueID != "" {
				ctx = WithAuditDialogueID(ctx, tt.dialogueID)
			}

			it, err := svc.CreateCompletion(ctx, tt.request)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				var got []Event
				for {
					ev, ok := it.Next(ctx)
					if !ok {
						break
					}
					got = append(got, ev)
				}
				it.Close() //nolint:errcheck
				assert.Len(t, got, len(tt.inner.events))
			}

			entries, err := store.Get(ctx, tt.dialogueID)
			require.NoError(t, err)
			require.Len(t, entries, tt.wantEntries)
			if tt.assertEntry != nil && tt.wantEntries > 0 {
				tt.assertEntry(t, entries[0])
			}
		})
	}
}

func TestAuditServicePassthrough(t *testing.T) {
	inner := &stubAuditService{tokenCount: 77, contextWin: 32000}
	store := NewAuditStore(storagestub.NewInMemoryService())
	svc := NewAuditService(inner, store, "test-model", "test-provider")

	n, err := svc.CountTokens([]Message{{Content: "x"}})
	require.NoError(t, err)
	assert.Equal(t, 77, n)

	assert.Equal(t, 32000, svc.ContextWindow())
}

func TestAuditStorePerDialogue(t *testing.T) {
	store := NewAuditStore(storagestub.NewInMemoryService())
	ctx := context.Background()

	store.Append(ctx, "d1", AuditEntry{EstimatedTokens: 10})
	store.Append(ctx, "d1", AuditEntry{EstimatedTokens: 20})
	store.Append(ctx, "d2", AuditEntry{EstimatedTokens: 30})

	d1, err := store.Get(ctx, "d1")
	require.NoError(t, err)
	require.Len(t, d1, 2)
	assert.Equal(t, 10, d1[0].EstimatedTokens)
	assert.Equal(t, 20, d1[1].EstimatedTokens)

	d2, err := store.Get(ctx, "d2")
	require.NoError(t, err)
	require.Len(t, d2, 1)
	assert.Equal(t, 30, d2[0].EstimatedTokens)

	// Non-existent dialogue returns nil.
	d3, err := store.Get(ctx, "d3")
	require.NoError(t, err)
	assert.Nil(t, d3)
}

func TestAuditStoreNilSafe(t *testing.T) {
	var store *AuditStore
	ctx := context.Background()

	// Should not panic.
	store.Append(ctx, "d1", AuditEntry{})

	entries, err := store.Get(ctx, "d1")
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestAuditRequestMessagesAreCopied(t *testing.T) {
	inner := &stubAuditService{
		events: []Event{
			{Type: EventStreamDone, DoneData: &DoneData{
				Message:      Message{Role: RoleAssistant, Content: "ok"},
				FinishReason: FinishReasonStop,
			}},
		},
	}
	store := NewAuditStore(storagestub.NewInMemoryService())
	svc := NewAuditService(inner, store, "test-model", "test-provider")

	ctx := WithAuditDialogueID(context.Background(), "copy-test")
	msgs := []Message{{Role: RoleUser, Content: "original"}}
	it, err := svc.CreateCompletion(ctx, Request{Messages: msgs})
	require.NoError(t, err)
	for {
		if _, ok := it.Next(ctx); !ok {
			break
		}
	}
	it.Close() //nolint:errcheck

	// Mutate the original slice after the call.
	msgs[0].Content = "mutated"

	entries, err := store.Get(ctx, "copy-test")
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "original", entries[0].Messages[0].Content)
}
